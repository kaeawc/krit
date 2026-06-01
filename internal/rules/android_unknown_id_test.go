package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// view builds a layout View with the given attributes and children. The
// android:id attribute (if any) is mirrored into View.ID like the parser does.
func view(viewType string, line int, attrs map[string]string, children ...*android.View) *android.View {
	v := &android.View{
		Type:       viewType,
		Attributes: attrs,
		Children:   children,
		Line:       line,
	}
	if id, ok := attrs["android:id"]; ok {
		v.ID = id
	}
	return v
}

func TestUnknownId(t *testing.T) {
	r := findResourceRule(t, "UnknownId")

	t.Run("dangling @id reference is flagged", func(t *testing.T) {
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:id": "@+id/button1"}),
			view("Button", 5, map[string]string{
				"android:id":                 "@+id/button2",
				"android:layout_toLeftOf":    "@id/ghost",
				"android:layout_alignBottom": "@id/button1",
			}),
		)
		idx := indexWithLayout("activity_main", "res/layout/activity_main.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d (%v)", len(findings), findings)
		}
		if findings[0].Line != 5 {
			t.Fatalf("expected finding on line 5, got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "ghost") {
			t.Fatalf("message missing dangling id: %q", findings[0].Message)
		}
	})

	t.Run("reference to a sibling id is clean", func(t *testing.T) {
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:id": "@+id/anchor"}),
			view("Button", 5, map[string]string{"android:layout_below": "@id/anchor"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("@+id create form is never flagged", func(t *testing.T) {
		// @+id/x in a constraint creates the id, so it is not a dangling ref
		// even when x is declared nowhere else.
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_toRightOf": "@+id/created_here"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for @+id create form, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("framework @android:id reference is clean", func(t *testing.T) {
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_below": "@android:id/text1"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for framework id, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("id declared via @+id in another attribute resolves", func(t *testing.T) {
		// button2 is only ever created via a @+id/ in a constraint attr, then
		// referenced via @id/ elsewhere. That is valid Android, not a typo.
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_alignTop": "@+id/button2"}),
			view("Button", 5, map[string]string{"android:layout_below": "@id/button2"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings when id created via @+id elsewhere, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("id declared in ids.xml resolves", func(t *testing.T) {
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_below": "@id/from_ids_xml"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		idx.AliasItems = append(idx.AliasItems, android.AliasItem{
			Name: "from_ids_xml", Type: "id", FilePath: "res/values/ids.xml", Line: 2,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings when id declared in ids.xml, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("id defined in a different layout resolves", func(t *testing.T) {
		// The declared-id set is project-wide: an id defined in layout A is
		// visible to a reference in layout B.
		rootA := view("LinearLayout", 1, map[string]string{},
			view("TextView", 2, map[string]string{"android:id": "@+id/shared"}),
		)
		rootB := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_below": "@id/shared"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", rootA)
		layoutB := &android.Layout{Name: "b", FilePath: "res/layout/b.xml", RootView: rootB}
		idx.Layouts["b"] = layoutB
		idx.LayoutConfigs["b"] = map[string]*android.Layout{"": layoutB}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for cross-layout id, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("multiple dangling references each report", func(t *testing.T) {
		root := view("RelativeLayout", 1, map[string]string{},
			view("Button", 3, map[string]string{"android:layout_toLeftOf": "@id/missing1"}),
			view("Button", 5, map[string]string{"android:layout_toRightOf": "@id/missing2"}),
		)
		idx := indexWithLayout("a", "res/layout/a.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d (%v)", len(findings), findings)
		}
	})
}
