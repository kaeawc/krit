package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
	for _, r := range api.Registry {
		if r.Needs.Has(api.NeedsResources) {
			registered[r.ID] = true
		}
	}

	for _, name := range expected {
		if !registered[name] {
			t.Errorf("resource rule %q not registered in rule registry (NeedsResources)", name)
		}
	}
}

// ---------------------------------------------------------------------------
// MissingQuantityResource
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

	t.Run("resources root in a layout folder triggers", func(t *testing.T) {
		idx := indexWithLayout("res", "res/layout/res.xml", &android.View{Type: "resources", Line: 1, Col: 1, Attributes: map[string]string{}})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Message != "This file should be placed in a `values`/ folder, not a `layout`/ folder" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("other roots and other folders are clean, like Lint", func(t *testing.T) {
		for path, root := range map[string]string{
			"res/layout/main.xml": "LinearLayout", "res/menu/res.xml": "resources", "res/drawable/res.xml": "resources",
		} {
			idx := indexWithLayout("x", path, &android.View{Type: root, Line: 1, Attributes: map[string]string{}})
			if got := runResourceRule(r, idx); len(got) != 0 {
				t.Errorf("%s with <%s>: expected no finding, got %d", path, root, len(got))
			}
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

func TestUnusedNamespaceResource(t *testing.T) {
	r := findResourceRule(t, "UnusedNamespaceResource")

	t.Run("unused custom-attribute namespace triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android":         "http://schemas.android.com/apk/res/android",
				"xmlns:foo":             "http://schemas.android.com/apk/res/com.foo",
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
		if !strings.Contains(findings[0].Message, "`foo`") {
			t.Fatalf("expected message about foo, got %q", findings[0].Message)
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

	t.Run("only custom-attribute namespaces are tracked, as in Lint", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"xmlns:android":         "http://schemas.android.com/apk/res/android",
				"xmlns:app":             "http://schemas.android.com/apk/res-auto",
				"xmlns:tools":           "http://schemas.android.com/tools",
				"xmlns:foo":             "http://schemas.android.com/apk/res/com.foo",
				"android:layout_width":  "match_parent",
				"android:layout_height": "match_parent",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected only foo, got %d", len(findings))
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

func unusedAttributeFindings(t *testing.T, minSdk int, path string, root *android.View) []scanner.Finding {
	t.Helper()
	r := findResourceRule(t, "UnusedAttributeResource")
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		ResourceIndex: indexWithLayout("main", path, root),
		Rule:          r,
		Collector:     collector,
		LibraryFacts:  librarymodel.FactsForProfile(librarymodel.ProjectProfile{MinSdkVersion: minSdk}),
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestUnusedAttributeResource(t *testing.T) {
	view := func(tag string, attrs map[string]string, kids ...*android.View) *android.View {
		return &android.View{Type: tag, Line: 5, Attributes: attrs, Children: kids}
	}

	t.Run("attribute newer than minSdk triggers with Lint's wording", func(t *testing.T) {
		got := unusedAttributeFindings(t, 19, "res/layout/main.xml", view("CardView", map[string]string{"android:elevation": "4dp"}))
		if len(got) != 1 || got[0].Message != "Attribute `elevation` is only used in API level 21 and higher (current min is 19)" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("the attribute table covers more than three attributes", func(t *testing.T) {
		attrs := map[string]string{"android:tooltipText": "x", "android:letterSpacing": "1", "android:backgroundTint": "#f00"}
		if got := unusedAttributeFindings(t, 19, "res/layout/main.xml", view("TextView", attrs)); len(got) != 3 {
			t.Fatalf("expected 3 findings, got %+v", got)
		}
	})

	t.Run("attributes the platform tolerates are left alone", func(t *testing.T) {
		attrs := map[string]string{
			"android:textAlignment": "center", "android:autofillHints": "name", "android:layout_marginStart": "4dp",
			"android:layout_width": "match_parent", "android:text": "x", "android:id": "@+id/a", "android:fontFamily": "sans",
		}
		if got := unusedAttributeFindings(t, 19, "res/layout/main.xml", view("TextView", attrs)); len(got) != 0 {
			t.Fatalf("expected no findings, got %+v", got)
		}
	})

	t.Run("minSdk at or above the level is clean", func(t *testing.T) {
		if got := unusedAttributeFindings(t, 21, "res/layout/main.xml", view("View", map[string]string{"android:translationZ": "2dp"})); len(got) != 0 {
			t.Fatalf("expected no findings, got %+v", got)
		}
	})

	t.Run("tools:targetApi on the element or an ancestor excuses the attribute", func(t *testing.T) {
		child := view("Button", map[string]string{"android:elevation": "4dp"})
		parent := view("LinearLayout", map[string]string{"tools:targetApi": "lollipop"}, child)
		if got := unusedAttributeFindings(t, 19, "res/layout/main.xml", parent); len(got) != 0 {
			t.Fatalf("expected no findings, got %+v", got)
		}
	})

	t.Run("foreground and include theme have their own messages", func(t *testing.T) {
		got := unusedAttributeFindings(t, 19, "res/layout/main.xml", view("LinearLayout", map[string]string{"android:foreground": "@drawable/a"},
			view("include", map[string]string{"android:theme": "@style/T"}),
			view("FrameLayout", map[string]string{"android:foreground": "@drawable/a"})))
		want := []string{
			"Attribute `android:foreground` has no effect on API levels lower than 23 (current min is 19)",
			"Attribute `android:theme` is only used by `<include>` tags in API level 23 and higher (current min is 19)",
		}
		if len(got) != 2 || got[0].Message != want[0] || got[1].Message != want[1] {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestUnusedAttributeResource_VersionedFolder(t *testing.T) {
	view := &android.View{Type: "View", Line: 5, Attributes: map[string]string{"android:elevation": "4dp"}}
	if findings := unusedAttributeFindings(t, 19, "res/layout-v21/main.xml", view); len(findings) != 0 {
		t.Fatalf("a -v21 folder understands elevation, got %d findings", len(findings))
	}
	ignored := &android.View{Type: "View", Line: 5, Attributes: map[string]string{"android:elevation": "4dp", "tools:ignore": "UnusedAttribute"}}
	if findings := unusedAttributeFindings(t, 19, "res/layout/main.xml", ignored); len(findings) != 0 {
		t.Fatalf("tools:ignore should suppress, got %d findings", len(findings))
	}
}
