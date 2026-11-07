package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestInvalidIdResource(t *testing.T) {
	r := findResourceRule(t, "InvalidIdResource")

	t.Run("id with spaces triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "@+id/my view",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("id without prefix triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "my_view",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("valid @+id/ is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			ID:   "@+id/my_view",
			Attributes: map[string]string{
				"android:id": "@+id/my_view",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("valid @id/ is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			ID:   "@id/my_view",
			Attributes: map[string]string{
				"android:id": "@id/my_view",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("@android:id/ is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "@android:id/empty",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MissingPrefixResource
// ---------------------------------------------------------------------------

func TestMissingIdResource(t *testing.T) {
	r := findResourceRule(t, "MissingIdResource")

	t.Run("fragment without id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "fragment",
					Line:       3,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "fragment") {
			t.Errorf("expected message to mention 'fragment', got %q", findings[0].Message)
		}
	})

	t.Run("include without id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "include",
					Line:       3,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("FragmentContainerView without id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{
				Type: "androidx.fragment.app.FragmentContainerView", Line: 3,
				Attributes: map[string]string{"android:name": "com.example.HostFragment"},
			}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("fragment with id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "fragment",
					ID:         "@+id/my_fragment",
					Line:       3,
					Attributes: map[string]string{"android:id": "@+id/my_fragment"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("fragment with tag is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "fragment",
					Line:       3,
					Attributes: map[string]string{"android:tag": "my_tag"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// InconsistentArraysResource
// ---------------------------------------------------------------------------

func TestInconsistentArraysResource(t *testing.T) {
	r := findResourceRule(t, "InconsistentArraysResource")
	variant := func(dir string, count int) android.StringArrayVariant {
		return android.StringArrayVariant{Dir: dir, Count: count, FilePath: "res/" + dir + "/arrays.xml", Line: 3}
	}

	t.Run("different counts across locales trigger once, in Lint wording", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrayVariants["colors"] = []android.StringArrayVariant{variant("values", 3), variant("values-fr", 2)}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		want := "Array `colors` has an inconsistent number of items (3 in `values/arrays.xml`, 2 in `values-fr/arrays.xml`)"
		if findings[0].Message != want || !strings.Contains(findings[0].File, "res/values/") {
			t.Errorf("unexpected finding %+v", findings[0])
		}
	})

	t.Run("three configurations report one finding", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrayVariants["colors"] = []android.StringArrayVariant{variant("values", 3), variant("values-de", 2), variant("values-fr", 1)}
		if findings := runResourceRule(r, idx); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("same counts are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrayVariants["colors"] = []android.StringArrayVariant{variant("values", 3), variant("values-fr", 3)}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single declaration is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrayVariants["only"] = []android.StringArrayVariant{variant("values", 3)}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		if findings := runResourceRule(r, emptyIndex()); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("merged indexes keep every variant", func(t *testing.T) {
		defaultIdx := emptyIndex()
		defaultIdx.StringArrayVariants["colors"] = []android.StringArrayVariant{variant("values", 3)}
		localeIdx := emptyIndex()
		localeIdx.StringArrayVariants["colors"] = []android.StringArrayVariant{variant("values-de", 1)}
		merged := android.MergeResourceIndexes(defaultIdx, localeIdx)
		if findings := runResourceRule(r, merged); len(findings) != 1 {
			t.Fatalf("expected 1 finding after merge, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// WrongRegionResource
// ---------------------------------------------------------------------------

func TestNotSiblingResource(t *testing.T) {
	r := findResourceRule(t, "NotSiblingResource")

	t.Run("non-sibling reference triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					ID:   "@+id/title",
					Line: 2,
					Attributes: map[string]string{
						"android:id": "@+id/title",
					},
				},
				{
					Type: "TextView",
					ID:   "@+id/subtitle",
					Line: 3,
					Attributes: map[string]string{
						"android:id":           "@+id/subtitle",
						"android:layout_below": "@id/nonexistent",
					},
				},
			},
		})
		idx.IDs["nonexistent"] = true // declared in another layout
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if want := "`@id/nonexistent` is not a sibling in the same `RelativeLayout`"; findings[0].Message != want {
			t.Fatalf("message = %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("sibling reference is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					ID:   "@+id/title",
					Line: 2,
					Attributes: map[string]string{
						"android:id": "@+id/title",
					},
				},
				{
					Type: "TextView",
					ID:   "@+id/subtitle",
					Line: 3,
					Attributes: map[string]string{
						"android:id":           "@+id/subtitle",
						"android:layout_below": "@id/title",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-RelativeLayout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 2,
					Attributes: map[string]string{
						"android:layout_below": "@id/nonexistent",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple constraint attrs trigger", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					ID:   "@+id/title",
					Line: 2,
					Attributes: map[string]string{
						"android:id": "@+id/title",
					},
				},
				{
					Type: "TextView",
					ID:   "@+id/subtitle",
					Line: 3,
					Attributes: map[string]string{
						"android:id":               "@+id/subtitle",
						"android:layout_below":     "@id/missing1",
						"android:layout_toRightOf": "@id/missing2",
					},
				},
			},
		})
		idx.IDs["missing1"], idx.IDs["missing2"] = true, true
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("an id declared nowhere is left to UnknownId", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "TextView", ID: "@+id/a", Line: 2, Attributes: map[string]string{"android:layout_below": "@id/ghost"}}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("relative to self reports", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "TextView", ID: "@+id/a", Line: 2, Attributes: map[string]string{"android:layout_below": "@id/a"}}},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Message != "Cannot be relative to self: id=a, layout_below=a" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("ConstraintLayout constraint and referenced ids", func(t *testing.T) {
		attrs := map[string]string{
			"app:layout_constraintStart_toEndOf": "@id/elsewhere",
			"app:constraint_referenced_ids":      "b,elsewhere",
		}
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "androidx.constraintlayout.widget.ConstraintLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "Group", ID: "@+id/g", Line: 2, Attributes: attrs},
				{Type: "TextView", ID: "@+id/b", Line: 3, Attributes: map[string]string{}},
			},
		})
		idx.IDs["elsewhere"] = true
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %+v", findings)
		}
		if findings[0].Message != "`elsewhere` is not a sibling in the same `ConstraintLayout`" && findings[1].Message != "`elsewhere` is not a sibling in the same `ConstraintLayout`" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("ids from an included layout are siblings", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", ID: "@+id/a", Line: 2, Attributes: map[string]string{"android:layout_below": "@id/row_title"}},
				{Type: "include", Line: 3, Attributes: map[string]string{"layout": "@layout/row"}},
			},
		})
		row := &android.Layout{Name: "row", FilePath: "res/layout/row.xml", RootView: &android.View{Type: "TextView", ID: "@+id/row_title", Line: 1}}
		idx.Layouts["row"] = row
		idx.LayoutConfigs["row"] = map[string]*android.Layout{"": row}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %+v", findings)
		}
	})

	t.Run("empty index is clean for NotSibling", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// IdTypePrefixMismatchResource
// ---------------------------------------------------------------------------

func TestIdTypePrefixMismatchResource(t *testing.T) {
	r := findResourceRule(t, "IdTypePrefixMismatchResource")

	t.Run("textview prefix on ImageView triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/tv_submit",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "@+id/tv_submit",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "copy-paste") {
			t.Fatalf("expected copy-paste message, got: %s", findings[0].Message)
		}
	})

	t.Run("btn prefix on ImageView triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/btn_icon",
			Line: 3,
			Attributes: map[string]string{
				"android:id": "@+id/btn_icon",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("button-family subclasses and clickable views satisfy btn prefix", func(t *testing.T) {
		cases := []*android.View{
			{Type: "com.example.widget.FooButton", ID: "@+id/btn_save", Attributes: map[string]string{}},
			{Type: "ImageButton", ID: "@+id/btn_close", Attributes: map[string]string{}},
			{Type: "com.google.android.material.button.MaterialButton", ID: "@+id/btn_ok", Attributes: map[string]string{}},
			{Type: "TextView", ID: "@+id/btn_link", Attributes: map[string]string{"android:clickable": "true"}},
			{Type: "ImageView", ID: "@+id/btn_icon", Attributes: map[string]string{"android:clickable": "true"}},
		}
		for _, v := range cases {
			v.Line = 4
			if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
				t.Errorf("%s %s: expected 0 findings, got %d", v.Type, v.ID, len(findings))
			}
		}
	})

	t.Run("a prefix alone is not evidence on a TextView or a subclass of the claimed family", func(t *testing.T) {
		cases := map[string]string{
			"btn_edit_drafts": "TextView", "iv_banner": "TextView", "et_hint": "TextView", "switch_label": "TextView",
			"tv_submit": "Button", "tv_name": "EditText", "btn_pick": "CheckBox", "rv_items": "TextView",
		}
		for id, typ := range cases {
			v := &android.View{Type: typ, ID: "@+id/" + id, Line: 4, Attributes: map[string]string{}}
			if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
				t.Errorf("%s on %s: expected 0 findings, got %d", id, typ, len(findings))
			}
		}
	})

	t.Run("button prefix on a button-like ImageView with onClick is clean", func(t *testing.T) {
		v := &android.View{Type: "ImageView", ID: "@+id/btn_icon", Line: 4, Attributes: map[string]string{"android:onClick": "onTap"}}
		if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("role prefix button_ is not a type claim", func(t *testing.T) {
		for _, typ := range []string{"TextView", "ImageView", "LinearLayout"} {
			v := &android.View{Type: typ, ID: "@+id/button_save", Line: 4, Attributes: map[string]string{}}
			if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", typ, len(findings))
			}
		}
	})

	t.Run("label suffix is not a type claim", func(t *testing.T) {
		v := &android.View{Type: "Button", ID: "@+id/tv_submit_label", Line: 4, Attributes: map[string]string{}}
		if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("custom view with unknown base class is skipped", func(t *testing.T) {
		v := &android.View{Type: "com.example.FancyThing", ID: "@+id/tv_title", Line: 4, Attributes: map[string]string{}}
		if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", v)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("matching prefix is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			ID:   "@+id/btn_submit",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "@+id/btn_submit",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no prefix is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			ID:   "@+id/submit",
			Line: 5,
			Attributes: map[string]string{
				"android:id": "@+id/submit",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Button",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for CutPasteId", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DuplicateIncludedIdsResource
// ---------------------------------------------------------------------------

func TestDuplicateIncludedIdsResource(t *testing.T) {
	r := findResourceRule(t, "DuplicateIncludedIdsResource")

	t.Run("ID collision via actual include triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Layouts["main"] = &android.Layout{
			Name: "main", FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/title", Line: 2, Attributes: map[string]string{"android:id": "@+id/title"}},
					{Type: "include", Line: 3, Attributes: map[string]string{"layout": "@layout/detail"}},
				},
			},
		}
		idx.Layouts["detail"] = &android.Layout{
			Name: "detail", FilePath: "res/layout/detail.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/title", Line: 2, Attributes: map[string]string{"android:id": "@+id/title"}},
				},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "title") {
			t.Fatalf("expected message about title, got: %s", findings[0].Message)
		}
	})

	t.Run("ID in 2 layouts is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Layouts["main"] = &android.Layout{
			Name: "main", FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/title", Line: 2, Attributes: map[string]string{"android:id": "@+id/title"}},
				},
			},
		}
		idx.Layouts["detail"] = &android.Layout{
			Name: "detail", FilePath: "res/layout/detail.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/title", Line: 2, Attributes: map[string]string{"android:id": "@+id/title"}},
				},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unique IDs are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Layouts["main"] = &android.Layout{
			Name: "main", FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/title", Line: 2, Attributes: map[string]string{"android:id": "@+id/title"}},
				},
			},
		}
		idx.Layouts["detail"] = &android.Layout{
			Name: "detail", FilePath: "res/layout/detail.xml",
			RootView: &android.View{
				Type: "FrameLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", ID: "@+id/subtitle", Line: 2, Attributes: map[string]string{"android:id": "@+id/subtitle"}},
				},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for DuplicateIncludedIds", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// InconsistentLayout
// ---------------------------------------------------------------------------

func inconsistentLayoutIndex(configs map[string]*android.Layout) *android.ResourceIndex {
	return &android.ResourceIndex{
		Layouts:       map[string]*android.Layout{"main": configs[""]},
		LayoutConfigs: map[string]map[string]*android.Layout{"main": configs},
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
}

func inconsistentLayoutConfig(dir, rootType string, ids ...string) *android.Layout {
	root := &android.View{Type: rootType, Line: 1, Attributes: map[string]string{}}
	for i, id := range ids {
		root.Children = append(root.Children, &android.View{Type: "TextView", ID: "@+id/" + id, Line: i + 2, Col: 5, Attributes: map[string]string{"android:id": "@+id/" + id}})
	}
	return &android.Layout{Name: "main", FilePath: "res/" + dir + "/main.xml", RootView: root}
}

func TestInconsistentLayoutResource(t *testing.T) {
	r := findResourceRule(t, "InconsistentLayout")

	t.Run("id missing from a configuration triggers on the file that has it", func(t *testing.T) {
		idx := inconsistentLayoutIndex(map[string]*android.Layout{
			"":     inconsistentLayoutConfig("layout", "LinearLayout", "title", "subtitle"),
			"land": inconsistentLayoutConfig("layout-land", "LinearLayout", "title"),
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %v", findingMessages(findings))
		}
		f := findings[0]
		if f.File != "res/layout/main.xml" || f.Line != 3 {
			t.Errorf("expected anchor res/layout/main.xml:3, got %s:%d", f.File, f.Line)
		}
		if !strings.Contains(f.Message, "`subtitle`") || !strings.Contains(f.Message, "layout-land") {
			t.Errorf("unexpected message %q", f.Message)
		}
	})

	t.Run("different root types and structure with the same ids is clean", func(t *testing.T) {
		idx := inconsistentLayoutIndex(map[string]*android.Layout{
			"":     inconsistentLayoutConfig("layout", "LinearLayout", "title", "body"),
			"land": inconsistentLayoutConfig("layout-land", "ConstraintLayout", "body", "title"),
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findingMessages(findings))
		}
	})

	t.Run("id present in two of three configurations lists the missing one", func(t *testing.T) {
		idx := inconsistentLayoutIndex(map[string]*android.Layout{
			"":        inconsistentLayoutConfig("layout", "LinearLayout", "title"),
			"land":    inconsistentLayoutConfig("layout-land", "LinearLayout", "title"),
			"sw600dp": inconsistentLayoutConfig("layout-sw600dp", "LinearLayout"),
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "layout-sw600dp (present in layout)") {
			t.Fatalf("expected one finding for layout-sw600dp, got %v", findingMessages(findings))
		}
	})

	t.Run("tools:ignore on the view suppresses the id", func(t *testing.T) {
		withExtra := inconsistentLayoutConfig("layout", "LinearLayout", "title", "extra")
		withExtra.RootView.Children[1].Attributes["tools:ignore"] = "InconsistentLayout"
		idx := inconsistentLayoutIndex(map[string]*android.Layout{
			"":     withExtra,
			"land": inconsistentLayoutConfig("layout-land", "LinearLayout", "title"),
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findingMessages(findings))
		}
	})

	t.Run("single config only is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings for single config, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for InconsistentLayout", func(t *testing.T) {
		if findings := runResourceRule(r, emptyIndex()); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestIdTypePrefixMismatchResourceLabelSuffixesAreNotTypeClaims(t *testing.T) {
	r := findResourceRule(t, "IdTypePrefixMismatchResource")
	run := func(typ, id string) int {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: typ, ID: "@+id/" + id, Line: 5, Attributes: map[string]string{},
		})
		return len(runResourceRule(r, idx))
	}
	for _, id := range []string{"switch_enabled_text", "switch_enabled_label", "switch_enabled_title", "switch_enabled_description", "switch_enabled_hint"} {
		if n := run("ImageView", id); n != 0 {
			t.Errorf("%s on a TextView is a label, got %d findings", id, n)
		}
	}
	if n := run("ImageView", "switch_enabled"); n != 1 {
		t.Errorf("a bare switch_ id on an ImageView still claims a Switch, got %d findings", n)
	}
	if n := run("ImageView", "tv_text"); n != 1 {
		t.Errorf("tv_text on an ImageView still claims a TextView, got %d findings", n)
	}
}
