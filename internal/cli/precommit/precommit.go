// Package precommit runs single-buffer rule analysis on every Kotlin
// file currently staged for a git commit, routing the work through a
// running krit daemon when one is available. It is the user-facing
// entry point for the daemon roadmap's pre-commit-hook goal.
package precommit

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/cli/daemonclient"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

// Run is the `krit precommit` subcommand entry point. It enumerates
// staged .kt/.kts files, runs per-file rules against each (preferring
// a running daemon over in-process analysis), and prints findings in
// the requested format. Exit code 0 means no findings; non-zero
// signals at least one finding so the hook can block the commit.
func Run(args []string) int {
	fs := flag.NewFlagSet("precommit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	rootFlag := fs.String("root", "", "Project root (default: git rev-parse --show-toplevel)")
	formatFlag := fs.String("format", "plain", "Output format: plain or json")
	noDaemonFlag := fs.Bool("no-daemon", false, "Skip the daemon dispatch path even if a socket is reachable")
	autoSpawnFlag := fs.Bool("auto-spawn", false, "Spawn a daemon if none is running")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	root := *rootFlag
	if root == "" {
		var err error
		root, err = gitToplevel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "krit precommit: %v\n", err)
			return 1
		}
	}

	staged, err := stagedKotlinFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit precommit: list staged: %v\n", err)
		return 1
	}
	if len(staged) == 0 {
		return 0
	}

	cli := newAnalyzer(root, *noDaemonFlag, *autoSpawnFlag)
	defer cli.Close()

	allFindings, err := cli.Analyze(staged)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit precommit: %v\n", err)
		return 1
	}
	emit(*formatFlag, staged, allFindings)
	if anyFindings(allFindings) {
		return 1
	}
	return 0
}

func gitToplevel() (string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// stagedKotlinFiles returns the absolute paths of every staged .kt /
// .kts file in the repo at root. Deletions are excluded.
func stagedKotlinFiles(root string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}
	var out []string
	for _, raw := range bytes.Split(stdout, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		name := string(raw)
		if !strings.HasSuffix(name, ".kt") && !strings.HasSuffix(name, ".kts") {
			continue
		}
		out = append(out, filepath.Join(root, name))
	}
	return out, nil
}

// analyzer dispatches per-file analyze requests, preferring the
// daemon when reachable and falling back to in-process otherwise.
type analyzer struct {
	root      string
	noDaemon  bool
	autoSpawn bool
	client    *daemonclient.Client
	fallback  *pipeline.SingleFileAnalyzer
	workspace *pipeline.WorkspaceState
}

func newAnalyzer(root string, noDaemon, autoSpawn bool) *analyzer {
	return &analyzer{root: root, noDaemon: noDaemon, autoSpawn: autoSpawn}
}

// Close is a hook for resource cleanup. Spawned daemons are
// intentionally left running so the next hook invocation finds them
// warm.
func (a *analyzer) Close() {}

// Analyze returns findings (one FindingColumns per input path) in
// the same order as paths. When a daemon is available the entire
// batch goes over one round trip; otherwise each file falls back to
// in-process analysis.
func (a *analyzer) Analyze(paths []string) ([]scanner.FindingColumns, error) {
	if !a.noDaemon {
		if c := a.connectClient(); c != nil {
			a.client = c
		}
	}
	contents := make([][]byte, len(paths))
	for i, path := range paths {
		c, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		contents[i] = c
	}

	if a.client != nil {
		if results, ok := a.analyzeBatchViaDaemon(paths, contents); ok {
			return results, nil
		}
	}

	results := make([]scanner.FindingColumns, len(paths))
	for i, path := range paths {
		cols, err := a.analyzeOneInProcess(path, contents[i])
		if err != nil {
			return nil, fmt.Errorf("analyze %s: %w", path, err)
		}
		results[i] = cols
	}
	return results, nil
}

