package fetcher

import "net/http"

var pornhubProvider = Provider{
	Name: "Pornhub / MindGeek",
	Slug: "pornhub",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS55222 — MindGeek (Pornhub), AS29789 — Reflected Networks (MindGeek CDN)
		return fetchRIPEPrefixes(c, "AS55222", "AS29789")
	},
}
