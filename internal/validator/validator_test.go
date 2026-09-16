package validator_test

import (
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/validator"
)

func TestValidate_Valid(t *testing.T) {
	entries := []parser.Entry{
		{Address: "1.1.1.1"},
		{Address: "8.8.8.0/24"},
		{Address: "2001:db8::1"},
		{Address: "example.com"},
	}

	rep := validator.Validate(entries)
	if !rep.Passed {
		t.Errorf("expected valid report to pass, got errors: %d", rep.Errors)
	}
	if rep.Stats.Total != 4 || rep.Stats.Valid != 4 {
		t.Errorf("unexpected stats: %+v", rep.Stats)
	}
	if rep.Errors != 0 || rep.Warnings != 0 {
		t.Errorf("expected 0 errors and warnings, got errors=%d, warnings=%d", rep.Errors, rep.Warnings)
	}
}

func TestValidate_InvalidAndWarnings(t *testing.T) {
	entries := []parser.Entry{
		{Address: "999.999.999.999"},      // Error: invalid IP
		{Address: "192.168.1.5/24"},       // Warning: non-canonical CIDR host bits
		{Address: "172.16.0.1"},
		{Address: "172.16.0.1"},           // Warning: duplicate
		{Address: "10.0.0.0/16"},
		{Address: "10.0.1.0/24"},          // Info: shadowed by 10.0.0.0/16
	}

	rep := validator.Validate(entries)
	if rep.Passed {
		t.Errorf("expected report with invalid IP to fail")
	}
	if rep.Errors != 1 {
		t.Errorf("expected 1 error, got %d", rep.Errors)
	}
	if rep.Warnings != 2 { // 1 non-canonical, 1 duplicate
		t.Errorf("expected 2 warnings, got %d", rep.Warnings)
	}
	if rep.Infos != 1 { // 1 shadowed
		t.Errorf("expected 1 info, got %d", rep.Infos)
	}
}
