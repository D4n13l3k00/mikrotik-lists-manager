package validator

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

type Issue struct {
	Index    int      `json:"index"`
	Address  string   `json:"address"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type Stats struct {
	Total       int `json:"total"`
	Valid       int `json:"valid"`
	HostsIPv4   int `json:"hosts_ipv4"`
	SubnetsIPv4 int `json:"subnets_ipv4"`
	IPv6        int `json:"ipv6"`
	Domains     int `json:"domains"`
	Disabled    int `json:"disabled"`
	Duplicates  int `json:"duplicates"`
	Shadowed    int `json:"shadowed"`
}

type Report struct {
	Passed   bool    `json:"passed"`
	Stats    Stats   `json:"stats"`
	Issues   []Issue `json:"issues,omitempty"`
	Errors   int     `json:"errors_count"`
	Warnings int     `json:"warnings_count"`
	Infos    int     `json:"infos_count"`
}

var reDomain = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Validate checks a slice of parsed entries against formatting, subnet, and duplication rules.
func Validate(entries []parser.Entry) *Report {
	rep := &Report{
		Passed: true,
		Stats: Stats{
			Total: len(entries),
		},
	}

	seenAddrs := make(map[string]int) // normalized -> first index

	type prefixEntry struct {
		index  int
		prefix netip.Prefix
		raw    string
	}
	var prefixes []prefixEntry

	for i, e := range entries {
		raw := strings.TrimSpace(e.Address)
		if e.Disabled {
			rep.Stats.Disabled++
		}

		if raw == "" {
			rep.addIssue(i, raw, SeverityError, "пустой адрес")
			continue
		}

		// 1. Syntax check: Addr, Prefix, or Domain
		addr, errAddr := netip.ParseAddr(raw)
		prefix, errPrefix := netip.ParsePrefix(raw)
		isDomain := reDomain.MatchString(raw)

		if errAddr != nil && errPrefix != nil && !isDomain {
			rep.addIssue(i, raw, SeverityError, fmt.Sprintf("некорректный IP, CIDR или домен: %q", raw))
			continue
		}

		rep.Stats.Valid++

		// Categorize type
		if errAddr == nil {
			if addr.Is4() {
				rep.Stats.HostsIPv4++
				prefixes = append(prefixes, prefixEntry{
					index:  i,
					prefix: netip.PrefixFrom(addr, 32),
					raw:    raw,
				})
			} else {
				rep.Stats.IPv6++
				prefixes = append(prefixes, prefixEntry{
					index:  i,
					prefix: netip.PrefixFrom(addr, 128),
					raw:    raw,
				})
			}
		} else if errPrefix == nil {
			if prefix.Addr().Is4() {
				if prefix.Bits() == 32 {
					rep.Stats.HostsIPv4++
				} else {
					rep.Stats.SubnetsIPv4++
				}
			} else {
				rep.Stats.IPv6++
			}

			// Check canonical host bits
			masked := prefix.Masked()
			if masked.Addr() != prefix.Addr() {
				rep.addIssue(i, raw, SeverityWarning, fmt.Sprintf("неканонический CIDR: ненулевые биты хоста, рекомендуется %s", masked))
			}
			prefixes = append(prefixes, prefixEntry{
				index:  i,
				prefix: masked,
				raw:    raw,
			})
		} else if isDomain {
			rep.Stats.Domains++
		}

		// 2. Duplicate check
		norm := parser.NormalizeAddr(raw)
		if prevIdx, found := seenAddrs[norm]; found {
			rep.Stats.Duplicates++
			rep.addIssue(i, raw, SeverityWarning, fmt.Sprintf("дубликат адреса (уже встречался на позиции %d)", prevIdx+1))
		} else {
			seenAddrs[norm] = i
		}
	}

	// 3. Subnet overlap (shadowing) check
	for i := 0; i < len(prefixes); i++ {
		for j := 0; j < len(prefixes); j++ {
			if i == j {
				continue
			}
			pi := prefixes[i]
			pj := prefixes[j]

			// If pj is strictly larger than pi and pj contains pi
			if pj.prefix.Bits() < pi.prefix.Bits() && pj.prefix.Contains(pi.prefix.Addr()) {
				rep.Stats.Shadowed++
				rep.addIssue(pi.index, pi.raw, SeverityInfo, fmt.Sprintf("адрес поглощается более широкой подсетью %s (строка %d)", pj.raw, pj.index+1))
				break // only report the first covering subnet
			}
		}
	}

	rep.Passed = rep.Errors == 0
	return rep
}

func (r *Report) addIssue(index int, addr string, sev Severity, msg string) {
	r.Issues = append(r.Issues, Issue{
		Index:    index + 1,
		Address:  addr,
		Severity: sev,
		Message:  msg,
	})
	switch sev {
	case SeverityError:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	case SeverityInfo:
		r.Infos++
	}
}
