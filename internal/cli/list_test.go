package cli

import (
	"sort"
	"testing"
)

func TestCompareAddresses(t *testing.T) {
	input := []string{
		"10.0.0.10",
		"10.0.0.2",
		"10.0.0.0/16",
		"10.0.0.0/24",
		"192.168.1.1",
		"2001:db8::1",
		"example.com",
		"apple.com",
	}

	expected := []string{
		"10.0.0.0/16",
		"10.0.0.0/24",
		"10.0.0.2",
		"10.0.0.10",
		"192.168.1.1",
		"2001:db8::1",
		"apple.com",
		"example.com",
	}

	sort.Slice(input, func(i, j int) bool {
		return compareAddresses(input[i], input[j])
	})

	for i, v := range input {
		if v != expected[i] {
			t.Errorf("at index %d: got %s, want %s", i, v, expected[i])
		}
	}
}
