package scan

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// captureStdout swaps os.Stdout for the duration of fn and returns what was
// written. Mirrors the helper used by other scan tests but kept local so this
// file does not depend on test-only exports from elsewhere.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}

// registerEvolvingRule registers a rule with StabilityEvolving so the audit
// report can flag baselined findings that hit it. The registry is global so we
// restore it after the test.
func registerEvolvingRule(t *testing.T, id string) {
	t.Helper()
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })
	api.Register(&api.Rule{
		ID:          id,
		Category:    "test",
		Description: "evolving rule for baseline audit test",
		Sev:         api.SeverityInfo,
		Stability:   api.StabilityEvolving,
		Check:       func(*api.Context) {},
	})
}

func TestBaselineWarnsEvolving(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Foo.kt")
	if err := os.WriteFile(file, []byte("fun demo() = 1\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	registerEvolvingRule(t, "TestEvolvingRule")

	findings := []scanner.Finding{{File: file, Rule: "TestEvolvingRule", Message: "evolving finding"}}
	columns := scanner.CollectFindings(findings)
	baseline := &scanner.Baseline{
		CurrentIssues: map[string]bool{
			scanner.BaselineID(findings[0], "", dir): true,
		},
		ManuallySuppressed: map[string]bool{},
	}
	baselinePath := filepath.Join(dir, "baseline.xml")

	out := captureStdout(t, func() {
		_ = RunBaselineAuditColumns(&columns, baseline, baselinePath, dir, []string{dir}, "plain")
	})

	if !strings.Contains(out, "Stability warnings:") {
		t.Fatalf("expected stability warnings header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TestEvolvingRule") || !strings.Contains(out, "stability=evolving") {
		t.Fatalf("expected evolving rule annotation in output, got:\n%s", out)
	}
}

func TestBaselineWarnsEvolving_JSONIncludesWarnings(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Foo.kt")
	if err := os.WriteFile(file, []byte("fun demo() = 1\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	registerEvolvingRule(t, "TestEvolvingRuleJSON")

	findings := []scanner.Finding{{File: file, Rule: "TestEvolvingRuleJSON", Message: "evolving finding"}}
	columns := scanner.CollectFindings(findings)
	baseline := &scanner.Baseline{
		CurrentIssues: map[string]bool{
			scanner.BaselineID(findings[0], "", dir): true,
		},
		ManuallySuppressed: map[string]bool{},
	}
	baselinePath := filepath.Join(dir, "baseline.xml")

	out := captureStdout(t, func() {
		_ = RunBaselineAuditColumns(&columns, baseline, baselinePath, dir, []string{dir}, "json")
	})

	var report struct {
		Warnings []struct {
			Stability string `json:"stability"`
			Entry     struct {
				Rule string `json:"Rule"`
			} `json:"entry"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode JSON: %v\nraw:\n%s", err, out)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("want 1 warning, got %d (raw: %s)", len(report.Warnings), out)
	}
	if report.Warnings[0].Stability != "evolving" || report.Warnings[0].Entry.Rule != "TestEvolvingRuleJSON" {
		t.Fatalf("unexpected warning entry: %+v", report.Warnings[0])
	}
}
