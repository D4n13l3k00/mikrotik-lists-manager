package fetcher

import "net/http"

var eaProvider = Provider{
	Name: "EA / Electronic Arts",
	Slug: "ea",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS12128, AS14068 — Electronic Arts
		return fetchRIPEPrefixes(c, "AS12128", "AS14068")
	},
}
