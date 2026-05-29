package syncer_test

import (
	"fmt"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

// ── Diff tests ────────────────────────────────────────────────────────────────

func entry(addr, comment string) parser.Entry {
	return parser.Entry{Address: addr, Comment: comment}
}

func entryDisabled(addr string) parser.Entry {
	return parser.Entry{Address: addr, Disabled: true}
}

func current(id, addr, comment string, disabled bool) mikrotik.AddressListEntry {
	return mikrotik.AddressListEntry{
		ID:       id,
		Address:  addr,
		Comment:  comment,
		Disabled: mikrotik.BoolString(disabled),
	}
}

func TestDiffAdd(t *testing.T) {
	desired := []parser.Entry{entry("8.8.8.8", "DNS")}
	changes, _ := syncer.Diff(desired, nil)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Action != syncer.ActionAdd {
		t.Errorf("expected ActionAdd, got %v", changes[0].Action)
	}
	if changes[0].Address != "8.8.8.8" || changes[0].NewComment != "DNS" {
		t.Errorf("unexpected change: %+v", changes[0])
	}
}

func TestDiffDelete(t *testing.T) {
	cur := []mikrotik.AddressListEntry{current("*1", "8.8.8.8", "", false)}
	changes, _ := syncer.Diff(nil, cur)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Action != syncer.ActionDelete {
		t.Errorf("expected ActionDelete, got %v", changes[0].Action)
	}
	if changes[0].ID != "*1" {
		t.Errorf("expected ID *1, got %q", changes[0].ID)
	}
}

func TestDiffUpdateComment(t *testing.T) {
	desired := []parser.Entry{entry("8.8.8.8", "NEW")}
	cur := []mikrotik.AddressListEntry{current("*1", "8.8.8.8", "OLD", false)}
	changes, _ := syncer.Diff(desired, cur)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	ch := changes[0]
	if ch.Action != syncer.ActionUpdate {
		t.Errorf("expected ActionUpdate, got %v", ch.Action)
	}
	if ch.OldComment != "OLD" || ch.NewComment != "NEW" {
		t.Errorf("comments: old=%q new=%q", ch.OldComment, ch.NewComment)
	}
}

func TestDiffUpdateDisabled(t *testing.T) {
	desired := []parser.Entry{entryDisabled("1.1.1.1")}
	cur := []mikrotik.AddressListEntry{current("*2", "1.1.1.1", "", false)}
	changes, _ := syncer.Diff(desired, cur)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	ch := changes[0]
	if ch.Action != syncer.ActionUpdate {
		t.Errorf("expected ActionUpdate, got %v", ch.Action)
	}
	if !ch.NewDisabled || ch.OldDisabled {
		t.Errorf("disabled: old=%v new=%v", ch.OldDisabled, ch.NewDisabled)
	}
}

func TestDiffNoChange(t *testing.T) {
	desired := []parser.Entry{entry("8.8.8.8", "DNS")}
	cur := []mikrotik.AddressListEntry{current("*1", "8.8.8.8", "DNS", false)}
	changes, _ := syncer.Diff(desired, cur)

	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d: %+v", len(changes), changes)
	}
}

func TestDiffMixed(t *testing.T) {
	desired := []parser.Entry{
		entry("8.8.8.8", "DNS"),   // unchanged
		entry("1.1.1.1", "CF"),    // new
		entry("9.9.9.9", "QUAD9"), // comment update
	}
	cur := []mikrotik.AddressListEntry{
		current("*1", "8.8.8.8", "DNS", false),
		current("*2", "9.9.9.9", "OLD", false),
		current("*3", "2.2.2.2", "", false), // to delete
	}
	changes, _ := syncer.Diff(desired, cur)

	counts := map[syncer.Action]int{}
	for _, ch := range changes {
		counts[ch.Action]++
	}
	if counts[syncer.ActionAdd] != 1 {
		t.Errorf("expected 1 add, got %d", counts[syncer.ActionAdd])
	}
	if counts[syncer.ActionDelete] != 1 {
		t.Errorf("expected 1 delete, got %d", counts[syncer.ActionDelete])
	}
	if counts[syncer.ActionUpdate] != 1 {
		t.Errorf("expected 1 update, got %d", counts[syncer.ActionUpdate])
	}
}

func TestDiffDuplicates(t *testing.T) {
	desired := []parser.Entry{entry("8.8.8.8", "first"), entry("8.8.8.8", "second")}
	_, dups := syncer.Diff(desired, nil)
	if len(dups) != 1 || dups[0] != "8.8.8.8" {
		t.Errorf("expected one duplicate 8.8.8.8, got %v", dups)
	}
}

// ── Apply tests ───────────────────────────────────────────────────────────────

type mockClient struct {
	added   []string
	updated []string
	deleted []string
	failOn  string
}

func (m *mockClient) AddEntry(list, addr, comment string, disabled bool) error {
	if m.failOn == addr {
		return fmt.Errorf("mock error")
	}
	m.added = append(m.added, addr)
	return nil
}

func (m *mockClient) UpdateEntry(id, comment string, disabled bool) error {
	if m.failOn == id {
		return fmt.Errorf("mock error")
	}
	m.updated = append(m.updated, id)
	return nil
}

func (m *mockClient) DeleteEntry(id string) error {
	if m.failOn == id {
		return fmt.Errorf("mock error")
	}
	m.deleted = append(m.deleted, id)
	return nil
}

func TestApplyExecutesChanges(t *testing.T) {
	client := &mockClient{}
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "8.8.8.8", NewComment: "DNS"},
		{Action: syncer.ActionDelete, Address: "1.1.1.1", ID: "*1"},
		{Action: syncer.ActionUpdate, Address: "9.9.9.9", ID: "*2", NewComment: "NEW"},
	}

	if err := syncer.Apply(client, "test", changes, false, false); err != nil {
		t.Fatal(err)
	}
	if len(client.added) != 1 || client.added[0] != "8.8.8.8" {
		t.Errorf("added: %v", client.added)
	}
	if len(client.deleted) != 1 || client.deleted[0] != "*1" {
		t.Errorf("deleted: %v", client.deleted)
	}
	if len(client.updated) != 1 || client.updated[0] != "*2" {
		t.Errorf("updated: %v", client.updated)
	}
}

func TestApplyDryRunSkipsAPI(t *testing.T) {
	client := &mockClient{}
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "8.8.8.8"},
		{Action: syncer.ActionDelete, ID: "*1"},
	}

	if err := syncer.Apply(client, "test", changes, true, false); err != nil {
		t.Fatal(err)
	}
	if len(client.added)+len(client.deleted)+len(client.updated) != 0 {
		t.Error("dry-run should not call API")
	}
}

func TestApplyPropagatesError(t *testing.T) {
	client := &mockClient{failOn: "8.8.8.8"}
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "8.8.8.8"},
	}
	if err := syncer.Apply(client, "test", changes, false, false); err == nil {
		t.Error("expected error, got nil")
	}
}
