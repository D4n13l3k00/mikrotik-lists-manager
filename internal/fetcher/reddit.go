package fetcher

import "net/http"

var redditProvider = Provider{
	Name: "Reddit",
	Slug: "reddit",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS54009, AS22616 — Reddit Inc.
		return fetchRIPEPrefixes(c, "AS54009", "AS22616")
	},
}
