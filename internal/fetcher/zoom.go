package fetcher

import "net/http"

var zoomProvider = Provider{
	Name: "Zoom",
	Slug: "zoom",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS8100, AS21929 — Zoom Video Communications
		return fetchRIPEPrefixes(c, "AS8100", "AS21929")
	},
}
