package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func captureCLIOutput(t *testing.T, fn func() int) (stdout string, stderr string, code int) {
	t.Helper()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	code = fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return string(stdoutBytes), string(stderrBytes), code
}

func TestRunSampleFindingsColumns_MatchesSliceOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(file, []byte("fun demo() {\n    val a = 1\n    val b = 2\n}\n"), 0644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	findings := []scanner.Finding{
		{File: file, Line: 2, Col: 9, Rule: "UnusedVariable", Message: "a is unused"},
		{File: file, Line: 3, Col: 9, Rule: "UnusedVariable", Message: "b is unused"},
		{File: file, Line: 1, Col: 1, Rule: "OtherRule", Message: "ignore"},
	}
	columns := scanner.CollectFindings(findings)

	wantStdout, wantStderr, wantCode := captureCLIOutput(t, func() int {
		return runSampleFindings(findings, "UnusedVariable", 2, 1, dir)
	})
	gotStdout, gotStderr, gotCode := captureCLIOutput(t, func() int {
		return runSampleFindingsColumns(&columns, "UnusedVariable", 2, 1, dir)
	})

	if gotCode != wantCode {
		t.Fatalf("exit code mismatch: want %d, got %d", wantCode, gotCode)
	}
	if gotStdout != wantStdout {
		t.Fatalf("stdout mismatch:\nwant:\n%s\ngot:\n%s", wantStdout, gotStdout)
	}
	if gotStderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant:\n%s\ngot:\n%s", wantStderr, gotStderr)
	}
}

func TestRunBaselineAuditColumns_MatchesSliceOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Foo.kt")
	if err := os.WriteFile(file, []byte("fun demo() = 1\n"), 0644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	findings := []scanner.Finding{
		{File: file, Rule: "MagicNumber", Message: "avoid magic numbers"},
	}
	columns := scanner.CollectFindings(findings)
	baseline := &scanner.Baseline{
		CurrentIssues: map[string]bool{
			scanner.BaselineID(findings[0], "", dir):   true,
			"DeletedRule:Missing.kt:$DeletedRule$gone": true,
		},
		ManuallySuppressed: map[string]bool{},
	}
	baselinePath := filepath.Join(dir, "baseline.xml")

	wantStdout, wantStderr, wantCode := captureCLIOutput(t, func() int {
		return runBaselineAudit(findings, baseline, baselinePath, dir, []string{dir}, "plain")
	})
	gotStdout, gotStderr, gotCode := captureCLIOutput(t, func() int {
		return runBaselineAuditColumns(&columns, baseline, baselinePath, dir, []string{dir}, "plain")
	})

	if gotCode != wantCode {
		t.Fatalf("exit code mismatch: want %d, got %d", wantCode, gotCode)
	}
	if gotStdout != wantStdout {
		t.Fatalf("stdout mismatch:\nwant:\n%s\ngot:\n%s", wantStdout, gotStdout)
	}
	if gotStderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant:\n%s\ngot:\n%s", wantStderr, gotStderr)
	}
}

func TestRunRuleAuditColumns_MatchesSliceOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Audit.kt")
	if err := os.WriteFile(file, []byte("fun demo() {\n    val value = 1\n}\n"), 0644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	findings := []scanner.Finding{
		{File: file, Line: 2, Col: 9, Rule: "UnusedVariable", Message: "value is unused"},
		{File: file, Line: 1, Col: 1, Rule: "OtherRule", Message: "other finding"},
		{File: file, Line: 2, Col: 5, Rule: "UnusedVariable", Message: "value is still unused"},
	}
	columns := scanner.CollectFindings(findings)
	opts := ruleAuditOpts{
		DetailRules:    1,
		SamplesPerRule: 2,
		SampleContext:  1,
		Targets:        []string{dir},
		Format:         "plain",
	}

	wantStdout, wantStderr, wantCode := captureCLIOutput(t, func() int {
		return runRuleAudit(findings, opts)
	})
	gotStdout, gotStderr, gotCode := captureCLIOutput(t, func() int {
		return runRuleAuditColumns(&columns, opts)
	})

	if gotCode != wantCode {
		t.Fatalf("exit code mismatch: want %d, got %d", wantCode, gotCode)
	}
	if gotStdout != wantStdout {
		t.Fatalf("stdout mismatch:\nwant:\n%s\ngot:\n%s", wantStdout, gotStdout)
	}
	if gotStderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant:\n%s\ngot:\n%s", wantStderr, gotStderr)
	}
}

