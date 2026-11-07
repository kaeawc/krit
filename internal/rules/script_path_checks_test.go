package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var scriptSuffixCheckRe = regexp.MustCompile(`HasSuffix\([^,()]*(?:\([^()]*\))?[^,()]*,\s*"\.(?:gradle\.|main\.)?kts"\)`)

// scriptSuffixAllowed lists the rule source files that may test a path for a
// .kts suffix. A rule that skips scripts declares it in the script scope table
// (api.RuleScriptScope) instead, so Gradle and standalone scripts, config and
// --list-rules all see the same policy. These files test the suffix for another
// reason.
var scriptSuffixAllowed = map[string]string{
	"android_gradle_dsl.go":                            "Gradle-language rules that apply to the Kotlin DSL only",
	"gradle_kotlin_support.go":                         "Gradle-language fixes that rewrite the Kotlin DSL only",
	"potentialbugs_nullsafety_casts_never_succeeds.go": "selects Kotlin sources when matching oracle file names",
	"registry_security.go":                             "XmlExternalEntity lowers its confidence in scripts; it is not a skip",
	"release_engineering.go":                           "build file discovery and Kotlin-source filters",
	"release_engineering_debug_artifacts.go":           "Kotlin-source filters of rules that read both .kt and .kts",
	"release_engineering_helpers.go":                   "maps a precompiled script plugin file name to its plugin id",
	"style_classes.go":                                 "selects Kotlin sources when matching index files",
	"supply_catalog.go":                                "build file discovery",
}

// A per-rule .kts path check duplicates the script scope table and drifts from
// it. Declare the skip in api/script_scope.go instead.
func TestNoAdHocScriptSuffixSkipsInRules(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !scriptSuffixCheckRe.Match(data) {
			continue
		}
		if _, ok := scriptSuffixAllowed[name]; !ok {
			t.Errorf("%s tests a path for a .kts suffix; declare the script skip in api/script_scope.go (or add the file to scriptSuffixAllowed with a reason)", name)
		}
	}
	for name := range scriptSuffixAllowed {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("scriptSuffixAllowed names %s: %v", name, err)
			continue
		}
		if !scriptSuffixCheckRe.Match(data) {
			t.Errorf("%s no longer tests a .kts suffix; remove it from scriptSuffixAllowed", name)
		}
	}
}
