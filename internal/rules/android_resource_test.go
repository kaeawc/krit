package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func findResourceRule(t *testing.T, name string) *v2rules.Rule {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.Needs.Has(v2rules.NeedsResources) && r.ID == name {
			return r
		}
	}
	t.Fatalf("resource rule %q not found in v2 Registry (NeedsResources)", name)
	return nil
}

func runResourceRule(r *v2rules.Rule, idx *android.ResourceIndex) []scanner.Finding {
	ctx := &v2rules.Context{
		ResourceIndex: idx,
		Rule:          r,
	}
	r.Check(ctx)
	return ctx.Findings
}

// helper: build a ResourceIndex with a single layout containing the given root view.
func indexWithLayout(name, filePath string, root *android.View) *android.ResourceIndex {
	layout := &android.Layout{Name: name, FilePath: filePath, RootView: root}
	return &android.ResourceIndex{
		Layouts: map[string]*android.Layout{
			name: layout,
		},
		LayoutConfigs: map[string]map[string]*android.Layout{
			name: {"": layout},
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
}

func emptyIndex() *android.ResourceIndex {
	return &android.ResourceIndex{
		Layouts:         make(map[string]*android.Layout),
		LayoutConfigs:   make(map[string]map[string]*android.Layout),
		Strings:         make(map[string]string),
		StringsLocation: make(map[string]android.StringLocation),
		Colors:          make(map[string]string),
		Dimensions:      make(map[string]string),
		Styles:          make(map[string]*android.Style),
		StringArrays:    make(map[string][]string),
		Plurals:         make(map[string]map[string]string),
		Integers:        make(map[string]string),
		Booleans:        make(map[string]string),
		IDs:             make(map[string]bool),
	}
}

// ---------------------------------------------------------------------------
// HardcodedValuesResource
// ---------------------------------------------------------------------------

func TestHardcodedValuesResource(t *testing.T) {
	r := findResourceRule(t, "HardcodedValuesResource")

	t.Run("hardcoded text triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Text: "Hello World",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Hello World",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("string reference is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Text: "@string/hello",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/hello",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("hardcoded hint triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText",
			Line: 3,
			Attributes: map[string]string{
				"android:hint": "Type here",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("empty layout is clean", func(t *testing.T) {
		idx := emptyIndex()
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MissingContentDescriptionResource
// ---------------------------------------------------------------------------

func TestMissingContentDescriptionResource(t *testing.T) {
	r := findResourceRule(t, "MissingContentDescriptionResource")

	t.Run("ImageView without contentDescription triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ImageView",
			Line:       10,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("ImageView with contentDescription is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:               "ImageView",
			ContentDescription: "@string/logo_desc",
			Line:               10,
			Attributes: map[string]string{
				"android:contentDescription": "@string/logo_desc",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ImageView with tools:ignore is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 10,
			Attributes: map[string]string{
				"tools:ignore": "ContentDescription",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("TextView is clean (not an image view)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       10,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// PxUsageResource
// ---------------------------------------------------------------------------

func TestPxUsageResource(t *testing.T) {
	r := findResourceRule(t, "PxUsageResource")

	t.Run("px in layout_width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "100px",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("dp in layout_width is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "100dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("px in dimens values triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["margin_large"] = "24px"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NestedScrollingResource
// ---------------------------------------------------------------------------

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

func TestInefficientWeightResource(t *testing.T) {
	r := findResourceRule(t, "InefficientWeightResource")

	t.Run("LinearLayout with weight but no orientation triggers", func(t *testing.T) {
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
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with weight and orientation is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "vertical",
			},
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
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DisableBaselineAlignmentResource
// ---------------------------------------------------------------------------

func TestDisableBaselineAlignmentResource(t *testing.T) {
	r := findResourceRule(t, "DisableBaselineAlignmentResource")

	t.Run("LinearLayout with weighted children but no baselineAligned triggers", func(t *testing.T) {
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
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with baselineAligned=false is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:baselineAligned": "false",
			},
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
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("LinearLayout without weight children is clean", func(t *testing.T) {
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
// NestedWeightsResource
// ---------------------------------------------------------------------------

func TestNestedWeightsResource(t *testing.T) {
	r := findResourceRule(t, "NestedWeightsResource")

	t.Run("nested LinearLayouts both with weights triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "LinearLayout",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_weight": "1",
					},
					Children: []*android.View{
						{
							Type: "TextView",
							Line: 5,
							Attributes: map[string]string{
								"android:layout_weight": "1",
							},
						},
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("single level weights is clean", func(t *testing.T) {
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
// UselessLeafResource
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

func TestRequiredSizeResource(t *testing.T) {
	r := findResourceRule(t, "RequiredSizeResource")

	t.Run("view missing both width and height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("view missing only width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_height": "wrap_content",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("view with both width and height is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "wrap_content",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SpUsageResource
// ---------------------------------------------------------------------------

func TestSpUsageResource(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")

	t.Run("textSize with dp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("textSize with sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize with dip triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14dip",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// TextFieldsResource
// ---------------------------------------------------------------------------

func TestTextFieldsResource(t *testing.T) {
	r := findResourceRule(t, "TextFieldsResource")

	t.Run("EditText without inputType or hint triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "EditText",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("EditText with inputType is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType": "text",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("EditText with hint is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:hint": "@string/hint",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NegativeMarginResource
// ---------------------------------------------------------------------------

func TestNegativeMarginResource(t *testing.T) {
	r := findResourceRule(t, "NegativeMarginResource")

	t.Run("negative margin triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop": "-8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("positive margin is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple negative margins trigger single combined finding", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop":  "-8dp",
				"android:layout_marginLeft": "-4dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 combined finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_marginTop") ||
			!strings.Contains(findings[0].Message, "layout_marginLeft") {
			t.Fatalf("expected combined message mentioning both attributes, got %q", findings[0].Message)
		}
	})
}

// ---------------------------------------------------------------------------
// RtlHardcodedResource
// ---------------------------------------------------------------------------

func TestRtlHardcodedResource(t *testing.T) {
	r := findResourceRule(t, "RtlHardcodedResource")

	t.Run("marginLeft triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("paddingRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("marginStart is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginStart": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LabelForResource
// ---------------------------------------------------------------------------

func TestLabelForResource(t *testing.T) {
	r := findResourceRule(t, "LabelForResource")

	t.Run("EditText without labelFor sibling triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "EditText",
					ID:   "@+id/email",
					Line: 3,
					Attributes: map[string]string{
						"android:id": "@+id/email",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("EditText with labelFor sibling is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:labelFor": "@+id/email",
					},
				},
				{
					Type: "EditText",
					ID:   "@+id/email",
					Line: 5,
					Attributes: map[string]string{
						"android:id": "@+id/email",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("EditText without id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "EditText",
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
}

// ---------------------------------------------------------------------------
// OnClickResource
// ---------------------------------------------------------------------------

func TestOnClickResource(t *testing.T) {
	r := findResourceRule(t, "OnClickResource")

	t.Run("android:onClick triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:onClick": "handleClick",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no onClick is clean", func(t *testing.T) {
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

	t.Run("onClick on non-button also triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 3,
			Attributes: map[string]string{
				"android:onClick": "onTextClick",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("empty layout is clean", func(t *testing.T) {
		idx := emptyIndex()
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// BackButtonResource
// ---------------------------------------------------------------------------

func TestBackButtonResource(t *testing.T) {
	r := findResourceRule(t, "BackButtonResource")

	t.Run("Button with text Back triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Text: "Back",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Back",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Button with text back (lowercase) triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Text: "back",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "back",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Button with @string/back triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Text: "@string/back",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/back",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Button with text Submit is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Text: "Submit",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Submit",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("TextView with text Back is clean (not a button)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Text: "Back",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Back",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ButtonOrderResource
// ---------------------------------------------------------------------------

func TestButtonOrderResource(t *testing.T) {
	r := findResourceRule(t, "ButtonOrderResource")

	t.Run("Cancel after OK triggers", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Text: "OK",
					Line: 3,
					Attributes: map[string]string{
						"android:text": "OK",
					},
				},
				{
					Type: "Button",
					Text: "Cancel",
					Line: 7,
					Attributes: map[string]string{
						"android:text": "Cancel",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Cancel before OK is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Text: "Cancel",
					Line: 3,
					Attributes: map[string]string{
						"android:text": "Cancel",
					},
				},
				{
					Type: "Button",
					Text: "OK",
					Line: 7,
					Attributes: map[string]string{
						"android:text": "OK",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("Confirm and Dismiss with wrong order triggers", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Text: "Confirm",
					Line: 3,
					Attributes: map[string]string{
						"android:text": "Confirm",
					},
				},
				{
					Type: "Button",
					Text: "Dismiss",
					Line: 7,
					Attributes: map[string]string{
						"android:text": "Dismiss",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no button pair is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Text: "Submit",
					Line: 3,
					Attributes: map[string]string{
						"android:text": "Submit",
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
// OverdrawResource
// ---------------------------------------------------------------------------

func TestOverdrawResource(t *testing.T) {
	r := findResourceRule(t, "OverdrawResource")

	t.Run("root and child layout both with background triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Background: "@drawable/bg",
			Attributes: map[string]string{"android:background": "@drawable/bg"},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       3,
					Background: "@color/white",
					Attributes: map[string]string{"android:background": "@color/white"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("root with background but child without is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Background: "@drawable/bg",
			Attributes: map[string]string{"android:background": "@drawable/bg"},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
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

	t.Run("root without background is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "FrameLayout",
					Line:       3,
					Background: "@color/white",
					Attributes: map[string]string{"android:background": "@color/white"},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("child is non-layout view with background is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Background: "@drawable/bg",
			Attributes: map[string]string{"android:background": "@drawable/bg"},
			Children: []*android.View{
				{
					Type:       "TextView",
					Line:       3,
					Background: "@color/white",
					Attributes: map[string]string{"android:background": "@color/white"},
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
// RtlSymmetryResource
// ---------------------------------------------------------------------------

func TestRtlSymmetryResource(t *testing.T) {
	r := findResourceRule(t, "RtlSymmetryResource")

	t.Run("paddingLeft without paddingRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("paddingRight without paddingLeft triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("both paddingLeft and paddingRight is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft":  "8dp",
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("marginLeft without marginRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("both marginLeft and marginRight is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft":  "16dp",
				"android:layout_marginRight": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no padding or margin is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// AlwaysShowActionResource
// ---------------------------------------------------------------------------

func TestAlwaysShowActionResource(t *testing.T) {
	r := findResourceRule(t, "AlwaysShowActionResource")

	t.Run("showAsAction always triggers", func(t *testing.T) {
		idx := indexWithLayout("menu", "res/menu/main.xml", &android.View{
			Type: "item",
			Line: 3,
			Attributes: map[string]string{
				"app:showAsAction": "always",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("android:showAsAction always triggers", func(t *testing.T) {
		idx := indexWithLayout("menu", "res/menu/main.xml", &android.View{
			Type: "item",
			Line: 3,
			Attributes: map[string]string{
				"android:showAsAction": "always",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("showAsAction ifRoom is clean", func(t *testing.T) {
		idx := indexWithLayout("menu", "res/menu/main.xml", &android.View{
			Type: "item",
			Line: 3,
			Attributes: map[string]string{
				"app:showAsAction": "ifRoom",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no showAsAction is clean", func(t *testing.T) {
		idx := indexWithLayout("menu", "res/menu/main.xml", &android.View{
			Type:       "item",
			Line:       3,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ClickableViewAccessibilityResource
// ---------------------------------------------------------------------------

func TestClickableViewAccessibilityResource(t *testing.T) {
	r := findResourceRule(t, "ClickableViewAccessibilityResource")

	t.Run("clickable without contentDescription triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 5,
			Attributes: map[string]string{
				"android:clickable": "true",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("clickable with contentDescription is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:               "LinearLayout",
			ContentDescription: "@string/desc",
			Line:               5,
			Attributes: map[string]string{
				"android:clickable":          "true",
				"android:contentDescription": "@string/desc",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("not clickable is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 5,
			Attributes: map[string]string{
				"android:clickable": "false",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("clickable with tools:ignore is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 5,
			Attributes: map[string]string{
				"android:clickable": "true",
				"tools:ignore":      "ContentDescription",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RelativeOverlapResource
// ---------------------------------------------------------------------------

func TestRelativeOverlapResource(t *testing.T) {
	r := findResourceRule(t, "RelativeOverlapResource")

	t.Run("two children with alignParentLeft and no vertical constraint triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("two children with alignParentLeft but different vertical constraints is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
						"android:layout_alignParentTop":  "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft":   "true",
						"android:layout_alignParentBottom": "true",
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
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("alignParentStart overlap also triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentStart": "true",
					},
				},
				{
					Type: "Button",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentStart": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Registration test — ensure all 30 resource rules are registered
// ---------------------------------------------------------------------------

func TestResourceRulesRegistered(t *testing.T) {
	expected := []string{
		"HardcodedValuesResource",
		"MissingContentDescriptionResource",
		"PxUsageResource",
		"NestedScrollingResource",
		"TooManyViewsResource",
		"TooDeepLayoutResource",
		"UselessParentResource",
		"ObsoleteLayoutParamsResource",
		"MergeRootFrameResource",
		"DuplicateIdsResource",
		"WebViewInScrollViewResource",
		"InefficientWeightResource",
		"DisableBaselineAlignmentResource",
		"NestedWeightsResource",
		"UselessLeafResource",
		"ScrollViewCountResource",
		"RequiredSizeResource",
		"SpUsageResource",
		"TextFieldsResource",
		"NegativeMarginResource",
		"RtlHardcodedResource",
		"LabelForResource",
		"OnClickResource",
		"BackButtonResource",
		"ButtonOrderResource",
		"OverdrawResource",
		"RtlSymmetryResource",
		"AlwaysShowActionResource",
		"ClickableViewAccessibilityResource",
		"RelativeOverlapResource",
		"MissingQuantityResource",
		"UnusedQuantityResource",
		"ImpliedQuantityResource",
		"StringFormatTrivialResource",
		"StringNotLocalizableResource",
		"GoogleApiKeyInResources",
	}

	registered := make(map[string]bool)
	for _, r := range v2rules.Registry {
		if r.Needs.Has(v2rules.NeedsResources) {
			registered[r.ID] = true
		}
	}

	for _, name := range expected {
		if !registered[name] {
			t.Errorf("resource rule %q not registered in v2 Registry (NeedsResources)", name)
		}
	}
}

// ---------------------------------------------------------------------------
// MissingQuantityResource
// ---------------------------------------------------------------------------

func TestMissingQuantityResource(t *testing.T) {
	r := findResourceRule(t, "MissingQuantityResource")

	t.Run("missing other triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one": "%d apple",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "MissingQuantityResource" {
			t.Fatalf("expected rule MissingQuantityResource, got %s", findings[0].Rule)
		}
	})

	t.Run("has other is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty plurals is clean", func(t *testing.T) {
		idx := emptyIndex()
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UnusedQuantityResource
// ---------------------------------------------------------------------------

func TestUnusedQuantityResource(t *testing.T) {
	r := findResourceRule(t, "UnusedQuantityResource")

	t.Run("unused zero triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"zero":  "no apples",
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unused two few many triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"two":   "%d items",
			"few":   "%d items",
			"many":  "%d items",
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("only one and other is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ImpliedQuantityResource
// ---------------------------------------------------------------------------

func TestImpliedQuantityResource(t *testing.T) {
	r := findResourceRule(t, "ImpliedQuantityResource")

	t.Run("one without %d triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "One apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("one with %d is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no one quantity is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatTrivialResource
// ---------------------------------------------------------------------------

func TestStringFormatTrivialResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatTrivialResource")

	t.Run("single %s triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello, %s!"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiple specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello, %s! You have %d messages."
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single %d is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["count"] = "You have %d items"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["hello"] = "Hello World"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("escaped %% is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["percent"] = "100%% complete"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringNotLocalizableResource
// ---------------------------------------------------------------------------

func TestStringNotLocalizableResource(t *testing.T) {
	r := findResourceRule(t, "StringNotLocalizableResource")

	t.Run("URL triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["website"] = "https://example.com"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("http URL triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["api"] = "http://api.example.com/v1"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("email triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["support_email"] = "support@example.com"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("all uppercase triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["api_key_label"] = "API_KEY"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("normal string is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello World"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty string is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["empty"] = ""
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mixed case is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["title"] = "My Application"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single char uppercase is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["single"] = "A"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GoogleApiKeyInResources
// ---------------------------------------------------------------------------

func TestGoogleApiKeyInResources(t *testing.T) {
	r := findResourceRule(t, "GoogleApiKeyInResources")

	t.Run("hardcoded key in strings xml triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["google_maps_api_key"] = "AIzaSyExampleApiKeyValue1234567890"
		idx.StringsLocation["google_maps_api_key"] = android.StringLocation{
			FilePath: "app/src/main/res/values/strings.xml",
			Line:     7,
		}

		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].File != "app/src/main/res/values/strings.xml" {
			t.Fatalf("expected finding on strings.xml, got %q", findings[0].File)
		}
		if findings[0].Line != 7 {
			t.Fatalf("expected finding on line 7, got %d", findings[0].Line)
		}
	})

	t.Run("@string reference is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["google_maps_api_key"] = "  @string/injected_at_build_time  "
		idx.StringsLocation["google_maps_api_key"] = android.StringLocation{
			FilePath: "app/src/main/res/values-es/strings.xml",
			Line:     4,
		}

		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non matching resource name is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["google_maps_token"] = "AIzaSyExampleApiKeyValue1234567890"
		idx.StringsLocation["google_maps_token"] = android.StringLocation{
			FilePath: "app/src/main/res/values/strings.xml",
			Line:     9,
		}

		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non strings xml path is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["google_maps_api_key"] = "AIzaSyExampleApiKeyValue1234567890"
		idx.StringsLocation["google_maps_api_key"] = android.StringLocation{
			FilePath: "app/src/main/res/values/secrets.xml",
			Line:     3,
		}

		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ButtonCaseResource
// ---------------------------------------------------------------------------

func TestButtonCaseResource(t *testing.T) {
	r := findResourceRule(t, "ButtonCaseResource")

	t.Run("ok lowercase triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "ok",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "should be `OK`") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("Ok mixed case triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Ok",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("OK correct is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "OK",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cancel lowercase triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "cancel",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "should be `CANCEL`") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("CANCEL correct is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "CANCEL",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("string resource is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/ok_button",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-Button is ignored", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "ok",
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
// ButtonStyleResource
// ---------------------------------------------------------------------------

func TestButtonStyleResource(t *testing.T) {
	r := findResourceRule(t, "ButtonStyleResource")

	t.Run("button in dialog without borderless triggers", func(t *testing.T) {
		idx := indexWithLayout("dialog_confirm", "res/layout/dialog_confirm.xml", &android.View{
			Type:       "Button",
			Line:       10,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "borderless") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("button in alert layout without borderless triggers", func(t *testing.T) {
		idx := indexWithLayout("alert_warning", "res/layout/alert_warning.xml", &android.View{
			Type:       "Button",
			Line:       10,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("button with borderless style is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog_confirm", "res/layout/dialog_confirm.xml", &android.View{
			Type: "Button",
			Line: 10,
			Attributes: map[string]string{
				"style": "?android:attr/borderlessButtonStyle",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("button with Borderless in style is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog_confirm", "res/layout/dialog_confirm.xml", &android.View{
			Type: "Button",
			Line: 10,
			Attributes: map[string]string{
				"style": "@style/Widget.AppCompat.Button.Borderless",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("button in non-dialog layout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Button",
			Line:       10,
			Attributes: map[string]string{},
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
// AppCompatResource
// ---------------------------------------------------------------------------

func TestAppCompatResource(t *testing.T) {
	r := findResourceRule(t, "AppCompatResource")

	t.Run("android:showAsAction triggers", func(t *testing.T) {
		idx := indexWithLayout("menu_main", "res/menu/menu_main.xml", &android.View{
			Type: "item",
			Line: 5,
			Attributes: map[string]string{
				"android:showAsAction": "ifRoom",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "app:showAsAction") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("app:showAsAction is clean", func(t *testing.T) {
		idx := indexWithLayout("menu_main", "res/menu/menu_main.xml", &android.View{
			Type: "item",
			Line: 5,
			Attributes: map[string]string{
				"app:showAsAction": "ifRoom",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no showAsAction is clean", func(t *testing.T) {
		idx := indexWithLayout("menu_main", "res/menu/menu_main.xml", &android.View{
			Type:       "item",
			Line:       5,
			Attributes: map[string]string{},
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
// RtlSuperscriptResource
// ---------------------------------------------------------------------------

func TestRtlSuperscriptResource(t *testing.T) {
	r := findResourceRule(t, "RtlSuperscriptResource")

	t.Run("superscript triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "superscript",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "RTL") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("subscript triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "subscript",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("bold is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "bold",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no textStyle is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
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
// AdapterViewChildrenResource
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

func TestMissingPrefixResource(t *testing.T) {
	r := findResourceRule(t, "MissingPrefixResource")

	t.Run("bare text attribute triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"text": "Hello",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "android:text") {
			t.Fatalf("expected message to suggest android:text, got: %s", findings[0].Message)
		}
	})

	t.Run("bare id attribute triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"id": "@+id/foo",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("android:text is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/hello",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("style attribute is clean (no prefix needed)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"style": "@style/MyStyle",
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
// NamespaceTypoResource
// ---------------------------------------------------------------------------

func TestNamespaceTypoResource(t *testing.T) {
	r := findResourceRule(t, "NamespaceTypoResource")

	t.Run("typo in namespace triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android": "http://schemas.android.com/apk/res/androd",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "typo") {
			t.Fatalf("expected message to mention typo, got: %s", findings[0].Message)
		}
	})

	t.Run("correct namespace is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android": "http://schemas.android.com/apk/res/android",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("completely different namespace is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:app": "http://schemas.android.com/apk/res-auto",
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
// OrientationResource
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

func TestSmallSpResource(t *testing.T) {
	r := findResourceRule(t, "SmallSpResource")

	t.Run("textSize 8sp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "8sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "8sp") {
			t.Fatalf("expected message to mention 8sp, got: %s", findings[0].Message)
		}
	})

	t.Run("textSize 11sp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "11sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("textSize 12sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "12sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize 16sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "16sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize in dp is clean (not sp)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "8dp",
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
// Suspicious0dpResource
// ---------------------------------------------------------------------------

func TestSuspicious0dpResource(t *testing.T) {
	r := findResourceRule(t, "Suspicious0dpResource")

	t.Run("0dp height in horizontal LinearLayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "wrap_content",
						"android:layout_height": "0dp",
					},
					LayoutWidth:  "wrap_content",
					LayoutHeight: "0dp",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_height") {
			t.Fatalf("expected message to mention layout_height, got: %s", findings[0].Message)
		}
	})

	t.Run("0dp width in vertical LinearLayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "vertical",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "0dp",
						"android:layout_height": "wrap_content",
					},
					LayoutWidth:  "0dp",
					LayoutHeight: "wrap_content",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_width") {
			t.Fatalf("expected message to mention layout_width, got: %s", findings[0].Message)
		}
	})

	t.Run("0dp width in horizontal LinearLayout is clean (correct usage)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "0dp",
						"android:layout_height": "wrap_content",
						"android:layout_weight": "1",
					},
					LayoutWidth:  "0dp",
					LayoutHeight: "wrap_content",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("0dp height in vertical LinearLayout is clean (correct usage)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "vertical",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "wrap_content",
						"android:layout_height": "0dp",
						"android:layout_weight": "1",
					},
					LayoutWidth:  "wrap_content",
					LayoutHeight: "0dp",
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
// InOrMmUsageResource
// ---------------------------------------------------------------------------

func TestInOrMmUsageResource(t *testing.T) {
	r := findResourceRule(t, "InOrMmUsageResource")

	t.Run("mm in layout_width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "10mm",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "mm") {
			t.Fatalf("expected message to mention mm, got: %s", findings[0].Message)
		}
	})

	t.Run("in in layout_height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_height": "0.5in",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "in") {
			t.Fatalf("expected message to mention in, got: %s", findings[0].Message)
		}
	})

	t.Run("dp in layout_width is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mm in dimens triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["button_height"] = "5mm"
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
// StateListReachableResource
// ---------------------------------------------------------------------------

func TestStateListReachableResource(t *testing.T) {
	r := findResourceRule(t, "StateListReachableResource")

	t.Run("empty index returns no findings (placeholder)", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

func TestWrongCaseResource(t *testing.T) {
	r := findResourceRule(t, "WrongCaseResource")

	t.Run("Textview triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Textview",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "TextView") {
			t.Fatalf("expected suggestion for TextView, got %q", findings[0].Message)
		}
	})

	t.Run("linearlayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "linearlayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "LinearLayout") {
			t.Fatalf("expected suggestion for LinearLayout, got %q", findings[0].Message)
		}
	})

	t.Run("correct casing is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("fully qualified name is skipped", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "com.example.CustomView",
			Line:       5,
			Attributes: map[string]string{},
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
// ExtraTextResource
// ---------------------------------------------------------------------------

func TestExtraTextResource(t *testing.T) {
	r := findResourceRule(t, "ExtraTextResource")

	t.Run("stray text in values file triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.ExtraTexts = []android.ExtraTextEntry{
			{FilePath: "res/values/strings.xml", Line: 3, Text: "some stray text"},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Extraneous text") {
			t.Fatalf("expected message about extraneous text, got %q", findings[0].Message)
		}
	})

	t.Run("no extra text is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// IllegalResourceRefResource
// ---------------------------------------------------------------------------

func TestIllegalResourceRefResource(t *testing.T) {
	r := findResourceRule(t, "IllegalResourceRefResource")

	t.Run("malformed reference without type triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:background": "@/missing_type",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Missing resource type") {
			t.Fatalf("expected message about missing type, got %q", findings[0].Message)
		}
	})

	t.Run("reference without slash triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:background": "@badref",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Malformed resource reference") {
			t.Fatalf("expected message about malformed reference, got %q", findings[0].Message)
		}
	})

	t.Run("reference with empty name triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 5,
			Attributes: map[string]string{
				"android:src": "@drawable/",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Missing resource name") {
			t.Fatalf("expected message about missing name, got %q", findings[0].Message)
		}
	})

	t.Run("valid resource reference is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:text":       "@string/hello",
				"android:background": "@drawable/bg",
				"android:id":         "@+id/title",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("@null is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:background": "@null",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("@android:type/name is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@android:string/ok",
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
// WrongFolderResource
// ---------------------------------------------------------------------------

func TestWrongFolderResource(t *testing.T) {
	r := findResourceRule(t, "WrongFolderResource")

	t.Run("drawable reference not found triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 5,
			Attributes: map[string]string{
				"android:src": "@drawable/missing_icon",
			},
		})
		idx.Drawables = []string{"existing_icon"}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "not found") {
			t.Fatalf("expected message about not found, got %q", findings[0].Message)
		}
	})

	t.Run("drawable reference found is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 5,
			Attributes: map[string]string{
				"android:src": "@drawable/existing_icon",
			},
		})
		idx.Drawables = []string{"existing_icon"}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no drawables in index skips check", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			Line: 5,
			Attributes: map[string]string{
				"android:src": "@drawable/anything",
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
// ResAutoResource
// ---------------------------------------------------------------------------

func TestResAutoResource(t *testing.T) {
	r := findResourceRule(t, "ResAutoResource")

	t.Run("hardcoded package namespace triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:app": "http://schemas.android.com/apk/res/com.example.app",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "res-auto") {
			t.Fatalf("expected message about res-auto, got %q", findings[0].Message)
		}
	})

	t.Run("res-auto namespace is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:app": "http://schemas.android.com/apk/res-auto",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("android namespace is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android": "http://schemas.android.com/apk/res/android",
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
// UseCompoundDrawablesResource
// ---------------------------------------------------------------------------

func TestUseCompoundDrawablesResource(t *testing.T) {
	r := findResourceRule(t, "UseCompoundDrawablesResource")

	t.Run("LinearLayout with ImageView + TextView triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "ImageView",
					Line:       3,
					Attributes: map[string]string{},
				},
				{
					Type:       "TextView",
					Line:       7,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "compound drawable") {
			t.Fatalf("expected message about compound drawable, got %q", findings[0].Message)
		}
	})

	t.Run("LinearLayout with ImageView + TextView and scaleType is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "ImageView",
					Line: 3,
					Attributes: map[string]string{
						"android:scaleType": "centerCrop",
					},
				},
				{
					Type:       "TextView",
					Line:       7,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with 3 children is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "ImageView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
				{Type: "Button", Line: 7, Attributes: map[string]string{}},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with two TextViews is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
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
// UnusedNamespaceResource
// ---------------------------------------------------------------------------

func TestUnusedNamespaceResource(t *testing.T) {
	r := findResourceRule(t, "UnusedNamespaceResource")

	t.Run("unused xmlns:tools triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android":         "http://schemas.android.com/apk/res/android",
				"xmlns:tools":           "http://schemas.android.com/tools",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 5,
					Attributes: map[string]string{
						"android:text": "Hello",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "xmlns:tools") {
			t.Fatalf("expected message about xmlns:tools, got %q", findings[0].Message)
		}
	})

	t.Run("used xmlns:tools is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android":         "http://schemas.android.com/apk/res/android",
				"xmlns:tools":           "http://schemas.android.com/tools",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 5,
					Attributes: map[string]string{
						"android:text": "Hello",
						"tools:ignore": "HardcodedText",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple unused namespaces all trigger", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android":         "http://schemas.android.com/apk/res/android",
				"xmlns:app":             "http://schemas.android.com/apk/res-auto",
				"xmlns:tools":           "http://schemas.android.com/tools",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings (app and tools), got %d", len(findings))
		}
	})

	t.Run("no xmlns declarations is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
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
// InvalidResourceFolderResource
// ---------------------------------------------------------------------------

func TestInvalidResourceFolderResource(t *testing.T) {
	r := findResourceRule(t, "InvalidResourceFolderResource")

	t.Run("invalid folder triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/lyout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "lyout") {
			t.Errorf("expected message to mention 'lyout', got %q", findings[0].Message)
		}
	})

	t.Run("valid layout folder is clean", func(t *testing.T) {
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

	t.Run("valid layout-land folder is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout-land/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("valid drawable folder is clean", func(t *testing.T) {
		idx := indexWithLayout("icon", "res/drawable-hdpi/icon.xml", &android.View{
			Type:       "vector",
			Line:       1,
			Attributes: map[string]string{},
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
// MissingIdResource
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
}

// ---------------------------------------------------------------------------
// WrongRegionResource
// ---------------------------------------------------------------------------

func TestWrongRegionResource(t *testing.T) {
	r := findResourceRule(t, "WrongRegionResource")

	t.Run("suspicious en-rBR triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/values-en-rBR/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "en") {
			t.Errorf("expected message to mention 'en', got %q", findings[0].Message)
		}
	})

	t.Run("valid en-rUS is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/values-en-rUS/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no qualifier is clean", func(t *testing.T) {
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

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UnusedAttributeResource
// ---------------------------------------------------------------------------

func TestUnusedAttributeResource(t *testing.T) {
	r := findResourceRule(t, "UnusedAttributeResource")

	t.Run("elevation attribute triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "CardView",
			Line: 5,
			Attributes: map[string]string{
				"android:elevation": "4dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "elevation") {
			t.Errorf("expected message to mention 'elevation', got %q", findings[0].Message)
		}
	})

	t.Run("translationZ triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "View",
			Line: 5,
			Attributes: map[string]string{
				"android:translationZ": "2dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("stateListAnimator triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:stateListAnimator": "@null",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("view without api21 attributes is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/hello",
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
// StringFormatInvalidResource
// ---------------------------------------------------------------------------

func TestStringFormatInvalidResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatInvalidResource")

	t.Run("bare percent at end triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello %"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "bare") {
			t.Fatalf("expected message about bare %%, got: %s", findings[0].Message)
		}
	})

	t.Run("invalid conversion char triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["bad"] = "Value is %z"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "invalid conversion") {
			t.Fatalf("expected message about invalid conversion, got: %s", findings[0].Message)
		}
	})

	t.Run("valid format specifiers are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok1"] = "Hello %s, you have %d items"
		idx.Strings["ok2"] = "Price: %1$f"
		idx.Strings["ok3"] = "100%% complete"
		idx.Strings["ok4"] = "line%n"
		idx.Strings["ok5"] = "hex: %x"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no format specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["plain"] = "Hello world"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatInvalid", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatCountResource
// ---------------------------------------------------------------------------

func TestStringFormatCountResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")

	t.Run("gap in positional args triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%1$s and %3$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "gap") {
			t.Fatalf("expected message about gap, got: %s", findings[0].Message)
		}
		if !strings.Contains(findings[0].Message, "%2$") {
			t.Fatalf("expected message to mention %%2$, got: %s", findings[0].Message)
		}
	})

	t.Run("consecutive positional args are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%1$s and %2$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-positional args are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%s and %d"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single positional arg is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%1$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatCount", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatMatchesResource
// ---------------------------------------------------------------------------

func TestStringFormatMatchesResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatMatchesResource")

	t.Run("type mismatch across quantities triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%s items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "mismatch") {
			t.Fatalf("expected message about mismatch, got: %s", findings[0].Message)
		}
	})

	t.Run("different arg counts triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%d of %d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "format args") {
			t.Fatalf("expected message about format arg count, got: %s", findings[0].Message)
		}
	})

	t.Run("consistent types are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single quantity is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no format specifiers are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "item",
			"other": "items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatMatches", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NotSiblingResource
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

func TestLocaleConfigStale(t *testing.T) {
	r := findResourceRule(t, "LocaleConfigStale")

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	scan := func(t *testing.T, root string) *android.ResourceIndex {
		t.Helper()
		idx, err := android.ScanResourceDir(root)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", root, err)
		}
		return idx
	}

	t.Run("extra values locale triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"), "<resources><string name=\"app_name\">Appli</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"), "<resources><string name=\"app_name\">App DE</string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"), "<resources><string name=\"app_name\">App ES</string></resources>\n")
		write(t, filepath.Join(resDir, "xml", "locales_config.xml"), `<?xml version="1.0" encoding="utf-8"?>
<locale-config xmlns:android="http://schemas.android.com/apk/res/android">
    <locale android:name="en" />
    <locale android:name="fr" />
    <locale android:name="de" />
</locale-config>
`)

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "extra values locales: es") {
			t.Fatalf("expected finding to mention extra locale es, got %q", findings[0].Message)
		}
	})

	t.Run("default plus matching variants is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"), "<resources><string name=\"app_name\">Appli</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"), "<resources><string name=\"app_name\">App DE</string></resources>\n")
		write(t, filepath.Join(resDir, "xml", "locales_config.xml"), `<?xml version="1.0" encoding="utf-8"?>
<locale-config xmlns:android="http://schemas.android.com/apk/res/android">
    <locale android:name="en" />
    <locale android:name="fr" />
    <locale android:name="de" />
</locale-config>
`)

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("missing locales_config is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutClickableWithoutMinSize
// ---------------------------------------------------------------------------

func TestLayoutClickableWithoutMinSize(t *testing.T) {
	r := findResourceRule(t, "LayoutClickableWithoutMinSize")

	t.Run("small clickable triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:         "Button",
			LayoutWidth:  "32dp",
			LayoutHeight: "32dp",
			Line:         5,
			Attributes: map[string]string{
				"android:clickable":     "true",
				"android:layout_width":  "32dp",
				"android:layout_height": "32dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("48dp clickable is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:         "Button",
			LayoutWidth:  "48dp",
			LayoutHeight: "48dp",
			Line:         5,
			Attributes: map[string]string{
				"android:clickable":     "true",
				"android:layout_width":  "48dp",
				"android:layout_height": "48dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("wrap_content is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:         "Button",
			LayoutWidth:  "wrap_content",
			LayoutHeight: "wrap_content",
			Line:         5,
			Attributes: map[string]string{
				"android:clickable":     "true",
				"android:layout_width":  "wrap_content",
				"android:layout_height": "wrap_content",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutEditTextMissingImportance
// ---------------------------------------------------------------------------

func TestLayoutEditTextMissingImportance(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")

	t.Run("EditText without importantForAutofill triggers", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType": "textEmailAddress",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("EditText with importantForAutofill is clean", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:importantForAutofill": "yes",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutImportantForAccessibilityNo
// ---------------------------------------------------------------------------

func TestLayoutImportantForAccessibilityNo(t *testing.T) {
	r := findResourceRule(t, "LayoutImportantForAccessibilityNo")

	t.Run("clickable with importantForAccessibility=no triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button",
			Line: 5,
			Attributes: map[string]string{
				"android:importantForAccessibility": "no",
				"android:clickable":                 "true",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("non-interactive with importantForAccessibility=no is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "View",
			Line: 5,
			Attributes: map[string]string{
				"android:importantForAccessibility": "no",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutAutofillHintMismatch
// ---------------------------------------------------------------------------

func TestLayoutAutofillHintMismatch(t *testing.T) {
	r := findResourceRule(t, "LayoutAutofillHintMismatch")

	t.Run("email input without autofillHints triggers", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType": "textEmailAddress",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("email input with autofillHints is clean", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType":     "textEmailAddress",
				"android:autofillHints": "emailAddress",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("text input type without mapping is clean", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType": "text",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutMinTouchTargetInButtonRow
// ---------------------------------------------------------------------------

func TestLayoutMinTouchTargetInButtonRow(t *testing.T) {
	r := findResourceRule(t, "LayoutMinTouchTargetInButtonRow")

	t.Run("button in linear layout with wrap_content and no minHeight triggers", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("button with minHeight >= 48dp is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type:       "LinearLayout",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
						"android:minHeight":     "48dp",
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
// StringNotSelectable
// ---------------------------------------------------------------------------

func TestStringNotSelectable(t *testing.T) {
	r := findResourceRule(t, "StringNotSelectable")

	t.Run("non-selectable text with URL triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textIsSelectable": "false",
				"android:text":            "Visit https://example.com for info",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("non-selectable text without URLs is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textIsSelectable": "false",
				"android:text":            "Hello World",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringRepeatedInContentDescription
// ---------------------------------------------------------------------------

func TestStringRepeatedInContentDescription(t *testing.T) {
	r := findResourceRule(t, "StringRepeatedInContentDescription")

	t.Run("contentDescription duplicates sibling text triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:               "ImageView",
					ContentDescription: "Settings",
					Line:               5,
					Attributes:         map[string]string{},
				},
				{
					Type:       "TextView",
					Text:       "Settings",
					Line:       6,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("contentDescription differs from sibling text is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:               "ImageView",
					ContentDescription: "Open settings",
					Line:               5,
					Attributes:         map[string]string{},
				},
				{
					Type:       "TextView",
					Text:       "Settings",
					Line:       6,
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
// StringSpanInContentDescription
// ---------------------------------------------------------------------------

func TestStringSpanInContentDescription(t *testing.T) {
	r := findResourceRule(t, "StringSpanInContentDescription")

	t.Run("string with HTML used in contentDescription triggers", func(t *testing.T) {
		layout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type:               "ImageView",
				ContentDescription: "@string/img_desc",
				Line:               5,
				Attributes:         map[string]string{},
			},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": layout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {"": layout},
			},
			Strings: map[string]string{
				"img_desc": "<b>Bold</b> image description",
			},
			StringsLocation: map[string]android.StringLocation{
				"img_desc": {FilePath: "res/values/strings.xml", Line: 3},
			},
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
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("plain string in contentDescription is clean", func(t *testing.T) {
		layout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type:               "ImageView",
				ContentDescription: "@string/img_desc",
				Line:               5,
				Attributes:         map[string]string{},
			},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": layout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {"": layout},
			},
			Strings: map[string]string{
				"img_desc": "Profile image",
			},
			StringsLocation: map[string]android.StringLocation{
				"img_desc": {FilePath: "res/values/strings.xml", Line: 3},
			},
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
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
