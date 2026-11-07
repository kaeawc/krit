package scanner

import "testing"

func TestParseInlineIgnores_OnlyRealLineComments(t *testing.T) {
	src := "val a = \"// krit:ignore[R]\"\n" + // 1: inside a string
		"val b = 1 // krit:ignore[R]\n" + // 2: real comment
		"val c = \"\"\"\n// krit:ignore[R]\n\"\"\"\n" + // 3-5: inside a raw string
		"/* // krit:ignore[R] */\n" + // 6: inside a block comment
		"val d = \"${\"x\"} // krit:ignore[R]\"\n" + // 7: template then string text
		"val e = '\"' // krit:ignore[R]\n" + // 8: char literal quote, then real comment
		"val f = 2 // krit:ignored\n" // 9: not a directive
	got := parseInlineIgnores([]byte(src))
	for line, want := range map[int]bool{1: false, 2: true, 3: false, 4: false, 6: false, 7: false, 8: true, 9: false} {
		if has := got[line]["R"]; has != want {
			t.Errorf("line %d: suppressed=%v, want %v", line, has, want)
		}
	}
	if got[9] != nil {
		t.Errorf("line 9: `krit:ignored` must not be a directive, got %v", got[9])
	}
}
