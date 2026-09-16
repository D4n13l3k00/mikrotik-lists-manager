package dnsresolver

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/errgroup"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/netutil"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

// Resolver resolves domain entries into IP entries.
type Resolver struct {
	CustomServer string
	ProxyURL     string
	Timeout      time.Duration
	Concurrency  int
	NetResolver  *net.Resolver
	proxyDialer  proxy.ContextDialer
}

// Option configures the Resolver.
type Option func(*Resolver)

// WithServer sets a custom DNS server (e.g. "8.8.8.8:53" or "1.1.1.1").
func WithServer(server string) Option {
	return func(r *Resolver) {
		if server != "" && !strings.Contains(server, ":") {
			server = net.JoinHostPort(server, "53")
		}
		r.CustomServer = server
	}
}

// WithProxy sets a proxy URL (e.g. "socks5://127.0.0.1:1080", "socks5h://...", "http://...").
func WithProxy(proxyURL string) Option {
	return func(r *Resolver) {
		r.ProxyURL = proxyURL
	}
}

// WithTimeout sets DNS lookup timeout.
func WithTimeout(t time.Duration) Option {
	return func(r *Resolver) {
		r.Timeout = t
	}
}

// WithConcurrency sets max concurrent lookups.
func WithConcurrency(c int) Option {
	return func(r *Resolver) {
		if c > 0 {
			r.Concurrency = c
		}
	}
}

// New creates a Resolver with options.
func New(opts ...Option) *Resolver {
	r := &Resolver{
		Timeout:     5 * time.Second,
		Concurrency: 10,
	}
	for _, opt := range opts {
		opt(r)
	}

	if r.ProxyURL != "" {
		dialer, err := netutil.NewDialer(r.ProxyURL, r.Timeout)
		if err == nil {
			r.proxyDialer = dialer
		}
		if r.CustomServer == "" {
			// When proxied, default to Cloudflare public DNS over TCP
			r.CustomServer = "1.1.1.1:53"
		}
	}

	if r.NetResolver == nil {
		if r.CustomServer != "" {
			r.NetResolver = &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: r.Timeout}
					return d.DialContext(ctx, "udp", r.CustomServer)
				},
			}
		} else {
			r.NetResolver = net.DefaultResolver
		}
	}

	return r
}

// IsIPOrCIDR returns true if s is a valid IP address or CIDR notation.
func IsIPOrCIDR(s string) bool {
	s = strings.TrimSpace(s)
	if _, err := netip.ParseAddr(s); err == nil {
		return true
	}
	if _, err := netip.ParsePrefix(s); err == nil {
		return true
	}
	return false
}

// ResolveResult contains the resolved entries and any lookup warnings.
type ResolveResult struct {
	Entries  []parser.Entry
	Warnings []string
}

func (r *Resolver) queryDNSTCP(ctx context.Context, domain string, qType dnsmessage.Type) ([]netip.Addr, error) {
	if r.proxyDialer == nil {
		return nil, fmt.Errorf("proxy dialer not initialized")
	}

	server := r.CustomServer
	if !strings.Contains(server, ":") {
		server = net.JoinHostPort(server, "53")
	}

	conn, err := r.proxyDialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("подключение к DNS %s через прокси: %w", server, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	name, err := dnsmessage.NewName(strings.TrimSuffix(domain, ".") + ".")
	if err != nil {
		return nil, fmt.Errorf("некорректное доменное имя %q: %w", domain, err)
	}

	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               uint16(time.Now().UnixNano() & 0xffff),
			RecursionDesired: true,
		},
		Questions: []dnsmessage.Question{
			{
				Name:  name,
				Type:  qType,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	packed, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("упаковка DNS-запроса: %w", err)
	}

	var lengthPrefix [2]byte
	binary.BigEndian.PutUint16(lengthPrefix[:], uint16(len(packed)))
	if _, err := conn.Write(lengthPrefix[:]); err != nil {
		return nil, fmt.Errorf("отправка длины DNS-запроса: %w", err)
	}
	if _, err := conn.Write(packed); err != nil {
		return nil, fmt.Errorf("отправка DNS-запроса: %w", err)
	}

	if _, err := io.ReadFull(conn, lengthPrefix[:]); err != nil {
		return nil, fmt.Errorf("чтение длины ответа DNS: %w", err)
	}
	respLen := binary.BigEndian.Uint16(lengthPrefix[:])

	respBuf := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respBuf); err != nil {
		return nil, fmt.Errorf("чтение ответа DNS: %w", err)
	}

	var resp dnsmessage.Message
	if err := resp.Unpack(respBuf); err != nil {
		return nil, fmt.Errorf("распаковка ответа DNS: %w", err)
	}

	var addrs []netip.Addr
	for _, ans := range resp.Answers {
		if ans.Header.Type == dnsmessage.TypeA {
			if a, ok := ans.Body.(*dnsmessage.AResource); ok {
				if addr, ok := netip.AddrFromSlice(a.A[:]); ok {
					addrs = append(addrs, addr)
				}
			}
		} else if ans.Header.Type == dnsmessage.TypeAAAA {
			if aaaa, ok := ans.Body.(*dnsmessage.AAAAResource); ok {
				if addr, ok := netip.AddrFromSlice(aaaa.AAAA[:]); ok {
					addrs = append(addrs, addr)
				}
			}
		}
	}

	return addrs, nil
}

