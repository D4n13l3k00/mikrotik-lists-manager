// Package optimizer deduplicates and summarizes address-list entries.
// Works only on native format — preserves section comments and human notes.
package optimizer

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// Line represents one parsed line from a native .list file, preserving
// the original text so we can reconstruct the file faithfully.
type Line struct {
	Raw       string // original text
	IsComment bool   // # or ## block comment, or empty
	Address   string // parsed address/CIDR (empty for comment lines)
	Comment   string // ## mikrotik comment
	HumanNote string // # human note
	IsIP      bool   // true = IP/CIDR, false = domain
	Network   *net.IPNet
}

// Result holds the optimized entries and a log of what was removed.
type Result struct {
	Lines      []Line
	Removed    []RemovedEntry
	Normalized []string // addresses converted from /32 to bare IP
}

type RemovedEntry struct {
	Address string
	Reason  string
}

// Optimize reads a native .list file and returns deduplicated,
// subnet-summarized lines with the original structure preserved.
func Optimize(content string) (Result, error) {
	lines, err := parseLines(content)
	if err != nil {
		return Result{}, err
	}

	lines, removed := dedupe(lines)
	lines, normalized := normalizeHostRoutes(lines)
	lines, summarized := summarize(lines)
	removed = append(removed, summarized...)

	return Result{Lines: lines, Removed: removed, Normalized: normalized}, nil
}

// Render serializes optimized lines back to a .list string.
func Render(lines []Line) string {
	var sb strings.Builder
	for _, l := range lines {
		if l.IsComment {
			sb.WriteString(l.Raw + "\n")
			continue
		}
		addr := l.Address
		// pad address to align comments (best-effort, 24 chars)
		padded := fmt.Sprintf("%-24s", addr)
		if l.Comment != "" {
			sb.WriteString(padded + " ## " + l.Comment + "\n")
		} else {
			sb.WriteString(strings.TrimRight(padded, " ") + "\n")
		}
	}
	return sb.String()
}

// ── internal ─────────────────────────────────────────────────────────────────

func parseLines(content string) ([]Line, error) {
	var lines []Line
	scanner := bufio.NewScanner(strings.NewReader(content))
	var pendingComment string

	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			lines = append(lines, Line{Raw: raw, IsComment: true})
			continue
		}
		if strings.HasPrefix(trimmed, "##") {
			pendingComment = strings.TrimSpace(trimmed[2:])
			lines = append(lines, Line{Raw: raw, IsComment: true})
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			lines = append(lines, Line{Raw: raw, IsComment: true})
			continue
		}

		addr, mtComment, humanNote := parseDataLine(trimmed)
		if addr == "" {
			lines = append(lines, Line{Raw: raw, IsComment: true})
			continue
		}
		if pendingComment != "" {
			mtComment = pendingComment
			pendingComment = ""
		}

		l := Line{
			Raw:       raw,
			Address:   addr,
			Comment:   mtComment,
			HumanNote: humanNote,
		}

		// try to parse as IP/CIDR
		ip := net.ParseIP(addr)
		_, network, err := net.ParseCIDR(addr)
		if ip != nil {
			// host address — wrap as /32 or /128
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			network = &net.IPNet{IP: ip.Mask(net.CIDRMask(bits, bits)), Mask: net.CIDRMask(bits, bits)}
			l.IsIP = true
			l.Network = network
		} else if err == nil {
			l.IsIP = true
			l.Network = network
		}

		lines = append(lines, l)
	}
	return lines, scanner.Err()
}

func parseDataLine(line string) (address, mtComment, humanNote string) {
	if idx := strings.Index(line, "##"); idx != -1 {
		address = strings.TrimSpace(line[:idx])
		rest := strings.TrimSpace(line[idx+2:])
		if hi := strings.Index(rest, "#"); hi != -1 {
			mtComment = strings.TrimSpace(rest[:hi])
			humanNote = strings.TrimSpace(rest[hi+1:])
		} else {
			mtComment = rest
		}
		return
	}
	if idx := strings.Index(line, "#"); idx != -1 {
		address = strings.TrimSpace(line[:idx])
		humanNote = strings.TrimSpace(line[idx+1:])
		return
	}
	address = line
	return
}

// normalizeHostRoutes converts "x.x.x.x/32" to "x.x.x.x" in-place.
// Returns the modified slice and a list of addresses that were changed.
func normalizeHostRoutes(lines []Line) ([]Line, []string) {
	var normalized []string
	for i, l := range lines {
		if l.IsComment || !l.IsIP || l.Network == nil {
			continue
		}
		ones, bits := l.Network.Mask.Size()
		if ones == 32 && bits == 32 && strings.Contains(l.Address, "/") {
			normalized = append(normalized, l.Address)
			lines[i].Address = l.Network.IP.String()
		}
	}
	return lines, normalized
}

// dedupe removes exact duplicate addresses (case-insensitive for domains).
// Keeps the first occurrence; if a later duplicate has a comment and the
// first doesn't, the comment is promoted to the first.
func dedupe(lines []Line) ([]Line, []RemovedEntry) {
	seen := make(map[string]int) // address → index in result
	var result []Line
	var removed []RemovedEntry

	for _, l := range lines {
		if l.IsComment {
			result = append(result, l)
			continue
		}
		key := strings.ToLower(l.Address)
		if idx, exists := seen[key]; exists {
			// promote comment if first had none
			if result[idx].Comment == "" && l.Comment != "" {
				result[idx].Comment = l.Comment
			}
			removed = append(removed, RemovedEntry{Address: l.Address, Reason: "дубль"})
			continue
		}
		seen[key] = len(result)
		result = append(result, l)
	}
	return result, removed
}

// summarize removes IP entries that are fully covered by a broader subnet
// in the same list, and collapses adjacent same-prefix subnets.
func summarize(lines []Line) ([]Line, []RemovedEntry) {
	// collect all networks with their line indices
	type netRef struct {
		idx int
		net *net.IPNet
	}
	var nets []netRef
	for i, l := range lines {
		if !l.IsComment && l.IsIP && l.Network != nil {
			nets = append(nets, netRef{i, l.Network})
		}
	}

	// mark lines that are covered by a broader network
	covered := make(map[int]int) // covered idx → covering idx
	for i := 0; i < len(nets); i++ {
		for j := 0; j < len(nets); j++ {
			if i == j {
				continue
			}
			iOnes, _ := nets[i].net.Mask.Size()
			jOnes, _ := nets[j].net.Mask.Size()
			// j covers i if j is broader (smaller prefix) and i is subnet of j
			if jOnes < iOnes && nets[j].net.Contains(nets[i].net.IP) {
				covered[nets[i].idx] = nets[j].idx
				break
			}
		}
	}

	var result []Line
	var removed []RemovedEntry

	for i, l := range lines {
		if l.IsComment {
			result = append(result, l)
			continue
		}
		if coveringIdx, isCovered := covered[i]; isCovered {
			// promote comment to covering line if it has none
			if lines[i].Comment != "" && lines[coveringIdx].Comment == "" {
				// find covering line in result and update
				for ri := range result {
					if result[ri].Address == lines[coveringIdx].Address {
						result[ri].Comment = lines[i].Comment
						break
					}
				}
			}
			removed = append(removed, RemovedEntry{
				Address: l.Address,
				Reason:  fmt.Sprintf("покрывается %s", lines[coveringIdx].Address),
			})
			continue
		}
		result = append(result, l)
	}

	return result, removed
}
