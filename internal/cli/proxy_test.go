package cli

import (
	"os"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/config"
)

func TestResolveProxy_Priority(t *testing.T) {
	origEnv := os.Getenv("MT_PROXY")
	defer os.Setenv("MT_PROXY", origEnv)

	// 1. Config only
	loadedConfig = config.Config{Proxy: "socks5://127.0.0.1:1080"}
	os.Unsetenv("MT_PROXY")
	if got := resolveProxy(""); got != "socks5://127.0.0.1:1080" {
		t.Errorf("expected config proxy, got %q", got)
	}

	// 2. Env overrides config
	os.Setenv("MT_PROXY", "http://env-proxy:8080")
	if got := resolveProxy(""); got != "http://env-proxy:8080" {
		t.Errorf("expected env proxy, got %q", got)
	}

	// 3. Flag overrides env and config
	if got := resolveProxy("socks5h://flag-proxy:9050"); got != "socks5h://flag-proxy:9050" {
		t.Errorf("expected flag proxy, got %q", got)
	}
}
