package parser_test

import (
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

func TestParseNative(t *testing.T) {
	input := `
# human comment, ignored
## Google DNS
8.8.8.8
8.8.4.4

1.1.1.1  ## Cloudflare DNS
2.2.2.2  # human note only
`
	entries, err := parser.ParseNative(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	cases := []struct {
		addr, comment, human string
	}{
		{"8.8.8.8", "Google DNS", ""},
		{"8.8.4.4", "", ""},
		{"1.1.1.1", "Cloudflare DNS", ""},
		{"2.2.2.2", "", "human note only"},
	}
	for i, c := range cases {
		e := entries[i]
		if e.Address != c.addr {
			t.Errorf("[%d] address: got %q want %q", i, e.Address, c.addr)
		}
		if e.Comment != c.comment {
			t.Errorf("[%d] comment: got %q want %q", i, e.Comment, c.comment)
		}
		if e.HumanNote != c.human {
			t.Errorf("[%d] human: got %q want %q", i, e.HumanNote, c.human)
		}
	}
}

func TestParseNativeDisabled(t *testing.T) {
	input := `
8.8.8.8
!1.1.1.1  ## DISABLED ENTRY
!10.0.0.0/8
`
	entries, err := parser.ParseNative(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Disabled {
		t.Error("entry 0 should not be disabled")
	}
	if !entries[1].Disabled {
		t.Error("entry 1 should be disabled")
	}
	if entries[1].Comment != "DISABLED ENTRY" {
		t.Errorf("entry 1 comment: got %q want %q", entries[1].Comment, "DISABLED ENTRY")
	}
	if !entries[2].Disabled {
		t.Error("entry 2 should be disabled")
	}
}

func TestParseNativeBlockComment(t *testing.T) {
	// block ## applies to next entry, inline ## overrides it
	input := `
## BLOCK COMMENT
8.8.8.8
9.9.9.9  ## INLINE COMMENT
`
	entries, err := parser.ParseNative(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Comment != "BLOCK COMMENT" {
		t.Errorf("block comment not applied: got %q", entries[0].Comment)
	}
	if entries[1].Comment != "INLINE COMMENT" {
		t.Errorf("inline comment: got %q", entries[1].Comment)
	}
}

func TestParseNativeBlockCommentNotLeaking(t *testing.T) {
	// block ## must not leak to the entry after the next one
	input := `
## ONLY FOR FIRST
8.8.8.8
1.1.1.1
`
	entries, err := parser.ParseNative(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Comment != "ONLY FOR FIRST" {
		t.Errorf("first entry comment: got %q", entries[0].Comment)
	}
	if entries[1].Comment != "" {
		t.Errorf("second entry should have no comment, got %q", entries[1].Comment)
	}
}

func TestParseMikrotik(t *testing.T) {
	input := `/ip firewall address-list
add list=vpn address=8.8.8.8 comment="Google DNS"
add list=vpn address=1.1.1.1
`
	entries, err := parser.ParseMikrotik(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Address != "8.8.8.8" || entries[0].Comment != "Google DNS" {
		t.Errorf("entry 0: %+v", entries[0])
	}
	if entries[1].Address != "1.1.1.1" || entries[1].Comment != "" {
		t.Errorf("entry 1: %+v", entries[1])
	}
}

func TestParseMikrotikBareAdd(t *testing.T) {
	// without the /ip header line
	input := `add list=x address=192.168.1.0/24 comment="LAN"
add list=x address=10.0.0.1`
	entries, err := parser.ParseMikrotik(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2, got %d", len(entries))
	}
	if entries[0].Address != "192.168.1.0/24" || entries[0].Comment != "LAN" {
		t.Errorf("entry 0: %+v", entries[0])
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"# comment\n8.8.8.8\n", "native"},
		{"/ip firewall address-list\nadd list=x address=1.1.1.1\n", "mikrotik"},
		{"add list=x address=1.1.1.1\n", "mikrotik"},
		{"", "native"},
		{"# only comments\n# nothing else\n", "native"},
	}
	for _, c := range cases {
		got := parser.DetectFormat(c.input)
		if got != c.expect {
			t.Errorf("DetectFormat(%q) = %q, want %q", c.input, got, c.expect)
		}
	}
}
