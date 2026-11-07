package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func idView(typ, id string, children ...*android.View) *android.View {
	attrs := map[string]string{}
	v := &android.View{Type: typ, Line: 3, Col: 3, Attributes: attrs, Children: children}
	if id != "" {
		v.ID = id
		attrs["android:id"] = id
	}
	return v
}

func layoutsIndex(layouts ...*android.Layout) *android.ResourceIndex {
	idx := emptyIndex()
	idx.LayoutVariants = layouts
	for _, l := range layouts {
		idx.Layouts[l.Name] = l
	}
	return idx
}

func TestDuplicateIdsResource_OnlyNewIDDefinitions(t *testing.T) {
	r := findResourceRule(t, "DuplicateIdsResource")
	root := idView("LinearLayout", "",
		idView("TextView", "@+id/a"), idView("TextView", "@+id/a"), idView("TextView", "@id/a"),
		idView("TextView", "@android:id/text1"), idView("TextView", "@android:id/text1"))
	got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
	if len(got) != 1 || got[0].Message != "Duplicate id `@+id/a`, already defined earlier in this layout" {
		t.Fatalf("unexpected findings: %+v", got)
	}
}

func TestDuplicateIncludedIdsResource_ReportsAtTheLaterInclude(t *testing.T) {
	r := findResourceRule(t, "DuplicateIncludedIdsResource")
	include := func(name string, line int, attrs map[string]string) *android.View {
		if attrs == nil {
			attrs = map[string]string{}
		}
		attrs["layout"] = "@layout/" + name
		return &android.View{Type: "include", Line: line, Col: 3, Attributes: attrs}
	}
	layout := func(name string, root *android.View) *android.Layout {
		return &android.Layout{Name: name, FilePath: "res/layout/" + name + ".xml", RootView: root}
	}
	defines := func(name, id string) *android.Layout {
		return layout(name, idView("FrameLayout", "", idView("TextView", id)))
	}

	t.Run("two includes defining the same id", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", include("b", 3, nil), include("c", 4, nil)))
		got := runResourceRule(r, layoutsIndex(a, defines("b", "@+id/x"), defines("c", "@+id/x")))
		if len(got) != 1 || got[0].Line != 4 {
			t.Fatalf("expected one finding at the second include, got %+v", got)
		}
		want := "Duplicate id @+id/x, defined or included multiple times in layout/a.xml: [layout/a.xml => layout/b.xml defines @+id/x, layout/a.xml => layout/c.xml defines @+id/x]"
		if got[0].Message != want {
			t.Fatalf("unexpected message %q", got[0].Message)
		}
	})

	t.Run("a local definition and a transitive include", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/y"), include("b", 5, nil)))
		b := layout("b", idView("FrameLayout", "", include("c", 3, nil)))
		got := runResourceRule(r, layoutsIndex(a, b, defines("c", "@+id/y")))
		want := "Duplicate id @+id/y, defined or included multiple times in layout/a.xml: [layout/a.xml defines @+id/y, layout/a.xml => layout/b.xml => layout/c.xml defines @+id/y]"
		if len(got) != 1 || got[0].Line != 5 || got[0].Message != want {
			t.Fatalf("unexpected findings %+v", got)
		}
	})

	t.Run("an id on the include does not replace the included root id", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/z"), include("b", 4, map[string]string{"android:id": "@+id/inc"})))
		b := layout("b", idView("FrameLayout", "@+id/z"))
		// Lint 32.4.1 ignores the include's own android:id, so the duplicate is still reported.
		if got := runResourceRule(r, layoutsIndex(a, b)); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %+v", got)
		}
	})

	t.Run("tools:ignore on the include silences the finding", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/x"), include("b", 4, map[string]string{"tools:ignore": "DuplicateIncludedIds"})))
		if got := runResourceRule(r, layoutsIndex(a, defines("b", "@+id/x"))); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %+v", got)
		}
	})

	// Expectations below come from running Android Lint 32.4.1 on equivalent projects.
	t.Run("one layout included repeatedly is listed once and anchors on the last include", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/x"),
			include("b", 3, nil), include("b", 4, nil), include("b", 5, nil), include("b", 6, nil)))
		got := runResourceRule(r, layoutsIndex(a, defines("b", "@+id/x")))
		want := "Duplicate id @+id/x, defined or included multiple times in layout/a.xml: [layout/a.xml defines @+id/x, layout/a.xml => layout/b.xml defines @+id/x]"
		if len(got) != 1 || got[0].Line != 6 || got[0].Message != want {
			t.Fatalf("unexpected findings %+v", got)
		}
	})

	t.Run("repeated includes with no other definition are not duplicates", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", include("b", 3, nil), include("b", 4, nil)))
		if got := runResourceRule(r, layoutsIndex(a, defines("b", "@+id/x"))); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %+v", got)
		}
	})

	t.Run("each conflicting included layout gets its own finding at its last include", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/x"),
			include("b", 3, nil), include("c", 4, nil), include("b", 5, nil), include("c", 6, nil)))
		got := runResourceRule(r, layoutsIndex(a, defines("b", "@+id/x"), defines("c", "@+id/x")))
		want := "Duplicate id @+id/x, defined or included multiple times in layout/a.xml: [layout/a.xml defines @+id/x, layout/a.xml => layout/b.xml defines @+id/x, layout/a.xml => layout/c.xml defines @+id/x]"
		if len(got) != 2 || got[0].Line != 5 || got[1].Line != 6 || got[0].Message != want || got[1].Message != want {
			t.Fatalf("unexpected findings %+v", got)
		}
	})

	t.Run("a layout included twice through a middle layout is listed once", func(t *testing.T) {
		a := layout("a", idView("LinearLayout", "", idView("TextView", "@+id/x"), include("m", 3, nil), include("m", 4, nil)))
		m := layout("m", idView("FrameLayout", "", include("b", 3, nil)))
		got := runResourceRule(r, layoutsIndex(a, m, defines("b", "@+id/x")))
		want := "Duplicate id @+id/x, defined or included multiple times in layout/a.xml: [layout/a.xml defines @+id/x, layout/a.xml => layout/m.xml => layout/b.xml defines @+id/x]"
		if len(got) != 1 || got[0].Line != 4 || got[0].Message != want {
			t.Fatalf("unexpected findings %+v", got)
		}
	})
}
