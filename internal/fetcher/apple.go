package fetcher

import "net/http"

var appleProvider = Provider{
	Name: "Apple",
	Slug: "apple",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS714 — Apple Inc., AS6185 — Apple Services
		return fetchRIPEPrefixes(c, "AS714", "AS6185")
	},
}
