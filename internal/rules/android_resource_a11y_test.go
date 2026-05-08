package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

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

	t.Run("single button in linear layout is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
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
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("horizontal button row with wrap_content and no minHeight triggers", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
			Children: []*android.View{
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         6,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("vertical button container is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{
				"android:orientation": "vertical",
			},
			Children: []*android.View{
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         6,
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

	t.Run("styled button row is clean without style resolution", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
			Children: []*android.View{
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
						"style":                 "@style/AppButton",
					},
				},
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         6,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
						"style":                 "@style/AppButton",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("button with minHeight >= 48dp is clean", func(t *testing.T) {
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout",
			Line: 3,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
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
				{
					Type:         "Button",
					LayoutHeight: "wrap_content",
					Line:         6,
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
