package rules_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
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

	t.Run("the 80th element is reported once, like Lint", func(t *testing.T) {
		root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}}
		for i := 0; i < 79; i++ {
			root.Children = append(root.Children, &android.View{Type: "View", Line: i + 2, Col: 3, Attributes: map[string]string{}})
		}
		findings := runResourceRule(r, indexWithLayout("huge", "res/layout/huge.xml", root))
		if len(findings) != 1 || findings[0].Line != 80 {
			t.Fatalf("expected one finding at line 80, got %+v", findings)
		}
		root.Children = root.Children[:78]
		if findings := runResourceRule(r, indexWithLayout("huge", "res/layout/huge.xml", root)); len(findings) != 0 {
			t.Fatalf("79 elements must not report, got %+v", findings)
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

	t.Run("wrapper with background only one of the pair has asks to transfer it", func(t *testing.T) {
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	frameRoot := func() *android.View {
		return &android.View{
			Type:       "FrameLayout",
			Line:       1,
			Attributes: map[string]string{"android:layout_width": "match_parent", "android:layout_height": "match_parent"},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
			},
		}
	}
	withHost := func(hostRoot *android.View) *android.ResourceIndex {
		idx := indexWithLayout("main", "res/layout/main.xml", frameRoot())
		host := &android.Layout{Name: "host", FilePath: "res/layout/host.xml", RootView: hostRoot}
		idx.Layouts["host"] = host
		idx.LayoutConfigs["host"] = map[string]*android.Layout{"": host}
		return idx
	}

	t.Run("included plain root FrameLayout triggers", func(t *testing.T) {
		idx := withHost(&android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
			{Type: "include", Line: 2, Attributes: map[string]string{"layout": "@layout/main"}},
		}})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("root FrameLayout never included is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", frameRoot())
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("root FrameLayout used by a ViewStub is clean", func(t *testing.T) {
		idx := withHost(&android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
			{Type: "ViewStub", Line: 2, Attributes: map[string]string{"android:layout": "@layout/main"}},
		}})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("fixed-size root FrameLayout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "FrameLayout",
			Line:       1,
			Attributes: map[string]string{"android:layout_width": "200dp", "android:layout_height": "match_parent"},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("message and root conditions follow Lint", func(t *testing.T) {
		include := func() *android.View {
			return &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
				{Type: "include", Line: 2, Attributes: map[string]string{"layout": "@layout/main"}},
			}}
		}
		run := func(attrs map[string]string) []scanner.Finding {
			root := frameRoot()
			root.Attributes = attrs
			idx := withHost(include())
			idx.Layouts["main"].RootView = root
			return runResourceRule(r, idx)
		}
		fill := map[string]string{"android:layout_width": "match_parent", "android:layout_height": "match_parent"}
		with := func(k, v string) map[string]string {
			m := map[string]string{}
			for a, b := range fill {
				m[a] = b
			}
			m[k] = v
			return m
		}
		got := run(fill)
		if len(got) != 1 || got[0].Message != "This `<FrameLayout>` can be replaced with a `<merge>` tag" {
			t.Fatalf("got %+v", got)
		}
		// Start and End padding do not stop Lint; a fixed size does not either without a layout_gravity.
		if got := run(with("android:paddingStart", "4dp")); len(got) != 1 {
			t.Errorf("paddingStart: expected 1 finding, got %d", len(got))
		}
		if got := run(map[string]string{"android:layout_width": "200dp", "android:layout_height": "wrap_content"}); len(got) != 1 {
			t.Errorf("fixed size: expected 1 finding, got %d", len(got))
		}
		for _, attr := range []string{"android:padding", "android:paddingTop", "android:foreground"} {
			if got := run(with(attr, "4dp")); len(got) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", attr, len(got))
			}
		}
		if got := run(with("android:fitsSystemWindows", "true")); len(got) != 0 {
			t.Errorf("fitsSystemWindows: expected 0 findings, got %d", len(got))
		}
		if got := run(map[string]string{"android:layout_width": "wrap_content", "android:layout_height": "match_parent", "android:layout_gravity": "center"}); len(got) != 0 {
			t.Errorf("layout_gravity without fill: expected 0 findings, got %d", len(got))
		}
		if got := run(with("android:layout_gravity", "center")); len(got) != 1 {
			t.Errorf("layout_gravity with fill: expected 1 finding, got %d", len(got))
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

	t.Run("message and severity follow Lint", func(t *testing.T) {
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
		if len(findings) != 1 || findings[0].Message != "A scroll view can have only one child" || findings[0].Severity != "warning" {
			t.Fatalf("unexpected findings: %+v", findings)
		}
	})

	t.Run("marker tags are not children and unqualified NestedScrollView is ignored", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "ScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "requestFocus", Line: 4, Attributes: map[string]string{}},
			},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
		idx = indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "NestedScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
			},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
		idx = indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "androidx.core.widget.NestedScrollView",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{}},
				{Type: "TextView", Line: 5, Attributes: map[string]string{}},
			},
		})
		if findings := runResourceRule(r, idx); len(findings) != 1 {
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
		if findings[0].Message != "A list/grid should have no children declared in XML" {
			t.Fatalf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("Spinner Gallery and ExpandableListView children are not reported", func(t *testing.T) {
		for _, typ := range []string{"Spinner", "Gallery", "ExpandableListView"} {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: typ, Line: 3, Attributes: map[string]string{},
				Children: []*android.View{{Type: "TextView", Line: 5, Attributes: map[string]string{}}},
			})
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Fatalf("%s: expected 0 findings, got %d", typ, len(findings))
			}
		}
	})

	t.Run("marker tags are not children", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ListView", Line: 3, Attributes: map[string]string{},
			Children: []*android.View{{Type: "requestFocus", Line: 4, Attributes: map[string]string{}}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("margin without width and height triggers once per parameter", func(t *testing.T) {
		include := func(attrs map[string]string) *android.View {
			return &android.View{
				Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
				Children: []*android.View{{Type: "include", Line: 3, Attributes: attrs}},
			}
		}
		idx := indexWithLayout("main", "res/layout/main.xml", include(map[string]string{
			"layout":                   "@layout/toolbar",
			"android:layout_marginTop": "4dp",
			"android:layout_gravity":   "top",
			"android:visibility":       "gone",
		}))
		if findings := runResourceRule(r, idx); len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		idx = indexWithLayout("main", "res/layout/main.xml", include(map[string]string{
			"layout":                    "@layout/toolbar",
			"android:layout_width":      "match_parent",
			"android:layout_height":     "wrap_content",
			"android:layout_marginTop":  "4dp",
			"app:layout_constraintTop":  "x",
			"tools:layout_marginBottom": "1dp",
		}))
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings with both dimensions, got %d", len(findings))
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

	childWithWidth := func(width string) *android.View {
		return &android.View{
			Type:         "TextView",
			Line:         2,
			LayoutWidth:  width,
			LayoutHeight: "wrap_content",
			Attributes: map[string]string{
				"android:layout_width":  width,
				"android:layout_height": "wrap_content",
			},
		}
	}
	matchParentChild := func() *android.View { return childWithWidth("match_parent") }
	wrapContentChild := func() *android.View { return childWithWidth("wrap_content") }

	t.Run("multi-child match_parent triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{matchParentChild(), wrapContentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		want := "Wrong orientation? No orientation specified, and the default is horizontal, yet this layout has multiple children where at least one has `layout_width=\"match_parent\"`"
		if findings[0].Message != want {
			t.Errorf("message = %q", findings[0].Message)
		}
	})

	t.Run("multi-child fill_parent triggers", func(t *testing.T) {
		fill := childWithWidth("fill_parent")
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{fill, wrapContentChild()},
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
			Children: []*android.View{matchParentChild(), wrapContentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("style attribute suppresses", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"style":                 "@style/MyLinear",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{matchParentChild(), wrapContentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single child is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{matchParentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no match_parent siblings is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{wrapContentChild(), wrapContentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("last-child-only match_parent is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{wrapContentChild(), matchParentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("match_parent with layout_weight is clean", func(t *testing.T) {
		weighted := matchParentChild()
		weighted.Attributes["android:layout_weight"] = "1"
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
			Children: []*android.View{weighted, wrapContentChild()},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty LinearLayout without id is clean", func(t *testing.T) {
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

	t.Run("empty LinearLayout with id triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			ID:   "@+id/container",
			Attributes: map[string]string{
				"android:id":            "@+id/container",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	t.Run("idiomatic disjoint states before default is clean", func(t *testing.T) {
		// Constrained states first (pressed, focused), unconstrained default
		// last. This is the correct, idiomatic ordering: no constrained item
		// is a subset of an earlier item, and the catch-all is last, so
		// nothing is unreachable.
		idx := emptyIndex()
		idx.DrawableSelectors = map[string][]android.SelectorItem{
			"button": {
				{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{"android:state_pressed": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 6, StateAttrs: map[string]string{"android:state_focused": "true"}},
				{FilePath: "res/drawable/button.xml", Line: 9, StateAttrs: map[string]string{}},
			},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for idiomatic ordering, got %d", len(findings))
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

	t.Run("message follows Lint, WebView grandchild and nested scroll views are skipped", func(t *testing.T) {
		match := map[string]string{"android:layout_height": "match_parent"}
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ScrollView", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "LinearLayout", Line: 3, Attributes: match}},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Message != "This LinearLayout should use `android:layout_height=\"wrap_content\"`" {
			t.Fatalf("unexpected findings: %+v", findings)
		}
		idx = indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ScrollView", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "LinearLayout", Line: 3, Attributes: match,
				Children: []*android.View{{Type: "WebView", Line: 4, Attributes: map[string]string{}}}}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("WebView grandchild: expected 0 findings, got %d", len(findings))
		}
		idx = indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "androidx.core.widget.NestedScrollView", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "LinearLayout", Line: 3, Attributes: match}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("NestedScrollView: expected 0 findings, got %d", len(findings))
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

	t.Run("NestedScrollView child with fill_parent height is not checked (Lint visits only ScrollView and HorizontalScrollView)", func(t *testing.T) {
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
// WrongCaseResource
// ---------------------------------------------------------------------------

func TestNestedScrollingResource_OnlySameAxis(t *testing.T) {
	r := findResourceRule(t, "NestedScrollingResource")
	nest := func(outer, inner string) int {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: outer, Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: inner, Line: 2, Attributes: map[string]string{}}},
		})
		return len(runResourceRule(r, idx))
	}
	if n := nest("ScrollView", "HorizontalScrollView"); n != 0 {
		t.Errorf("horizontal scroller in vertical one should be clean, got %d", n)
	}
	if n := nest("HorizontalScrollView", "ScrollView"); n != 0 {
		t.Errorf("vertical scroller in horizontal one should be clean, got %d", n)
	}
	if n := nest("ScrollView", "ScrollView"); n != 1 {
		t.Errorf("same-axis nesting should report, got %d", n)
	}
	if n := nest("ScrollView", "androidx.core.widget.NestedScrollView"); n != 0 {
		t.Errorf("Lint does not know NestedScrollView, got %d", n)
	}
	if n := nest("ScrollView", "ListView"); n != 1 {
		t.Errorf("ListView in ScrollView should report, got %d", n)
	}
	if n := nest("HorizontalScrollView", "Gallery"); n != 1 {
		t.Errorf("Gallery in HorizontalScrollView should report, got %d", n)
	}
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "ScrollView", Line: 1, Attributes: map[string]string{},
		Children: []*android.View{{Type: "ListView", Line: 2, Attributes: map[string]string{}}},
	})
	got := runResourceRule(r, idx)
	if want := "The vertically scrolling `ScrollView` should not contain another vertically scrolling widget (`ListView`)"; len(got) != 1 || got[0].Message != want {
		t.Errorf("got %+v", got)
	}
}

func TestNotSiblingResource_IncludedRootIDs(t *testing.T) {
	r := findResourceRule(t, "NotSiblingResource")
	build := func(withInclude bool, ref string) int {
		children := []*android.View{
			{Type: "TextView", ID: "@+id/a", Line: 2, Attributes: map[string]string{"android:layout_below": ref}},
		}
		if withInclude {
			children = append(children, &android.View{Type: "include", Line: 3, Attributes: map[string]string{"layout": "@layout/row"}})
		}
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout", Line: 1, Attributes: map[string]string{}, Children: children})
		idx.IDs["included_root"] = true
		return len(runResourceRule(r, idx))
	}
	if n := build(true, "@id/included_root"); n != 1 {
		t.Errorf("an include whose layout is unknown does not make the id a sibling, got %d", n)
	}
	if n := build(false, "@id/included_root"); n != 1 {
		t.Errorf("unknown id without includes should report, got %d", n)
	}
}

func TestObsoleteLayoutParams_OnlyKnownNonLinearParents(t *testing.T) {
	r := findResourceRule(t, "ObsoleteLayoutParamsResource")
	run := func(parent string) int {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: parent, Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "TextView", Line: 2, Attributes: map[string]string{"android:layout_weight": "1"}}},
		})
		return len(runResourceRule(r, idx))
	}
	for _, parent := range []string{"RadioGroup", "TableRow", "androidx.appcompat.widget.LinearLayoutCompat", "com.example.RowLayout", "com.google.android.material.button.MaterialButtonToggleGroup"} {
		if n := run(parent); n != 0 {
			t.Errorf("%s may be a LinearLayout, got %d findings", parent, n)
		}
	}
	for _, parent := range []string{"FrameLayout", "RelativeLayout", "androidx.constraintlayout.widget.ConstraintLayout"} {
		if n := run(parent); n != 1 {
			t.Errorf("%s ignores layout_weight, got %d findings", parent, n)
		}
	}
}

func TestOnClickResource_SkipsDataBindingExpressions(t *testing.T) {
	r := findResourceRule(t, "OnClickResource")
	run := func(handler string) int {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "Button", Line: 1, Attributes: map[string]string{"android:onClick": handler}})
		return len(runResourceRule(r, idx))
	}
	if n := run("@{() -> vm.save()}"); n != 0 {
		t.Errorf("data-binding lambda should be clean, got %d", n)
	}
	if n := run("save"); n != 1 {
		t.Errorf("reflective handler should report, got %d", n)
	}
}

func TestReferenceType_DrawableAliasMayReferenceColor(t *testing.T) {
	r := findResourceRule(t, "ReferenceType")
	run := func(typ, value string) int {
		idx := emptyIndex()
		idx.AliasItems = []android.AliasItem{{Name: "x", Type: typ, Value: value, FilePath: "res/values/refs.xml", Line: 2}}
		return len(runResourceRule(r, idx))
	}
	if n := run("drawable", "@color/primary"); n != 0 {
		t.Errorf("drawable alias of a color is valid, got %d", n)
	}
	if n := run("string", "@color/primary"); n != 1 {
		t.Errorf("string alias of a color is a mismatch, got %d", n)
	}
}

func TestNestedWeightsReportsTheWeightedChild(t *testing.T) {
	r := findResourceRule(t, "NestedWeightsResource")
	inner := func(weight string) *android.View {
		return &android.View{
			Type: "LinearLayout", Line: 2,
			Attributes: map[string]string{"android:layout_weight": weight},
			Children: []*android.View{
				{Type: "TextView", Line: 3, Attributes: map[string]string{"android:layout_weight": "1"}},
			},
		}
	}
	for weight, want := range map[string]int{"1": 1, "0": 1, "0.0": 1, "@dimen/w": 1} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{inner(weight)},
		})
		if got := len(runResourceRule(r, idx)); got != want {
			t.Errorf("weight %q: expected %d findings, got %d", weight, want, got)
		}
	}
}

func TestNotSiblingIgnoresSuppressedAndFrameworkReferences(t *testing.T) {
	r := findResourceRule(t, "NotSiblingResource")
	child := func(attrs map[string]string) *android.View {
		return &android.View{Type: "TextView", Line: 3, Attributes: attrs}
	}
	for name, tc := range map[string]struct {
		attrs map[string]string
		want  int
	}{
		"missing sibling":     {map[string]string{"android:layout_below": "@id/ghost"}, 1},
		"framework id":        {map[string]string{"android:layout_below": "@android:id/title"}, 0},
		"data binding":        {map[string]string{"android:layout_below": "@{ref}"}, 0},
		"tools:ignore":        {map[string]string{"android:layout_below": "@id/ghost", "tools:ignore": "NotSibling"}, 0},
		"ignored via lint id": {map[string]string{"android:layout_below": "@id/ghost", "tools:ignore": "all"}, 0},
	} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "RelativeLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{child(tc.attrs)},
		})
		idx.IDs["ghost"] = true
		if got := len(runResourceRule(r, idx)); got != tc.want {
			t.Errorf("%s: expected %d findings, got %d", name, tc.want, got)
		}
	}
}

