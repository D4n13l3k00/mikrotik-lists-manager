package netutil

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// ContextDialer is an interface for dialing network connections with context.
type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// httpConnectDialer dials TCP connections through an HTTP proxy via CONNECT.
type httpConnectDialer struct {
	proxyURL *url.URL
	timeout  time.Duration
}

func (h *httpConnectDialer) Dial(network, addr string) (net.Conn, error) {
	return h.DialContext(context.Background(), network, addr)
}

func (h *httpConnectDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("http proxy only supports tcp connections, got %s", network)
	}

	proxyAddr := h.proxyURL.Host
	if !strings.Contains(proxyAddr, ":") {
		if h.proxyURL.Scheme == "https" {
			proxyAddr += ":443"
		} else {
			proxyAddr += ":80"
		}
	}

	d := &net.Dialer{Timeout: h.timeout}
	var rawConn net.Conn
	var err error

	if h.proxyURL.Scheme == "https" {
		rawConn, err = tls.DialWithDialer(d, "tcp", proxyAddr, &tls.Config{
			ServerName: h.proxyURL.Hostname(),
		})
	} else {
		rawConn, err = d.DialContext(ctx, "tcp", proxyAddr)
	}

	if err != nil {
		return nil, fmt.Errorf("подключение к http прокси %s: %w", proxyAddr, err)
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetAddr},
		Host:   targetAddr,
		Header: make(http.Header),
	}
	req.Header.Set("User-Agent", "mikrotik-lists-manager")

	if h.proxyURL.User != nil {
		user := h.proxyURL.User.Username()
		pass, _ := h.proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	if err := req.Write(rawConn); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("отправка CONNECT к прокси: %w", err)
	}

	br := bufio.NewReader(rawConn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("чтение ответа CONNECT от прокси: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		rawConn.Close()
		return nil, fmt.Errorf("прокси отклонил CONNECT: статус %s", resp.Status)
	}

	if br.Buffered() > 0 {
		return &bufferedConn{Conn: rawConn, r: br}, nil
	}
	return rawConn, nil
}

type bufferedConn struct {
	net.Conn
	r io.Reader
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

// NewDialer creates a ContextDialer supporting direct, socks5(h), and http(s) proxies.
func NewDialer(proxyURLStr string, timeout time.Duration) (proxy.ContextDialer, error) {
	if proxyURLStr == "" {
		return &net.Dialer{Timeout: timeout}, nil
	}

	if !strings.Contains(proxyURLStr, "://") {
		proxyURLStr = "http://" + proxyURLStr
	}

	u, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("некорректный адрес прокси %q: %w", proxyURLStr, err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "socks5", "socks5h":
		forward := &net.Dialer{Timeout: timeout}
		d, err := proxy.FromURL(u, forward)
		if err != nil {
			return nil, fmt.Errorf("создание socks5 диалера: %w", err)
		}
		if cd, ok := d.(proxy.ContextDialer); ok {
			return cd, nil
		}
		return &contextDialerShim{d: d}, nil

	case "http", "https":
		return &httpConnectDialer{
			proxyURL: u,
			timeout:  timeout,
		}, nil

	default:
		return nil, fmt.Errorf("неподдерживаемый протокол прокси %q (поддерживаются socks5, socks5h, http, https)", scheme)
	}
}

type contextDialerShim struct {
	d proxy.Dialer
}

func (s *contextDialerShim) Dial(network, addr string) (net.Conn, error) {
	return s.d.Dial(network, addr)
}

func (s *contextDialerShim) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if cd, ok := s.d.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return s.d.Dial(network, addr)
}

// HTTPTransport creates an *http.Transport configured with the given proxy and TLS settings.
func HTTPTransport(proxyURLStr string, skipTLS bool) (*http.Transport, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS, //nolint:gosec
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyURLStr == "" {
		transport.Proxy = http.ProxyFromEnvironment
		return transport, nil
	}

	if !strings.Contains(proxyURLStr, "://") {
		proxyURLStr = "http://" + proxyURLStr
	}

	u, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("некорректный адрес прокси %q: %w", proxyURLStr, err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		dialer, err := NewDialer(proxyURLStr, 30*time.Second)
		if err != nil {
			return nil, err
		}
		transport.DialContext = dialer.DialContext
	default:
		return nil, fmt.Errorf("неподдерживаемый протокол прокси %q (поддерживаются socks5, socks5h, http, https)", scheme)
	}

	return transport, nil
}

// HTTPClient creates an *http.Client configured with proxy, TLS, and timeout.
func HTTPClient(proxyURLStr string, skipTLS bool, timeout time.Duration) (*http.Client, error) {
	tr, err := HTTPTransport(proxyURLStr, skipTLS)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}, nil
}
