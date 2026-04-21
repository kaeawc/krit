package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

// OutputPhase is the sixth and final phase of the Krit analysis pipeline.
// It applies post-dispatch filtering (baseline + git-diff) and writes the
// final findings to the configured Writer in the requested format.
//
// The phase deliberately omits a number of side-effects that currently live
// in main.go (rule-audit and sample-rule short-circuits, warnings-as-errors
// promotion, --min-confidence filter, Oracle stats perf output, the
// "Found N issue(s)" stderr message, and output-file creation). Those will
// fold in during the Stage 6 main.go cleanup; keeping them out of the phase
// for now lets main.go continue to drive those knobs without a behavioural
// regression.
type OutputPhase struct{}

// Name returns the stable phase identifier used for timing and error tags.
func (OutputPhase) Name() string { return "output" }

// Run executes the Output phase. The side effect is the formatted findings
// written to in.Writer. The returned OutputResult carries the final set of
// findings (after baseline/diff filters) so downstream code (tests, perf
// summaries) can inspect what was actually emitted.
func (OutputPhase) Run(ctx context.Context, in OutputInput) (OutputResult, error) {
	columns := &in.FixupResult.Findings

	// basePath defaults to the first scan path when not explicitly set.
	basePath := in.BasePath
	if basePath == "" && len(in.FixupResult.Paths) > 0 {
		basePath = in.FixupResult.Paths[0]
	}

	// Baseline filtering.
	if in.BaselinePath != "" {
		baseline, err := scanner.LoadBaseline(in.BaselinePath)
		if err != nil {
			return OutputResult{}, fmt.Errorf("load baseline %s: %w", in.BaselinePath, err)
		}
		filtered := scanner.FilterColumnsByBaseline(columns, baseline, basePath)
		columns = &filtered
	}

	// Git-diff filtering: restrict findings to files changed since DiffRef.
	// If git fails, fall back to "no filter" silently — matches the
	// existing main.go behaviour where git errors do not abort the run.
	if in.DiffRef != "" {
		changed, err := GitChangedFiles(in.DiffRef, in.FixupResult.Paths)
		if err == nil {
			filtered := scanner.FilterColumnsByFilePaths(columns, changed)
			columns = &filtered
		}
	}

	// Promote warnings → errors before min-confidence / format dispatch.
	if in.WarningsAsErrors {
		columns.PromoteWarningsToErrors()
	}

	// Drop findings below the configured confidence threshold, if any.
	if in.MinConfidence > 0 {
		filtered := columns.FilterByMinConfidence(in.MinConfidence)
		columns = &filtered
	}

	fileCount := len(in.FixupResult.KotlinFiles)
	ruleCount := len(in.FixupResult.ActiveRules)

	switch in.Format {
	case "json":
		if err := output.FormatJSONColumns(
			in.Writer,
			columns,
			in.Version,
			fileCount,
			ruleCount,
			in.StartTime,
			in.PerfTimings,
			in.FixupResult.ActiveRules,
			in.ExperimentNames,
			in.CacheStats,
			in.Caches,
			in.CacheBudget,
		); err != nil {
			return OutputResult{}, fmt.Errorf("format json: %w", err)
		}
	case "plain":
		output.FormatPlainColumns(in.Writer, columns)
	case "sarif":
		if err := output.FormatSARIFColumns(in.Writer, columns, in.Version); err != nil {
			return OutputResult{}, fmt.Errorf("format sarif: %w", err)
		}
	case "checkstyle":
		output.FormatCheckstyleColumns(in.Writer, columns)
	default:
		return OutputResult{}, fmt.Errorf("unknown format: %s", in.Format)
	}

	return OutputResult{
		FinalFindings: *columns,
		Timings:       in.FixupResult.Timings,
	}, nil
}

// GitChangedFiles returns the set of absolute file paths that have changed
// since the given git ref, restricted to scanPaths. The implementation
// mirrors the cmd/krit/main.go helper of the same shape: invoke
// `git diff --name-only --diff-filter=ACMR <ref> -- <paths...>` and
// absolute-path the results.
//
// Errors (non-git repos, unknown ref, git not on PATH) are returned to the
// caller; Output treats any error as "no filter" and emits all findings.
func GitChangedFiles(ref string, scanPaths []string) (map[string]bool, error) {
	args := []string{"diff", "--name-only", "--diff-filter=ACMR", ref, "--"}
	args = append(args, scanPaths...)
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", ref, err)
	}

	result := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		absPath, absErr := filepath.Abs(line)
		if absErr != nil || absPath == "" {
			continue
		}
		result[absPath] = true
	}
	return result, nil
}

var _ Phase[OutputInput, OutputResult] = OutputPhase{}