func (r *Resolver) lookupIP(ctx context.Context, domain string) ([]netip.Addr, error) {
	if r.proxyDialer != nil {
		var allAddrs []netip.Addr
		aAddrs, errA := r.queryDNSTCP(ctx, domain, dnsmessage.TypeA)
		if errA == nil {
			allAddrs = append(allAddrs, aAddrs...)
		}
		aaaaAddrs, errAAAA := r.queryDNSTCP(ctx, domain, dnsmessage.TypeAAAA)
		if errAAAA == nil {
			allAddrs = append(allAddrs, aaaaAddrs...)
		}

		if len(allAddrs) == 0 {
			if errA != nil {
				return nil, errA
			}
			if errAAAA != nil {
				return nil, errAAAA
			}
			return nil, fmt.Errorf("нет A или AAAA записей для %s", domain)
		}
		return allAddrs, nil
	}

	return r.NetResolver.LookupNetIP(ctx, "ip4", domain)
}

// ResolveEntries iterates through entries, resolving any domain names into IP addresses.
func (r *Resolver) ResolveEntries(ctx context.Context, entries []parser.Entry) (ResolveResult, error) {
	type entryJob struct {
		index int
		entry parser.Entry
	}

	var toResolve []entryJob
	resolvedMap := make(map[int][]parser.Entry)
	var warnings []string
	var mu sync.Mutex

	for i, e := range entries {
		addr := strings.TrimSpace(e.Address)
		if IsIPOrCIDR(addr) {
			resolvedMap[i] = []parser.Entry{e}
		} else {
			toResolve = append(toResolve, entryJob{index: i, entry: e})
		}
	}

	if len(toResolve) > 0 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(r.Concurrency)

		for _, job := range toResolve {
			j := job
			domain := strings.TrimSpace(j.entry.Address)
			g.Go(func() error {
				lookupCtx, cancel := context.WithTimeout(gctx, r.Timeout)
				defer cancel()

				ips, err := r.lookupIP(lookupCtx, domain)
				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					warnings = append(warnings, fmt.Sprintf("failed to resolve %s: %v", domain, err))
					resolvedMap[j.index] = []parser.Entry{j.entry}
					return nil
				}

				if len(ips) == 0 {
					warnings = append(warnings, fmt.Sprintf("no IP records found for %s", domain))
					resolvedMap[j.index] = []parser.Entry{j.entry}
					return nil
				}

				var newEntries []parser.Entry
				seenIP := make(map[string]bool)
				for _, ip := range ips {
					ipStr := ip.String()
					if seenIP[ipStr] {
						continue
					}
					seenIP[ipStr] = true

					comment := j.entry.Comment
					if comment == "" {
						comment = "resolved: " + domain
					} else {
						comment = comment + " [resolved: " + domain + "]"
					}

					newEntries = append(newEntries, parser.Entry{
						Address:  ipStr,
						Comment:  comment,
						Disabled: j.entry.Disabled,
					})
				}
				resolvedMap[j.index] = newEntries
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return ResolveResult{}, err
		}
	}

	var finalEntries []parser.Entry
	for i := 0; i < len(entries); i++ {
		finalEntries = append(finalEntries, resolvedMap[i]...)
	}

	return ResolveResult{
		Entries:  finalEntries,
		Warnings: warnings,
	}, nil
}

// ResolveEntries resolves domain names across entries using a default or custom DNS server and optional proxy.
func ResolveEntries(ctx context.Context, entries []parser.Entry, customServer string, proxyURL string) (ResolveResult, error) {
	r := New(WithServer(customServer), WithProxy(proxyURL))
	return r.ResolveEntries(ctx, entries)
}
