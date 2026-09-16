package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

func TestValidateCmdValid(t *testing.T) {
	tempDir := t.TempDir()
	f := filepath.Join(tempDir, "valid.lst")
	content := "192.168.1.0/24\n1.1.1.1 # Cloudflare\nexample.com\n"
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"validate", f})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Всего записей") {
		t.Errorf("expected statistics summary in output, got:\n%s", out)
	}
}

func TestValidateCmdStrict(t *testing.T) {
	tempDir := t.TempDir()
	f := filepath.Join(tempDir, "warning.lst")
	// Non-canonical CIDR has host bits set, causing a warning
	content := "192.168.1.5/24\n"
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"validate", f, "--strict"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error with --strict on warnings, got nil")
	}
}

func TestValidateCmdJSON(t *testing.T) {
	tempDir := t.TempDir()
	f := filepath.Join(tempDir, "test.lst")
	content := "10.0.0.1\n"
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"validate", f, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate --json failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"passed": true`) {
		t.Errorf("expected json with passed: true, got:\n%s", out)
	}
}
