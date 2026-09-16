package snapshot

import (
	"fmt"
	"testing"
	"time"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
)

func TestSnapshotSaveListLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempDir)

	host := "192.168.88.1:8728"
	listName := "vpn-list"

	entries := []mikrotik.AddressListEntry{
		{ID: "*1", List: listName, Address: "1.1.1.1", Comment: "Cloudflare DNS", Disabled: false},
		{ID: "*2", List: listName, Address: "8.8.8.8", Comment: "Google DNS", Disabled: true},
	}

	meta, err := Save(host, listName, entries)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if meta.Total != 2 {
		t.Errorf("expected total 2, got %d", meta.Total)
	}

	metas, err := List(host, listName)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(metas))
	}
	if metas[0].ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, metas[0].ID)
	}

	// Load by ID
	loaded, err := Load(host, listName, meta.ID)
	if err != nil {
		t.Fatalf("Load by ID failed: %v", err)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 loaded entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Address != "1.1.1.1" || loaded.Entries[1].Address != "8.8.8.8" {
		t.Errorf("loaded entries mismatch: %+v", loaded.Entries)
	}

	// Load latest
	latest, err := Load(host, listName, "latest")
	if err != nil {
		t.Fatalf("Load latest failed: %v", err)
	}
	if latest.ID != meta.ID {
		t.Errorf("expected latest ID %s, got %s", meta.ID, latest.ID)
	}

	// Delete
	if err := Delete(host, listName, meta.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	metasAfter, err := List(host, listName)
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(metasAfter) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(metasAfter))
	}
}

func TestSnapshotPrune(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("MT_SNAPSHOTS_DIR", tempDir)

	host := "10.0.0.1"
	listName := "test-prune"

	for i := 0; i < 15; i++ {
		time.Sleep(10 * time.Millisecond)
		_, err := Save(host, listName, []mikrotik.AddressListEntry{
			{ID: fmt.Sprintf("*%d", i), Address: fmt.Sprintf("10.0.0.%d", i)},
		})
		if err != nil {
			t.Fatalf("Save %d failed: %v", i, err)
		}
	}

	metas, err := List(host, listName)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// Default maxSnapshotsPerList is 10
	if len(metas) > 10 {
		t.Errorf("expected at most 10 snapshots after prune, got %d", len(metas))
	}

	// Test manual prune to 3
	if err := Prune(host, listName, 3); err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	metasPruned, err := List(host, listName)
	if err != nil {
		t.Fatalf("List after prune failed: %v", err)
	}
	if len(metasPruned) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(metasPruned))
	}
}
