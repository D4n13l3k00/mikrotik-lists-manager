package parser_test

import (
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"8.8.8.8", "8.8.8.8/32"},
		{"8.8.8.8/32", "8.8.8.8/32"},
		{" 8.8.8.8  ", "8.8.8.8/32"},
		{"192.168.1.5/24", "192.168.1.0/24"},
		{"192.168.1.0/24", "192.168.1.0/24"},
		{"2001:db8::1", "2001:db8::1/128"},
		{"2001:0db8::0001", "2001:db8::1/128"},
		{"2001:db8::1/128", "2001:db8::1/128"},
		{"2001:db8::/32", "2001:db8::/32"},
		{"EXAMPLE.COM", "example.com"},
		{"  api.github.com ", "api.github.com"},
		{"invalid/cidr/extra", "invalid/cidr/extra"},
	}

	for _, tt := range tests {
		got := parser.NormalizeAddr(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeAddr(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
