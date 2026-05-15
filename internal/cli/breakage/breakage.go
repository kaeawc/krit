// Package breakage implements the `krit breakage` CLI verb. It ingests
// failure events from JUnit XML, `go test -json`, or generic JSON
// payloads and appends them to the snapshot-adjacent event store.
package breakage

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	brk "github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/cli/clishared"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

const usage = `usage: krit breakage <record|list> [flags]

  record   ingest failure events into the breakage store
  list     print recorded events as JSON

Record flags:
  --repo PATH       repo root (default: cwd)
  --kind FORMAT     ingest format: junit|gotest|generic (default: generic)
  --from PATH       read events from file (default: stdin)
  --commit SHA      commit sha events are attributed to (default: HEAD)
  --source NAME     source label (ci|local|krit-finding|runtime) (default: depends on kind)
`

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}
	switch args[0] {
	case "record":
		return runRecord(args[1:])
	case "list":
		return runList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown breakage subcommand: %s\n%s", args[0], usage)
		return 1
	}
}

func runRecord(args []string) int {
	fs := flag.NewFlagSet("breakage record", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	kindFlag := fs.String("kind", "generic", "junit|gotest|generic")
	fromFlag := fs.String("from", "", "input file (default: stdin)")
	commitFlag := fs.String("commit", "HEAD", "commit sha events are attributed to")
	sourceFlag := fs.String("source", "", "source label")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	repoRoot, code := clishared.ResolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	sha := clishared.ResolveCommitOrLiteral(repoRoot, *commitFlag)

	r, closer, err := openInput(*fromFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer closer()

	opts := brk.IngestOptions{
		CommitSHA:  sha,
		Source:     *sourceFlag,
		OccurredAt: time.Now(),
	}

	var events []brk.Event
	switch *kindFlag {
	case "junit":
		events, err = brk.IngestJUnit(r, opts)
	case "gotest":
		events, err = brk.IngestGoTest(r, opts)
	case "generic":
		events, err = brk.IngestGeneric(r, opts)
	default:
		fmt.Fprintf(os.Stderr, "unknown kind %q (want junit|gotest|generic)\n", *kindFlag)
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	root := snap.SnapshotsDir(repoRoot)
	added, err := brk.Record(root, events...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "breakage: %d parsed, %d newly recorded (%d duplicates skipped)\n",
		len(events), added, len(events)-added)
	return 0
}

func runList(args []string) int {
	fs := flag.NewFlagSet("breakage list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	repoRoot, code := clishared.ResolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	root := snap.SnapshotsDir(repoRoot)
	events, err := brk.LoadAll(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(events); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "" || path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}
