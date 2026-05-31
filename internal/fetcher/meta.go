package fetcher

import "net/http"

var metaProvider = Provider{
	Name: "Meta (Facebook/Instagram/WhatsApp)",
	Slug: "meta",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS32934, AS63293, AS54115 — Meta Platforms
		return fetchRIPEPrefixes(c, "AS32934", "AS63293", "AS54115")
	},
}
