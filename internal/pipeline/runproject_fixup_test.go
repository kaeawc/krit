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

const fixupFixtureContent = "package demo\n\nclass Foo : Any()\n"

// setupFixupFixture writes a single-file Kotlin fixture that
// triggers UnnecessaryInheritance (FixIdiomatic) and returns the
// tempdir root, the file path, and the active rule.
func setupFixupFixture(t *testing.T) (string, string, *api.Rule) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(file, []byte(fixupFixtureContent), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root, file, findV2RuleForTest(t, "UnnecessaryInheritance")
}

func TestRunProject_FixupNoOpByDefault(t *testing.T) {
	root, file, rule := setupFixupFixture(t)
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
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(got) != fixupFixtureContent {
		t.Errorf("file was modified without --fix:\nwant=%q\ngot =%q", fixupFixtureContent, got)
	}
}

func TestRunProject_DryRunCountsButDoesNotApply(t *testing.T) {
	root, file, rule := setupFixupFixture(t)
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
	if string(got) != fixupFixtureContent {
		t.Errorf("file was modified during --dry-run:\nwant=%q\ngot =%q", fixupFixtureContent, got)
	}
}

func TestRunProject_FixAppliesText(t *testing.T) {
	root, file, rule := setupFixupFixture(t)
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
	if string(got) == fixupFixtureContent {
		t.Errorf("file was NOT modified despite --fix:\n%s", fixupFixtureContent)
	}
}
