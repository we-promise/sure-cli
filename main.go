package main

import "github.com/we-promise/sure-cli/cmd/sure-cli/root"

// Set by goreleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root.SetVersion(version, commit, date)
	root.Execute()
}
