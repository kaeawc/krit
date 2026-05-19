package strutil

import (
	"regexp"
	"testing"
)

func TestContainsTokenWordBoundary(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		token string
		want  bool
	}{
		{"exact-match-only", "foo", "foo", true},
		{"surrounded-by-spaces", " foo ", "foo", true},
		{"at-start", "foo bar", "foo", true},
		{"at-end", "bar foo", "foo", true},
		{"identifier-prefix-blocks", "myfoo", "foo", false},
		{"identifier-suffix-blocks", "foobar", "foo", false},
		{"digit-suffix-blocks", "foo1", "foo", false},
		{"underscore-suffix-blocks", "foo_bar", "foo", false},
		{"punctuation-boundary", "foo.bar()", "foo", true},
		{"multiple-occurrences-second-matches", "myfoo, foo", "foo", true},
		{"empty-token-is-false", "anything", "", false},
		{"empty-text", "", "foo", false},
		{"token-with-regex-meta-chars", "a.b.c", "a.b", true},
		{"token-with-regex-meta-chars-blocked", "xa.bc", "a.b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContainsTokenWordBoundary(tc.text, tc.token)
			if got != tc.want {
				t.Errorf("ContainsTokenWordBoundary(%q, %q) = %v, want %v", tc.text, tc.token, got, tc.want)
			}
			// Confirm parity with the regex form callers replaced.
			if tc.token != "" {
				ref := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(tc.token) + `([^A-Za-z0-9_]|$)`)
				refGot := ref.MatchString(tc.text)
				if refGot != got {
					t.Errorf("regex parity drift on (%q, %q): hand-rolled=%v regex=%v", tc.text, tc.token, got, refGot)
				}
			}
		})
	}
}

func BenchmarkContainsTokenWordBoundaryHandRolled(b *testing.B) {
	text := "fun setupBar(): Bar = bar.also { it.something() } // mocks.bar.bar"
	for i := 0; i < b.N; i++ {
		_ = ContainsTokenWordBoundary(text, "bar")
	}
}

func BenchmarkContainsTokenWordBoundaryRegex(b *testing.B) {
	text := "fun setupBar(): Bar = bar.also { it.something() } // mocks.bar.bar"
	for i := 0; i < b.N; i++ {
		re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta("bar") + `([^A-Za-z0-9_]|$)`)
		_ = re.MatchString(text)
	}
}
