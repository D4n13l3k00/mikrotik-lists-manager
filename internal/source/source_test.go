package source_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
)

func TestReadLocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.list")
	content := "1.1.1.1\n8.8.8.8\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write tmp file: %v", err)
	}

	data, err := source.Read(context.Background(), filePath)
	if err != nil {
		t.Fatalf("source.Read failed: %v", err)
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestReadURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "mikrotik-lists-manager/") {
			t.Errorf("expected User-Agent starting with mikrotik-lists-manager/, got: %s", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("192.168.1.1\n10.0.0.1\n"))
	}))
	defer ts.Close()

	data, err := source.Read(context.Background(), ts.URL+"/list.txt")
	if err != nil {
		t.Fatalf("source.Read failed: %v", err)
	}
	expected := "192.168.1.1\n10.0.0.1\n"
	if string(data) != expected {
		t.Errorf("got %q, want %q", string(data), expected)
	}
}

func TestReadURL_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := source.Read(context.Background(), ts.URL+"/404.txt")
	if err == nil {
		t.Fatalf("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention 404, got: %v", err)
	}
}
