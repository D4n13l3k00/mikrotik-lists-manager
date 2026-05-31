package fetcher

import "net/http"

var blizzardProvider = Provider{
	Name: "Blizzard",
	Slug: "blizzard",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS57976 — Blizzard Entertainment, AS209242 — Blizzard EU
		return fetchRIPEPrefixes(c, "AS57976", "AS209242")
	},
}
