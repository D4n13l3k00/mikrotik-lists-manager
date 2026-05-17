package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type googleIPRanges struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix"`
	} `json:"prefixes"`
}

func fetchGoogle(client *http.Client) ([]string, error) {
	googCIDRs, err1 := fetchGoogleJSON(client, "https://www.gstatic.com/ipranges/goog.json")
	cloudCIDRs, err2 := fetchGoogleJSON(client, "https://www.gstatic.com/ipranges/cloud.json")

	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("goog.json: %v; cloud.json: %v", err1, err2)
	}

	seen := make(map[string]bool)
	var cidrs []string
	for _, c := range append(googCIDRs, cloudCIDRs...) {
		if !seen[c] {
			seen[c] = true
			cidrs = append(cidrs, c)
		}
	}
	return cidrs, nil
}

func fetchGoogleJSON(client *http.Client, url string) ([]string, error) {
	body, err := get(client, url)
	if err != nil {
		return nil, err
	}
	var data googleIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	var cidrs []string
	for _, p := range data.Prefixes {
		if isIPv4CIDR(p.IPv4Prefix) {
			cidrs = append(cidrs, p.IPv4Prefix)
		}
	}
	return cidrs, nil
}
