package dnsresolver_test

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/dnsresolver"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

func TestIsIPOrCIDR(t *testing.T) {
	if !dnsresolver.IsIPOrCIDR("1.1.1.1") {
		t.Errorf("expected 1.1.1.1 to be IP")
	}
	if !dnsresolver.IsIPOrCIDR("10.0.0.0/16") {
		t.Errorf("expected 10.0.0.0/16 to be CIDR")
	}
	if !dnsresolver.IsIPOrCIDR("2001:db8::1") {
		t.Errorf("expected 2001:db8::1 to be IP")
	}
	if dnsresolver.IsIPOrCIDR("example.com") {
		t.Errorf("expected example.com NOT to be IP")
	}
}

func TestResolveEntries_PreservesIPs(t *testing.T) {
	res := dnsresolver.New()
	entries := []parser.Entry{
		{Address: "1.1.1.1", Comment: "Cloudflare"},
		{Address: "10.0.0.0/24", Comment: "Subnet"},
	}

	result, err := res.ResolveEntries(context.Background(), entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Address != "1.1.1.1" || result.Entries[1].Address != "10.0.0.0/24" {
		t.Errorf("entries changed unexpectedly: %+v", result.Entries)
	}
}

func TestResolveEntries_InvalidDomain(t *testing.T) {
	res := dnsresolver.New()
	entries := []parser.Entry{
		{Address: "invalid.domain.that.does.not.exist.example.test", Comment: "Test"},
	}

	result, err := res.ResolveEntries(context.Background(), entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Errorf("expected warnings for invalid domain")
	}
	// Original entry preserved
	if len(result.Entries) != 1 || !strings.Contains(result.Entries[0].Address, "invalid.domain") {
		t.Errorf("expected original entry preserved, got: %+v", result.Entries)
	}
}

func TestResolveEntries_ViaProxy(t *testing.T) {
	// 1. Start mock TCP DNS server
	dnsListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dnsListener.Close()

	go func() {
		for {
			conn, err := dnsListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var lenBuf [2]byte
				if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
					return
				}
				qLen := binary.BigEndian.Uint16(lenBuf[:])
				qBuf := make([]byte, qLen)
				if _, err := io.ReadFull(c, qBuf); err != nil {
					return
				}

				var req dnsmessage.Message
				if err := req.Unpack(qBuf); err != nil {
					return
				}

				resp := dnsmessage.Message{
					Header: dnsmessage.Header{
						ID:       req.Header.ID,
						Response: true,
					},
					Questions: req.Questions,
					Answers: []dnsmessage.Resource{
						{
							Header: dnsmessage.ResourceHeader{
								Name:  req.Questions[0].Name,
								Type:  dnsmessage.TypeA,
								Class: dnsmessage.ClassINET,
								TTL:   60,
							},
							Body: &dnsmessage.AResource{
								A: [4]byte{93, 184, 215, 14},
							},
						},
					},
				}

				packed, err := resp.Pack()
				if err != nil {
					return
				}
				var respLen [2]byte
				binary.BigEndian.PutUint16(respLen[:], uint16(len(packed)))
				_, _ = c.Write(respLen[:])
				_, _ = c.Write(packed)
			}(conn)
		}
	}()

	dnsAddr := dnsListener.Addr().String()

	// 2. Start mock HTTP CONNECT proxy
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxyListener.Close()

	go func() {
		for {
			clientConn, err := proxyListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil || req.Method != http.MethodConnect {
					return
				}

				targetConn, err := net.Dial("tcp", req.Host)
				if err != nil {
					_, _ = io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
					return
				}
				defer targetConn.Close()

				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")

				go func() { _, _ = io.Copy(targetConn, br) }()
				_, _ = io.Copy(c, targetConn)
			}(clientConn)
		}
	}()

	proxyURL := fmt.Sprintf("http://%s", proxyListener.Addr().String())

	// 3. Resolve domain through the proxy
	res := dnsresolver.New(
		dnsresolver.WithServer(dnsAddr),
		dnsresolver.WithProxy(proxyURL),
		dnsresolver.WithTimeout(3*time.Second),
	)

	entries := []parser.Entry{
		{Address: "example.com", Comment: "testsite"},
	}

	result, err := res.ResolveEntries(context.Background(), entries)
	if err != nil {
		t.Fatalf("ResolveEntries via proxy failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Address != "93.184.215.14" {
		t.Errorf("expected 93.184.215.14, got %s", result.Entries[0].Address)
	}
	if !strings.Contains(result.Entries[0].Comment, "[resolved: example.com]") {
		t.Errorf("expected comment annotation, got: %s", result.Entries[0].Comment)
	}
}
