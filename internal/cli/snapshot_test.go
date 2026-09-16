package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
)

func TestSnapshotListAndJSON(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempDir)

	host := "192.168.88.1"
	list := "vpn"

	// Create 2 test snapshots
	_, err := snapshot.Save(host, list, []mikrotik.AddressListEntry{
		{Address: "1.1.1.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	output.SetWriter(&buf)
	defer output.ResetWriter()

	cmd := rootCmd
	cmd.SetArgs([]string{"snapshot", "list", "-H", host, "-l", list})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("snapshot list failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1 записей") {
		t.Errorf("expected snapshot list output to show entries count, got:\n%s", out)
	}

	// Test JSON
	buf.Reset()
	cmd.SetArgs([]string{"snapshot", "list", "-H", host, "-l", list, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("snapshot list --json failed: %v", err)
	}

	jsonOut := buf.String()
	if !strings.Contains(jsonOut, `"list_name": "vpn"`) {
		t.Errorf("expected json with list_name vpn, got:\n%s", jsonOut)
	}
}
