package fetcher

import "net/http"

var steamProvider = Provider{
	Name: "Steam / Valve",
	Slug: "steam",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS32590 — Valve Corporation
		return fetchRIPEPrefixes(c, "AS32590")
	},
}
