package fetcher

import "net/http"

var vkProvider = Provider{
	Name: "VK",
	Slug: "vk",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS47541 — VK, AS44507 — VK-SPB
		return fetchRIPEPrefixes(c, "AS47541", "AS44507")
	},
}
