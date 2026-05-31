package fetcher

import (
	"net/http"
	"strings"
	"time"
)

// Provider describes a single IP-range source.
type Provider struct {
	Name             string
	Slug             string
	Fetch            func(client *http.Client) ([]string, error)
	SubProviders     []Provider                                    // static sub-providers (e.g. GitHub services)
	LoadSubProviders func(client *http.Client) ([]Provider, error) // dynamic sub-providers (e.g. Oracle regions)
}

// HasSubs returns true if the provider has sub-providers (static or dynamic).
func (p Provider) HasSubs() bool {
	return len(p.SubProviders) > 0 || p.LoadSubProviders != nil
}

// HTTPClient is an alias for http.Client used in function signatures.
type HTTPClient = http.Client

// NewClient returns an http.Client with the given timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// All is the ordered list of supported providers.
var All = []Provider{
	{Name: "Cloudflare",  Slug: "cloudflare",  Fetch: fetchCloudflare},
	{Name: "Google",      Slug: "google",      Fetch: fetchGoogle},
	{Name: "AWS",         Slug: "aws",         Fetch: fetchAWS},
	{Name: "Azure",       Slug: "azure",       Fetch: fetchAzure},
	{Name: "Fastly",      Slug: "fastly",      Fetch: fetchFastly},
	akamaiProvider,
	digitalOceanProvider,
	hetznerProvider,
	ovhProvider,
	metaProvider,
	twitterProvider,
	tiktokProvider,
	discordProvider,
	linkedinProvider,
	pornhubProvider,
	{Name: "Telegram",    Slug: "telegram",    Fetch: fetchTelegram},
	torProvider,
	githubProvider,
	oracleProvider,
}

// BySlug returns a provider by its slug (case-insensitive).
// Sub-providers are addressed as "github/copilot", "oracle/us-ashburn-1" etc.
func BySlug(slug string) (Provider, bool) {
	slug = strings.ToLower(slug)
	for _, p := range All {
		if p.Slug == slug {
			return p, true
		}
		for _, sub := range p.SubProviders {
			if sub.Slug == slug {
				return sub, true
			}
		}
	}
	return Provider{}, false
}
