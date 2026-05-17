package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var oracleProvider = Provider{
	Name:             "Oracle Cloud",
	Slug:             "oracle",
	LoadSubProviders: loadOracleRegions,
}

// OracleProvider returns the Oracle Cloud provider entry.
func OracleProvider() Provider { return oracleProvider }

// MakeOracleRegionProvider creates a Provider for a specific Oracle Cloud region.
// Used when the region slug is specified directly via --provider oracle/region-name.
func MakeOracleRegionProvider(region string) Provider {
	return Provider{
		Name:  "Oracle / " + region,
		Slug:  "oracle/" + region,
		Fetch: makeOracleFetch(region),
	}
}

type oracleIPRanges struct {
	Regions []struct {
		Region string `json:"region"`
		CIDRs  []struct {
			CIDR string   `json:"cidr"`
			Tags []string `json:"tags"`
		} `json:"cidrs"`
	} `json:"regions"`
}

func loadOracleRegions(client *http.Client) ([]Provider, error) {
	body, err := get(client, "https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json")
	if err != nil {
		return nil, err
	}
	var data oracleIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	providers := make([]Provider, 0, len(data.Regions))
	for _, r := range data.Regions {
		region := r.Region
		providers = append(providers, Provider{
			Name:  "Oracle / " + region,
			Slug:  "oracle/" + region,
			Fetch: makeOracleFetch(region),
		})
	}
	return providers, nil
}

func makeOracleFetch(region string) func(*http.Client) ([]string, error) {
	return func(client *http.Client) ([]string, error) {
		body, err := get(client, "https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json")
		if err != nil {
			return nil, err
		}
		var data oracleIPRanges
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		var cidrs []string
		for _, r := range data.Regions {
			if r.Region != region {
				continue
			}
			for _, c := range r.CIDRs {
				if isIPv4CIDR(c.CIDR) {
					cidrs = append(cidrs, c.CIDR)
				}
			}
		}
		return cidrs, nil
	}
}
