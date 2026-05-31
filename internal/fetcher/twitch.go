package fetcher

import "net/http"

var twitchProvider = Provider{
	Name: "Twitch",
	Slug: "twitch",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS46489 — Twitch Interactive
		return fetchRIPEPrefixes(c, "AS46489")
	},
}
