package optimizer_test

import (
	"strings"
	"testing"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/optimizer"
)

func TestOptimizeDedupe(t *testing.T) {
	input := `8.8.8.8  ## DNS
1.1.1.1
8.8.8.8
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
	if result.Removed[0].Address != "8.8.8.8" {
		t.Errorf("wrong removed address: %q", result.Removed[0].Address)
	}
	if result.Removed[0].Reason != "дубль" {
		t.Errorf("wrong reason: %q", result.Removed[0].Reason)
	}
}

func TestOptimizeDedupePromotesComment(t *testing.T) {
	// first occurrence has no comment, duplicate has one — should be promoted
	input := `8.8.8.8
8.8.8.8  ## DNS
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
	// find the kept entry
	var kept *optimizer.Line
	for i := range result.Lines {
		if !result.Lines[i].IsComment && result.Lines[i].Address == "8.8.8.8" {
			kept = &result.Lines[i]
			break
		}
	}
	if kept == nil {
		t.Fatal("kept entry not found")
	}
	if kept.Comment != "DNS" {
		t.Errorf("comment not promoted: got %q", kept.Comment)
	}
}

func TestOptimizeDedupeCaseInsensitive(t *testing.T) {
	input := `example.com
EXAMPLE.COM
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
}

func TestOptimizeSubnetCoverage(t *testing.T) {
	// /23 is covered by /21
	input := `160.79.104.0/21  ## WIDE
160.79.104.0/23  ## NARROW
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d: %+v", len(result.Removed), result.Removed)
	}
	if result.Removed[0].Address != "160.79.104.0/23" {
		t.Errorf("wrong subnet removed: %q", result.Removed[0].Address)
	}
	if !strings.Contains(result.Removed[0].Reason, "160.79.104.0/21") {
		t.Errorf("reason should mention covering subnet: %q", result.Removed[0].Reason)
	}
}

func TestOptimizeHostCoveredBySubnet(t *testing.T) {
	input := `10.0.0.0/8
10.1.2.3
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
	if result.Removed[0].Address != "10.1.2.3" {
		t.Errorf("wrong entry removed: %q", result.Removed[0].Address)
	}
}

func TestOptimizeDomainsNotSummarized(t *testing.T) {
	// domains should never be removed by subnet logic
	input := `example.com
sub.example.com
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Errorf("domains should not be removed by subnet logic, got: %+v", result.Removed)
	}
}

func TestOptimizeNoChanges(t *testing.T) {
	input := `8.8.8.8  ## DNS
1.1.1.1  ## CF
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected no removals, got %d", len(result.Removed))
	}
}

func TestOptimizePreservesComments(t *testing.T) {
	input := `# human comment
## MIKROTIK COMMENT
8.8.8.8
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	rendered := optimizer.Render(result.Lines)
	if !strings.Contains(rendered, "# human comment") {
		t.Error("human comment lost")
	}
	if !strings.Contains(rendered, "## MIKROTIK COMMENT") {
		t.Error("mikrotik comment lost")
	}
}

func TestRender(t *testing.T) {
	input := `8.8.8.8  ## DNS
1.1.1.1
`
	result, err := optimizer.Optimize(input)
	if err != nil {
		t.Fatal(err)
	}
	rendered := optimizer.Render(result.Lines)
	if !strings.Contains(rendered, "8.8.8.8") {
		t.Error("address missing from render")
	}
	if !strings.Contains(rendered, "## DNS") {
		t.Error("comment missing from render")
	}
	if !strings.Contains(rendered, "1.1.1.1") {
		t.Error("second address missing from render")
	}
}
