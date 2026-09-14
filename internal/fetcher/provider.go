package fetcher

import (
	"net/http"
	"strings"
	"time"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/version"
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

type userAgentTransport struct {
	base http.RoundTripper
	ua   string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", t.ua)
	}
	return t.base.RoundTrip(r)
}

// NewClient returns an http.Client with the given timeout and standard User-Agent.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &userAgentTransport{
			base: http.DefaultTransport,
			ua:   version.UserAgent(),
		},
	}
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
	netflixProvider,
	twitchProvider,
	steamProvider,
	blizzardProvider,
	riotProvider,
	ubisoftProvider,
	eaProvider,
	epicProvider,
	robloxProvider,
	appleProvider,
	yandexProvider,
	vkProvider,
	telegaProvider,
	mailruProvider,
	zoomProvider,
	redditProvider,
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
