package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kaeawc/krit/internal/cli/scan"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// handleListRules implements the list-rules verb. Captures the same
// listing the in-process --list-rules flag emits and returns it as
// MetaResult.Stdout / Stderr / ExitCode. Plugin rules are not loaded
// here — the CLI's daemon-compatibility gate already keeps callers
// with --custom-rule-jars on the in-process path because plugin
// discovery requires the krit-types JVM the daemon doesn't manage on
// behalf of meta queries.
func handleListRules(_ context.Context, _ *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.ListRulesArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	var stdout, stderr bytes.Buffer
	code, _ := scan.PrintListRules(&stdout, &stderr, args.Verbose, args.Maturity, args.TaxonomyID, nil, nil)
	return daemon.MetaResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: code,
	}, nil
}

// handleListExperiments implements the list-experiments verb. Mirrors
// the in-process --list-experiments handler byte-for-byte (compiled-in
// experiment catalog, no project context required).
func handleListExperiments(_ context.Context, _ *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.ListExperimentsArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	var stdout bytes.Buffer
	scan.WriteListExperiments(&stdout, args.Format, kritVersion())
	return daemon.MetaResult{
		Stdout:   stdout.Bytes(),
		ExitCode: 0,
	}, nil
}

// handleValidateConfig implements the validate-config verb. When
// args.ConfigPath is non-empty the daemon loads that explicit config
// for validation; otherwise it uses its already-loaded resident
// config so subsequent calls don't repeat the krit.yml read.
func handleValidateConfig(_ context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.ValidateConfigArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	cfg, err := state.configForValidation(args.ConfigPath)
	if err != nil {
		var stderr bytes.Buffer
		fmt.Fprintf(&stderr, "error: load config: %v\n", err)
		return daemon.MetaResult{Stderr: stderr.Bytes(), ExitCode: 2}, nil
	}
	var stderr bytes.Buffer
	code := scan.ValidateConfigTo(&stderr, cfg)
	return daemon.MetaResult{Stderr: stderr.Bytes(), ExitCode: code}, nil
}

// configForValidation returns the *config.Config validate-config
// should check. ConfigPath="" reuses the daemon's resident config (the
// hot path); a non-empty path forces a one-shot load so explicit
// `--config FILE` invocations still get the file the user pointed at,
// not the daemon's autodetected one.
func (s *daemonState) configForValidation(explicitPath string) (*config.Config, error) {
	if explicitPath == "" {
		return s.ensureConfig()
	}
	cfg, err := config.LoadConfig(explicitPath)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = config.NewConfig()
	}
	return cfg, nil
}

// handleOracleFilterFingerprint implements the
// oracle-filter-fingerprint verb. Walks the requested scan paths for
// Kotlin sources, builds the active rule set the same way the CLI
// does (default + AllRules), and emits the JSON fingerprint report
// the CI drift gate consumes. Does NOT invoke the krit-types JVM —
// this is the byte-substring pre-filter only.
func handleOracleFilterFingerprint(_ context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.OracleFilterFingerprintArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	paths := args.Paths
	if len(paths) == 0 {
		paths = []string{state.root}
	}
	cfg, err := state.ensureConfig()
	if err != nil {
		var stderr bytes.Buffer
		fmt.Fprintf(&stderr, "error: load config: %v\n", err)
		return daemon.MetaResult{Stderr: stderr.Bytes(), ExitCode: 2}, nil
	}
	rules.ApplyConfig(cfg)
	experimental := cfg.GetTopLevelBool("experimental", false)
	strict := cfg.GetTopLevelBool("strict", false)
	activeRules := rules.ActiveRulesV2(nil, nil, args.AllRules, experimental, strict)

	files, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		var stderr bytes.Buffer
		fmt.Fprintf(&stderr, "error: collect files: %v\n", err)
		return daemon.MetaResult{Stderr: stderr.Bytes(), ExitCode: 2}, nil
	}
	// Sort for determinism — CollectKotlinFiles returns walk order;
	// the in-process CLI path also normalises before fingerprinting via
	// oracle.StableFingerprint, but pre-sort keeps the file list
	// matching the CLI's collectFiles output for downstream consumers
	// that do their own audit.
	sort.Strings(files)

	var stdout, stderr bytes.Buffer
	code := scan.WriteOracleFilterFingerprint(&stdout, &stderr, paths, files, activeRules, args.AllRules)
	return daemon.MetaResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: code,
	}, nil
}
