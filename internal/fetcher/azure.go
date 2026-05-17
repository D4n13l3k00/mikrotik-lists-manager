package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

const azureDownloadPageURL = "https://www.microsoft.com/en-us/download/confirmation.aspx?id=56519"

var azureJSONURLRe = regexp.MustCompile(`https://download\.microsoft\.com/download/[^"'\s]+ServiceTags_Public_\d+\.json`)

type azureIPRanges struct {
	Values []struct {
		Properties struct {
			AddressPrefixes []string `json:"addressPrefixes"`
		} `json:"properties"`
	} `json:"values"`
}

func fetchAzure(client *http.Client) ([]string, error) {
	pageBody, err := get(client, azureDownloadPageURL)
	if err != nil {
		return nil, fmt.Errorf("fetch download page: %w", err)
	}
	jsonURL := azureJSONURLRe.FindString(string(pageBody))
	if jsonURL == "" {
		return nil, fmt.Errorf("не удалось найти URL JSON на странице Microsoft Download")
	}
	body, err := get(client, jsonURL)
	if err != nil {
		return nil, fmt.Errorf("fetch JSON: %w", err)
	}
	var data azureIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	seen := make(map[string]bool)
	var cidrs []string
	for _, v := range data.Values {
		for _, prefix := range v.Properties.AddressPrefixes {
			if isIPv4CIDR(prefix) && !seen[prefix] {
				seen[prefix] = true
				cidrs = append(cidrs, prefix)
			}
		}
	}
	return cidrs, nil
}
