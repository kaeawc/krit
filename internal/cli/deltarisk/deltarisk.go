// Package deltarisk implements the `krit delta-risk` CLI verb. It
// scores a structural delta between two captured snapshots against
// historical breakage events and returns the resemblance score plus
// the top matching events.
package deltarisk

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	bisectpkg "github.com/kaeawc/krit/internal/bisect"
	brk "github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/cli/clishared"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

const usage = `usage: krit delta-risk --from <sha> --to <sha> [flags]

  Score a structural delta against historical breakage events. Returns
  the maximum cosine similarity against any historical delta plus the
  top matches.

Flags:
  --repo PATH     repo root (default: cwd)
  --from SHA      from sha
  --to SHA        to sha
  --format FMT    text|json (default: text)
  --max-matches N cap on returned matches (default: 5)
`

func Run(args []string) int {
	fs := flag.NewFlagSet("delta-risk", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	fromFlag := fs.String("from", "", "from sha")
	toFlag := fs.String("to", "", "to sha")
	formatFlag := fs.String("format", "text", "text|json")
	maxMatchesFlag := fs.Int("max-matches", 5, "cap on returned matches")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *fromFlag == "" || *toFlag == "" {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	repoRoot, code := clishared.ResolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	root := snap.SnapshotsDir(repoRoot)
	fromSHA := clishared.ResolveCommitOrLiteral(repoRoot, *fromFlag)
	toSHA := clishared.ResolveCommitOrLiteral(repoRoot, *toFlag)

	all, err := brk.LoadAll(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	res, err := bisectpkg.ScoreDelta(bisectpkg.DeltaRiskInput{
		SnapshotsRoot:    root,
		FromSHA:          fromSHA,
		ToSHA:            toSHA,
		HistoricalEvents: all,
		MaxMatches:       *maxMatchesFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *formatFlag == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	printText(res)
	return 0
}

func printText(res *bisectpkg.DeltaRiskResult) {
	fmt.Printf("delta %s -> %s\n", clishared.ShortSHA(res.From), clishared.ShortSHA(res.To))
	fmt.Printf("score: %.2f (max cosine across historical events)\n", res.Score)
	if len(res.Vector) == 0 {
		fmt.Println("  empty delta vector (no module metrics changed)")
		return
	}
	if len(res.Matches) == 0 {
		fmt.Println("  no historical matches")
		return
	}
	fmt.Println("top matches:")
	for _, m := range res.Matches {
		fmt.Printf("  cos=%.2f  %s  %s  %s\n", m.Cosine, clishared.ShortSHA(m.CommitSHA), m.FailureKind, m.Module)
	}
}
