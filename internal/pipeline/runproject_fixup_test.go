package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestRunProject_FixSkipsFindingInParseErrorRegion(t *testing.T) {
	root := t.TempDir()
	brokenPath := filepath.Join(root, "Broken.kt")
	cleanPath := filepath.Join(root, "Clean.kt")
	brokenSource := []byte("class Broken : Any() { val value = # }\n")
	cleanSource := []byte("class Clean\n")
	if err := os.WriteFile(brokenPath, brokenSource, 0o644); err != nil {
		t.Fatalf("write broken fixture: %v", err)
	}
	if err := os.WriteFile(cleanPath, cleanSource, 0o644); err != nil {
		t.Fatalf("write clean fixture: %v", err)
	}
	parsedBroken, err := scanner.ParseFile(context.Background(), brokenPath)
	if err != nil {
		t.Fatalf("parse broken fixture: %v", err)
	}
	var hasAnchoringErrorRegion bool
	for idx := uint32(0); idx < uint32(parsedBroken.FlatTree.Len()); idx++ {
		node := parsedBroken.FlatTree.Node(idx)
		if node.IsErrorNode() && node.EndByte > node.StartByte && parsedBroken.OffsetInErrorRegion(0) {
			hasAnchoringErrorRegion = true
			break
		}
	}
	if !hasAnchoringErrorRegion {
		t.Fatal("broken fixture produced no non-empty ERROR/MISSING span covering the finding anchor")
	}

	var verbose bytes.Buffer
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{findV2RuleForTest(t, "AbsentOrWrongFileLicense")},
			Format:      "json",
			Version:     "test",
			Fix:         true,
			MaxFixLevel: rules.FixCosmetic,
		},
		Host: ProjectHostState{Reporter: &diag.Reporter{Verbose: &verbose}},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}

	if res.Fixup.AppliedFixes != 1 {
		t.Fatalf("AppliedFixes = %d, want 1 (clean finding only); fixup=%#v", res.Fixup.AppliedFixes, res.Fixup)
	}
	if len(res.Fixup.ModifiedFiles) != 1 || res.Fixup.ModifiedFiles[0] != cleanPath {
		t.Fatalf("ModifiedFiles = %v, want [%s]", res.Fixup.ModifiedFiles, cleanPath)
	}
	gotBroken, err := os.ReadFile(brokenPath)
	if err != nil {
		t.Fatalf("read broken fixture: %v", err)
	}
	if !bytes.Equal(gotBroken, brokenSource) {
		t.Fatalf("broken fixture was modified:\nwant=%q\ngot =%q", brokenSource, gotBroken)
	}
	gotClean, err := os.ReadFile(cleanPath)
	if err != nil {
		t.Fatalf("read clean fixture: %v", err)
	}
	if bytes.Equal(gotClean, cleanSource) || !bytes.HasPrefix(gotClean, []byte("/* Copyright */\n")) {
		t.Fatalf("clean fixture did not receive the expected fix:\n%s", gotClean)
	}

	if res.FinalFindings.Len() != 1 || res.FinalFindings.FileAt(0) != cleanPath {
		t.Fatalf("FinalFindings = %#v, want only the clean-file finding", res.FinalFindings.Findings())
	}
	if res.Stats.FindingsInErrorRegions != 1 {
		t.Fatalf("FindingsInErrorRegions = %d, want 1", res.Stats.FindingsInErrorRegions)
	}
	const droppedLine = "verbose: 1 finding(s) dropped: anchored inside a parse-error region\n"
	if got := strings.Count(verbose.String(), droppedLine); got != 1 {
		t.Fatalf("parse-error drop verbose line count = %d, want 1; verbose=%q", got, verbose.String())
	}
}

func TestRunProjectAnalysis_FiltersCrossFileFindingInParseErrorRegion(t *testing.T) {
	root := t.TempDir()
	brokenPath := filepath.Join(root, "Broken.kt")
	if err := os.WriteFile(brokenPath, []byte("class Broken : Any() { val value = # }\n"), 0o644); err != nil {
		t.Fatalf("write broken fixture: %v", err)
	}

	lateRule := api.FakeRule(
		"LateErrorRegionFinding",
		api.WithNeeds(api.NeedsParsedFiles),
		api.WithCheck(func(ctx *api.Context) {
			file := ctx.ParsedFiles[0]
			ctx.Emit(scanner.Finding{
				File:      file.Path,
				Line:      1,
				Col:       1,
				StartByte: 0,
				Message:   "late finding inside parser recovery",
			})
		}),
	)
	analysis, err := RunProjectAnalysis(context.Background(), ProjectInput{Args: ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{findV2RuleForTest(t, "AbsentOrWrongFileLicense"), lateRule},
		Format:      "json",
		Version:     "test",
	}})
	if err != nil {
		t.Fatalf("RunProjectAnalysis: %v", err)
	}
	if got := analysis.CrossFileResult.Findings.Len(); got != 0 {
		t.Fatalf("CrossFileResult.Findings.Len() = %d, want 0", got)
	}
	if got := analysis.DispatchResult.Stats.FindingsInErrorRegions; got != 1 {
		t.Fatalf("dispatch FindingsInErrorRegions = %d, want 1", got)
	}
	if got := analysis.Stats.FindingsInErrorRegions; got != 2 {
		t.Fatalf("total FindingsInErrorRegions = %d, want 2", got)
	}
}
