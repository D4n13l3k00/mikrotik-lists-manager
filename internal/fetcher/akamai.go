package fetcher

import "net/http"

var akamaiProvider = Provider{
	Name: "Akamai",
	Slug: "akamai",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS20940 — Akamai International BV
		// AS16625 — Akamai Technologies, Inc.
		return fetchRIPEPrefixes(c, "AS20940", "AS16625")
	},
}
