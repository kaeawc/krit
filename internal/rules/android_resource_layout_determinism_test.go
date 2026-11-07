package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// TestInconsistentLayoutDeterministicOutput guards against map iteration order
// leaking into InconsistentLayout: anchors, messages and finding order must be
// identical on every run.
func TestInconsistentLayoutDeterministicOutput(t *testing.T) {
	r := findResourceRule(t, "InconsistentLayout")

	build := func() *android.ResourceIndex {
		return inconsistentLayoutIndex(map[string]*android.Layout{
			"":        inconsistentLayoutConfig("layout", "LinearLayout", "a", "b", "c"),
			"land":    inconsistentLayoutConfig("layout-land", "LinearLayout", "a"),
			"night":   inconsistentLayoutConfig("layout-night", "LinearLayout", "b"),
			"sw600dp": inconsistentLayoutConfig("layout-sw600dp", "LinearLayout", "a", "b", "c"),
			"v21":     inconsistentLayoutConfig("layout-v21", "LinearLayout"),
		})
	}
	type flat struct {
		file string
		line int
		msg  string
	}
	run := func() []flat {
		var out []flat
		for _, f := range runResourceRule(r, build()) {
			out = append(out, flat{f.File, f.Line, f.Message})
		}
		return out
	}
	want := run()
	if len(want) != 3 {
		t.Fatalf("expected 3 findings (a, b, c), got %+v", want)
	}
	for i := 0; i < 50; i++ {
		got := run()
		if len(got) != len(want) {
			t.Fatalf("iteration %d: expected %d findings, got %d", i, len(want), len(got))
		}
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("iteration %d: finding %d changed between runs:\n  first: %+v\n  now:   %+v", i, j, want[j], got[j])
			}
		}
	}
}
