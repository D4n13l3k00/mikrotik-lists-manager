package mikrotik_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
)

func TestGetAllEntries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/ip/firewall/address-list" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("dynamic") != "false" {
			t.Errorf("expected dynamic=false, got %s", r.URL.Query().Get("dynamic"))
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("expected User-Agent header")
		}

		entries := []map[string]any{
			{".id": "*1", "address": "1.1.1.1", "list": "vpn", "comment": "CF", "disabled": "false"},
			{".id": "*2", "address": "8.8.8.8", "list": "vpn", "comment": "Google", "disabled": "true"},
		}
		json.NewEncoder(w).Encode(entries)
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	list, err := client.GetAllEntries(context.Background())
	if err != nil {
		t.Fatalf("GetAllEntries failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Disabled.Bool() {
		t.Errorf("expected entry 0 to be enabled")
	}
	if !list[1].Disabled.Bool() {
		t.Errorf("expected entry 1 to be disabled")
	}
}

func TestAddEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["address"] == "dup" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"detail":"already have such entry"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	ctx := context.Background()

	if err := client.AddEntry(ctx, "vpn", "1.1.1.1", "test", false); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Should ignore "already have such entry"
	if err := client.AddEntry(ctx, "vpn", "dup", "test", false); err != nil {
		t.Fatalf("expected nil for existing entry, got: %v", err)
	}
}

func TestUpdateAndDeleteEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/ip/firewall/address-list/*1A" {
			t.Errorf("unexpected path (ID not preserved?): %s", r.URL.Path)
		}
		if r.Method == "PATCH" || r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	ctx := context.Background()

	if err := client.UpdateEntry(ctx, "*1A", "new comment", true); err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}
	if err := client.DeleteEntry(ctx, "*1A"); err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}
}

func TestRenameList(t *testing.T) {
	var patchCount atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			entries := []map[string]any{
				{".id": "*1", "address": "1.1.1.1", "list": "old"},
				{".id": "*2", "address": "2.2.2.2", "list": "old"},
			}
			json.NewEncoder(w).Encode(entries)
			return
		}
		if r.Method == "PATCH" {
			patchCount.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	var progressCalled atomic.Bool
	n, err := client.RenameList(context.Background(), "old", "new", 2, func(done, total int) {
		progressCalled.Store(true)
	})
	if err != nil {
		t.Fatalf("RenameList failed: %v", err)
	}
	if n != 2 || patchCount.Load() != 2 {
		t.Errorf("expected 2 renames, got n=%d, patchCount=%d", n, patchCount.Load())
	}
	if !progressCalled.Load() {
		t.Errorf("expected progress callback to be called")
	}
}

func TestGetRouterInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/system/resource" {
			w.Write([]byte(`{"board-name":"hEX","version":"7.15","cpu":"MIPS"}`))
			return
		}
		if r.URL.Path == "/rest/system/routerboard" {
			// Simulating CHR/x86 where routerboard returns 404
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	info, err := client.GetRouterInfo(context.Background())
	if err != nil {
		t.Fatalf("GetRouterInfo failed: %v", err)
	}
	if info.BoardName != "hEX" || info.Version != "7.15" {
		t.Errorf("unexpected resource info: %+v", info)
	}
	if info.Model != "" {
		t.Errorf("expected empty model on routerboard 404, got %q", info.Model)
	}
}

func TestRetriesOn500(t *testing.T) {
	var attempts atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := attempts.Add(1)
		if att < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	client := mikrotik.NewClient(ts.URL, "user", "pass", true)
	_, err := client.GetAllEntries(context.Background())
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}
