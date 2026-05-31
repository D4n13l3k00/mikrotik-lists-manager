package fetcher

import "net/http"

var ovhProvider = Provider{
	Name: "OVHcloud",
	Slug: "ovh",
	Fetch: func(c *http.Client) ([]string, error) {
		return fetchRIPEPrefixes(c, "AS16276")
	},
}
