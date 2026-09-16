package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
)

func TestPurgeForce(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	var mu sync.Mutex
	deletedIDs := make(map[string]bool)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"RB4011","version":"7.15","cpu":"arm64","cpu-count":"4","total-memory":"1073741824","free-memory":"800000000","uptime":"1d"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/system/routerboard" {
			w.Write([]byte(`{"model":"RB4011iGS+","serial-number":"12345"}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			listName := r.URL.Query().Get("list")
			if listName == "vpn" {
				entries := []map[string]any{
					{".id": "*1", "address": "1.1.1.1", "list": "vpn", "comment": "Cloudflare", "disabled": "false"},
					{".id": "*2", "address": "8.8.8.8", "list": "vpn", "comment": "Google", "disabled": "false"},
				}
				json.NewEncoder(w).Encode(entries)
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/rest/ip/firewall/address-list/") {
			id := strings.TrimPrefix(r.URL.Path, "/rest/ip/firewall/address-list/")
			mu.Lock()
			deletedIDs[id] = true
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	host := ts.URL

	cmd := rootCmd
	cmd.SetArgs([]string{"purge", "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "-y", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("purge failed: %v", err)
	}

	mu.Lock()
	if !deletedIDs["*1"] || !deletedIDs["*2"] {
		t.Errorf("expected *1 and *2 to be deleted, got: %+v", deletedIDs)
	}
	mu.Unlock()

	// Verify snapshot was created
	snaps, err := snapshot.List(host, "vpn")
	if err != nil {
		t.Fatalf("snapshot list failed: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Total != 2 {
		t.Errorf("expected snapshot with 2 entries, got %d", snaps[0].Total)
	}
}

func TestPurgeDryRun(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	var mu sync.Mutex
	deletedIDs := make(map[string]bool)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			entries := []map[string]any{
				{".id": "*1", "address": "1.1.1.1", "list": "vpn"},
			}
			json.NewEncoder(w).Encode(entries)
			return
		}

		if r.Method == "DELETE" {
			mu.Lock()
			deletedIDs[r.URL.Path] = true
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host := ts.URL

	cmd := rootCmd
	cmd.SetArgs([]string{"purge", "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "-n", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("purge dry-run failed: %v", err)
	}

	mu.Lock()
	if len(deletedIDs) > 0 {
		t.Errorf("dry-run should not make any DELETE requests, got: %+v", deletedIDs)
	}
	mu.Unlock()

	// Verify no snapshot was created in dry-run
	snaps, _ := snapshot.List(host, "vpn")
	if len(snaps) > 0 {
		t.Errorf("dry-run should not create snapshots, got %d", len(snaps))
	}
}

func TestPurgeNonInteractiveRequiresForce(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			entries := []map[string]any{
				{".id": "*1", "address": "1.1.1.1", "list": "vpn"},
			}
			json.NewEncoder(w).Encode(entries)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host := ts.URL

	cmd := rootCmd
	cmd.SetArgs([]string{"purge", "-H", host, "-u", "admin", "-p", "pass", "-l", "vpn", "-k"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error in non-interactive session without --force, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected error message to mention --force, got: %v", err)
	}
}

func TestPurgeAlreadyEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host := ts.URL

	cmd := rootCmd
	cmd.SetArgs([]string{"purge", "-H", host, "-u", "admin", "-p", "pass", "-l", "empty-list", "-y", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("purge on empty list failed: %v", err)
	}
}

func TestPurgeAll(t *testing.T) {
	tempSnapDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempSnapDir)

	var mu sync.Mutex
	deletedIDs := make(map[string]bool)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" && r.URL.Path == "/rest/ip/firewall/address-list" {
			listParam := r.URL.Query().Get("list")
			if listParam == "listA" {
				json.NewEncoder(w).Encode([]map[string]any{
					{".id": "*1", "address": "10.0.0.1", "list": "listA"},
				})
				return
			}
			if listParam == "listB" {
				json.NewEncoder(w).Encode([]map[string]any{
					{".id": "*2", "address": "10.0.0.2", "list": "listB"},
				})
				return
			}
			// GetAllEntries
			json.NewEncoder(w).Encode([]map[string]any{
				{".id": "*1", "address": "10.0.0.1", "list": "listA"},
				{".id": "*2", "address": "10.0.0.2", "list": "listB"},
			})
			return
		}

		if r.Method == "DELETE" {
			id := strings.TrimPrefix(r.URL.Path, "/rest/ip/firewall/address-list/")
			mu.Lock()
			deletedIDs[id] = true
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host := ts.URL

	cmd := rootCmd
	cmd.SetArgs([]string{"purge", "-H", host, "-u", "admin", "-p", "pass", "--all", "-y", "-k"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("purge --all failed: %v", err)
	}

	mu.Lock()
	if !deletedIDs["*1"] || !deletedIDs["*2"] {
		t.Errorf("expected both *1 and *2 to be deleted with --all, got: %+v", deletedIDs)
	}
	mu.Unlock()
}
