package main

import (
	"os"

	"github.com/kaeawc/krit/internal/cli/scan"
	"github.com/kaeawc/krit/internal/cli/serve"
	"github.com/kaeawc/krit/internal/oracle"
)

// version is set by goreleaser via ldflags: -X main.version=...
var version = "dev"

func main() {
	scan.Version = version
	serve.Version = version
	oracle.Version = version
	if dispatchSubcommand() {
		scan.BaselineAuditVerb = true
	}
	os.Exit(scan.Run())
}
