package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// runInitSubcommand is the entry point for `krit init [dir]`. It
// wires up a bubbletea TUI that walks the user through profile
// selection, the controversial-rule questionnaire, and config
// write-out. For non-interactive runs (integration tests, CI) it
// accepts --profile and --yes flags that bypass the TUI entirely.
func runInitSubcommand(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	profileFlag := fs.String("profile", "", "Preset profile (bypasses interactive selection)")
	yesFlag := fs.Bool("yes", false, "Accept per-profile defaults for every question")
	fs.BoolVar(yesFlag, "y", false, "Alias for --yes")
	helpFlag := fs.Bool("help", false, "Show init help")
	fs.BoolVar(helpFlag, "h", false, "Alias for --help")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	target := "."
	if len(fs.Args()) > 0 {
		target = fs.Args()[0]
	}

	if *helpFlag {
		fmt.Fprintln(os.Stderr, "Usage: krit init [flags] [target-dir]")
		fs.PrintDefaults()
		return 0
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if info, err := os.Stat(absTarget); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: target %q is not a directory\n", target)
		return 2
	}

	repoRoot, err := findOnboardingRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	kritBin, err := resolveKritBin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	opts := onboarding.ScanOptions{
		KritBin:  kritBin,
		RepoRoot: repoRoot,
		Target:   absTarget,
	}

	// Non-interactive path: mirrors the gum script with --profile --yes.
	if *profileFlag != "" && *yesFlag {
		return runHeadlessInit(opts, reg, *profileFlag)
	}

	m := newInitModel(opts, reg, absTarget, *profileFlag, *yesFlag)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if fm, ok := finalModel.(initModel); ok && fm.err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", fm.err)
		return 1
	}
	return 0
}

// runHeadlessInit performs the full onboarding flow without a UI.
// Used by CI / integration tests and by callers who already know
// what profile they want and trust the per-profile defaults. Runs
// every phase (scan, write, autofix, baseline) inline so the
// headless output mirrors what the TUI does interactively.
func runHeadlessInit(opts onboarding.ScanOptions, reg *onboarding.Registry, profile string) int {
	known := false
	for _, p := range onboarding.ProfileNames {
		if p == profile {
			known = true
			break
		}
	}
	if !known {
		fmt.Fprintf(os.Stderr, "error: unknown profile %q (valid: %s)\n", profile, strings.Join(onboarding.ProfileNames, ", "))
		return 2
	}

	ctx := context.Background()
	res, err := onboarding.ScanProfile(ctx, opts, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	answers, err := onboarding.ResolveAnswers(reg, profile, func(q *onboarding.Question, def bool) bool {
		return def
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	overrides := onboarding.BuildOverrides(reg, answers)

	profileYAML, err := os.ReadFile(onboarding.ProfilePath(opts.RepoRoot, profile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading profile: %v\n", err)
		return 1
	}

	configPath, err := onboarding.WriteConfigFile(opts.Target, onboarding.WriteConfigOptions{
		ProfileYAML: profileYAML,
		ProfileName: profile,
		Overrides:   overrides,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: writing config: %v\n", err)
		return 1
	}
	fmt.Printf("wrote %s (profile=%s, findings=%d, overrides=%d)\n",
		configPath, profile, res.Total, len(overrides))

	// Autofix pass: run krit --fix for its side effect, no output.
	// Non-zero exit from krit when unfixable findings remain is expected.
	_ = exec.CommandContext(ctx, opts.KritBin, "--config", configPath, "--fix", opts.Target).Run()

	// Baseline: suppress the remaining findings.
	baselineDir := filepath.Join(opts.Target, ".krit")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkdir %s: %v\n", baselineDir, err)
		return 1
	}
	baselinePath := filepath.Join(baselineDir, "baseline.xml")
	_ = exec.CommandContext(ctx, opts.KritBin,
		"--config", configPath, "--create-baseline", baselinePath, opts.Target).Run()
	if _, err := os.Stat(baselinePath); err != nil {
		fmt.Fprintf(os.Stderr, "error: baseline not written: %v\n", err)
		return 1
	}
	fmt.Printf("baseline written to %s\n", baselinePath)
	return 0
}

// findOnboardingRepoRoot locates the krit repo root — the directory
// containing config/default-krit.yml. Resolution order:
//
//  1. KRIT_REPO_ROOT env var (for tests and odd install layouts)
//  2. Walking up from the executable's directory
//  3. Walking up from the current working directory
func findOnboardingRepoRoot() (string, error) {
	if env := os.Getenv("KRIT_REPO_ROOT"); env != "" {
		if _, err := os.Stat(filepath.Join(env, "config", "default-krit.yml")); err == nil {
			return env, nil
		}
	}

	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}

	for _, start := range candidates {
		dir := start
		for i := 0; i < 8; i++ {
			if _, err := os.Stat(filepath.Join(dir, "config", "default-krit.yml")); err == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("cannot locate krit repo root (set KRIT_REPO_ROOT or run near config/default-krit.yml)")
}

// resolveKritBin finds a krit binary to invoke. Prefers KRIT_BIN env
// var (used by tests), then the current executable, then PATH.
func resolveKritBin() (string, error) {
	if env := os.Getenv("KRIT_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	exe, err := os.Executable()
	if err == nil {
		if _, err := os.Stat(exe); err == nil {
			return exe, nil
		}
	}
	return "", fmt.Errorf("krit binary not found (set KRIT_BIN or run from the repo with ./krit)")
}
