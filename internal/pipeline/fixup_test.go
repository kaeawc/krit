package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestFixupPhase_Name(t *testing.T) {
	if got := (FixupPhase{}).Name(); got != "fixup" {
		t.Errorf("Name() = %q, want %q", got, "fixup")
	}
}

func TestFixupPhase_NoOp_WhenApplyFalse(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     "/tmp/doesnotexist.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TrailingWhitespace",
			Severity: "warning",
			Message:  "trailing whitespace",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val x = 1",
			},
		},
	})

	in := FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{
				Findings: columns,
			},
		},
		Apply:       false,
		ApplyBinary: false,
	}

	out, err := (FixupPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if out.AppliedFixes != 0 {
		t.Errorf("AppliedFixes = %d, want 0", out.AppliedFixes)
	}
	if len(out.ModifiedFiles) != 0 {
		t.Errorf("ModifiedFiles = %v, want empty", out.ModifiedFiles)
	}
	if len(out.FixErrors) != 0 {
		t.Errorf("FixErrors = %v, want empty", out.FixErrors)
	}
	// Findings must be unchanged (still carries the fix).
	if out.Findings.Len() != 1 {
		t.Fatalf("Findings.Len() = %d, want 1", out.Findings.Len())
	}
	if !out.Findings.HasFix(0) {
		t.Errorf("expected fix on row 0 to be preserved")
	}
}

func TestFixupPhase_AppliesCosmeticFix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")
	original := "val x  =  1\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     path,
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TrailingWhitespace",
			Severity: "warning",
			Message:  "double spaces",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val x = 1",
			},
		},
	})

	in := FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{
				Findings: columns,
			},
		},
		Apply:  true,
		Suffix: "",
	}

	out, err := (FixupPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if out.AppliedFixes != 1 {
		t.Errorf("AppliedFixes = %d, want 1", out.AppliedFixes)
	}
	if len(out.ModifiedFiles) != 1 || out.ModifiedFiles[0] != path {
		t.Errorf("ModifiedFiles = %v, want [%s]", out.ModifiedFiles, path)
	}
	if len(out.FixErrors) != 0 {
		t.Errorf("FixErrors = %v, want empty", out.FixErrors)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "val x = 1\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestFixupPhase_RespectsMaxFixLevel(t *testing.T) {
	// Sanity-check the real registry's fix levels for the rules we
	// piggyback on, so this test stays honest if they ever change.
	cosmeticLevel := rules.FixLevel(0)
	semanticLevel := rules.FixLevel(0)
	for _, r := range rules.Registry {
		switch r.Name() {
		case "TrailingWhitespace":
			cosmeticLevel = rules.GetFixLevel(r)
		case "BooleanPropertyNaming":
			semanticLevel = rules.GetFixLevel(r)
		}
	}
	if cosmeticLevel != rules.FixCosmetic {
		t.Fatalf("TrailingWhitespace fix level = %v, want FixCosmetic", cosmeticLevel)
	}
	if semanticLevel != rules.FixSemantic {
		t.Fatalf("BooleanPropertyNaming fix level = %v, want FixSemantic", semanticLevel)
	}

	dir := t.TempDir()
	cosmeticPath := filepath.Join(dir, "Cosmetic.kt")
	semanticPath := filepath.Join(dir, "Semantic.kt")
	cosmeticBefore := "val x  =  1\n"
	semanticBefore := "val flagDone = true\n"
	if err := os.WriteFile(cosmeticPath, []byte(cosmeticBefore), 0644); err != nil {
		t.Fatalf("WriteFile cosmetic: %v", err)
	}
	if err := os.WriteFile(semanticPath, []byte(semanticBefore), 0644); err != nil {
		t.Fatalf("WriteFile semantic: %v", err)
	}

	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     cosmeticPath,
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TrailingWhitespace",
			Severity: "warning",
			Message:  "cosmetic",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val x = 1",
			},
		},
		{
			File:     semanticPath,
			Line:     1,
			Col:      1,
			RuleSet:  "naming",
			Rule:     "BooleanPropertyNaming",
			Severity: "warning",
			Message:  "semantic",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val isDone = true",
			},
		},
	})

	in := FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{
				Findings: columns,
			},
		},
		Apply:       true,
		MaxFixLevel: rules.FixCosmetic,
	}

	out, err := (FixupPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if out.AppliedFixes != 1 {
		t.Errorf("AppliedFixes = %d, want 1 (only cosmetic)", out.AppliedFixes)
	}
	if len(out.ModifiedFiles) != 1 || out.ModifiedFiles[0] != cosmeticPath {
		t.Errorf("ModifiedFiles = %v, want [%s]", out.ModifiedFiles, cosmeticPath)
	}

	cosmeticAfter, err := os.ReadFile(cosmeticPath)
	if err != nil {
		t.Fatalf("ReadFile cosmetic: %v", err)
	}
	if want := "val x = 1\n"; string(cosmeticAfter) != want {
		t.Errorf("cosmetic file = %q, want %q", string(cosmeticAfter), want)
	}
	semanticAfter, err := os.ReadFile(semanticPath)
	if err != nil {
		t.Fatalf("ReadFile semantic: %v", err)
	}
	if string(semanticAfter) != semanticBefore {
		t.Errorf("semantic file changed: got %q, want %q (untouched)",
			string(semanticAfter), semanticBefore)
	}
}

func TestFixupPhase_PreservesFindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(path, []byte("val flagDone = true\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     path,
			Line:     1,
			Col:      1,
			RuleSet:  "naming",
			Rule:     "BooleanPropertyNaming",
			Severity: "warning",
			Message:  "stripped fix",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val isDone = true",
			},
		},
	})
	wantFindings := columns.Findings()

	in := FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{
				Findings: columns,
			},
		},
		Apply:       true,
		MaxFixLevel: rules.FixCosmetic,
	}

	out, err := (FixupPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if out.AppliedFixes != 0 {
		t.Errorf("AppliedFixes = %d, want 0 (semantic fix filtered out)", out.AppliedFixes)
	}
	if len(out.ModifiedFiles) != 0 {
		t.Errorf("ModifiedFiles = %v, want empty", out.ModifiedFiles)
	}

	// Finding must still be present, but its Fix pointer has been
	// stripped by the fix-level filter.
	if out.Findings.Len() != 1 {
		t.Fatalf("Findings.Len() = %d, want 1 (finding must survive filter)", out.Findings.Len())
	}
	if out.Findings.HasFix(0) {
		t.Errorf("expected fix to be stripped on row 0, but HasFix=true")
	}
	if got := out.Findings.RuleAt(0); got != "BooleanPropertyNaming" {
		t.Errorf("RuleAt(0) = %q, want %q", got, "BooleanPropertyNaming")
	}

	// Non-fix metadata should be preserved relative to the input.
	gotFindings := out.Findings.Findings()
	for i := range gotFindings {
		gotFindings[i].Fix = nil
		wantFindings[i].Fix = nil
	}
	if !reflect.DeepEqual(gotFindings, wantFindings) {
		t.Errorf("finding metadata changed after fix strip:\ngot:  %+v\nwant: %+v",
			gotFindings, wantFindings)
	}

	// File on disk must be untouched (fix was filtered).
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "val flagDone = true\n"; string(got) != want {
		t.Errorf("file content = %q, want %q (unchanged)", string(got), want)
	}
}
