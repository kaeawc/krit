package rules

import "testing"

func TestGradleStripStringsAndComments(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain code", `targetSdk = 33`, `targetSdk = 33`},
		{"trailing line comment", `targetSdk = 33 // bump later`, `targetSdk = 33 `},
		{"// inside double-quoted string", `val url = "http://example.com"`, `val url = ""`},
		{"// inside single-quoted string", `val url = 'http://example.com'`, `val url = ''`},
		{"escaped quote in string", `val s = "a\"b//c" + 1`, `val s = "" + 1`},
		{"keyword inside string is masked", `val msg = "set targetSdk in your build"`, `val msg = ""`},
		{"keyword inside trailing comment is dropped", `something() // mavenLocal() note`, `something() `},
		{"unterminated string", `val s = "oops`, `val s = "`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gradleStripStringsAndComments(tc.in)
			if got != tc.want {
				t.Fatalf("gradleStripStringsAndComments(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindGradleLineStrSkipsStringsAndComments(t *testing.T) {
	t.Run("matches code on a later line, skips earlier string-literal occurrence", func(t *testing.T) {
		content := `// header
val url = "http://example.com/mavenLocal()"
val msg = "use mavenLocal() carefully"
mavenLocal()
`
		got := findGradleLineStr(content, "mavenLocal()")
		if got != 4 {
			t.Fatalf("expected match on line 4 (real code), got %d", got)
		}
	})

	t.Run("matches code on a later line, skips trailing-comment occurrence", func(t *testing.T) {
		content := `something() // targetSdk note
targetSdk = 33
`
		got := findGradleLineStr(content, "targetSdk")
		if got != 2 {
			t.Fatalf("expected match on line 2, got %d", got)
		}
	})

	t.Run("URL inside string literal is not flagged as a comment line", func(t *testing.T) {
		// Regression: isGradleCommentLine must not mistake a `//` inside
		// a string for a whole-line comment. A genuine code match on the
		// same line should still be found.
		content := `val u = "http://example.com" ; targetSdk = 33
`
		got := findGradleLineStr(content, "targetSdk")
		if got != 1 {
			t.Fatalf("expected match on line 1, got %d", got)
		}
	})

	t.Run("returns 0 when substr only appears in strings or comments", func(t *testing.T) {
		content := `// mavenLocal()
val s = "mavenLocal()"
plain()
`
		got := findGradleLineStr(content, "mavenLocal()")
		if got != 0 {
			t.Fatalf("expected no match, got line %d", got)
		}
	})
}
