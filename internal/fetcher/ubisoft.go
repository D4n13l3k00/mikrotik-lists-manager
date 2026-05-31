package fetcher

import "net/http"

var ubisoftProvider = Provider{
	Name: "Ubisoft",
	Slug: "ubisoft",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS39561 — Ubisoft Entertainment
		return fetchRIPEPrefixes(c, "AS39561")
	},
}
