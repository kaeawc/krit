package rules

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRulesListByLanguageAndStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"list", "--language", "java", "--status", "supported"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "AddJavascriptInterface") {
		t.Fatalf("expected supported rule AddJavascriptInterface in output:\n%s", out)
	}
	if strings.Contains(out, "LongMethod") {
		t.Fatalf("LongMethod is pending; should not be in supported-only listing:\n%s", out)
	}
	if !strings.Contains(out, "rule(s)") {
		t.Fatalf("expected summary in output:\n%s", out)
	}
}

func TestRunRulesListNegatedStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"list", "--language", "java", "--status", "!supported"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "AddJavascriptInterface\t") {
		// AddJavascriptInterface explicitly supported, must be excluded.
		t.Fatalf("supported rule leaked into !supported listing:\n%s", out)
	}
}

func TestRunRulesListUnknownStatusErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"list", "--language", "java", "--status", "bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected error exit, got 0; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Fatalf("expected error mentioning 'bogus' in stderr, got %q", stderr.String())
	}
}

func TestRunRulesListStatusRequiresLanguage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"list", "--status", "partial"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected error exit when --status is given without --language")
	}
	if !strings.Contains(stderr.String(), "language") {
		t.Fatalf("expected error to mention language requirement, got %q", stderr.String())
	}
}

func TestRunRulesCoverageJavaContainsHeaderAndTotals(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"coverage", "--language", "java"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "JAVA STATUS") {
		t.Fatalf("expected JAVA STATUS header in coverage output:\n%s", out)
	}
	if !strings.Contains(out, "Total:") {
		t.Fatalf("expected Total: summary in coverage output:\n%s", out)
	}
	// Every Kotlin-supported (= default-active) rule must appear in the table.
	if !strings.Contains(out, "AddJavascriptInterface") {
		t.Fatalf("coverage output missing AddJavascriptInterface:\n%s", out)
	}
	if !strings.Contains(out, "LongMethod") {
		t.Fatalf("coverage output missing LongMethod (ruleset-default classified):\n%s", out)
	}
}

func TestRunRulesUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"frobnicate"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown subcommand")
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Fatalf("expected unknown subcommand name in stderr, got %q", stderr.String())
	}
}

// TestRunRulesCoverageJavaGolden snapshots the coverage table head so a
// regression in registry ordering or status classification shows up loudly.
// The snapshot only locks the first few rows + summary so adding new
// classified rules doesn't churn the golden — only changes near the
// alphabetic prefix matter. Set KRIT_UPDATE_GOLDEN=1 to refresh.
func TestRunRulesCoverageJavaGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"coverage", "--language", "java"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	got := normalizeCoverageGolden(stdout.String())

	goldenPath := filepath.Join("testdata", "coverage_java.golden")
	if os.Getenv("KRIT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Skip("golden updated")
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (set KRIT_UPDATE_GOLDEN=1 to regenerate): %v", err)
	}
	if string(want) != got {
		t.Fatalf("coverage golden mismatch.\nDIFF:\n--- want\n%s\n--- got\n%s", string(want), got)
	}
}

// normalizeCoverageGolden strips the per-rule rows that follow the header
// and just keeps the header, the first three rows, and the summary block.
// The first three rows are stable (sorted by ID) and the totals are
// classification-driven, so the snapshot stays meaningful without
// turning into a giant churn magnet.
func normalizeCoverageGolden(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return s
	}
	var kept []string
	// Header.
	kept = append(kept, lines[0])
	// First three data rows (or fewer).
	dataCount := 0
	idx := 1
	for ; idx < len(lines) && dataCount < 3; idx++ {
		if lines[idx] == "" {
			continue
		}
		kept = append(kept, lines[idx])
		dataCount++
	}
	// Walk forward to the blank line that separates rows from the summary.
	for ; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "" {
			kept = append(kept, "...")
			kept = append(kept, lines[idx])
			break
		}
	}
	// Summary tail.
	for idx++; idx < len(lines); idx++ {
		kept = append(kept, lines[idx])
	}
	return strings.Join(kept, "\n")
}
