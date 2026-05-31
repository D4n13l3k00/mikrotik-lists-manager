package fetcher

import "net/http"

var twitterProvider = Provider{
	Name: "Twitter / X",
	Slug: "twitter",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS13414, AS35995 — Twitter / X Corp
		return fetchRIPEPrefixes(c, "AS13414", "AS35995")
	},
}
