package fetcher

import "net/http"

var digitalOceanProvider = Provider{
	Name: "DigitalOcean",
	Slug: "digitalocean",
	Fetch: func(c *http.Client) ([]string, error) {
		return fetchRIPEPrefixes(c, "AS14061")
	},
}
