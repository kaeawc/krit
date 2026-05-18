package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestNestedScrollingResource(t *testing.T) {
	r := findResourceRule(t, "NestedScrollingResource")

	t.Run("ScrollView inside ScrollView triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "ScrollView",
					Line:       5,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("single ScrollView is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "LinearLayout",
					Line:       5,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// TooManyViewsResource
// ---------------------------------------------------------------------------

func TestTooManyViewsResource(t *testing.T) {
	r := findResourceRule(t, "TooManyViewsResource")

	t.Run("layout under threshold is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 2, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("layout exceeding threshold triggers", func(t *testing.T) {
		// Build a layout with 81+ views
		root := &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		}
		for i := 0; i < 81; i++ {
			root.Children = append(root.Children, &android.View{
				Type:       "TextView",
				Line:       i + 2,
				Attributes: map[string]string{},
			})
		}
		idx := indexWithLayout("huge", "res/layout/huge.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// TooDeepLayoutResource
// ---------------------------------------------------------------------------

func TestTooDeepLayoutResource(t *testing.T) {
	r := findResourceRule(t, "TooDeepLayoutResource")

	t.Run("shallow layout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 2, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("deep layout triggers", func(t *testing.T) {
		// Build a chain 11 levels deep
		deepest := &android.View{Type: "TextView", Line: 12, Attributes: map[string]string{}}
		current := deepest
		for i := 10; i >= 1; i-- {
			current = &android.View{
				Type:       "LinearLayout",
				Line:       i,
				Attributes: map[string]string{},
				Children:   []*android.View{current},
			}
		}
		idx := indexWithLayout("deep", "res/layout/deep.xml", current)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UselessParentResource
// ---------------------------------------------------------------------------

func TestUselessParentResource(t *testing.T) {
	r := findResourceRule(t, "UselessParentResource")

	t.Run("single-child wrapper without extras triggers", func(t *testing.T) {
		child := &android.View{
			Type:       "LinearLayout",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       5,
					Attributes: map[string]string{},
					Children: []*android.View{
						{Type: "TextView", Line: 7, Attributes: map[string]string{}},
					},
				},
			},
		}
		idx := indexWithLayout("main", "res/layout/main.xml", child)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("wrapper with background is clean", func(t *testing.T) {
		child := &android.View{
			Type:       "LinearLayout",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       5,
					Background: "@drawable/bg",
					Attributes: map[string]string{"android:background": "@drawable/bg"},
					Children: []*android.View{
						{Type: "TextView", Line: 7, Attributes: map[string]string{}},
					},
				},
			},
		}
		idx := indexWithLayout("main", "res/layout/main.xml", child)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("root single-child is clean (cannot remove root)", func(t *testing.T) {
		root := &android.View{
			Type:       "FrameLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
			},
		}
		idx := indexWithLayout("main", "res/layout/main.xml", root)
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ObsoleteLayoutParamsResource
// ---------------------------------------------------------------------------

func TestObsoleteLayoutParamsResource(t *testing.T) {
	r := findResourceRule(t, "ObsoleteLayoutParamsResource")

	t.Run("layout_weight in RelativeLayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_weight": "1",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("layout_weight in LinearLayout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_weight": "1",
						"android:orientation":   "horizontal",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MergeRootFrameResource
// ---------------------------------------------------------------------------

func TestMergeRootFrameResource(t *testing.T) {
	r := findResourceRule(t, "MergeRootFrameResource")

	t.Run("plain root FrameLayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "FrameLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("root FrameLayout with background is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "FrameLayout",
			Line:       1,
			Background: "@drawable/bg",
			Attributes: map[string]string{"android:background": "@drawable/bg"},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("root LinearLayout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DuplicateIdsResource
// ---------------------------------------------------------------------------

func TestDuplicateIdsResource(t *testing.T) {
	r := findResourceRule(t, "DuplicateIdsResource")

	t.Run("duplicate id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView", ID: "@+id/title", Line: 3,
					Attributes: map[string]string{"android:id": "@+id/title"},
				},
				{
					Type: "TextView", ID: "@+id/title", Line: 7,
					Attributes: map[string]string{"android:id": "@+id/title"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unique ids are clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView", ID: "@+id/title", Line: 3,
					Attributes: map[string]string{"android:id": "@+id/title"},
				},
				{
					Type: "TextView", ID: "@+id/subtitle", Line: 7,
					Attributes: map[string]string{"android:id": "@+id/subtitle"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// WebViewInScrollViewResource
// ---------------------------------------------------------------------------

func TestWebViewInScrollViewResource(t *testing.T) {
	r := findResourceRule(t, "WebViewInScrollViewResource")

	t.Run("WebView in ScrollView triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "WebView",
					Line:       5,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("WebView outside ScrollView is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "WebView",
					Line:       5,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// InefficientWeightResource
// ---------------------------------------------------------------------------

func TestUselessLeafResource(t *testing.T) {
	r := findResourceRule(t, "UselessLeafResource")

	t.Run("empty ViewGroup without background or id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
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

	t.Run("empty ViewGroup with background is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       3,
					Background: "@drawable/bg",
					Attributes: map[string]string{"android:background": "@drawable/bg"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty ViewGroup with id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       3,
					ID:         "@+id/container",
					Attributes: map[string]string{"android:id": "@+id/container"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("leaf TextView is clean (not a ViewGroup)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "TextView",
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
}

// ---------------------------------------------------------------------------
// ScrollViewCountResource
// ---------------------------------------------------------------------------

func TestScrollViewCountResource(t *testing.T) {
	r := findResourceRule(t, "ScrollViewCountResource")

	t.Run("ScrollView with 2 children triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("ScrollView with 1 child is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "LinearLayout", Line: 3, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("HorizontalScrollView with 3 children triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "HorizontalScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
				{Type: "TextView", Line: 7, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RequiredSizeResource
// ---------------------------------------------------------------------------

func TestAdapterViewChildrenResource(t *testing.T) {
	r := findResourceRule(t, "AdapterViewChildrenResource")

	t.Run("ListView with children triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ListView",
			Line: 3,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "ListView") {
			t.Fatalf("expected message to mention ListView, got: %s", findings[0].Message)
		}
	})

	t.Run("GridView with children triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "GridView",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "ImageView", Line: 5, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Spinner without children is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Spinner",
			Line:       3,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with children is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
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
// IncludeLayoutParamResource
// ---------------------------------------------------------------------------

func TestIncludeLayoutParamResource(t *testing.T) {
	r := findResourceRule(t, "IncludeLayoutParamResource")

	t.Run("include with layout_width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "include",
					Line: 3,
					Attributes: map[string]string{
						"layout":               "@layout/toolbar",
						"android:layout_width": "match_parent",
						// height omitted — partial override is silently ignored
					},
					LayoutWidth: "match_parent",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("include without layout params is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "include",
					Line: 3,
					Attributes: map[string]string{
						"layout": "@layout/toolbar",
					},
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
// InvalidIdResource
// ---------------------------------------------------------------------------

func TestOrientationResource(t *testing.T) {
	r := findResourceRule(t, "OrientationResource")

	t.Run("LinearLayout without orientation triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with orientation is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
				"android:orientation":   "vertical",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("RelativeLayout without orientation is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
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
// SmallSpResource
// ---------------------------------------------------------------------------

func TestStateListReachableResource(t *testing.T) {
	r := findResourceRule(t, "StateListReachableResource")

	t.Run("empty index returns no findings", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("catch-all item masks later state", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{"android:state_pressed": "true"}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 6 {
			t.Fatalf("line = %d, want 6 (masked item)", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "item #1") {
			t.Fatalf("message should reference masking item #1: %q", findings[0].Message)
		}
	})

	t.Run("catch-all last item is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{"android:state_pressed": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("earlier subset masks later superset", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{"android:state_pressed": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{"android:state_pressed": "true", "android:state_focused": "true"}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 6 {
			t.Fatalf("line = %d, want 6", findings[0].Line)
		}
	})

	t.Run("conflicting values are not a subset", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{"android:state_pressed": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{"android:state_pressed": "false"}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("custom-namespace state attr is treated as a qualifier", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{"app:state_custom": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("only first masking pair reported per selector", func(t *testing.T) {
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{"android:state_pressed": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 9, StateAttrs: map[string]string{"android:state_focused": "true"}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings (one per masked item), got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ScrollViewSizeResource
// ---------------------------------------------------------------------------

func TestScrollViewSizeResource(t *testing.T) {
	r := findResourceRule(t, "ScrollViewSizeResource")

	t.Run("vertical ScrollView child with match_parent height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:         "LinearLayout",
					LayoutHeight: "match_parent",
					Line:         3,
					Attributes: map[string]string{
						"android:layout_height": "match_parent",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "wrap_content") {
			t.Fatalf("expected message about wrap_content, got %q", findings[0].Message)
		}
	})

	t.Run("vertical ScrollView child with wrap_content height is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:         "LinearLayout",
					LayoutHeight: "wrap_content",
					Line:         3,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("HorizontalScrollView child with match_parent width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "HorizontalScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:        "LinearLayout",
					LayoutWidth: "match_parent",
					Line:        3,
					Attributes: map[string]string{
						"android:layout_width": "match_parent",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("HorizontalScrollView child with wrap_content width is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "HorizontalScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:        "LinearLayout",
					LayoutWidth: "wrap_content",
					Line:        3,
					Attributes: map[string]string{
						"android:layout_width": "wrap_content",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("NestedScrollView child with fill_parent height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "NestedScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:         "LinearLayout",
					LayoutHeight: "fill_parent",
					Line:         3,
					Attributes: map[string]string{
						"android:layout_height": "fill_parent",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
// WrongCaseResource
// ---------------------------------------------------------------------------
