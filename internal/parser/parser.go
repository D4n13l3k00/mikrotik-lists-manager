// Package parser handles two input formats:
//
// 1. Native format (.list):
//
//	# human-only comment (not sent to MikroTik)
//	## mikrotik comment for the NEXT entry
//	192.168.1.0/24
//	10.0.0.1        ## inline mikrotik comment
//	8.8.8.8         # inline human-only note
//
// 2. MikroTik export format:
//
//	/ip firewall address-list
//	add address=1.2.3.4 list=mylist comment="some comment"
package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	reAddCmd    = regexp.MustCompile(`(?i)^add\s+`)
	reAddress   = regexp.MustCompile(`(?i)\baddress=(\S+)`)
	reComment   = regexp.MustCompile(`(?i)\bcomment="([^"]*)"`)
	reCommentNQ = regexp.MustCompile(`(?i)\bcomment=(\S+)`)
)

// ParseNative parses the native .list format.
func ParseNative(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	var pendingMTComment string

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "##") {
			pendingMTComment = strings.TrimSpace(trimmed[2:])
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// ! prefix = disabled entry
		disabled := false
		if strings.HasPrefix(trimmed, "!") {
			disabled = true
			trimmed = strings.TrimSpace(trimmed[1:])
		}

		address, mtComment, humanNote := ParseDataLine(trimmed)
		if address == "" {
			continue
		}

		// Block ## comment takes precedence over inline ## comment
		if pendingMTComment != "" {
			mtComment = pendingMTComment
			pendingMTComment = ""
		}

		entries = append(entries, Entry{
			Address:   address,
			Comment:   mtComment,
			HumanNote: humanNote,
			Disabled:  disabled,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return entries, nil
}

// ParseDataLine splits "address [## mtcomment] [# humannote]" into parts.
func ParseDataLine(line string) (address, mtComment, humanNote string) {
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

// ParseMikrotik parses MikroTik export format.
func ParseMikrotik(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "/") {
			continue
		}
		if !reAddCmd.MatchString(line) {
			continue
		}

		addrMatch := reAddress.FindStringSubmatch(line)
		if addrMatch == nil {
			continue
		}

		var comment string
		if m := reComment.FindStringSubmatch(line); m != nil {
			comment = m[1]
		} else if m := reCommentNQ.FindStringSubmatch(line); m != nil {
			comment = m[1]
		}

		entries = append(entries, Entry{
			Address: addrMatch[1],
			Comment: comment,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return entries, nil
}

// DetectFormat returns "mikrotik" or "native" by inspecting the first meaningful line.
func DetectFormat(content string) string {
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "/ip") || strings.HasPrefix(t, "/ipv6") || reAddCmd.MatchString(t) {
			return "mikrotik"
		}
		return "native"
	}
	return "native"
}
