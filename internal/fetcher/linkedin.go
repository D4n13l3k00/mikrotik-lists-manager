package fetcher

import "net/http"

var linkedinProvider = Provider{
	Name: "LinkedIn",
	Slug: "linkedin",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS14413 — LinkedIn Corporation
		return fetchRIPEPrefixes(c, "AS14413")
	},
}
