package rules

import "testing"

func TestGradleHasDependenciesKeyword(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"bare-token", "dependencies", true},
		{"trailing-space", "dependencies ", true},
		{"leading-space", " dependencies", true},
		{"surrounded-by-spaces", "  dependencies  ", true},
		{"after-punct", "buildscript { dependencies", true},

		{"substring-camel-prefix", "subDependencies", false},
		{"substring-camel-suffix", "dependenciesScope", false},
		{"substring-underscore-prefix", "_dependencies", false},
		{"substring-underscore-suffix", "dependencies_extra", false},
		{"substring-digit-suffix", "dependencies2", false},

		// Bug 21 regression: a multi-byte UTF-8 identifier character
		// (Cyrillic ya, two bytes 0xD1 0x8F) directly before / after
		// the candidate match. The previous rune(s[i-1]) byte cast
		// produced a continuation-byte value that IsLetter rejects,
		// so the boundary check passed and the rule false-positived.
		{"cyrillic-prefix-blocks", "я" + "dependencies", false},
		{"cyrillic-suffix-blocks", "dependencies" + "я", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gradleHasDependenciesKeyword(tc.s); got != tc.want {
				t.Errorf("gradleHasDependenciesKeyword(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}