// analyzeBatchViaDaemon dispatches every staged file in one batched
// analyze-buffers call. Returns ok=false on any wire-level failure;
// the caller falls back to per-file in-process analysis.
func (a *analyzer) analyzeBatchViaDaemon(paths []string, contents [][]byte) ([]scanner.FindingColumns, bool) {
	args := daemon.AnalyzeBuffersArgs{Buffers: make([]daemon.AnalyzeBufferArgs, len(paths))}
	for i, path := range paths {
		args.Buffers[i] = daemon.AnalyzeBufferArgs{Path: path, Content: string(contents[i])}
	}
	resp, err := a.client.AnalyzeBuffers(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit precommit: daemon AnalyzeBuffers failed (%v); falling back to in-process\n", err)
		a.client = nil
		return nil, false
	}
	if len(resp.Results) != len(paths) {
		fmt.Fprintf(os.Stderr, "krit precommit: daemon returned %d results for %d buffers; falling back to in-process\n",
			len(resp.Results), len(paths))
		return nil, false
	}
	out := make([]scanner.FindingColumns, len(paths))
	for i, entry := range resp.Results {
		if entry.Error != "" {
			fmt.Fprintf(os.Stderr, "krit precommit: daemon error on %s: %s\n", paths[i], entry.Error)
			cols, err := a.analyzeOneInProcess(paths[i], contents[i])
			if err != nil {
				return nil, false
			}
			out[i] = cols
			continue
		}
		var cols scanner.FindingColumns
		if err := json.Unmarshal(entry.Findings, &cols); err != nil {
			fmt.Fprintf(os.Stderr, "krit precommit: decode %s: %v; falling back to in-process\n", paths[i], err)
			cols, err = a.analyzeOneInProcess(paths[i], contents[i])
			if err != nil {
				return nil, false
			}
		}
		out[i] = cols
	}
	return out, true
}

// connectClient returns a daemon client. Discovery is tried first; if
// nothing is running and auto-spawn is enabled, EnsureRunning is
// attempted. Returns nil for in-process fallback.
func (a *analyzer) connectClient() *daemonclient.Client {
	if c, ok := daemonclient.Discover(a.root); ok {
		return c
	}
	if !a.autoSpawn {
		return nil
	}
	c, err := daemonclient.EnsureRunning(a.root, daemonclient.SpawnOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit precommit: spawn daemon: %v (falling back to in-process)\n", err)
		return nil
	}
	return c
}

// analyzeOneInProcess is the fallback path used when the daemon is
// absent or returns an error.
func (a *analyzer) analyzeOneInProcess(path string, content []byte) (scanner.FindingColumns, error) {
	if a.fallback == nil {
		a.fallback = pipeline.NewSingleFileAnalyzer(nil, nil)
		a.workspace = pipeline.NewWorkspaceState(a.root)
	}
	file, err := a.workspace.ParseFile(context.Background(), path, content)
	if err != nil {
		return scanner.FindingColumns{}, err
	}
	return a.fallback.AnalyzeFileColumns(file), nil
}

func anyFindings(all []scanner.FindingColumns) bool {
	for _, c := range all {
		if len(c.Line) > 0 {
			return true
		}
	}
	return false
}

// mergeFindings appends every per-path FindingColumns into one merged
// columns block so existing output formatters can render the combined
// result. The per-path slice is preserved upstream for any caller
// that wants per-file attribution.
func mergeFindings(all []scanner.FindingColumns) scanner.FindingColumns {
	collector := scanner.NewFindingCollector(0)
	for i := range all {
		collector.AppendColumns(&all[i])
	}
	return *collector.Columns()
}

// emit prints findings in the requested format. plain reuses the
// canonical FormatPlainColumns formatter so users see the same shape
// they get from `krit scan`. json wraps each per-file FindingColumns
// alongside its absolute path, since pre-commit consumers usually
// want to attribute findings to staged paths.
func emit(format string, paths []string, all []scanner.FindingColumns) {
	switch format {
	case "json":
		emitJSON(paths, all)
	default:
		merged := mergeFindings(all)
		output.FormatPlainColumns(os.Stdout, &merged)
	}
}

func emitJSON(paths []string, all []scanner.FindingColumns) {
	out := make([]map[string]any, 0, len(paths))
	for i, c := range all {
		out = append(out, map[string]any{
			"path":     paths[i],
			"findings": c,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "krit precommit: marshal: %v\n", err)
	}
}
