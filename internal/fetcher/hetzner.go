package fetcher

import "net/http"

var hetznerProvider = Provider{
	Name: "Hetzner",
	Slug: "hetzner",
	Fetch: func(c *http.Client) ([]string, error) {
		return fetchRIPEPrefixes(c, "AS24940")
	},
}
