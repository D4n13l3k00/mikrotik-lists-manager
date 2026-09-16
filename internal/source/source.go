package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/netutil"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/version"
)

// DefaultHTTPTimeout is the default timeout for fetching remote list sources.
const DefaultHTTPTimeout = 30 * time.Second

// Read fetches or reads content from a local file, stdin ("-"), or remote HTTP/HTTPS URL.
func Read(ctx context.Context, target string) ([]byte, error) {
	return ReadWithProxy(ctx, target, "")
}

// ReadWithProxy fetches or reads content, routing HTTP/HTTPS requests through the given proxy if set.
func ReadWithProxy(ctx context.Context, target, proxyURL string) ([]byte, error) {
	if target == "-" {
		return io.ReadAll(os.Stdin)
	}

	if IsURL(target) {
		return fetchURL(ctx, target, proxyURL)
	}

	return os.ReadFile(target)
}

// IsURL returns true if target begins with http:// or https://.
func IsURL(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

func fetchURL(ctx context.Context, url, proxyURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", version.UserAgent())

	client, err := netutil.HTTPClient(proxyURL, false, DefaultHTTPTimeout)
	if err != nil {
		return nil, fmt.Errorf("настройка HTTP-клиента с прокси: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: unexpected HTTP status %d %s", url, resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	return data, nil
}
