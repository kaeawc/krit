package pipeline

import (
	"context"
	"fmt"
	"os"
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
func (OutputPhase) Run(_ context.Context, in OutputInput) (OutputResult, error) {
	columns := &in.Findings
	filtered, findingsInErrorRegions := filterColumnsByParsedErrorRegions(columns, in.KotlinFiles, in.JavaFiles)
	columns = &filtered
	if findingsInErrorRegions > 0 {
		in.Reporter.Verbosef("verbose: %d finding(s) dropped: anchored inside a parse-error region\n", findingsInErrorRegions)
	}

	// Dispatch merges per-file findings in worker-completion order, so
	// the column slice handed to Output is non-deterministic across
	// runs. Sort once here so FinalFindings, the format emitters, and
	// the daemon's wire payload all agree on row order.
	columns.SortByFileLine()

	// basePath defaults to the first scan path when not explicitly set.
	basePath := in.BasePath
	if basePath == "" && len(in.Paths) > 0 {
		basePath = in.Paths[0]
	}

	// Baseline filtering.
	if in.BaselinePath != "" {
		baseline, err := scanner.LoadBaseline(in.BaselinePath)
		if err != nil {
			return OutputResult{}, fmt.Errorf("load baseline %s: %w", in.BaselinePath, err)
		}
		before := columns.Len()
		filtered := scanner.FilterColumnsByBaseline(columns, baseline, basePath)
		columns = &filtered
		suppressed := before - columns.Len()
		in.BaselineSuppressed = &suppressed
	}

	// Git-diff filtering: restrict findings to files changed since DiffRef.
	// If git fails, fall back to "no filter" silently — matches the
	// existing main.go behaviour where git errors do not abort the run.
	if in.DiffRef != "" {
		changed, err := GitChangedFiles(in.DiffRef, in.Paths)
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

	fileCount := len(in.SourceFiles())
	ruleCount := len(in.ActiveRules)

	// Reports use one path spelling whatever form the scan paths took.
	reported := columns
	if rewritten := columns.RewriteFiles(ReportPathFunc(reportBasePath(in.BasePath, in.Paths), in.AbsolutePaths)); rewritten.N > 0 {
		reported = &rewritten
	}

	switch in.Format {
	case "json":
		if err := output.FormatJSONColumnsWithOptions(
			in.Writer,
			reported,
			in.Version,
			fileCount,
			ruleCount,
			in.StartTime,
			in.PerfTimings,
			in.ActiveRules,
			in.ExperimentNames,
			in.CacheStats,
			in.Caches,
			in.CacheBudget,
			output.JSONOptions{Compact: in.JSONCompact, BaselineSuppressed: in.BaselineSuppressed},
			in.PerfRuleStats,
		); err != nil {
			return OutputResult{}, fmt.Errorf("format json: %w", err)
		}
	case "plain":
		output.FormatPlainColumns(in.Writer, reported)
	case "sarif":
		if err := output.FormatSARIFColumns(in.Writer, reported, in.Version); err != nil {
			return OutputResult{}, fmt.Errorf("format sarif: %w", err)
		}
	case "checkstyle":
		output.FormatCheckstyleColumns(in.Writer, reported)
	default:
		return OutputResult{}, fmt.Errorf("unknown format: %s", in.Format)
	}

	return OutputResult{
		FinalFindings:          *columns,
		FindingsInErrorRegions: findingsInErrorRegions,
		Timings:                in.Timings,
	}, nil
}

func filterColumnsByParsedErrorRegions(columns *scanner.FindingColumns, kotlinFiles, javaFiles []*scanner.File) (scanner.FindingColumns, int) {
	files := make(map[string]*scanner.File, len(kotlinFiles)+len(javaFiles))
	for _, file := range kotlinFiles {
		if file != nil {
			files[file.Path] = file
		}
	}
	for _, file := range javaFiles {
		if file != nil {
			files[file.Path] = file
		}
	}
	return scanner.FilterColumnsByErrorRegions(columns, files)
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
	cmd := exec.CommandContext(context.Background(), "git", args...)
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

// ReportPathFunc returns the mapping applied to finding file paths in reports.
// By default a path under base (see reportBasePath) is reported relative to it
// with forward slashes, and any other path is reported absolute. With absolute
// set every path is reported absolute.
func ReportPathFunc(base string, absolute bool) func(string) string {
	return func(file string) string {
		abs := absOrSelf(file)
		if !absolute && base != "" && isWithinDir(base, abs) {
			rel, _ := filepath.Rel(base, abs)
			return filepath.ToSlash(rel)
		}
		return filepath.ToSlash(abs)
	}
}

// reportBasePath resolves the directory finding paths are reported against:
// --base-path when set, else the first scan path (its directory for a file).
func reportBasePath(basePath string, paths []string) string {
	if basePath == "" && len(paths) > 0 {
		basePath = paths[0]
	}
	if basePath == "" {
		return ""
	}
	abs := absOrSelf(basePath)
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		return filepath.Dir(abs)
	}
	return abs
}

func absOrSelf(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
