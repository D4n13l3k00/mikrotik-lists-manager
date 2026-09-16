package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

func TestDiffCmdOffline(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "base.lst")
	f2 := filepath.Join(tempDir, "target.lst")

	content1 := "1.1.1.1 ## Cloudflare\n8.8.8.8 ## Old DNS\n"
	content2 := "1.1.1.1 ## Cloudflare\n8.8.8.8 ## Google DNS\n9.9.9.9 ## Quad9\n"

	if err := os.WriteFile(f1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"diff", f1, f2})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}

	out := buf.String()
	// Should show addition of 9.9.9.9 and update of 8.8.8.8
	if !strings.Contains(out, "9.9.9.9") {
		t.Errorf("expected output to contain 9.9.9.9, got:\n%s", out)
	}
	if !strings.Contains(out, "8.8.8.8") {
		t.Errorf("expected output to contain 8.8.8.8, got:\n%s", out)
	}
}

func TestDiffCmdJSON(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "base.lst")
	f2 := filepath.Join(tempDir, "target.lst")

	content1 := "1.1.1.1\n"
	content2 := "1.1.1.1\n2.2.2.2\n"

	if err := os.WriteFile(f1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"diff", f1, f2, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff --json failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"action": "add"`) || !strings.Contains(out, `"2.2.2.2"`) {
		t.Errorf("expected json output with add action, got:\n%s", out)
	}
}
