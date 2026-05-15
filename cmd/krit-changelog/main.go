// krit-changelog emits a markdown changelog grouped by Rule.IntroducedIn
// from the built-in v2 rule registry. Output is written to stdout by
// default or to the -output path when supplied.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/changelog"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func main() {
	var (
		outputPath string
		limit      int
	)
	flag.StringVar(&outputPath, "output", "", "write markdown to this path (default: stdout)")
	flag.IntVar(&limit, "versions", 0, "only emit the most recent N versions (0 = all)")
	flag.Parse()

	snapshots := changelog.SnapshotRegistry(api.Registry, rules.MetaForRule)
	groups := changelog.GroupByVersion(snapshots)
	md := changelog.Render(groups, limit)

	if outputPath == "" {
		fmt.Print(md)
		return
	}
	if err := os.WriteFile(outputPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "krit-changelog: %v\n", err)
		os.Exit(1)
	}
}