func TestRunDeadCodeRemovalColumns_MatchesSliceOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Dead.kt")
	content := "fun dead() = 1\n\nfun live() = 2\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	removeEnd := len(content) - len("fun live() = 2\n")
	findings := []scanner.Finding{
		{
			File:    file,
			Line:    1,
			Rule:    "DeadCode",
			Message: "Public function 'dead' appears to be unused. It is not referenced from any other file.",
			Fix: &scanner.Fix{
				ByteMode:    true,
				StartByte:   0,
				EndByte:     removeEnd,
				Replacement: "",
			},
		},
		{
			File:    file,
			Line:    1,
			Rule:    "ModuleDeadCode",
			Message: "Public function 'dead' in module :app is not used by any module (including itself).",
		},
	}
	columns := scanner.CollectFindings(findings)

	wantStdout, wantStderr, wantCode := captureCLIOutput(t, func() int {
		return runDeadCodeRemoval(findings, "json", true, "")
	})
	gotStdout, gotStderr, gotCode := captureCLIOutput(t, func() int {
		return runDeadCodeRemovalColumns(&columns, "json", true, "")
	})

	if gotCode != wantCode {
		t.Fatalf("exit code mismatch: want %d, got %d", wantCode, gotCode)
	}
	if gotStdout != wantStdout {
		t.Fatalf("stdout mismatch:\nwant:\n%s\ngot:\n%s", wantStdout, gotStdout)
	}
	if gotStderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant:\n%s\ngot:\n%s", wantStderr, gotStderr)
	}
}

type testFixLevelRule struct {
	rules.BaseRule
	rules.FlatDispatchBase
	level rules.FixLevel
}

func (r *testFixLevelRule) FixLevel() rules.FixLevel { return r.level }

func TestFilterFixesByLevelColumns_StripsOnlyDisallowedTextFixes(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     "a.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "CosmeticRule",
			Severity: "warning",
			Message:  "keep text fix",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "kept()",
			},
		},
		{
			File:     "a.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "style",
			Rule:     "SemanticRule",
			Severity: "warning",
			Message:  "drop text fix",
			Fix: &scanner.Fix{
				StartLine:   2,
				EndLine:     2,
				Replacement: "dropped()",
			},
		},
		{
			File:     "a.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "BinaryRule",
			Severity: "warning",
			Message:  "binary fix remains",
			BinaryFix: &scanner.BinaryFix{
				Type:    scanner.BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
	})

	registry := []rules.Rule{
		&testFixLevelRule{
			BaseRule: rules.BaseRule{RuleName: "CosmeticRule", RuleSetName: "style", Sev: "warning"},
			level:    rules.FixCosmetic,
		},
		&testFixLevelRule{
			BaseRule: rules.BaseRule{RuleName: "SemanticRule", RuleSetName: "style", Sev: "warning"},
			level:    rules.FixSemantic,
		},
		&testFixLevelRule{
			BaseRule: rules.BaseRule{RuleName: "BinaryRule", RuleSetName: "style", Sev: "warning"},
			level:    rules.FixSemantic,
		},
	}

	fixableCount, strippedByLevel := filterFixesByLevelColumns(&columns, registry, rules.FixCosmetic)

	if strippedByLevel != 1 {
		t.Fatalf("strippedByLevel = %d, want 1", strippedByLevel)
	}
	if fixableCount != 1 {
		t.Fatalf("fixableCount = %d, want 1", fixableCount)
	}
	if !columns.HasFix(0) {
		t.Fatal("expected cosmetic fix to remain")
	}
	if columns.HasFix(1) {
		t.Fatal("expected semantic fix to be stripped")
	}
	if columns.BinaryFixStart[2] == 0 {
		t.Fatal("expected binary fix to remain available")
	}

	got := columns.Findings()
	want := []scanner.Finding{
		{
			File:     "a.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "CosmeticRule",
			Severity: "warning",
			Message:  "keep text fix",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "kept()",
			},
		},
		{
			File:     "a.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "style",
			Rule:     "SemanticRule",
			Severity: "warning",
			Message:  "drop text fix",
		},
		{
			File:     "a.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "BinaryRule",
			Severity: "warning",
			Message:  "binary fix remains",
			BinaryFix: &scanner.BinaryFix{
				Type:    scanner.BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered columns mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}
