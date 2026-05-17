package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var githubProvider = Provider{
	Name: "GitHub",
	Slug: "github",
	SubProviders: []Provider{
		{Name: "GitHub / hooks",                      Slug: "github/hooks",                      Fetch: makeGitHubFetch("hooks")},
		{Name: "GitHub / web",                        Slug: "github/web",                        Fetch: makeGitHubFetch("web")},
		{Name: "GitHub / api",                        Slug: "github/api",                        Fetch: makeGitHubFetch("api")},
		{Name: "GitHub / git",                        Slug: "github/git",                        Fetch: makeGitHubFetch("git")},
		{Name: "GitHub / packages",                   Slug: "github/packages",                   Fetch: makeGitHubFetch("packages")},
		{Name: "GitHub / pages",                      Slug: "github/pages",                      Fetch: makeGitHubFetch("pages")},
		{Name: "GitHub / importer",                   Slug: "github/importer",                   Fetch: makeGitHubFetch("importer")},
		{Name: "GitHub / actions",                    Slug: "github/actions",                    Fetch: makeGitHubFetch("actions")},
		{Name: "GitHub / actions_macos",              Slug: "github/actions_macos",              Fetch: makeGitHubFetch("actions_macos")},
		{Name: "GitHub / codespaces",                 Slug: "github/codespaces",                 Fetch: makeGitHubFetch("codespaces")},
		{Name: "GitHub / copilot",                    Slug: "github/copilot",                    Fetch: makeGitHubFetch("copilot")},
		{Name: "GitHub / github_enterprise_importer", Slug: "github/github_enterprise_importer", Fetch: makeGitHubFetch("github_enterprise_importer")},
	},
}

// GitHubProvider returns the GitHub provider entry.
func GitHubProvider() Provider { return githubProvider }

type githubMeta struct {
	Hooks                    []string `json:"hooks"`
	Web                      []string `json:"web"`
	API                      []string `json:"api"`
	Git                      []string `json:"git"`
	Packages                 []string `json:"packages"`
	Pages                    []string `json:"pages"`
	Importer                 []string `json:"importer"`
	Actions                  []string `json:"actions"`
	ActionsMacOS             []string `json:"actions_macos"`
	Codespaces               []string `json:"codespaces"`
	Copilot                  []string `json:"copilot"`
	GithubEnterpriseImporter []string `json:"github_enterprise_importer"`
}

func (g *githubMeta) service(slug string) []string {
	switch slug {
	case "hooks":                    return g.Hooks
	case "web":                      return g.Web
	case "api":                      return g.API
	case "git":                      return g.Git
	case "packages":                 return g.Packages
	case "pages":                    return g.Pages
	case "importer":                 return g.Importer
	case "actions":                  return g.Actions
	case "actions_macos":            return g.ActionsMacOS
	case "codespaces":               return g.Codespaces
	case "copilot":                  return g.Copilot
	case "github_enterprise_importer": return g.GithubEnterpriseImporter
	}
	return nil
}

func makeGitHubFetch(service string) func(*http.Client) ([]string, error) {
	return func(client *http.Client) ([]string, error) {
		body, err := get(client, "https://api.github.com/meta")
		if err != nil {
			return nil, err
		}
		var meta githubMeta
		if err := json.Unmarshal(body, &meta); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		var cidrs []string
		for _, addr := range meta.service(service) {
			if isIPv4CIDR(addr) {
				cidrs = append(cidrs, addr)
			}
		}
		return cidrs, nil
	}
}
