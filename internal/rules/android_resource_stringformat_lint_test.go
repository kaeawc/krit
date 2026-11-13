package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// resourceFindings scans resource files and returns "file:line message" for a rule.
func resourceFindings(t *testing.T, ruleID string, files map[string]string) []string {
	t.Helper()
	findings, resDir := resourceRuleFindings(t, ruleID, files)
	var out []string
	for _, f := range findings {
		rel, _ := filepath.Rel(resDir, f.File)
		out = append(out, filepath.ToSlash(rel)+":"+fmt.Sprintf("%02d", f.Line)+" "+f.Message)
	}
	sort.Strings(out)
	return out
}

// resourceFindingColumns returns the sorted "file:line:column" of every finding a rule reports on resource files.
func resourceFindingColumns(t *testing.T, ruleID string, files map[string]string) string {
	t.Helper()
	findings, resDir := resourceRuleFindings(t, ruleID, files)
	var out []string
	for _, f := range findings {
		rel, _ := filepath.Rel(resDir, f.File)
		out = append(out, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), f.Line, f.Col))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func resourceRuleFindings(t *testing.T, ruleID string, files map[string]string) ([]scanner.Finding, string) {
	t.Helper()
	resDir := filepath.Join(t.TempDir(), "res")
	for name, body := range files {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	return runResourceRule(findResourceRule(t, ruleID), idx), resDir
}

const stringFormatDefaults = `<resources>
  <string name="greet">Hello %1$s, you have %2$d items</string>
  <string name="skip">%1$s and %3$s</string>
  <string name="mix">%2$s then %s then %d</string>
  <string name="lt">%1$s %&lt;s</string>
  <string name="nf" formatted="false">%s here</string>
  <string name="plain">no format</string>
  <string name="pct">100% sure</string>
  <string name="odd">%z then %s</string>
</resources>`

// The expectations below were produced by running Android Lint 32.4.1 on the same files.
func TestStringFormatLintParity(t *testing.T) {
	files := map[string]string{
		"values/strings.xml": stringFormatDefaults,
		"values-fr/strings.xml": `<resources>
  <string name="greet">Bonjour %1$d</string>
  <string name="pct">100% sur</string>
  <string name="odd">%s</string>
</resources>`,
		"values-de/strings.xml": `<resources>
  <string name="greet">Hallo</string>
  <string name="pct">100%% sicher</string>
  <string name="nf">%s here</string>
</resources>`,
		"values-night/strings.xml": `<resources>
  <string name="greet">Night %1$s</string>
  <string name="plain">%d now</string>
</resources>`,
	}

	t.Run("StringFormatCount", func(t *testing.T) {
		got := resourceFindings(t, "StringFormatCountResource", files)
		want := []string{
			"values-de/strings.xml:02 Inconsistent number of arguments in formatting string `greet`; found both 0 here and 2 in values/strings.xml",
			"values-de/strings.xml:03 Inconsistent number of arguments in formatting string `pct`; found both 0 here and 1 in values/strings.xml",
			"values-de/strings.xml:04 Inconsistent number of arguments in formatting string `nf`; found both 1 here and 0 in values/strings.xml",
			"values-fr/strings.xml:02 Inconsistent number of arguments in formatting string `greet`; found both 1 here and 2 in values/strings.xml",
			"values-night/strings.xml:02 Inconsistent number of arguments in formatting string `greet`; found both 1 here and 2 in values/strings.xml",
			"values-night/strings.xml:03 Inconsistent number of arguments in formatting string `plain`; found both 1 here and 0 in values/strings.xml",
			"values/strings.xml:03 Formatting string '`skip`' is not referencing numbered arguments [2]",
			"values/strings.xml:04 Formatting string '`mix`' is not referencing numbered arguments [1]",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	})

	t.Run("StringFormatMatches", func(t *testing.T) {
		got := resourceFindings(t, "StringFormatMatchesResource", files)
		want := "values-fr/strings.xml:02 Inconsistent formatting types for argument #1 in format string `greet` ('%1$d'): Found both '`d`' here and '`s`' in values/strings.xml"
		if len(got) != 1 || got[0] != want {
			t.Fatalf("got %v, want [%s]", got, want)
		}
	})

	t.Run("StringFormatInvalid", func(t *testing.T) {
		got := resourceFindings(t, "StringFormatInvalidResource", files)
		want := []string{
			"values-fr/strings.xml:03 Incorrect formatting string `pct`; missing conversion character in '`% s`'?",
			"values/strings.xml:08 Incorrect formatting string `pct`; missing conversion character in '`% s`'?",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	})
}

func TestStringFormatDefaultSeverities(t *testing.T) {
	// Lint: StringFormatInvalid and StringFormatMatches are errors, StringFormatCount is a warning,
	// and StringFormatTrivial is disabled by default.
	for id, want := range map[string]string{
		"StringFormatInvalidResource": "error", "StringFormatMatchesResource": "error", "StringFormatCountResource": "warning",
	} {
		if got := string(findResourceRule(t, id).Sev); got != want {
			t.Errorf("%s severity = %s, want %s", id, got, want)
		}
	}
}
