package fetcher

import "net/http"

var tiktokProvider = Provider{
	Name: "TikTok / ByteDance",
	Slug: "tiktok",
	Fetch: func(c *http.Client) ([]string, error) {
		// AS396986, AS138699 — ByteDance Ltd (US + Asia)
		return fetchRIPEPrefixes(c, "AS396986", "AS138699")
	},
}
