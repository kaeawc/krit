// Package snapshot implements the `krit snapshot` CLI verb.
package snapshot

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/cli/clishared"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

// Version is set by main via ldflags.
var Version string

const usage = `usage: krit snapshot <capture|backfill|status|timeline|info|diff|gate|install-hook> [flags]

  capture [<sha>]      capture a structural snapshot for sha (default: HEAD)
  backfill             capture snapshots for past commits via git worktrees
  status               list captured snapshots in this repo
  timeline             print scalar metric over captured snapshots
  info <sha>           print the manifest for a captured sha
  diff <from> <to>     show structural delta between two captured snapshots
  gate <from> <to>     fail (exit 2) if a delta exceeds a configured threshold
  install-hook         install a post-commit hook that captures HEAD on each commit
  simulate <rule>      score a rule across history (would this rule have been useful?)

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
	case "diff":
		return runDiff(args[1:])
	case "gate":
		return runGate(args[1:])
	case "install-hook":
		return runInstallHook(args[1:])
	case "simulate":
		return runSimulate(args[1:])
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

// formatInfoError shapes a LoadManifest failure for `snapshot info <arg>`.
// Missing-snapshot errors get a friendly hint; other failures fall through
// as the underlying message.
func formatInfoError(arg string, err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Sprintf("error: %q is not a captured snapshot. Try `krit snapshot status` to list captured shas.", arg)
	}
	return fmt.Sprintf("error: %v", err)
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
		fmt.Fprintln(os.Stderr, formatInfoError(positional[0], err))
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

func runSimulate(args []string) int {
	fs := flag.NewFlagSet("snapshot simulate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	branchFlag := fs.String("branch", "", "branch or revspec to walk (default: HEAD)")
	sinceFlag := fs.Duration("since", 0, "only score commits in the last duration (e.g. 720h for 30d)")
	maxFlag := fs.Int("max", 0, "max number of commits to score (0 = unlimited)")
	workersFlag := fs.Int("workers", 0, "parallel worker count (0 = NumCPU)")
	formatFlag := fs.String("format", "text", "output format: text|json")

	positional, rest := clishared.SplitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit snapshot simulate <rule> [--since DUR] [--max N] [--workers N]")
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	res, err := snap.Simulate(snap.SimulateOptions{
		RepoRoot:   repoRoot,
		Rule:       positional[0],
		Branch:     *branchFlag,
		Since:      *sinceFlag,
		MaxCommits: *maxFlag,
		Workers:    *workersFlag,
		Reporter: func(ev snap.SimulateEvent) {
			short := shortSHA(ev.CommitSHA)
			switch ev.Kind {
			case "scored":
				fmt.Fprintf(os.Stderr, "scored   %s findings=%d (%s)\n", short, ev.Findings, ev.Duration.Round(time.Millisecond))
			case "failed":
				fmt.Fprintf(os.Stderr, "failed   %s: %v\n", short, ev.Error)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *formatFlag == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}
	for _, p := range res.Points {
		fmt.Printf("%s\t%d\t%d\n", shortSHA(p.CommitSHA), p.CommittedAt, p.Findings)
	}
	if len(res.Failed) > 0 {
		fmt.Fprintf(os.Stderr, "%d commit(s) failed; see stderr above\n", len(res.Failed))
	}
	return 0
}

func runInstallHook(args []string) int {
	fs := flag.NewFlagSet("snapshot install-hook", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	uninstallFlag := fs.Bool("uninstall", false, "remove the krit-installed hook")
	forceFlag := fs.Bool("force", false, "overwrite an existing post-commit hook")
	printFlag := fs.Bool("print", false, "print the hook script to stdout instead of installing")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *printFlag {
		fmt.Print(snap.PostCommitHook)
		return 0
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	if *uninstallFlag {
		path, err := snap.UninstallHook(repoRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "removed %s\n", path)
		return 0
	}

	path, err := snap.InstallHook(repoRoot, *forceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "installed %s\n", path)
	return 0
}

func runGate(args []string) int {
	fs := flag.NewFlagSet("snapshot gate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	formatFlag := fs.String("format", "text", "output format: text|json")
	var maxAbs, maxDelta, maxPct clishared.MultiString
	fs.Var(&maxAbs, "max-abs", "[module/]metric=value (absolute cap on to-side); repeatable")
	fs.Var(&maxDelta, "max-delta", "[module/]metric=value (cap on absolute increase); repeatable")
	fs.Var(&maxPct, "max-pct", "[module/]metric=value (cap on percent increase from from-side); repeatable")
	positional, rest := clishared.SplitPositional(args, 2)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) != 2 {
		fmt.Fprintln(os.Stderr, "usage: krit snapshot gate <from> <to> [--max-abs metric=v]... [--max-delta metric=v]... [--max-pct metric=v]...")
		return 1
	}

	thresholds, err := parseGateThresholds(maxAbs, maxDelta, maxPct)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(thresholds) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one --max-abs / --max-delta / --max-pct required")
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	root := snap.SnapshotsDir(repoRoot)
	fromSHA, err := snap.ResolveCommitSHA(repoRoot, positional[0])
	if err != nil {
		fromSHA = positional[0]
	}
	toSHA, err := snap.ResolveCommitSHA(repoRoot, positional[1])
	if err != nil {
		toSHA = positional[1]
	}

	res, err := snap.Gate(snap.GateOptions{
		Root: root, FromSHA: fromSHA, ToSHA: toSHA, Thresholds: thresholds,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *formatFlag == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		printGateText(res)
	}
	if len(res.Violations) > 0 {
		return 2
	}
	return 0
}

func parseGateThresholds(maxAbs, maxDelta, maxPct []string) ([]snap.GateThreshold, error) {
	type key struct{ module, metric string }
	byKey := make(map[key]*snap.GateThreshold)
	add := func(raw, kind string) error {
		spec, value, err := parseMetricKV(raw)
		if err != nil {
			return err
		}
		module, metric := splitModuleMetric(spec)
		k := key{module: module, metric: metric}
		t := byKey[k]
		if t == nil {
			t = &snap.GateThreshold{Module: module, Metric: metric}
			byKey[k] = t
		}
		v := value
		switch kind {
		case "abs":
			t.MaxAbsolute = &v
		case "delta":
			t.MaxIncrease = &v
		case "pct":
			t.MaxIncreasePct = &v
		}
		return nil
	}
	for _, s := range maxAbs {
		if err := add(s, "abs"); err != nil {
			return nil, err
		}
	}
	for _, s := range maxDelta {
		if err := add(s, "delta"); err != nil {
			return nil, err
		}
	}
	for _, s := range maxPct {
		if err := add(s, "pct"); err != nil {
			return nil, err
		}
	}
	out := make([]snap.GateThreshold, 0, len(byKey))
	for _, t := range byKey {
		out = append(out, *t)
	}
	return out, nil
}

// splitModuleMetric splits a threshold spec into (module, metric).
// Bare metric names ("loc") return ("", "loc"); a module-prefixed
// form (":app/cyclomatic", "app/loc") returns the leading segment as
// module and the trailing segment as metric. The split is on the
// LAST '/' so module paths that themselves contain '/' (rare but
// legal in nested Gradle module IDs) survive intact.
func splitModuleMetric(spec string) (string, string) {
	idx := strings.LastIndexByte(spec, '/')
	if idx < 0 {
		return "", spec
	}
	return spec[:idx], spec[idx+1:]
}

func parseMetricKV(raw string) (string, float64, error) {
	idx := strings.IndexByte(raw, '=')
	if idx <= 0 || idx == len(raw)-1 {
		return "", 0, fmt.Errorf("expected metric=value, got %q", raw)
	}
	metric := strings.TrimSpace(raw[:idx])
	v, err := strconv.ParseFloat(strings.TrimSpace(raw[idx+1:]), 64)
	if err != nil {
		return "", 0, fmt.Errorf("parse %q value: %w", raw, err)
	}
	return metric, v, nil
}

func printGateText(res *snap.GateResult) {
	fmt.Printf("from %s -> to %s\n", shortSHA(res.From), shortSHA(res.To))
	if len(res.Violations) == 0 {
		fmt.Println("gate: pass")
		return
	}
	fmt.Printf("gate: %d violation(s)\n", len(res.Violations))
	for _, v := range res.Violations {
		label := v.Metric
		if v.Module != "" {
			label = v.Module + "/" + v.Metric
		}
		fmt.Printf("  %-32s %s: limit=%g got=%g (from=%g to=%g)\n",
			label, v.Constraint, v.Limit, v.Got, v.From, v.To)
	}
}

func runDiff(args []string) int {
	fs := flag.NewFlagSet("snapshot diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	formatFlag := fs.String("format", "text", "output format: text|json")
	positional, rest := clishared.SplitPositional(args, 2)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) != 2 {
		fmt.Fprintln(os.Stderr, "usage: krit snapshot diff <from> <to>")
		return 1
	}

	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}

	root := snap.SnapshotsDir(repoRoot)
	fromSHA, err := snap.ResolveCommitSHA(repoRoot, positional[0])
	if err != nil {
		fromSHA = positional[0]
	}
	toSHA, err := snap.ResolveCommitSHA(repoRoot, positional[1])
	if err != nil {
		toSHA = positional[1]
	}

	d, err := snap.Diff(root, fromSHA, toSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *formatFlag == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(d); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	printDiffText(d)
	return 0
}

func printDiffText(d *snap.DiffResult) {
	fmt.Printf("from %s -> to %s\n", shortSHA(d.From.CommitSHA), shortSHA(d.To.CommitSHA))
	if len(d.AddedModules) > 0 || len(d.RemovedModules) > 0 {
		fmt.Printf("\nmodules: +%d / -%d\n", len(d.AddedModules), len(d.RemovedModules))
		for _, m := range d.AddedModules {
			fmt.Printf("  + %s\n", m)
		}
		for _, m := range d.RemovedModules {
			fmt.Printf("  - %s\n", m)
		}
	}
	if len(d.AddedEdges) > 0 || len(d.RemovedEdges) > 0 {
		fmt.Printf("\nedges: +%d / -%d\n", len(d.AddedEdges), len(d.RemovedEdges))
		for _, e := range d.AddedEdges {
			fmt.Printf("  + %s -> %s (%s)\n", e.From, e.To, e.Configuration)
		}
		for _, e := range d.RemovedEdges {
			fmt.Printf("  - %s -> %s (%s)\n", e.From, e.To, e.Configuration)
		}
	}
	fmt.Printf("\nfiles: +%d / -%d\n", len(d.AddedFiles), len(d.RemovedFiles))
	fmt.Printf("symbols: +%d / -%d\n", len(d.AddedSymbols), len(d.RemovedSymbols))
	if len(d.RepoMetrics) > 0 {
		fmt.Println("\nrepo metrics:")
		for _, name := range []string{"loc", "files", "symbols", "public_symbols", "cyclomatic", "modules"} {
			if md, ok := d.RepoMetrics[name]; ok {
				fmt.Printf("  %-16s %g -> %g (%+g)\n", name, md.From, md.To, md.Delta)
			}
		}
	}
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
