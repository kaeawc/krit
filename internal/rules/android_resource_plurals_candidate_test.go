package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestPluralsCandidateResource(t *testing.T) {
	r := findResourceRule(t, "PluralsCandidateResource")
	run := func(path, text, formatted string) int {
		idx := emptyIndex()
		idx.StringDefinitions = []android.StringDefinition{{
			Name: "s", FilePath: path, Line: 4, Text: text, Formatted: formatted,
		}}
		return len(runResourceRule(r, idx))
	}
	const def = "res/values/strings.xml"
	cases := []struct {
		name, path, text, formatted string
		want                        int
	}{
		{"count then word", def, "%d items", "", 1},
		{"positional count", def, "%1$d items in %2$s", "", 1},
		{"word after punctuation-free gap", def, "Selected: %d files", "", 1},
		{"hex conversion", def, "%x bytes", "", 1},
		{"string conversion", def, "%s items", "", 0},
		{"float conversion", def, "%.1f items", "", 0},
		{"no word", def, "%d", "", 0},
		{"punctuation after count", def, "%d, then more", "", 0},
		{"two letter word", def, "%d of %d", "", 0},
		{"capitalized first letter", def, "%d Items", "", 1},
		{"capital inside word", def, "%d itEms", "", 0},
		{"no vowel", def, "%d pcs", "", 0},
		{"min word", def, "%d min", "", 0},
		{"percent literal", def, "%d%% done", "", 0},
		{"formatted false", def, "%d items", "false", 0},
		{"non-breaking space gap", def, "%d items", "", 0},
		{"english folder", "res/values-en/strings.xml", "%d items", "", 1},
		{"english region folder", "res/values-en-rGB/strings.xml", "%d items", "", 1},
		{"other language folder", "res/values-de/strings.xml", "%d Dinge", "", 0},
		{"night folder without locale", "res/values-night/strings.xml", "%d items", "", 1},
		{"quoted text is stripped", def, "\"%d\" items", "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.path, tc.text, tc.formatted); got != tc.want {
				t.Fatalf("%q in %s: got %d findings, want %d", tc.text, tc.path, got, tc.want)
			}
		})
	}

	t.Run("reports once per string at its line", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringDefinitions = []android.StringDefinition{{Name: "s", FilePath: def, Line: 9, Text: "%d items and %d files"}}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Line != 9 {
			t.Fatalf("expected one finding at line 9, got %v", findings)
		}
	})
}
