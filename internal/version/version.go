package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
)

const RepoURL = "https://github.com/D4n13l3k00/mikrotik-lists-manager"

// UserAgent returns the standard User-Agent header string.
func UserAgent() string {
	return fmt.Sprintf("mikrotik-lists-manager/%s (+%s)", Version, RepoURL)
}
