package rules

import (
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestNoTabs_FixHonorsConfiguredIndentSize ensures the fix replaces tab
// characters with the configured number of spaces, not a hardcoded 4.
func TestNoTabs_FixHonorsConfiguredIndentSize(t *testing.T) {
	cases := []struct {
		name   string
		indent int
		want   string
	}{
		{"default", 0, "    println(\"hi\")"},
		{"two", 2, "  println(\"hi\")"},
		{"eight", 8, "        println(\"hi\")"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := parseInlineKt(t, "package test\nfun main() {\n\tprintln(\"hi\")\n}\n")
			rule := &NoTabsRule{
				BaseRule:   BaseRule{RuleName: "NoTabs", RuleSetName: "style", Sev: "warning"},
				IndentSize: tc.indent,
			}
			v2rule := &v2.Rule{
				ID: "NoTabs", Category: "style", Sev: v2.Severity("warning"),
				Needs: v2.NeedsLinePass, Fix: v2.FixCosmetic, OriginalV1: rule,
				Check: rule.check,
			}
			d := NewDispatcherV2([]*v2.Rule{v2rule})
			cols := d.Run(file)
			findings := cols.Findings()
			if len(findings) == 0 {
				t.Fatal("expected a finding")
			}
			fix := findings[0].Fix
			if fix == nil {
				t.Fatal("expected a fix")
			}
			if fix.Replacement != tc.want {
				t.Fatalf("indent=%d replacement = %q, want %q", tc.indent, fix.Replacement, tc.want)
			}
		})
	}
}

func parseInlineKt(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
