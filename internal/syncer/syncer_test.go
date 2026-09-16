package syncer_test

import (
	"context"
	"fmt"
	"strings"
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
	changes, dups := syncer.Diff(desired, nil)
	if len(dups) != 1 || dups[0] != "8.8.8.8" {
		t.Errorf("expected one duplicate 8.8.8.8, got %v", dups)
	}
	if len(changes) != 1 {
		t.Fatalf("expected exactly 1 change for duplicates, got %d", len(changes))
	}
	if changes[0].Action != syncer.ActionAdd || changes[0].Address != "8.8.8.8" {
		t.Errorf("unexpected change: %+v", changes[0])
	}
}

func TestDiffAddrNormalization(t *testing.T) {
	// Router returns 8.8.8.8/32, file has bare 8.8.8.8 — must be treated as the same entry.
	desired := []parser.Entry{entry("8.8.8.8", "DNS")}
	cur := []mikrotik.AddressListEntry{current("*1", "8.8.8.8/32", "DNS", false)}
	changes, _ := syncer.Diff(desired, cur)
	if len(changes) != 0 {
		t.Errorf("expected no changes after normalization, got %d: %+v", len(changes), changes)
	}
}

func TestDiffAddrNormalizationUpdate(t *testing.T) {
	// Same address mismatch but comment differs — must produce Update, not Add+Delete.
	desired := []parser.Entry{entry("1.1.1.1", "NEW")}
	cur := []mikrotik.AddressListEntry{current("*2", "1.1.1.1/32", "OLD", false)}
	changes, _ := syncer.Diff(desired, cur)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Action != syncer.ActionUpdate {
		t.Errorf("expected ActionUpdate, got %v", changes[0].Action)
	}
	if changes[0].ID != "*2" {
		t.Errorf("expected ID *2, got %q", changes[0].ID)
	}
}

// ── Apply tests ───────────────────────────────────────────────────────────────

type mockClient struct {
	added   []string
	updated []string
	deleted []string
	failOn  string
}

func (m *mockClient) AddEntry(_ context.Context, list, addr, comment string, disabled bool) error {
	if m.failOn == addr {
		return fmt.Errorf("mock error")
	}
	m.added = append(m.added, addr)
	return nil
}

func (m *mockClient) UpdateEntry(_ context.Context, id, comment string, disabled bool) error {
	if m.failOn == id {
		return fmt.Errorf("mock error")
	}
	m.updated = append(m.updated, id)
	return nil
}

func (m *mockClient) DeleteEntry(_ context.Context, id string) error {
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

	if err := syncer.Apply(context.Background(), client, "test", changes, false, false, 4); err != nil {
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

	if err := syncer.Apply(context.Background(), client, "test", changes, true, false, 4); err != nil {
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
	if err := syncer.Apply(context.Background(), client, "test", changes, false, false, 4); err == nil {
		t.Error("expected error, got nil")
	}
}

type mockBatchClient struct {
	mockClient
	executedScripts []string
	failExecute     bool
}

func (m *mockBatchClient) Execute(ctx context.Context, script string) error {
	if m.failExecute {
		return fmt.Errorf("execute error")
	}
	m.executedScripts = append(m.executedScripts, script)
	return nil
}

func TestBuildBatchScript(t *testing.T) {
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "1.1.1.1", NewComment: "Cloudflare", NewDisabled: true},
		{Action: syncer.ActionDelete, Address: "2.2.2.2"},
		{Action: syncer.ActionUpdate, Address: "3.3.3.3", NewComment: "Updated", NewDisabled: false},
	}

	script := syncer.BuildBatchScript("vpn", changes)
	expectedLines := []string{
		`/ip firewall address-list`,
		`:do { add list="vpn" address="1.1.1.1" comment="Cloudflare" disabled=yes } on-error={}`,
		`:do { remove [find where list="vpn" and address="2.2.2.2"] } on-error={}`,
		`:do { set [find where list="vpn" and address="3.3.3.3"] comment="Updated" disabled=no } on-error={}`,
	}

	for _, line := range expectedLines {
		if !strings.Contains(script, line) {
			t.Errorf("expected script to contain %q, got:\n%s", line, script)
		}
	}
}

func TestApplyBatch(t *testing.T) {
	client := &mockBatchClient{}
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "1.1.1.1"},
		{Action: syncer.ActionAdd, Address: "2.2.2.2"},
		{Action: syncer.ActionAdd, Address: "3.3.3.3"},
	}

	// Test batch with batchSize 2
	err := syncer.ApplyBatch(context.Background(), client, "vpn", changes, false, false, 2)
	if err != nil {
		t.Fatalf("ApplyBatch failed: %v", err)
	}

	if len(client.executedScripts) != 2 {
		t.Errorf("expected 2 batch scripts, got %d", len(client.executedScripts))
	}
}

func TestApplyBatchFallbackOnExecuteError(t *testing.T) {
	client := &mockBatchClient{failExecute: true}
	changes := []syncer.Change{
		{Action: syncer.ActionAdd, Address: "1.1.1.1"},
	}

	// Should fallback to individual Apply
	err := syncer.ApplyBatch(context.Background(), client, "vpn", changes, false, false, 10)
	if err != nil {
		t.Fatalf("ApplyBatch fallback failed: %v", err)
	}
	if len(client.added) != 1 || client.added[0] != "1.1.1.1" {
		t.Errorf("expected fallback to add via APIClient, got: %v", client.added)
	}
}

