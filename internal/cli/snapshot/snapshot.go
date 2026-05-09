// Package snapshot implements the `krit snapshot` CLI verb. Phase A
// supports `capture` (write a graph blob for HEAD or a given sha) and
// `status` (list captured snapshots). Metrics queries and timelines
// arrive in later phases.
package snapshot

import (
	"flag"
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/cli/clishared"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

// Version is set by main via ldflags. Empty falls back to "dev" inside
// the snapshot package.
var Version string

const usage = `usage: krit snapshot <capture|status> [flags]

  capture [<sha>]   capture a structural snapshot for sha (default: HEAD)
  status            list captured snapshots in this repo

Flags (apply to capture):
  --repo PATH       repo root (default: cwd)
  --output PATH     write blob path on success to PATH (default: stderr)
`

// Run dispatches to the chosen sub-verb. Returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}
	switch args[0] {
	case "capture":
		return runCapture(args[1:])
	case "status":
		return runStatus(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown snapshot subcommand: %s\n%s", args[0], usage)
		return 1
	}
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

	repoRoot := *repoFlag
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		repoRoot = cwd
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

	blob, err := snap.Capture(snap.CaptureOptions{
		RepoRoot:    repoRoot,
		CommitSHA:   sha,
		KritVersion: Version,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	root := snap.SnapshotsDir(repoRoot)
	path, err := snap.Save(root, blob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	fmt.Fprintf(os.Stderr, "captured snapshot %s (%d files, %d symbols, %d modules) -> %s\n",
		short, len(blob.Files), len(blob.Symbols), len(blob.Modules), path)

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

	repoRoot := *repoFlag
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		repoRoot = cwd
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
	for _, e := range entries {
		fmt.Printf("%s\t%d\t%s\n", e.CommitSHA, e.Bytes, e.Path)
	}
	return 0
}
