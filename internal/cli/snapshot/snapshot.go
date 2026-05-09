// Package snapshot implements the `krit snapshot` CLI verb.
package snapshot

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/cli/clishared"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

// Version is set by main via ldflags.
var Version string

const usage = `usage: krit snapshot <capture|backfill|status|timeline|info> [flags]

  capture [<sha>]   capture a structural snapshot for sha (default: HEAD)
  backfill          capture snapshots for past commits via git worktrees
  status            list captured snapshots in this repo
  timeline          print scalar metric over captured snapshots
  info <sha>        print the manifest for a captured sha

Capture flags:
  --repo PATH       repo root (default: cwd)
  --output PATH     write blob path on success to PATH (default: stderr)

Timeline flags:
  --repo PATH       repo root (default: cwd)
  --scope SCOPE     repo|module|file  (default: repo)
  --target TARGET   module path or file path; required for module/file scope
  --metric METRIC   loc|bytes|symbols|public_symbols|cyclomatic|files|fan_in|fan_out
                    (default: loc)
`

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}
	switch args[0] {
	case "capture":
		return runCapture(args[1:])
	case "backfill":
		return runBackfill(args[1:])
	case "status":
		return runStatus(args[1:])
	case "timeline":
		return runTimeline(args[1:])
	case "info":
		return runInfo(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown snapshot subcommand: %s\n%s", args[0], usage)
		return 1
	}
}

// resolveRepoRoot honors the --repo flag, falling back to cwd. Returns
// the resolved root and an exit code; non-zero exit means an error has
// been reported.
func resolveRepoRoot(flagValue string) (string, int) {
	if flagValue != "" {
		return flagValue, 0
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return "", 1
	}
	return cwd, 0
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func runCapture(args []string) int {
	fs := flag.NewFlagSet("snapshot capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	outputFlag := fs.String("output", "", "write blob path to this file on success")

	positional, rest := clishared.SplitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	ref := "HEAD"
	if len(positional) > 0 {
		ref = positional[0]
	}
	sha, err := snap.ResolveCommitSHA(repoRoot, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	res, err := snap.Capture(snap.CaptureOptions{
		RepoRoot:    repoRoot,
		CommitSHA:   sha,
		KritVersion: Version,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	root := snap.SnapshotsDir(repoRoot)
	path, err := snap.SaveResult(root, res, repoRoot, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "captured snapshot %s (%d files, %d symbols, %d modules) -> %s\n",
		shortSHA(sha), len(res.Blob.Files), len(res.Blob.Symbols), len(res.Blob.Modules), path)

	if *outputFlag != "" {
		if err := os.WriteFile(*outputFlag, []byte(path+"\n"), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *outputFlag, err)
			return 1
		}
	}
	return 0
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("snapshot status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	root := snap.SnapshotsDir(repoRoot)
	entries, err := snap.List(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "no snapshots in %s\n", root)
		return 0
	}
	fmt.Println("commit\tcaptured_at\tkrit_version\tfiles\tsymbols\tbytes")
	for _, e := range entries {
		short := shortSHA(e.CommitSHA)
		m, err := snap.LoadManifest(root, e.CommitSHA)
		if err != nil || m == nil {
			fmt.Printf("%s\t-\t-\t-\t-\t%d\n", short, e.Bytes)
			continue
		}
		fmt.Printf("%s\t%d\t%s\t%d\t%d\t%d\n", short, m.CapturedAt, m.KritVersion, m.Files, m.Symbols, e.Bytes)
	}
	return 0
}

func runInfo(args []string) int {
	fs := flag.NewFlagSet("snapshot info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	positional, rest := clishared.SplitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit snapshot info <sha>")
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	root := snap.SnapshotsDir(repoRoot)
	// Fall back to the literal arg when git can't resolve it — captured
	// shas may no longer be reachable from the current branch.
	sha, err := snap.ResolveCommitSHA(repoRoot, positional[0])
	if err != nil {
		sha = positional[0]
	}
	m, err := snap.LoadManifest(root, sha)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func runBackfill(args []string) int {
	fs := flag.NewFlagSet("snapshot backfill", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	branchFlag := fs.String("branch", "", "branch or revspec to walk (default: HEAD)")
	sinceFlag := fs.Duration("since", 0, "only capture commits in the last duration (e.g. 720h for 30d)")
	maxFlag := fs.Int("max", 0, "max number of commits to capture (0 = unlimited)")
	workersFlag := fs.Int("workers", 0, "parallel worker count (0 = NumCPU)")
	forceFlag := fs.Bool("force", false, "recapture even when a snapshot already exists")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	res, err := snap.Backfill(snap.BackfillOptions{
		RepoRoot:    repoRoot,
		Branch:      *branchFlag,
		Since:       *sinceFlag,
		MaxCommits:  *maxFlag,
		Workers:     *workersFlag,
		Force:       *forceFlag,
		KritVersion: Version,
		Reporter: func(ev snap.BackfillEvent) {
			short := shortSHA(ev.CommitSHA)
			switch ev.Kind {
			case "captured":
				fmt.Fprintf(os.Stderr, "captured %s (%s)\n", short, ev.Duration.Round(time.Millisecond))
			case "skipped":
				fmt.Fprintf(os.Stderr, "skipped  %s\n", short)
			case "failed":
				fmt.Fprintf(os.Stderr, "failed   %s: %v\n", short, ev.Error)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "backfill: %d captured, %d skipped, %d failed\n", res.Captured, res.Skipped, res.Failed)
	if res.Failed > 0 {
		return 1
	}
	return 0
}

func runTimeline(args []string) int {
	fs := flag.NewFlagSet("snapshot timeline", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	scopeFlag := fs.String("scope", "repo", "repo|module|file")
	targetFlag := fs.String("target", "", "module path or file path (required for module/file)")
	metricFlag := fs.String("metric", "loc", "loc|bytes|symbols|public_symbols|cyclomatic|files|fan_in|fan_out|modules")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	root := snap.SnapshotsDir(repoRoot)
	points, err := snap.Timeline(root, snap.TimelineQuery{
		Scope:  snap.TimelineScope(*scopeFlag),
		Target: *targetFlag,
		Metric: *metricFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(points) == 0 {
		fmt.Fprintln(os.Stderr, "no timeline points (no captured snapshots match)")
		return 0
	}
	for _, p := range points {
		fmt.Printf("%s\t%d\t%g\n", shortSHA(p.CommitSHA), p.CapturedAt, p.Value)
	}
	return 0
}
