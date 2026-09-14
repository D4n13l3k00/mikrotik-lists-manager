package fetcher

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/version"
)

const (
	maxHTTPRetries = 3
	baseRetryDelay = 1000 * time.Millisecond
)

func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	if sec, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return fallback
}

func backoffWithJitter(attempt int) time.Duration {
	delay := baseRetryDelay * time.Duration(1<<attempt) // 1s, 2s, 4s
	jitter := time.Duration(rand.Intn(400)) * time.Millisecond
	return delay + jitter
}

func get(client *http.Client, targetURL string) ([]byte, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 0; attempt <= maxHTTPRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoffWithJitter(attempt - 1))
		}

		req, reqErr := http.NewRequest(http.MethodGet, targetURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", version.UserAgent())

		resp, err = client.Do(req)
		if err != nil {
			if attempt < maxHTTPRetries {
				continue
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}

		// Retry on 429 (Too Many Requests) or 5xx (Server Error)
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < maxHTTPRetries {
			delay := parseRetryAfter(resp.Header.Get("Retry-After"), backoffWithJitter(attempt))
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()
			time.Sleep(delay)
			continue
		}

		defer resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil, err
}

func isIPv4CIDR(s string) bool {
	if s == "" || len(s) > 18 || containsColon(s) {
		return false
	}
	if _, _, err := net.ParseCIDR(s); err == nil {
		return true
	}
	return net.ParseIP(s) != nil
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
