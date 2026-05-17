package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type fastlyIPList struct {
	Addresses     []string `json:"addresses"`
	IPv6Addresses []string `json:"ipv6_addresses"`
}

func fetchFastly(client *http.Client) ([]string, error) {
	body, err := get(client, "https://api.fastly.com/public-ip-list")
	if err != nil {
		return nil, err
	}
	var data fastlyIPList
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	var cidrs []string
	for _, addr := range data.Addresses {
		if isIPv4CIDR(addr) {
			cidrs = append(cidrs, addr)
		}
	}
	return cidrs, nil
}
