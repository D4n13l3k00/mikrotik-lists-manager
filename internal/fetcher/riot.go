package fetcher

import "net/http"

var riotProvider = Provider{
	Name: "Riot Games",
	Slug: "riot",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS6507, AS26008 — Riot Games
		return fetchRIPEPrefixes(c, "AS6507", "AS26008")
	},
}
