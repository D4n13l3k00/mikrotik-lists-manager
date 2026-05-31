package fetcher

import "net/http"

var discordProvider = Provider{
	Name: "Discord",
	Slug: "discord",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS36459 — Discord Inc. (собственные диапазоны; CDN через Cloudflare)
		return fetchRIPEPrefixes(c, "AS36459")
	},
}
