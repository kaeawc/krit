package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
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
