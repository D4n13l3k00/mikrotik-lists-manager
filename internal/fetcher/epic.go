package fetcher

import "net/http"

var epicProvider = Provider{
	Name: "Epic Games",
	Slug: "epic",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS46562 — Epic Games
		return fetchRIPEPrefixes(c, "AS46562")
	},
}
