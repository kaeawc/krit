package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// FindingsSchemaVersion versions the findings sidecar wire format. Bump
// when ByRule/ByRuleFile semantics change in a way that breaks decode of
// previously written sidecars.
const FindingsSchemaVersion = 1

const findingsFileName = "findings.gob.zst"

// Findings is the per-commit per-rule findings rollup persisted next to
// the structural blob. simulate reads it as a Timeline; diff uses
// RuleSetHash to refuse cross-rule-set comparisons.
type Findings struct {
	SchemaVersion int
	CommitSHA     string
	RuleSetHash   string
	// ByRule maps rule ID -> total finding count across the snapshot.
	ByRule map[string]int
	// ByRuleFile maps rule ID -> (repo-relative file path -> count).
	// Optional; consumers that only need per-rule totals can ignore it.
	ByRuleFile map[string]map[string]int
}

// FindingsPath returns the on-disk location for a snapshot's findings
// sidecar. Mirrors BlobPath / MetricsPath layout (sharded by sha[:2]).
func FindingsPath(root, sha string) (string, error) {
	dir, err := shaDir(root, sha)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, findingsFileName), nil
}

func SaveFindings(root string, f *Findings) (string, error) {
	if f == nil {
		return "", errors.New("snapshot: nil findings")
	}
	if f.CommitSHA == "" {
		return "", errors.New("snapshot: findings has no CommitSHA")
	}
	path, err := FindingsPath(root, f.CommitSHA)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", filepath.Dir(path), err)
	}
	payload, err := cacheutil.EncodeZstdGob(f)
	if err != nil {
		return "", fmt.Errorf("snapshot: encode findings: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	return path, nil
}

func LoadFindings(root, sha string) (*Findings, error) {
	path, err := FindingsPath(root, sha)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open %s: %w", path, err)
	}
	defer file.Close()
	var f Findings
	if err := cacheutil.DecodeZstdGob(file, &f); err != nil {
		return nil, fmt.Errorf("snapshot: decode %s: %w", path, err)
	}
	return &f, nil
}

// FindingsRunOptions tunes RunFindings without forcing every caller to
// build a full pipeline.ProjectArgs.
type FindingsRunOptions struct {
	// Config is the resolved krit.yml; nil falls back to defaults.
	Config *config.Config
	// ActiveRules is the rule set dispatched against the worktree. When
	// empty, RunFindings selects the default active set.
	ActiveRules []*api.Rule
	// Workers overrides per-phase worker counts.
	Workers int
	// IncludeGenerated forwards through to ParsePhase.
	IncludeGenerated bool
	// RepoRelativeTo, when non-empty, rewrites the absolute file paths
	// returned in ByRuleFile to be repo-relative. Empty leaves paths
	// untouched.
	RepoRelativeTo string
}

// RunFindings runs the full rule pipeline against repoRoot once and
// returns a populated Findings ready to persist alongside the structural
// blob. The function exists so internal/snapshot can keep the heavy
// pipeline import contained and so Capture can opt in via a single call.
func RunFindings(ctx context.Context, repoRoot, commitSHA string, opts FindingsRunOptions) (*Findings, error) {
	if repoRoot == "" {
		return nil, errors.New("snapshot: RunFindings: repoRoot required")
	}
	if commitSHA == "" {
		return nil, errors.New("snapshot: RunFindings: commitSHA required")
	}
	cfg := opts.Config
	if cfg == nil {
		cfg = config.NewConfig()
	}
	activeRules := opts.ActiveRules
	if len(activeRules) == 0 {
		activeRules = rules.ActiveRulesV2(nil, nil, false, false)
	}

	res, err := pipeline.RunProject(ctx, pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:           cfg,
			Paths:            []string{repoRoot},
			ActiveRules:      activeRules,
			Format:           "json",
			IncludeGenerated: opts.IncludeGenerated,
			Workers:          opts.Workers,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot: RunFindings: %w", err)
	}

	out := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     commitSHA,
		RuleSetHash:   ruleSetHash(activeRules, cfg),
		ByRule:        make(map[string]int),
		ByRuleFile:    make(map[string]map[string]int),
	}
	collectFindings(&res.FinalFindings, opts.RepoRelativeTo, out)
	return out, nil
}

func collectFindings(cols *scanner.FindingColumns, repoRoot string, dst *Findings) {
	if cols == nil {
		return
	}
	for i := 0; i < cols.Len(); i++ {
		rule := cols.RuleAt(i)
		if rule == "" {
			continue
		}
		dst.ByRule[rule]++
		path := cols.FileAt(i)
		if repoRoot != "" {
			path = relPath(path, repoRoot)
		}
		perFile := dst.ByRuleFile[rule]
		if perFile == nil {
			perFile = make(map[string]int)
			dst.ByRuleFile[rule] = perFile
		}
		perFile[path]++
	}
}

// ruleSetHash computes the same fingerprint the cache layer uses so a
// findings sidecar can be cross-checked against a fresh active set.
// ComputeConfigHash sorts its input, so we don't need to here.
func ruleSetHash(activeRules []*api.Rule, cfg *config.Config) string {
	names := make([]string, 0, len(activeRules))
	for _, r := range activeRules {
		if r != nil {
			names = append(names, r.ID)
		}
	}
	return cache.ComputeConfigHash(names, cfg, false)
}
