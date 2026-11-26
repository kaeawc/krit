package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
)

// TestComposeLintsParity checks the compose-lint-checks (slackhq/compose-lints) ports against the real detectors.
//
// Each directory under testdata/compose_lints holds Kotlin inputs, checks.txt (the issue IDs) and lint-1.6.1.txt: the
// findings the Lint CLI reported for those IDs on the same inputs, with com.slack.lint.compose:compose-lint-checks
// 1.6.1 on the rule path and the Compose, kotlin-stdlib, kotlinx-coroutines and android.jar classes on the
// classpath, one `path:line:column: ID: message` line per finding (message whitespace collapsed to one space).
// lint.xml, when present, holds the Lint options the CLI ran with, and krit-options.txt the same options for krit
// (`RuleID.option = a, b`). known-false-negatives.txt lists Lint findings krit deliberately does not report (a
// lint-1.6.1.txt line, then ` # reason`); krit must report exactly the remaining findings and nothing else.
//
// A case whose checks.txt names a krit rule that Lint IDs alias (alias.txt: `LintID = KritRuleID`) is compared on
// locations only, since the krit rule keeps its own messages; krit-only.txt lists `path:line:column # reason` for
// locations krit reports and Lint does not.
func TestComposeLintsParity(t *testing.T) {
	root := filepath.Join("testdata", "compose_lints")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			applyComposeLintsOptions(t, dir)
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			knownFN := map[string]bool{}
			for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "known-false-negatives.txt"))) {
				finding, _, _ := strings.Cut(line, " # ")
				knownFN[strings.TrimSpace(finding)] = true
			}
			aliases := map[string]string{}
			for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "alias.txt"))) {
				lintID, kritID, _ := strings.Cut(line, "=")
				aliases[strings.TrimSpace(lintID)] = strings.TrimSpace(kritID)
			}
			var want []string
			lintSet := map[string]bool{}
			for _, line := range parityLines(readParityFile(t, filepath.Join(dir, "lint-1.6.1.txt"))) {
				lintSet[line] = true
				if knownFN[line] {
					continue
				}
				if len(aliases) > 0 {
					line = composeLintsLocation(line)
				}
				want = append(want, line)
			}
			for fn := range knownFN {
				if !lintSet[fn] {
					t.Errorf("known-false-negatives.txt lists %q, which Lint does not report", fn)
				}
			}
			got := runParityChecks(t, dir, checks)
			if len(aliases) > 0 {
				for i, line := range got {
					got[i] = composeLintsLocation(line)
				}
				for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "krit-only.txt"))) {
					loc, _, _ := strings.Cut(line, " # ")
					want = append(want, strings.TrimSpace(loc))
				}
				sortStrings(want)
			}
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from compose-lints 1.6.1 (minus known false negatives)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

// composeLintsLocation trims a `path:line:column: ID: message` parity line to `path:line:column`.
func composeLintsLocation(line string) string {
	if i := strings.Index(line, ": "); i >= 0 {
		return line[:i]
	}
	return line
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// applyComposeLintsOptions applies the string-list options in a case's krit-options.txt to the registered rules and
// resets them to empty lists when the test ends.
func applyComposeLintsOptions(t *testing.T, dir string) {
	t.Helper()
	cfg := composeLintsOptions{}
	for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "krit-options.txt"))) {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("krit-options.txt: %q is not `RuleID.option = values`", line)
		}
		var values []string
		for _, v := range strings.Split(value, ",") {
			if v = strings.TrimSpace(v); v != "" {
				values = append(values, v)
			}
		}
		cfg[strings.TrimSpace(key)] = values
	}
	if len(cfg) == 0 {
		return
	}
	apply := func(c composeLintsOptions) {
		for _, r := range api.Registry {
			if mp, ok := r.Implementation.(api.MetaProvider); ok {
				api.ApplyConfig(r.Implementation, mp.Meta(), c)
			}
		}
	}
	apply(cfg)
	t.Cleanup(func() {
		reset := composeLintsOptions{}
		for key := range cfg {
			reset[key] = []string{}
		}
		apply(reset)
	})
}

// composeLintsOptions is an api.ConfigSource holding `RuleID.option` string lists.
type composeLintsOptions map[string][]string

func (c composeLintsOptions) GetInt(_, _, _ string, def int) int         { return def }
func (c composeLintsOptions) GetBool(_, _, _ string, def bool) bool      { return def }
func (c composeLintsOptions) GetString(_, _, _, def string) string       { return def }
func (c composeLintsOptions) IsRuleActive(_, _ string) *bool             { return nil }
func (c composeLintsOptions) IsRuleSetActive(_ string) *bool             { return nil }
func (c composeLintsOptions) GetStringList(_, rule, key string) []string { return c[rule+"."+key] }
func (c composeLintsOptions) HasKey(_, rule, key string) bool {
	_, ok := c[rule+"."+key]
	return ok
}