func TestRequiredSizeSkipsTableRow(t *testing.T) {
	r := findResourceRule(t, "RequiredSizeResource")
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "TableLayout", Line: 1,
		Attributes: map[string]string{"android:layout_width": "match_parent", "android:layout_height": "wrap_content"},
		Children:   []*android.View{{Type: "TableRow", Line: 2, Attributes: map[string]string{}}},
	})
	if got := runResourceRule(r, idx); len(got) != 0 {
		t.Fatalf("TableRow dimensions come from TableLayout, got %v", findingMessages(got))
	}
}

func TestRequiredSizeLintWordingAndGridChildren(t *testing.T) {
	r := findResourceRule(t, "RequiredSizeResource")
	build := func(parent string, attrs map[string]string) []scanner.Finding {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: parent, Line: 1,
			Attributes: map[string]string{"android:layout_width": "match_parent", "android:layout_height": "wrap_content"},
			Children:   []*android.View{{Type: "TextView", Line: 2, Attributes: attrs}},
		})
		return runResourceRule(r, idx)
	}
	for name, tc := range map[string]struct {
		attrs map[string]string
		want  string
	}{
		"both":   {map[string]string{}, "The required `layout_width` and `layout_height` attributes are missing"},
		"width":  {map[string]string{"android:layout_height": "wrap_content"}, "The required `layout_width` attribute is missing"},
		"height": {map[string]string{"android:layout_width": "wrap_content"}, "The required `layout_height` attribute is missing"},
	} {
		got := build("LinearLayout", tc.attrs)
		if len(got) != 1 || got[0].Message != tc.want {
			t.Errorf("%s: got %+v", name, got)
		}
	}
	if got := build("GridLayout", map[string]string{}); len(got) != 0 {
		t.Errorf("GridLayout children need no size, got %v", findingMessages(got))
	}
}

