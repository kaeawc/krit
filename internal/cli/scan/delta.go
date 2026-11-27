package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/selfexec"
)

func (r *runner) filterColumnsByDelta(ref string) (scanner.FindingColumns, error) {
	baseIDs, err := deltaBaseFindingIDsForFlags(ref, r.f, r.paths, r.prFiles)
	if err != nil {
		return scanner.FindingColumns{}, err
	}
	return filterColumnsNewSince(r.allColumns, baseIDs, r.basePath), nil
}

// deltaBaseFindingIDsForFlags spawns a base-ref worktree and re-execs
// krit against it to collect the baseline finding IDs the current run
// will subtract. Factored out of runner.deltaBaseFindingIDs so the
// daemon-routed --delta path can call it without owning a runner.
//
// prFiles, when non-nil, are the files the current run analyzed in PR mode
// (#1660). The base scan analyzes the same files, mapped into the worktree,
// so an unchanged file keeps Go's verdict on both sides and its findings
// compare like for like. The current run's build model goes to the base scan
// too (forwardModelToBase: it is untracked, so the checkout has none), so the
// base compiles the same modules against this checkout's build outputs.
func deltaBaseFindingIDsForFlags(ref string, f *scanFlags, paths []string, prFiles []string) (map[string]bool, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "krit-delta-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// The file list lives outside the worktree, which the base scan walks.
	listDir, err := os.MkdirTemp("", "krit-delta-list-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(listDir)
	if out, err := exec.CommandContext(context.Background(), "git", "-C", repoRoot, "worktree", "add", "--detach", "--quiet", tmp, ref).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("create base worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = exec.CommandContext(context.Background(), "git", "-C", repoRoot, "worktree", "remove", "--force", tmp).Run()
	}()

	base := baseScanInputs{}
	model, err := forwardModelToBase(f, paths, repoRoot, tmp, listDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: --delta: the base-ref scan runs without this checkout's build model (%v), so FIR and oracle verdicts can differ between the two sides; findings may be reported as new that are not\n", err)
	}
	base.GradleModel = model.GradleModelArg
	if model.Forwarded {
		if changed, err := changedBuildInputs(ref, repoRoot); err == nil && len(changed) > 0 {
			fmt.Fprintf(os.Stderr, "warning: --delta: build files changed since %s (%s); the base-ref scan compiles with this checkout's build model, so findings the build change causes (dependencies, compiler options, plugins) are not reported as new\n",
				ref, strings.Join(firstN(changed, 3), ", "))
		}
	}
	if prFiles != nil {
		base.PRFileList = filepath.Join(listDir, "pr-files.txt")
		if err := writePRFileList(base.PRFileList, prFiles, repoRoot, tmp); err != nil {
			return nil, err
		}
	}

	exe, err := deltaBaseExecutable()
	if err != nil {
		return nil, err
	}
	args := deltaSnapshotArgsForFlags(f, paths, repoRoot, tmp, base)
	cmd := exec.CommandContext(context.Background(), exe, args...)
	cmd.Dir = tmp
	// The base scan's project state (module compiles, PR-mode facts) is keyed
	// by the worktree's path, so under KRIT_STATE_DIR it would outlive the
	// worktree; it gets a directory of its own that goes with listDir.
	cmd.Env = append(os.Environ(), fsutil.StateDirEnv+"="+filepath.Join(listDir, "state"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if *f.Verbose {
		forwardBaseScanLog(os.Stderr, stderr.String())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() > 1 {
			return nil, fmt.Errorf("run base snapshot: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	var report output.JSONReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("parse base snapshot JSON: %w", err)
	}
	ids := make(map[string]bool, len(report.Findings))
	for _, finding := range report.Findings {
		file := normalizeDeltaFile(finding.File, tmp)
		ids[deltaFindingID(file, finding.Rule, finding.Message)] = true
	}
	return ids, nil
}

// deltaBaseExecutable is the krit the base scan runs; tests replace it.
var deltaBaseExecutable = selfexec.Executable

// baseScanInputs are what the base scan gets beyond the current run's flags:
// the PR-mode file list (writePRFileList) and the --gradle-model directory
// (forwardModelToBase), each "" when there is none.
type baseScanInputs struct {
	PRFileList  string
	GradleModel string
}

// firstN is at most n of items, with a count of the rest.
func firstN(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(slices.Clone(items[:n]), fmt.Sprintf("%d more", len(items)-n))
}

// deltaSnapshotArgsForFlags is the base scan's command line.
func deltaSnapshotArgsForFlags(f *scanFlags, paths []string, repoRoot, tmp string, base baseScanInputs) []string {
	args := []string{
		"--format", "json",
		"-q",
		"--no-cache",
		"--base-path", tmp,
	}
	if *f.Verbose {
		args = append(args, "-v")
	}
	if base.PRFileList != "" {
		args = append(args, "--pr-files", base.PRFileList)
	}
	switch {
	case base.GradleModel != "":
		args = append(args, "--gradle-model", base.GradleModel)
	case *f.NoGradleModel:
		args = append(args, "--no-gradle-model")
	}
	if *f.NoTypeInfer {
		args = append(args, "--no-type-inference")
	}
	if *f.NoTypeOracle {
		args = append(args, "--no-type-oracle")
	}
	if *f.NoFir {
		args = append(args, "--no-fir")
	}
	if *f.Fir {
		args = append(args, "--fir")
	}
	if *f.IncludeGenerated {
		args = append(args, "--include-generated")
	}
	if *f.AllRules {
		args = append(args, "--all-rules")
	}
	if *f.DisableRules != "" {
		args = append(args, "--disable-rules", *f.DisableRules)
	}
	if *f.EnableRules != "" {
		args = append(args, "--enable-rules", *f.EnableRules)
	}
	if *f.Config != "" {
		args = append(args, "--config", mapPathToWorktree(repoRoot, tmp, *f.Config))
	}
	for _, path := range paths {
		args = append(args, mapPathToWorktree(repoRoot, tmp, path))
	}
	return args
}

func filterColumnsNewSince(columns *scanner.FindingColumns, baseIDs map[string]bool, basePath string) scanner.FindingColumns {
	if columns == nil {
		return scanner.FindingColumns{}
	}
	return columns.FilterRows(func(row int) bool {
		id := deltaFindingID(normalizeCurrentDeltaFile(columns.FileAt(row), basePath), columns.RuleAt(row), columns.MessageAt(row))
		return !baseIDs[id]
	})
}

func deltaFindingID(file, rule, message string) string {
	return scanner.BaselineID(scanner.Finding{File: file, Rule: rule, Message: message}, "", "")
}

func gitRepoRoot() (string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve git root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func mapPathToWorktree(repoRoot, tmp, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Join(tmp, path)
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.Join(tmp, rel)
	}
	if err == nil && rel == "." {
		return tmp
	}
	return path
}

func normalizeDeltaFile(file, tmp string) string {
	if filepath.IsAbs(file) {
		if rel, err := filepath.Rel(tmp, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

func normalizeCurrentDeltaFile(file, basePath string) string {
	if filepath.IsAbs(file) && basePath != "" {
		if rel, err := filepath.Rel(basePath, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

// writePRFileList writes files, mapped from the repository into the base
// worktree, one absolute path per line: the --pr-files list the base scan
// reads (readPRFileList). A file the base ref lacks is simply not scanned
// there.
func writePRFileList(path string, files []string, repoRoot, tmp string) error {
	var b strings.Builder
	for _, file := range files {
		mapped, err := filepath.Abs(mapPathToWorktree(repoRoot, tmp, file))
		if err != nil {
			return err
		}
		b.WriteString(mapped)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// readPRFileList reads a --pr-files list into the set prModeFiles matches
// scanned files against: clean absolute paths. An empty list is PR mode over
// no file, not PR mode off.
func readPRFileList(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files[filepath.Clean(line)] = true
		}
	}
	return files, nil
}

// forwardBaseScanLog writes the base scan's -v output to w, each line marked
// as the base ref's, so a verbose --delta run shows what both sides did.
func forwardBaseScanLog(w io.Writer, log string) {
	for _, line := range strings.Split(strings.TrimRight(log, "\n"), "\n") {
		if line != "" {
			fmt.Fprintf(w, "--delta base: %s\n", line)
		}
	}
}
