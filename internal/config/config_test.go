package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/config"
)

func TestLoadNonExistent(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected nil error for empty path, got %v", err)
	}
	if cfg.Host != "" {
		t.Errorf("expected empty host, got %q", cfg.Host)
	}

	cfg, err = config.Load("does_not_exist.yaml")
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got %v", err)
	}
	if cfg.Host != "" {
		t.Errorf("expected empty host, got %q", cfg.Host)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	skip := true
	cfg := config.Config{
		Host:          "192.168.1.1",
		User:          "admin",
		Pass:          "secret",
		List:          "vpn",
		SkipTLSVerify: false,
		DefaultFormat: "native",
		DefaultProfile: "home",
		Profiles: map[string]config.ProfileConfig{
			"home": {
				Host:          "192.168.1.10",
				SkipTLSVerify: &skip,
			},
		},
	}

	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Host != "192.168.1.1" || loaded.User != "admin" {
		t.Errorf("loaded data mismatch: %+v", loaded)
	}
	if loaded.DefaultProfile != "home" {
		t.Errorf("expected default profile 'home', got %q", loaded.DefaultProfile)
	}
}

func TestEffectiveProfile(t *testing.T) {
	skipTrue := true
	skipFalse := false
	cfg := config.Config{
		Host:           "192.168.1.1",
		User:           "admin",
		Pass:           "masterpass",
		List:           "default-list",
		SkipTLSVerify:  false,
		Proxy:          "socks5://global:1080",
		DefaultProfile: "home",
		Profiles: map[string]config.ProfileConfig{
			"home": {
				Host:          "192.168.1.100",
				SkipTLSVerify: &skipTrue,
			},
			"office": {
				Host:          "10.0.0.1",
				User:          "office-admin",
				List:          "office-list",
				SkipTLSVerify: &skipFalse,
				Proxy:         "http://proxy.corp:8080",
			},
		},
	}

	// 1. Default profile ("home")
	effHome, err := cfg.EffectiveProfile("")
	if err != nil {
		t.Fatalf("EffectiveProfile(\"\") failed: %v", err)
	}
	if effHome.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %q", effHome.Host)
	}
	if effHome.User != "admin" { // inherited from global
		t.Errorf("expected user 'admin', got %q", effHome.User)
	}
	if !effHome.Insecure(false) {
		t.Errorf("expected Insecure to be true")
	}
	if effHome.Proxy != "socks5://global:1080" {
		t.Errorf("expected inherited proxy 'socks5://global:1080', got %q", effHome.Proxy)
	}

	// 2. Explicit profile "office"
	effOffice, err := cfg.EffectiveProfile("office")
	if err != nil {
		t.Fatalf("EffectiveProfile(\"office\") failed: %v", err)
	}
	if effOffice.Host != "10.0.0.1" || effOffice.User != "office-admin" {
		t.Errorf("expected office host and user, got %+v", effOffice)
	}
	if effOffice.Pass != "masterpass" { // inherited from global
		t.Errorf("expected inherited pass, got %q", effOffice.Pass)
	}
	if effOffice.List != "office-list" {
		t.Errorf("expected office-list, got %q", effOffice.List)
	}
	if effOffice.Insecure(true) {
		t.Errorf("expected Insecure to be false")
	}
	if effOffice.Proxy != "http://proxy.corp:8080" {
		t.Errorf("expected profile proxy 'http://proxy.corp:8080', got %q", effOffice.Proxy)
	}

	// 3. Non-existent profile
	_, err = cfg.EffectiveProfile("unknown")
	if err == nil {
		t.Errorf("expected error for unknown profile, got nil")
	}
}

func TestFindConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, ".mikrotik-lists-manager.yaml")
	if err := os.WriteFile(confPath, []byte("host: 1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	foundPath, found, err := config.FindConfigFile(confPath)
	if err != nil || !found || foundPath != confPath {
		t.Errorf("explicit path find failed: path=%q, found=%v, err=%v", foundPath, found, err)
	}

	_, _, err = config.FindConfigFile(filepath.Join(tmpDir, "missing.yaml"))
	if err == nil {
		t.Errorf("expected error for non-existent explicit path, got nil")
	}
}
