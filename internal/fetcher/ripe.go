package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ripeStatResponse struct {
	Data struct {
		Prefixes []struct {
			Prefix string `json:"prefix"`
		} `json:"prefixes"`
	} `json:"data"`
}

// fetchRIPEPrefixes queries RIPE STAT announced-prefixes for the given ASNs
// and returns deduplicated IPv4 CIDRs.
func fetchRIPEPrefixes(client *http.Client, asns ...string) ([]string, error) {
	seen := make(map[string]struct{})
	var cidrs []string
	for _, asn := range asns {
		body, err := get(client, "https://stat.ripe.net/data/announced-prefixes/data.json?resource="+asn)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", asn, err)
		}
		var resp ripeStatResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("%s: parse JSON: %w", asn, err)
		}
		for _, p := range resp.Data.Prefixes {
			if !isIPv4CIDR(p.Prefix) {
				continue
			}
			if _, dup := seen[p.Prefix]; !dup {
				seen[p.Prefix] = struct{}{}
				cidrs = append(cidrs, p.Prefix)
			}
		}
	}
	return cidrs, nil
}
