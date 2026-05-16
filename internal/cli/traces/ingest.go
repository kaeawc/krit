package traces

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/traces"
	"github.com/kaeawc/krit/internal/traces/ingest/jfr"
	"github.com/kaeawc/krit/internal/traces/ingest/junit"
	"github.com/kaeawc/krit/internal/traces/ingest/otel"
)

func runIngest(args []string) int {
	fs := flag.NewFlagSet("traces ingest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	otelPath := fs.String("otel", "", "OTLP/JSON span file")
	jfrPath := fs.String("jfr", "", "JFR (jfr print --json) sample file")
	junitPath := fs.String("junit-steps", "", "JUnit step-boundary JSON file")
	commitFlag := fs.String("commit", "", "commit sha this run is for")
	envFlag := fs.String("env", "", "environment label")
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	// Exactly one input must be provided.
	selected := 0
	for _, p := range []string{*otelPath, *jfrPath, *junitPath} {
		if p != "" {
			selected++
		}
	}
	if selected != 1 {
		fmt.Fprintln(os.Stderr, "error: exactly one of --otel, --jfr, --junit-steps is required")
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	var (
		events []traces.Event
		kind   traces.SourceKind
		path   string
		err    error
	)
	switch {
	case *otelPath != "":
		path = *otelPath
		kind = traces.SourceOTel
		events, err = parseFile(path, otel.Parse)
	case *jfrPath != "":
		path = *jfrPath
		kind = traces.SourceJFR
		events, err = parseFile(path, jfr.Parse)
	case *junitPath != "":
		path = *junitPath
		kind = traces.SourceJUnit
		events, err = parseFile(path, junit.Parse)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(events) == 0 {
		fmt.Fprintf(os.Stderr, "warning: %s produced 0 events\n", path)
	}

	// Stamp source ID derived from (kind, path, commit, start time) so
	// repeat ingests of the same file under the same commit are
	// deduplicated by Merge.
	source := traces.IngestSource{
		ID:         sourceID(kind, path, *commitFlag),
		Kind:       kind,
		StartedAt:  time.Now().UnixNano(),
		CommitSHA:  *commitFlag,
		Env:        *envFlag,
		Path:       path,
		EventCount: len(events),
	}
	for i := range events {
		events[i].SourceID = source.ID
	}
	states, transitions := traces.Reduce(events)

	store, err := traces.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	store.Merge(source, states, transitions)
	if err := store.Save(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "ingested %d events -> %d states, %d transitions (source=%s)\n",
		len(events), len(states), len(transitions), source.ID)
	return 0
}

func parseFile(path string, parse func([]byte) ([]traces.Event, error)) ([]traces.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parse(data)
}

func sourceID(kind traces.SourceKind, path, commit string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(commit))
	sum := h.Sum(nil)
	return string(kind) + "-" + hex.EncodeToString(sum[:6])
}
