package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_FixupNoOpByDefault is the contract that the new
// fixup wiring (#70 Step B) stays inert for callers that don't set
// any fix knob. The daemon's analyze-project verb relies on this:
// it must produce identical output before and after Step B.
func TestRunProject_FixupNoOpByDefault(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Foo.kt")
	const content = "package demo\n\nclass Foo : Any()\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.Fixup.AppliedFixes != 0 {
		t.Errorf("default RunProject must not apply fixes; AppliedFixes=%d", res.Fixup.AppliedFixes)
	}
	if res.Fixup.FixableCount != 0 {
		t.Errorf("default RunProject must not count fixes; FixableCount=%d", res.Fixup.FixableCount)
	}
	// Fixture file must be untouched.
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(got) != content {
		t.Errorf("file was modified without --fix:\nwant=%q\ngot =%q", content, got)
	}
}

// TestRunProject_DryRunCountsButDoesNotApply asserts that
// Args.DryRun causes FixupPhase to count fixable findings without
// touching the filesystem. Mirrors the CLI's `--dry-run` semantics.
func TestRunProject_DryRunCountsButDoesNotApply(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Foo.kt")
	const content = "package demo\n\nclass Foo : Any()\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
			DryRun:      true,
			MaxFixLevel: rules.FixIdiomatic,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.Fixup.FixableCount == 0 {
		t.Fatalf("expected at least one fixable finding for UnnecessaryInheritance, got 0\nfindings=%#v",
			res.FinalFindings.Findings())
	}
	if res.Fixup.AppliedFixes != 0 {
		t.Errorf("--dry-run must not apply fixes; AppliedFixes=%d", res.Fixup.AppliedFixes)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(got) != content {
		t.Errorf("file was modified during --dry-run:\nwant=%q\ngot =%q", content, got)
	}
}

// TestRunProject_FixAppliesText confirms that Args.Fix=true causes
// FixupPhase to actually rewrite the file on disk and report the
// applied count back through ProjectResult.Fixup.
func TestRunProject_FixAppliesText(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Foo.kt")
	const content = "package demo\n\nclass Foo : Any()\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
			Fix:         true,
			MaxFixLevel: rules.FixIdiomatic,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.Fixup.AppliedFixes == 0 {
		t.Fatalf("expected AppliedFixes > 0 with Args.Fix=true; got 0\nfixup=%#v", res.Fixup)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(got) == content {
		t.Errorf("file was NOT modified despite --fix:\n%s", content)
	}
}