func TestTooDeepLayoutResource_AnchorsOnTenthLevelAndHonoursIgnore(t *testing.T) {
	r := findResourceRule(t, "TooDeepLayoutResource")
	build := func(attrs map[string]string) *android.View {
		current := &android.View{Type: "TextView", Line: 14, Attributes: map[string]string{}}
		for i := 12; i >= 3; i-- {
			current = &android.View{Type: "LinearLayout", Line: i, Attributes: map[string]string{}, Children: []*android.View{current}}
		}
		return &android.View{Type: "FrameLayout", Line: 2, Col: 1, Attributes: attrs, Children: []*android.View{current}}
	}
	findings := runResourceRule(r, indexWithLayout("deep", "res/layout/deep.xml", build(map[string]string{})))
	if len(findings) != 1 || findings[0].Line != 11 {
		t.Fatalf("expected one finding on the tenth-level element at line 11, got %+v", findings)
	}
	ignored := build(map[string]string{"tools:ignore": "TooDeepLayout"})
	if findings := runResourceRule(r, indexWithLayout("deep", "res/layout/deep.xml", ignored)); len(findings) != 0 {
		t.Fatalf("expected tools:ignore to suppress, got %d findings", len(findings))
	}
}

func TestStateListReachableResource_QualifiedFoldersAreSeparateLists(t *testing.T) {
	r := findResourceRule(t, "StateListReachableResource")
	idx := emptyIndex()
	idx.DrawableSelectors = map[string][]android.SelectorItem{
		"button": {
			{FilePath: "res/drawable/button.xml", Line: 2, StateAttrs: map[string]string{"android:state_pressed": "true"}},
			{FilePath: "res/drawable/button.xml", Line: 3, StateAttrs: map[string]string{}},
			{FilePath: "res/drawable-v21/button.xml", Line: 2, StateAttrs: map[string]string{"android:state_enabled": "false"}},
			{FilePath: "res/drawable-v21/button.xml", Line: 3, StateAttrs: map[string]string{}},
		},
	}
	if findings := runResourceRule(r, idx); len(findings) != 0 {
		t.Fatalf("each file is a well-ordered state list, got %d findings: %+v", len(findings), findings)
	}
}

