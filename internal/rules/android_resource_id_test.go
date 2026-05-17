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

	t.Run("include without id triggers", func(t *testing.T) {
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
		if len(findings) != 1 {
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

	t.Run("empty array triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrays["empty_array"] = []string{}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "empty_array") {
			t.Errorf("expected message to mention 'empty_array', got %q", findings[0].Message)
		}
	})

	t.Run("non-empty array is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringArrays["colors"] = []string{"Red", "Green", "Blue"}
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

	// Regression: cross-locale merge must not let an empty <string-array> in a
	// translation overlay (e.g. values-fr/arrays.xml) erase the populated
	// default in values/arrays.xml, which would then make this rule fire on a
	// fully-defined array.
	t.Run("empty locale overlay does not erase populated default", func(t *testing.T) {
		defaultIdx := emptyIndex()
		defaultIdx.StringArrays["colors"] = []string{"Red", "Green", "Blue"}

		localeIdx := emptyIndex()
		localeIdx.StringArrays["colors"] = []string{}

		merged := android.MergeResourceIndexes(defaultIdx, localeIdx)
		findings := runResourceRule(r, merged)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings after merging empty French overlay onto populated default, got %d:", len(findings))
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
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "nonexistent") {
			t.Fatalf("expected message about nonexistent, got: %s", findings[0].Message)
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
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
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
// CutPasteIdResource
// ---------------------------------------------------------------------------

func TestCutPasteIdResource(t *testing.T) {
	r := findResourceRule(t, "CutPasteIdResource")

	t.Run("textview prefix on Button triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
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

func TestInconsistentLayoutResource(t *testing.T) {
	r := findResourceRule(t, "InconsistentLayout")

	t.Run("different root types across configs triggers", func(t *testing.T) {
		defaultLayout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}},
		}
		landLayout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout-land/main.xml",
			RootView: &android.View{Type: "ConstraintLayout", Line: 1, Attributes: map[string]string{}},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": landLayout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {
					"":     defaultLayout,
					"land": landLayout,
				},
			},
			Strings:      make(map[string]string),
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for different root types, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "inconsistent root view") {
			t.Fatalf("expected root view mismatch message, got %q", findings[0].Message)
		}
	})

	t.Run("same structure across configs is clean", func(t *testing.T) {
		child := &android.View{Type: "TextView", Line: 3, Attributes: map[string]string{}}
		defaultLayout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{Type: "LinearLayout", Line: 1, Children: []*android.View{child}, Attributes: map[string]string{}},
		}
		landLayout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout-land/main.xml",
			RootView: &android.View{Type: "LinearLayout", Line: 1, Children: []*android.View{child}, Attributes: map[string]string{}},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": landLayout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {
					"":     defaultLayout,
					"land": landLayout,
				},
			},
			Strings:      make(map[string]string),
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for same structure, got %d", len(findings))
		}
	})

	t.Run("single config only is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for single config, got %d", len(findings))
		}
	})

	t.Run("different root child counts triggers", func(t *testing.T) {
		child1 := &android.View{Type: "TextView", Line: 3, Attributes: map[string]string{}}
		child2 := &android.View{Type: "Button", Line: 5, Attributes: map[string]string{}}
		defaultLayout := &android.Layout{
			Name:     "settings",
			FilePath: "res/layout/settings.xml",
			RootView: &android.View{Type: "LinearLayout", Line: 1, Children: []*android.View{child1}, Attributes: map[string]string{}},
		}
		landLayout := &android.Layout{
			Name:     "settings",
			FilePath: "res/layout-land/settings.xml",
			RootView: &android.View{Type: "LinearLayout", Line: 1, Children: []*android.View{child1, child2}, Attributes: map[string]string{}},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"settings": landLayout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"settings": {
					"":     defaultLayout,
					"land": landLayout,
				},
			},
			Strings:      make(map[string]string),
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for different child counts, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "inconsistent root child count") {
			t.Fatalf("expected child count mismatch message, got %q", findings[0].Message)
		}
	})

	t.Run("significantly different view counts triggers", func(t *testing.T) {
		// Both have 1 direct child, but land has deeply nested extra views.
		// default: 2 views total; land: 5 views total (>50% diff)
		defaultLayout := &android.Layout{
			Name:     "detail",
			FilePath: "res/layout/detail.xml",
			RootView: &android.View{
				Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "TextView", Line: 2, Attributes: map[string]string{}},
				},
			},
		}
		landLayout := &android.Layout{
			Name:     "detail",
			FilePath: "res/layout-land/detail.xml",
			RootView: &android.View{
				Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{
					{Type: "FrameLayout", Line: 2, Attributes: map[string]string{}, Children: []*android.View{
						{Type: "ImageView", Line: 3, Attributes: map[string]string{}},
						{Type: "Button", Line: 4, Attributes: map[string]string{}},
						{Type: "EditText", Line: 5, Attributes: map[string]string{}},
					}},
				},
			},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"detail": landLayout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"detail": {
					"":     defaultLayout,
					"land": landLayout,
				},
			},
			Strings:      make(map[string]string),
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for different view counts, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "significantly different view counts") {
			t.Fatalf("expected view count mismatch message, got %q", findings[0].Message)
		}
	})

	t.Run("empty index is clean for InconsistentLayout", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
