package fetcher

import "net/http"

var mailruProvider = Provider{
	Name: "Mail.ru",
	Slug: "mailru",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS47764 — Mail.ru Group, AS57620 — VK-Company
		return fetchRIPEPrefixes(c, "AS47764", "AS57620")
	},
}
