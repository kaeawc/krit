package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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

	t.Run("EditText with only a hint is still reported, as in Lint", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:hint": "@string/hint",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NegativeMarginResource
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ButtonOrderResource
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// OverdrawResource
// ---------------------------------------------------------------------------

func TestOverdrawResource(t *testing.T) {
	r := findResourceRule(t, "OverdrawResource")

	t.Run("root and child layout both with background triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Background: "@color/white",
			Attributes: map[string]string{"android:background": "@color/white"},
			Children: []*android.View{
				{
					Type:         "FrameLayout",
					Line:         3,
					Background:   "@color/white",
					LayoutWidth:  "match_parent",
					LayoutHeight: "match_parent",
					Attributes:   map[string]string{"android:background": "@color/white"},
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

// ---------------------------------------------------------------------------
// ButtonStyleResource
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AppCompatResource
// ---------------------------------------------------------------------------

func runAppCompatResource(r *api.Rule, idx *android.ResourceIndex, profile librarymodel.ProjectProfile) []scanner.Finding {
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{ResourceIndex: idx, Rule: r, Collector: collector, LibraryFacts: &librarymodel.Facts{Profile: profile}}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestAppCompatResource(t *testing.T) {
	r := findResourceRule(t, "AppCompatResource")
	withAppCompat := librarymodel.ProjectProfile{
		HasGradle: true, DependencyExtractionComplete: true,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.appcompat", Name: "appcompat"}},
	}
	withoutAppCompat := librarymodel.ProjectProfile{HasGradle: true, DependencyExtractionComplete: true}
	unknown := librarymodel.ProjectProfile{HasGradle: true}
	menu := func(path string, attrs map[string]string) *android.ResourceIndex {
		return indexWithMenu("menu_main", path, &android.View{Type: "item", Line: 5, Attributes: attrs})
	}

	t.Run("android:showAsAction with appcompat triggers", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"android:showAsAction": "ifRoom"})
		findings := runAppCompatResource(r, idx, withAppCompat)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		want := "Should use `app:showAsAction` with the appcompat library with `xmlns:app=\"http://schemas.android.com/apk/res-auto\"`"
		if findings[0].Message != want {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("actionViewClass and actionProviderClass are covered", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"android:actionViewClass": "x", "android:actionProviderClass": "y"})
		if findings := runAppCompatResource(r, idx, withAppCompat); len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("v14 menu folder may use the android namespace", func(t *testing.T) {
		idx := menu("res/menu-v14/menu_main.xml", map[string]string{"android:showAsAction": "ifRoom"})
		if findings := runAppCompatResource(r, idx, withAppCompat); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("app:showAsAction without appcompat triggers", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"app:showAsAction": "ifRoom"})
		findings := runAppCompatResource(r, idx, withoutAppCompat)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Message != "Should use `android:showAsAction` when not using the appcompat library" {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("android:showAsAction without appcompat is clean", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"android:showAsAction": "ifRoom"})
		if findings := runAppCompatResource(r, idx, withoutAppCompat); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("app:showAsAction with appcompat is clean", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"app:showAsAction": "ifRoom"})
		if findings := runAppCompatResource(r, idx, withAppCompat); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unknown dependency graph is silent", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{"android:showAsAction": "ifRoom", "app:showAsAction": "never"})
		if findings := runAppCompatResource(r, idx, unknown); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no showAsAction is clean", func(t *testing.T) {
		idx := menu("res/menu/menu_main.xml", map[string]string{})
		if findings := runAppCompatResource(r, idx, withAppCompat); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		if findings := runAppCompatResource(r, emptyIndex(), withAppCompat); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RtlSuperscriptResource
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

func TestTextFieldsResource_StyleOrLegacyAttributeSetsInput(t *testing.T) {
	r := findResourceRule(t, "TextFieldsResource")
	// Lint resolves the style, and reports the legacy input attributes as a missing inputType.
	for attr, want := range map[string]int{"style": 0, "android:inputMethod": 0, "android:digits": 1, "android:numeric": 1, "android:phoneNumber": 1, "android:password": 1} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText", Line: 3, Attributes: map[string]string{attr: "x"},
		})
		if findings := runResourceRule(r, idx); len(findings) != want {
			t.Errorf("%s: expected %d findings, got %d", attr, want, len(findings))
		}
	}
}
