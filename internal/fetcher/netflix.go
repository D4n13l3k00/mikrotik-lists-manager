package fetcher

import "net/http"

var netflixProvider = Provider{
	Name: "Netflix",
	Slug: "netflix",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS2906 — Netflix Streaming Services
		return fetchRIPEPrefixes(c, "AS2906")
	},
}
