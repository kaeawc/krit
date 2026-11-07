package rules_test

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestKotlin2xCompatFixtures runs rules over synthetic Kotlin 2.x sources (explicit backing fields, `when`
// guards, context parameters, data objects, non-local jumps, multi-dollar strings, name-based destructuring) and
// holds each rule to the findings the fixture marks. A fixture starts with `// rules: A, B` naming the rules to
// run, and marks a line that must be reported with a trailing `// expect: RuleName[, RuleName]`. Every other
// line of those rules must stay silent, so a construct that breaks a rule shows up as an unexpected finding.
func TestKotlin2xCompatFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "kotlin2x_compat", "*.kt"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no kotlin2x compat fixtures found: %v", err)
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".kt"), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			var ruleNames []string
			want := map[string]bool{}
			for i, line := range strings.Split(source, "\n") {
				if rest, ok := strings.CutPrefix(line, "// rules:"); ok {
					for _, name := range strings.Split(rest, ",") {
						ruleNames = append(ruleNames, strings.TrimSpace(name))
					}
				}
				if _, rest, ok := strings.Cut(line, "// expect:"); ok {
					for _, name := range strings.Split(rest, ",") {
						want[strings.TrimSpace(name)+"@"+strconv.Itoa(i+1)] = true
					}
				}
			}
			if len(ruleNames) == 0 {
				t.Fatal("fixture names no rules")
			}
			got := map[string]bool{}
			for _, name := range ruleNames {
				for _, f := range runRuleByNameWithResolver(t, name, source) {
					got[f.Rule+"@"+strconv.Itoa(f.Line)] = true
				}
			}
			for key := range want {
				if !got[key] {
					t.Errorf("missing finding %s", key)
				}
			}
			var extra []string
			for key := range got {
				if !want[key] {
					extra = append(extra, key)
				}
			}
			sort.Strings(extra)
			for _, key := range extra {
				t.Errorf("unexpected finding %s", key)
			}
		})
	}
}
