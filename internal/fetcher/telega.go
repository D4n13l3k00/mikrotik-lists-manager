package fetcher

import "net/http"

var telegaProvider = Provider{
	Name: "Telega (VK)",
	Slug: "telega",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS203502 — Telega (VK)
		return fetchRIPEPrefixes(c, "AS203502")
	},
}
