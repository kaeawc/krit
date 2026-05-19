package rules

import "testing"

func TestStripCommentsStripsLineAndBlock(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"line-only", "x // trailing\n", "x \n"},
		{"block-inline", "a /* b */ c", "a  c"},
		{"block-multiline", "a /* line1\nline2 */ b", "a  b"},
		{"both", "/* leading */ code // trailing", " code "},
		{"no-comments", "fun f() {}", "fun f() {}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripComments(tc.in); got != tc.want {
				t.Errorf("stripComments(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestStripCommentsPackageRegexesAreHoisted asserts the package-level
// vars used by stripComments are populated. If a future refactor
// moves them back inside the function body, this fails.
func TestStripCommentsPackageRegexesAreHoisted(t *testing.T) {
	if stripCommentsBlockRe == nil {
		t.Fatal("stripCommentsBlockRe must be initialised at package level")
	}
	if stripCommentsLineRe == nil {
		t.Fatal("stripCommentsLineRe must be initialised at package level")
	}
}
