package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// TestInconsistentLayoutDeterministicAnchor guards against map iteration order
// leaking into InconsistentLayout: with many configurations per layout, the
// reference configuration, the anchor file, the message, and the finding order
// must be identical on every run.
func TestInconsistentLayoutDeterministicAnchor(t *testing.T) {
	r := findResourceRule(t, "InconsistentLayout")

	qualifiers := []string{"", "land", "sw600dp", "sw720dp", "v21", "night", "ldrtl", "w480dp", "h720dp", "large", "xlarge", "land-v21"}
	layoutNames := []string{"main", "detail", "settings", "profile", "list"}
	dirFor := func(q string) string {
		if q == "" {
			return "layout"
		}
		return "layout-" + q
	}
	build := func() *android.ResourceIndex {
		idx := &android.ResourceIndex{
			Layouts:       make(map[string]*android.Layout),
			LayoutConfigs: make(map[string]map[string]*android.Layout),
			Strings:       make(map[string]string),
			Colors:        make(map[string]string),
			Dimensions:    make(map[string]string),
			Styles:        make(map[string]*android.Style),
			StringArrays:  make(map[string][]string),
			Plurals:       make(map[string]map[string]string),
			Integers:      make(map[string]string),
			Booleans:      make(map[string]string),
			IDs:           make(map[string]bool),
		}
		for _, name := range layoutNames {
			configs := make(map[string]*android.Layout)
			for i, q := range qualifiers {
				// Every configuration has a distinct root type so the
				// message depends on which configuration is the reference.
				root := &android.View{
					Type:       "Root" + string(rune('A'+i)),
					Line:       i + 1,
					Attributes: map[string]string{},
				}
				configs[q] = &android.Layout{
					Name:     name,
					FilePath: "res/" + dirFor(q) + "/" + name + ".xml",
					RootView: root,
				}
			}
			idx.LayoutConfigs[name] = configs
			idx.Layouts[name] = configs[""]
		}
		return idx
	}

	type flat struct {
		file string
		line int
		msg  string
	}
	run := func() []flat {
		findings := runResourceRule(r, build())
		out := make([]flat, 0, len(findings))
		for _, f := range findings {
			out = append(out, flat{f.File, f.Line, f.Message})
		}
		return out
	}

	want := run()
	if len(want) != len(layoutNames)*(len(qualifiers)-1) {
		t.Fatalf("expected %d findings, got %d", len(layoutNames)*(len(qualifiers)-1), len(want))
	}
	// The default configuration is the reference, so it is never the anchor.
	for _, f := range want {
		if strings.Contains(f.file, "res/layout/") {
			t.Errorf("finding anchored on the default layout %s; expected a non-default anchor", f.file)
		}
		if !strings.Contains(f.msg, "'RootA' in default") {
			t.Errorf("expected default configuration as reference in message, got %q", f.msg)
		}
	}
	// Layout names are visited in sorted order.
	if !strings.Contains(want[0].msg, "'detail'") {
		t.Errorf("expected first finding for layout 'detail', got %q", want[0].msg)
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
