package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSyncFast(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	tempFile := filepath.Join(t.TempDir(), "test.list")
	if err := os.WriteFile(tempFile, []byte("1.1.1.1 ## Cloudflare\n2.2.2.2\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	var mu sync.Mutex
	var executedScripts []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"RB4011","version":"7.15"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/system/routerboard" {
			w.Write([]byte(`{"model":"RB4011iGS+"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			// Router currently has empty list
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		if r.Method == "POST" && r.URL.Path == "/rest/execute" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			executedScripts = append(executedScripts, body["script"])
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	host := ts.URL
	cmd := rootCmd
	cmd.SetArgs([]string{"sync", tempFile, "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "--fast", "-y", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --fast failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(executedScripts) != 1 {
		t.Fatalf("expected 1 executed script, got %d", len(executedScripts))
	}
	if !strings.Contains(executedScripts[0], `add list="vpn" address="1.1.1.1" comment="Cloudflare"`) {
		t.Errorf("script does not contain expected add command: %s", executedScripts[0])
	}
	if !strings.Contains(executedScripts[0], `add list="vpn" address="2.2.2.2"`) {
		t.Errorf("script does not contain expected add command: %s", executedScripts[0])
	}
}

func TestSyncBatch(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	tempFile := filepath.Join(t.TempDir(), "test.list")
	if err := os.WriteFile(tempFile, []byte("1.1.1.1\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	var mu sync.Mutex
	var executedScripts []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"RB4011","version":"7.15"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/system/routerboard" {
			w.Write([]byte(`{"model":"RB4011iGS+"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		if r.Method == "POST" && r.URL.Path == "/rest/execute" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			executedScripts = append(executedScripts, body["script"])
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	host := ts.URL
	cmd := rootCmd
	cmd.SetArgs([]string{"sync", tempFile, "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "--batch", "-y", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --batch failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(executedScripts) != 1 {
		t.Fatalf("expected 1 executed script, got %d", len(executedScripts))
	}
}

func TestSyncInterruptRollbackConfirmed(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	tempFile := filepath.Join(t.TempDir(), "test.list")
	if err := os.WriteFile(tempFile, []byte("1.1.1.1\n2.2.2.2\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	var mu sync.Mutex
	var executedScripts []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"RB4011","version":"7.15"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/rest/system/routerboard" {
			w.Write([]byte(`{"model":"RB4011iGS+"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		if r.Method == "POST" && r.URL.Path == "/rest/execute" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			script := body["script"]

			mu.Lock()
			executedScripts = append(executedScripts, script)
			mu.Unlock()

			// If it's the sync script adding entries, trigger interrupt!
			if strings.Contains(script, `add list="vpn"`) {
				cancel()
				http.Error(w, "client canceled", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	stdinReader = strings.NewReader("y\n")
	defer func() { stdinReader = nil }()

	host := ts.URL
	cmd := rootCmd
	cmd.SetArgs([]string{"sync", tempFile, "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "--fast", "-y", "-k"})
	err := cmd.ExecuteContext(ctx)
	if err == nil || !errors.Is(err, errInterrupted) {
		t.Fatalf("expected errInterrupted, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	// Should have executed the sync script (which was canceled) and then the rollback script
	var foundRollback bool
	for _, s := range executedScripts {
		if strings.Contains(s, `remove [find where list="vpn"]`) {
			foundRollback = true
			break
		}
	}
	if !foundRollback {
		t.Fatalf("expected rollback script to be executed, got: %v", executedScripts)
	}
}

func TestSyncInterruptRollbackDeclined(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	tempFile := filepath.Join(t.TempDir(), "test.list")
	if err := os.WriteFile(tempFile, []byte("1.1.1.1\n2.2.2.2\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	var mu sync.Mutex
	var executedScripts []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"RB4011","version":"7.15"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/rest/system/routerboard" {
			w.Write([]byte(`{"model":"RB4011iGS+"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		if r.Method == "POST" && r.URL.Path == "/rest/execute" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			script := body["script"]

			mu.Lock()
			executedScripts = append(executedScripts, script)
			mu.Unlock()

			if strings.Contains(script, `add list="vpn"`) {
				cancel()
				http.Error(w, "client canceled", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	stdinReader = strings.NewReader("n\n")
	defer func() { stdinReader = nil }()

	host := ts.URL
	cmd := rootCmd
	cmd.SetArgs([]string{"sync", tempFile, "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "--fast", "-y", "-k"})
	err := cmd.ExecuteContext(ctx)
	if err == nil || !errors.Is(err, errInterrupted) {
		t.Fatalf("expected errInterrupted, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, s := range executedScripts {
		if strings.Contains(s, `remove [find where list="vpn"]`) {
			t.Fatalf("did not expect rollback script, got: %v", executedScripts)
		}
	}
}

