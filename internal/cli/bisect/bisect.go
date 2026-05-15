// Package bisect implements the `krit bisect-structure` CLI verb. It
// fuses location signals across the snapshot timeline and returns
// ranked (commit, module, reason, confidence) explanations for a
// given breakage event.
package bisect

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

const usage = `usage: krit bisect-structure --from <sha> --to <sha> --event <id> [flags]

  Explain a recorded breakage by fusing location signals across the
  captured snapshot timeline. Returns a ranked list of (commit, module,
  reason, confidence) candidates.

Flags:
  --repo PATH     repo root (default: cwd)
  --from SHA      good (or last-known-good) sha bounding the search
  --to SHA        bad (or current) sha bounding the search
  --event ID      breakage event id (from ` + "`" + `krit breakage list` + "`" + `)
  --format FMT    text|json (default: text)
  --max-results N cap the number of candidates returned (default: 10)
`

func Run(args []string) int {
	fs := flag.NewFlagSet("bisect-structure", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	fromFlag := fs.String("from", "", "good sha")
	toFlag := fs.String("to", "", "bad sha")
	eventFlag := fs.String("event", "", "breakage event id")
	formatFlag := fs.String("format", "text", "text|json")
	maxResultsFlag := fs.Int("max-results", 10, "cap on returned candidates")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *eventFlag == "" {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	repoRoot, code := clishared.ResolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	root := snap.SnapshotsDir(repoRoot)

	event, err := brk.FindByID(root, *eventFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if event == nil {
		fmt.Fprintf(os.Stderr, "error: no breakage event with id %q (try `krit breakage list`)\n", *eventFlag)
		return 1
	}

	all, err := brk.LoadAll(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fromSHA := clishared.ResolveCommitOrLiteral(repoRoot, *fromFlag)
	toSHA := clishared.ResolveCommitOrLiteral(repoRoot, *toFlag)

	res, err := bisectpkg.Run(bisectpkg.Input{
		SnapshotsRoot:    root,
		FromSHA:          fromSHA,
		ToSHA:            toSHA,
		Event:            *event,
		HistoricalEvents: all,
		MaxResults:       *maxResultsFlag,
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

func printText(res *bisectpkg.Result) {
	fmt.Printf("event %s (%s)\n", res.Event.ID, res.Event.FailureKind)
	if res.Event.Message != "" {
		fmt.Printf("  message: %s\n", res.Event.Message)
	}
	fmt.Printf("range: %s -> %s\n", clishared.ShortSHA(res.From), clishared.ShortSHA(res.To))
	if len(res.Candidates) == 0 {
		fmt.Println("no candidates")
		return
	}
	for i, c := range res.Candidates {
		fmt.Printf("\n#%d  %s  %s  confidence=%.2f\n", i+1, clishared.ShortSHA(c.CommitSHA), c.Module, c.Confidence)
		for _, r := range c.Reasons {
			fmt.Printf("    %s\n", r)
		}
	}
}
