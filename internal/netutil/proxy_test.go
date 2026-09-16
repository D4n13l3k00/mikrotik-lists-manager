package netutil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewDialerSchemes(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"", false},
		{"socks5://127.0.0.1:1080", false},
		{"socks5h://user:pass@127.0.0.1:1080", false},
		{"http://127.0.0.1:8080", false},
		{"https://127.0.0.1:8443", false},
		{"ftp://127.0.0.1:21", true},
		{"://invalid-url", true},
	}

	for _, tt := range tests {
		_, err := NewDialer(tt.url, 2*time.Second)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewDialer(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
		}
	}
}

func TestHTTPConnectProxy(t *testing.T) {
	// Start a mock target echo server
	targetListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetListener.Close()

	go func() {
		for {
			conn, err := targetListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 5)
				if _, err := io.ReadFull(c, buf); err == nil {
					_, _ = io.WriteString(c, "ECHO:"+string(buf))
				}
			}(conn)
		}
	}()

	targetAddr := targetListener.Addr().String()

	// Start a mock HTTP CONNECT proxy
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

				// Bridge connections bidirectionally
				errc := make(chan error, 2)
				go func() {
					_, err := io.Copy(targetConn, br)
					errc <- err
				}()
				go func() {
					_, err := io.Copy(c, targetConn)
					errc <- err
				}()
				<-errc
			}(clientConn)
		}
	}()

	proxyURL := fmt.Sprintf("http://%s", proxyListener.Addr().String())

	dialer, err := NewDialer(proxyURL, 5*time.Second)
	if err != nil {
		t.Fatalf("NewDialer failed: %v", err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", targetAddr)
	if err != nil {
		t.Fatalf("DialContext through HTTP CONNECT proxy failed: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("HELLO")); err != nil {
		t.Fatalf("Write through HTTP CONNECT proxy failed: %v", err)
	}

	buf := make([]byte, 10)
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("Read from proxied connection failed: %v", err)
	}
	if string(buf[:n]) != "ECHO:HELLO" {
		t.Errorf("got %q, want ECHO:HELLO", string(buf[:n]))
	}
}

func TestHTTPTransportAndClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Direct (no proxy)
	client, err := HTTPClient("", false, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("expected OK, got %s", string(body))
	}
}
