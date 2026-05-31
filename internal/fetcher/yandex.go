package fetcher

import "net/http"

var yandexProvider = Provider{
	Name: "Yandex",
	Slug: "yandex",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS13238 — Yandex
		return fetchRIPEPrefixes(c, "AS13238")
	},
}
