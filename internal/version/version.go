package version

import (
	"fmt"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = "none"
)

func init() {
	if Version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			Version = bi.Main.Version
		}
	}
}

const RepoURL = "https://github.com/D4n13l3k00/mikrotik-lists-manager"

// UserAgent returns the standard User-Agent header string.
func UserAgent() string {
	return fmt.Sprintf("mikrotik-lists-manager/%s (+%s)", Version, RepoURL)
}
