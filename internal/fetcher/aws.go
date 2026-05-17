package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type awsIPRanges struct {
	Prefixes []struct {
		IPPrefix string `json:"ip_prefix"`
	} `json:"prefixes"`
}

func fetchAWS(client *http.Client) ([]string, error) {
	body, err := get(client, "https://ip-ranges.amazonaws.com/ip-ranges.json")
	if err != nil {
		return nil, err
	}
	var data awsIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	seen := make(map[string]bool)
	var cidrs []string
	for _, p := range data.Prefixes {
		if isIPv4CIDR(p.IPPrefix) && !seen[p.IPPrefix] {
			seen[p.IPPrefix] = true
			cidrs = append(cidrs, p.IPPrefix)
		}
	}
	return cidrs, nil
}
