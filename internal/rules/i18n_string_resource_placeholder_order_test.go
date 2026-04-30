package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestAnalyzeFormatPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		total    int
		positive bool
	}{
		{"none", "hello", 0, false},
		{"escaped", "100%% off", 0, false},
		{"single non-positional", "%s", 1, false},
		{"two non-positional", "%s, %s", 2, false},
		{"two positional", "%1$s, %2$s", 2, true},
		{"mixed conversion types", "%1$d items at %2$.2f", 2, true},
		{"trailing percent", "abc %", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := analyzeFormatPlaceholders(tc.in)
			if info.total != tc.total {
				t.Fatalf("total = %d, want %d", info.total, tc.total)
			}
			if info.hasPositional != tc.positive {
				t.Fatalf("hasPositional = %v, want %v", info.hasPositional, tc.positive)
			}
		})
	}
}

func TestStringResourcePlaceholderOrder_VariantDropsPositional(t *testing.T) {
	dir := writePlaceholderFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="greet">%1$s, %2$s</string></resources>`,
		"values-fr/strings.xml": `<resources><string name="greet">%s, %s</string></resources>`,
	})
	findings := runPlaceholderOrderRule(t, dir)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "values-fr/") {
		t.Fatalf("expected message to mention values-fr/, got: %s", findings[0].Message)
	}
}

func TestStringResourcePlaceholderOrder_BothPositionalIsClean(t *testing.T) {
	dir := writePlaceholderFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="greet">%1$s, %2$s</string></resources>`,
		"values-fr/strings.xml": `<resources><string name="greet">%2$s, %1$s</string></resources>`,
	})
	findings := runPlaceholderOrderRule(t, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestStringResourcePlaceholderOrder_BothNonPositionalIsClean(t *testing.T) {
	dir := writePlaceholderFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="greet">%s, %s</string></resources>`,
		"values-fr/strings.xml": `<resources><string name="greet">%s, %s</string></resources>`,
	})
	findings := runPlaceholderOrderRule(t, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestStringResourcePlaceholderOrder_VariantSingleSpecifierIsClean(t *testing.T) {
	dir := writePlaceholderFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="greet">%1$s and %2$s</string></resources>`,
		"values-fr/strings.xml": `<resources><string name="greet">%s</string></resources>`,
	})
	findings := runPlaceholderOrderRule(t, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestStringResourcePlaceholderOrder_FormattedFalseSkipped(t *testing.T) {
	dir := writePlaceholderFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="greet">%1$s, %2$s</string></resources>`,
		"values-fr/strings.xml": `<resources><string name="greet" formatted="false">%s, %s</string></resources>`,
	})
	findings := runPlaceholderOrderRule(t, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func writePlaceholderFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

func runPlaceholderOrderRule(t *testing.T, resRoot string) []scanner.Finding {
	t.Helper()
	r := &StringResourcePlaceholderOrderRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{
			RuleName:    "StringResourcePlaceholderOrder",
			RuleSetName: "i18n",
			Sev:         "warning",
		},
	}}
	idx := &android.ResourceIndex{
		StringsLocation: map[string]android.StringLocation{
			"__seed__": {
				FilePath: filepath.Join(resRoot, "values", "strings.xml"),
				Line:     1,
			},
		},
	}
	collector := scanner.NewFindingCollector(0)
	ctx := &v2.Context{ResourceIndex: idx, Collector: collector}
	r.check(ctx)
	return v2.ContextFindings(ctx)
}
