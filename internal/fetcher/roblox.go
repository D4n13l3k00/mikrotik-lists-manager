package fetcher

import "net/http"

var robloxProvider = Provider{
	Name: "Roblox",
	Slug: "roblox",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS22697 — Roblox Corporation
		return fetchRIPEPrefixes(c, "AS22697")
	},
}