func TestIncludeLayoutParamResource_NamesWhatIsMissing(t *testing.T) {
	r := findResourceRule(t, "IncludeLayoutParamResource")
	run := func(attrs map[string]string) []string {
		attrs["layout"] = "@layout/toolbar"
		root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
			{Type: "include", Line: 3, Attributes: attrs},
		}}
		var msgs []string
		for _, f := range runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)) {
			msgs = append(msgs, f.Message)
		}
		return msgs
	}
	const tail = " on `<include>` tag"
	cases := []struct {
		name  string
		attrs map[string]string
		want  []string
	}{
		{"width only", map[string]string{"android:layout_width": "match_parent"},
			[]string{"Layout parameter `layout_width` ignored unless `layout_height` is also specified" + tail}},
		{"height only", map[string]string{"android:layout_height": "match_parent"},
			[]string{"Layout parameter `layout_height` ignored unless `layout_width` is also specified" + tail}},
		{"width and another parameter names only the other one", map[string]string{"android:layout_width": "0dp", "android:layout_gravity": "top"},
			[]string{"Layout parameter `layout_gravity` ignored unless `layout_height` is also specified" + tail}},
		{"no size", map[string]string{"android:layout_margin": "1dp"},
			[]string{"Layout parameter `layout_margin` ignored unless both `layout_width` and `layout_height` are also specified" + tail}},
		{"both sizes", map[string]string{"android:layout_width": "0dp", "android:layout_height": "0dp", "android:layout_margin": "1dp"}, nil},
		{"non-android layout attributes are not parameters", map[string]string{"app:layout_constraintTop_toTopOf": "parent"}, nil},
		{"no layout attributes", map[string]string{"android:id": "@+id/x"}, nil},
	}
	for _, tc := range cases {
		got := run(tc.attrs)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestGridLayoutResource(t *testing.T) {
	r := findResourceRule(t, "GridLayoutResource")
	child := func(attrs map[string]string) *android.View {
		return &android.View{Type: "TextView", Line: 4, Col: 3, Attributes: attrs}
	}
	grid := func(typ string, attrs map[string]string, children ...*android.View) *android.ResourceIndex {
		return indexWithLayout("main", "res/layout/main.xml", &android.View{Type: typ, Line: 2, Col: 1, Attributes: attrs, Children: children})
	}
	msgs := func(idx *android.ResourceIndex) []string {
		var out []string
		for _, f := range runResourceRule(r, idx) {
			out = append(out, f.Message)
		}
		return out
	}

	got := msgs(grid("GridLayout", map[string]string{"android:columnCount": "2", "android:rowCount": "2"},
		child(map[string]string{"android:layout_column": "1", "android:layout_row": "2"}),
		child(map[string]string{"android:layout_column": "2", "android:layout_row": "3"})))
	want := []string{"Column attribute (2) exceeds declared grid column count (2)", "Row attribute (3) exceeds declared grid row count (2)"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := msgs(grid("GridLayout", map[string]string{}, child(map[string]string{"android:layout_column": "9"}))); len(got) != 0 {
		t.Errorf("no declared counts must be clean, got %q", got)
	}

	v7 := grid("androidx.gridlayout.widget.GridLayout", map[string]string{"android:columnCount": "3"}, child(map[string]string{"android:layout_row": "0"}))
	got = msgs(v7)
	if len(got) != 2 || got[0] != "Wrong namespace; with v7 `GridLayout` you should use app:columnCount (and add `xmlns:app=\"http://schemas.android.com/apk/res-auto\"` to your root element)" {
		t.Errorf("v7 namespace: got %q", got)
	}
	declared := grid("androidx.gridlayout.widget.GridLayout", map[string]string{"xmlns:a": "http://schemas.android.com/apk/res-auto", "android:columnCount": "3"})
	if got := msgs(declared); len(got) != 1 || got[0] != "Wrong namespace; with v7 `GridLayout` you should use a:columnCount" {
		t.Errorf("declared prefix: got %q", got)
	}
	if got := msgs(grid("GridLayout", map[string]string{"android:orientation": "vertical"})); len(got) != 0 {
		t.Errorf("platform grid keeps android: attributes, got %q", got)
	}
}

func TestDeprecatedResource(t *testing.T) {
	r := findResourceRule(t, "DeprecatedResource")
	root := &android.View{Type: "LinearLayout", Line: 1, Col: 1, Attributes: map[string]string{}, Children: []*android.View{
		{Type: "TextView", Line: 2, Col: 3, Attributes: map[string]string{"android:autoText": "true", "android:editable": "true", "android:singleLine": "true"}},
		{Type: "AbsoluteLayout", Line: 3, Col: 3, Attributes: map[string]string{}},
		{Type: "Gallery", Line: 4, Col: 3, Attributes: map[string]string{}},
	}}
	var msgs []string
	for _, f := range runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)) {
		msgs = append(msgs, f.Message)
	}
	want := []string{
		"`AbsoluteLayout` is deprecated",
		"`android:autoText` is deprecated: Use `inputType` instead",
		"`android:editable` is deprecated: Use an `<EditText>` to make it editable",
	}
	sort.Strings(msgs)
	sort.Strings(want)
	if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q, want %q", msgs, want)
	}
}

