package main

import "github.com/D4n13l3k00/mikrotik-lists-manager/internal/cli"

var (
	version = "dev"
	commit  = "none"
)

func main() {
	cli.Execute(version, commit)
}
