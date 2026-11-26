package rules

import (
	"bytes"
	"fmt"
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
// The snapshot locks only the header and first few rows, so adding new
// classified rules doesn't churn the golden: only changes near the
// alphabetic prefix matter. The summary counts change with every added rule
// (parallel rule ports all conflicted on them), so instead of snapshotting
// them, checkCoverageSummary verifies they agree with the rows. Set
// KRIT_UPDATE_GOLDEN=1 to refresh.
func TestRunRulesCoverageJavaGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"coverage", "--language", "java"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if err := checkCoverageSummary(stdout.String()); err != nil {
		t.Fatal(err)
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
	// The rows that follow and the summary block are left out; see
	// checkCoverageSummary.
	kept = append(kept, "...", "")
	return strings.Join(kept, "\n")
}

// checkCoverageSummary verifies the coverage summary against the table it
// summarizes: every status count equals the number of rows with that status,
// the statuses add up to the total, and the total equals the row count.
func checkCoverageSummary(out string) error {
	rows, summary, ok := strings.Cut(out, "\n\nTotal: ")
	if !ok {
		return fmt.Errorf("coverage output has no \"Total:\" summary:\n%s", out)
	}
	rowLines := strings.Split(strings.TrimRight(rows, "\n"), "\n")[1:] // drop the header
	byStatus := map[string]int{}
	for _, line := range rowLines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return fmt.Errorf("malformed coverage row %q", line)
		}
		byStatus[fields[2]]++
	}
	summaryLines := strings.Split(strings.TrimSpace(summary), "\n")
	var total int
	if _, err := fmt.Sscanf(summaryLines[0], "%d rule(s)", &total); err != nil {
		return fmt.Errorf("parse total %q: %w", summaryLines[0], err)
	}
	if total != len(rowLines) {
		return fmt.Errorf("summary total %d, table has %d rows", total, len(rowLines))
	}
	sum := 0
	for _, line := range summaryLines[1:] {
		name, countText, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		var count int
		if _, err := fmt.Sscanf(countText, "%d", &count); err != nil {
			return fmt.Errorf("parse summary line %q: %w", line, err)
		}
		sum += count
		if want := byStatus[name]; count != want {
			return fmt.Errorf("summary %s = %d, table has %d rows with that status", name, count, want)
		}
	}
	if sum != total {
		return fmt.Errorf("summary statuses add up to %d, total is %d", sum, total)
	}
	return nil
}

func TestCheckCoverageSummaryCatchesMismatches(t *testing.T) {
	table := "RULE CATEGORY JAVA REASON\nA style supported x\nB style partial y\nC style partial z\n\n"
	good := table + "Total: 3 rule(s)\n  supported: 1\n  partial: 2\n"
	if err := checkCoverageSummary(good); err != nil {
		t.Fatalf("consistent summary rejected: %v", err)
	}
	for name, bad := range map[string]string{
		"wrong total":    table + "Total: 4 rule(s)\n  supported: 1\n  partial: 3\n",
		"wrong status":   table + "Total: 3 rule(s)\n  supported: 2\n  partial: 1\n",
		"missing status": table + "Total: 3 rule(s)\n  supported: 1\n",
		"no summary":     table,
	} {
		if err := checkCoverageSummary(bad); err == nil {
			t.Errorf("%s: inconsistent summary accepted", name)
		}
	}
}
