package cli

import (
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

func TestMatchAddress(t *testing.T) {
	tests := []struct {
		needle    string
		entry     string
		wantMatch bool
		wantType  output.MatchType
	}{
		// Exact matches
		{"8.8.8.8", "8.8.8.8", true, output.MatchExact},
		{"8.8.8.8", "8.8.8.8/32", true, output.MatchExact},
		{"8.8.8.8/32", "8.8.8.8", true, output.MatchExact},
		{"example.com", "example.com", true, output.MatchExact},
		{"EXAMPLE.COM", "example.com", true, output.MatchExact},

		// Subnet containment: IP in CIDR
		{"8.8.8.8", "8.8.8.0/24", true, output.MatchSubnet},
		{"10.0.5.10", "10.0.0.0/16", true, output.MatchSubnet},

		// Subnet containment: CIDR in CIDR (needle is smaller)
		{"10.0.1.0/24", "10.0.0.0/16", true, output.MatchSubnet},

		// Subnet containment: CIDR in CIDR (needle is larger)
		{"10.0.0.0/16", "10.0.1.0/24", true, output.MatchSubnet},

		// Non-matches
		{"8.8.8.8", "8.8.4.4", false, ""},
		{"10.1.0.0/16", "10.0.0.0/16", false, ""},
		{"example.com", "other.com", false, ""},
		{"8.8.8.8", "example.com", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.needle+"_vs_"+tt.entry, func(t *testing.T) {
			gotMatch, gotType := matchAddress(tt.needle, tt.entry)
			if gotMatch != tt.wantMatch {
				t.Errorf("matchAddress(%q, %q) gotMatch = %v, want %v", tt.needle, tt.entry, gotMatch, tt.wantMatch)
			}
			if gotType != tt.wantType {
				t.Errorf("matchAddress(%q, %q) gotType = %q, want %q", tt.needle, tt.entry, gotType, tt.wantType)
			}
		})
	}
}
