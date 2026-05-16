package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

func (r *runner) filterColumnsByDelta(ref string) (scanner.FindingColumns, error) {
	baseIDs, err := deltaBaseFindingIDsForFlags(ref, r.f, r.paths)
	if err != nil {
		return scanner.FindingColumns{}, err
	}
	return filterColumnsNewSince(r.allColumns, baseIDs, r.basePath), nil
}

// deltaBaseFindingIDsForFlags spawns a base-ref worktree and re-execs
// krit against it to collect the baseline finding IDs the current run
// will subtract. Factored out of runner.deltaBaseFindingIDs so the
// daemon-routed --delta path can call it without owning a runner.
func deltaBaseFindingIDsForFlags(ref string, f *scanFlags, paths []string) (map[string]bool, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "krit-delta-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if out, err := exec.CommandContext(context.Background(), "git", "-C", repoRoot, "worktree", "add", "--detach", "--quiet", tmp, ref).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("create base worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = exec.CommandContext(context.Background(), "git", "-C", repoRoot, "worktree", "remove", "--force", tmp).Run()
	}()

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := deltaSnapshotArgsForFlags(f, paths, repoRoot, tmp)
	cmd := exec.CommandContext(context.Background(), exe, args...)
	cmd.Dir = tmp
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
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

func deltaSnapshotArgsForFlags(f *scanFlags, paths []string, repoRoot, tmp string) []string {
	args := []string{
		"--format", "json",
		"-q",
		"--no-cache",
		"--base-path", tmp,
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