func TestChildInNonViewGroupResource(t *testing.T) {
	r := findResourceRule(t, "ChildInNonViewGroupResource")
	child := func(typ string) *android.View {
		return &android.View{Type: typ, Line: 4, Col: 5, Attributes: map[string]string{}}
	}
	root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
		{Type: "TextView", Line: 2, Col: 3, Attributes: map[string]string{}, Children: []*android.View{child("Button")}},
		{Type: "ImageView", Line: 3, Col: 3, Attributes: map[string]string{}, Children: []*android.View{child("requestFocus")}},
		{Type: "com.example.Custom", Line: 5, Col: 3, Attributes: map[string]string{}, Children: []*android.View{child("Button")}},
		{Type: "FrameLayout", Line: 6, Col: 3, Attributes: map[string]string{}, Children: []*android.View{child("Button")}},
	}}
	got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
	if len(got) != 1 || got[0].Message != "A TextView should have no children declared in XML" || got[0].Line != 4 || got[0].Col != 6 {
		t.Fatalf("unexpected findings %+v", got)
	}
}

func TestEnforceUtf8Resource(t *testing.T) {
	r := findResourceRule(t, "EnforceUtf8Resource")
	res := filepath.Join(t.TempDir(), "res")
	write := func(rel, content string) string {
		p := filepath.Join(res, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	layout := write("layout/a.xml", `<?xml version="1.0" encoding="ISO-8859-1"?><LinearLayout/>`)
	write("values/b.xml", `<?xml version="1.0" encoding="utf8"?><resources/>`)
	write("values/c.xml", `<?xml version="1.0" encoding="UTF-8"?><resources/>`)
	write("values/d.xml", `<resources/>`)
	idx := indexWithLayout("a", layout, &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}})
	got := runResourceRule(r, idx)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %+v", got)
	}
	if got[0].Message != "ISO-8859-1: Not using UTF-8 as the file encoding. This can lead to subtle bugs with non-ascii characters" || got[0].Col != 31 {
		t.Fatalf("unexpected finding %+v", got[0])
	}
}
