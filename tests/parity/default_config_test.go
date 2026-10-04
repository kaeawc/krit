package parity_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
)

// applyShippedRuleDefaults applies config/default-krit.yml to every
// registered rule, as the CLI does before it dispatches. Rule option
// defaults live in that file, and the rule structs' own defaults can differ
// from it (InjectDispatcher's struct lists Dispatchers.Main, the YAML does
// not); krit-fir receives the configured options, so the Go side of every
// comparison in this package must run with them too.
//
// ApplyConfig mutates the registered rule structs, so TestMain calls this
// once before any test runs: every test in the package sees the same,
// production-default rules regardless of test order. The file is read
// directly (config.LoadConfig with a path), never by walking up for a
// user krit.yml, so a developer's own config cannot leak in.
func applyShippedRuleDefaults() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(filepath.Join(root, "config", "default-krit.yml"))
	if err != nil {
		return fmt.Errorf("loading config/default-krit.yml: %w", err)
	}
	if cfg == nil {
		return errors.New("config/default-krit.yml loaded as nil")
	}
	rules.ApplyConfig(cfg)
	return nil
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", errors.New("could not find repository root")
		}
		wd = parent
	}
}
