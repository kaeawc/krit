package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// findingMessages extracts finding messages for readable test failures.
func findingMessages(findings []scanner.Finding) []string {
	msgs := make([]string, 0, len(findings))
	for _, f := range findings {
		msgs = append(msgs, f.Message)
	}
	return msgs
}

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
						"android:text":     "Email",
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
	menu := func(path string, values ...string) *android.ResourceIndex {
		root := &android.View{Type: "menu", Line: 1}
		for i, v := range values {
			attrs := map[string]string{}
			if v != "" {
				attrs["app:showAsAction"] = v
			}
			root.Children = append(root.Children, &android.View{Type: "item", Line: 2 + i, Attributes: attrs})
		}
		return indexWithMenu("menu", path, root)
	}

	t.Run("single always item is clean", func(t *testing.T) {
		if got := runResourceRule(r, menu("res/menu/main.xml", "always")); len(got) != 0 {
			t.Fatalf("a lone always item is deliberate, got %v", got)
		}
	})

	t.Run("two always items and no ifRoom reports once on the first", func(t *testing.T) {
		got := runResourceRule(r, menu("res/menu/main.xml", "always", "always|withText"))
		if len(got) != 1 || got[0].Line != 2 {
			t.Fatalf("expected 1 finding at line 2, got %v", got)
		}
		if want := "Prefer \"`ifRoom`\" instead of \"`always`\""; got[0].Message != want {
			t.Fatalf("message %q, want %q", got[0].Message, want)
		}
	})

	t.Run("always plus ifRoom is clean", func(t *testing.T) {
		if got := runResourceRule(r, menu("res/menu/main.xml", "always", "ifRoom")); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %v", got)
		}
		if got := runResourceRule(r, menu("res/menu/main.xml", "always", "always", "ifRoom|withText")); len(got) != 0 {
			t.Fatalf("two always with an ifRoom item is clean, got %v", got)
		}
	})

	t.Run("more than two always items reports even with ifRoom", func(t *testing.T) {
		got := runResourceRule(r, menu("res/menu/main.xml", "ifRoom", "always", "always", "always"))
		if len(got) != 1 || got[0].Line != 3 {
			t.Fatalf("expected 1 finding at line 3, got %v", got)
		}
	})

	t.Run("the first recognised flag decides", func(t *testing.T) {
		if got := runResourceRule(r, menu("res/menu/main.xml", "ifRoom|always", "always")); len(got) != 0 {
			t.Fatalf("ifRoom|always counts as ifRoom, got %v", got)
		}
	})

	t.Run("android namespace counts too", func(t *testing.T) {
		root := &android.View{Type: "menu", Line: 1, Children: []*android.View{
			{Type: "item", Line: 2, Attributes: map[string]string{"android:showAsAction": "always"}},
			{Type: "item", Line: 3, Attributes: map[string]string{"android:showAsAction": "always"}},
		}}
		if got := runResourceRule(r, indexWithMenu("menu", "res/menu/main.xml", root)); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %v", got)
		}
	})

	t.Run("items without showAsAction are clean", func(t *testing.T) {
		if got := runResourceRule(r, menu("res/menu/main.xml", "", "")); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %v", got)
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

	clickable := func(typ string, attrs map[string]string, children ...*android.View) *android.View {
		attrs["android:clickable"] = "true"
		return &android.View{Type: typ, Line: 5, Attributes: attrs, Children: children}
	}
	cleanCases := map[string]*android.View{
		"android:text":        clickable("ImageView", map[string]string{"android:text": "Go"}),
		"hint":                clickable("EditText", map[string]string{"android:hint": "Name"}),
		"importantForA11y":    clickable("ImageView", map[string]string{"android:importantForAccessibility": "no"}),
		"noHideDescendants":   clickable("ImageView", map[string]string{"android:importantForAccessibility": "noHideDescendants"}),
		"text descendant":     clickable("View", map[string]string{}, &android.View{Type: "TextView", Text: "Open"}),
		"nested description":  clickable("View", map[string]string{}, &android.View{Type: "FrameLayout", Children: []*android.View{{Type: "ImageView", ContentDescription: "Close"}}}),
		"own text view":       clickable("TextView", map[string]string{}),
		"button":              clickable("Button", map[string]string{}),
		"compound button":     clickable("CheckBox", map[string]string{}),
		"material button":     clickable("com.google.android.material.button.MaterialButton", map[string]string{}),
		"custom button":       clickable("com.example.ui.WidgetButton", map[string]string{}),
		"custom text view":    clickable("com.example.ui.WidgetTextView", map[string]string{}),
		"chip":                clickable("com.google.android.material.chip.Chip", map[string]string{}),
		"touch-absorbing row": clickable("LinearLayout", map[string]string{}, &android.View{Type: "ImageView", Attributes: map[string]string{}}),
		"custom group":        clickable("com.example.ui.RowLayout", map[string]string{}, &android.View{Type: "ImageView", Attributes: map[string]string{}}),
		"full-screen scrim":   {Type: "View", LayoutWidth: "match_parent", LayoutHeight: "match_parent", Line: 5, Attributes: map[string]string{"android:clickable": "true"}},
		"non-focusable scrim": clickable("View", map[string]string{"android:focusable": "false"}),
	}
	for name, view := range cleanCases {
		t.Run("accessible name from "+name+" is clean", func(t *testing.T) {
			idx := indexWithLayout("main", "res/layout/main.xml", view)
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Fatalf("expected 0 findings, got %d", len(findings))
			}
		})
	}

	t.Run("view labelled through labelFor is clean", func(t *testing.T) {
		root := &android.View{Type: "LinearLayout", Attributes: map[string]string{}, Children: []*android.View{
			{Type: "TextView", Text: "Volume", Attributes: map[string]string{"android:labelFor": "@+id/slider"}},
			{Type: "ImageView", ID: "@+id/slider", Line: 7, Attributes: map[string]string{"android:clickable": "true"}},
		}}
		if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("icon-only clickable views without name trigger", func(t *testing.T) {
		for _, typ := range []string{"ImageButton", "View", "ImageView", "com.google.android.material.floatingactionbutton.FloatingActionButton", "com.example.ui.ChevronImageView"} {
			view := clickable(typ, map[string]string{})
			// A plain View needs a visible background to be a control rather than a touch blocker.
			view.Background = "@drawable/icon"
			idx := indexWithLayout("main", "res/layout/main.xml", view)
			if findings := runResourceRule(r, idx); len(findings) != 1 {
				t.Fatalf("%s: expected 1 finding, got %d", typ, len(findings))
			}
		}
	})

	t.Run("plain View touch blockers without content are clean", func(t *testing.T) {
		cases := map[string]*android.View{
			"zero width":    {Type: "View", LayoutWidth: "0dp", LayoutHeight: "48dp", Background: "@drawable/mask"},
			"zero height":   {Type: "View", LayoutWidth: "48dp", LayoutHeight: "0dp", Background: "@drawable/mask"},
			"no background": {Type: "View", LayoutWidth: "48dp", LayoutHeight: "48dp"},
		}
		for name, view := range cases {
			view.Line = 5
			view.Attributes = map[string]string{"android:clickable": "true"}
			if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view)); len(findings) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", name, len(findings))
			}
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

	t.Run("EditText without inputType or hint is clean", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type:       "EditText",
			Line:       5,
			Attributes: map[string]string{},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("message describes autofill participation", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type:       "EditText",
			Line:       5,
			Attributes: map[string]string{"android:hint": "Email"},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if strings.Contains(findings[0].Message, "accessibility") || !strings.Contains(findings[0].Message, "autofill service") {
			t.Fatalf("unexpected message: %s", findings[0].Message)
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
			ID:   "@+id/login_email",
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

	// Regression: a generic numeric field (e.g. a country-code input) must NOT
	// be told to use creditCardNumber. The old broad "number ⇒ creditCardNumber"
	// inference caused this false positive.
	t.Run("numeric country-code field is clean (no creditCardNumber suggestion)", func(t *testing.T) {
		idx := indexWithLayout("country", "res/layout/country_code_text.xml", &android.View{
			Type: "EditText",
			Line: 23,
			Attributes: map[string]string{
				"android:inputType": "number",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for numeric field, got %d: %v", len(findings), findingMessages(findings))
		}
	})

	// Regression: a numeric passphrase (numberPassword) must not be told to use
	// creditCardNumber. numberPassword maps to a password hint, and since no
	// autofillHints is present we still suggest password — but never a card hint.
	t.Run("numberPassword field never suggests creditCardNumber", func(t *testing.T) {
		idx := indexWithLayout("passphrase", "res/layout/enter_backup_passphrase_dialog.xml", &android.View{
			Type: "EditText",
			Line: 10,
			Attributes: map[string]string{
				"android:inputType": "numberPassword",
			},
		})
		findings := runResourceRule(r, idx)
		for _, f := range findings {
			if strings.Contains(f.Message, "creditCardNumber") {
				t.Fatalf("must not suggest creditCardNumber for numberPassword: %q", f.Message)
			}
		}
	})

	// Regression: generic textPersonName (often a search box) must not be told
	// to use a personName hint.
	t.Run("textPersonName field is clean (no personName suggestion)", func(t *testing.T) {
		idx := indexWithLayout("search", "res/layout/search.xml", &android.View{
			Type: "EditText",
			Line: 7,
			Attributes: map[string]string{
				"android:inputType": "textPersonName",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for textPersonName, got %d: %v", len(findings), findingMessages(findings))
		}
	})

	// Regression: a field explicitly opted out of autofill must be exempt.
	t.Run("importantForAutofill=no is exempt", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType":            "textEmailAddress",
				"android:importantForAutofill": "no",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for importantForAutofill=no, got %d: %v", len(findings), findingMessages(findings))
		}
	})

	// Positive: a genuine mismatch — autofillHints contradicts a specific
	// inputType — still flags.
	t.Run("password hint on phone inputType flags (genuine mismatch)", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType":     "phone",
				"android:autofillHints": "password",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for contradicting hint, got %d: %v", len(findings), findingMessages(findings))
		}
	})

	// Negative: a consistent autofillHints value on a pipe-combined inputType is
	// clean (email + flags).
	t.Run("consistent hint on combined inputType is clean", func(t *testing.T) {
		idx := indexWithLayout("form", "res/layout/form.xml", &android.View{
			Type: "EditText",
			Line: 5,
			Attributes: map[string]string{
				"android:inputType":     "textEmailAddress|textNoSuggestions",
				"android:autofillHints": "emailAddress",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for consistent combined inputType, got %d: %v", len(findings), findingMessages(findings))
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

	t.Run("LinearLayout without orientation is horizontal and triggers", func(t *testing.T) {
		btn := func(line int) *android.View {
			return &android.View{Type: "ImageButton", LayoutHeight: "wrap_content", Line: line,
				Attributes: map[string]string{"android:layout_height": "wrap_content"}}
		}
		idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
			Type: "LinearLayout", Line: 3, Attributes: map[string]string{},
			Children: []*android.View{btn(5), btn(6)},
		})
		if got := len(runResourceRule(r, idx)); got != 2 {
			t.Fatalf("expected 2 findings, got %d", got)
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
					Type:         "ImageButton",
					LayoutHeight: "wrap_content",
					Line:         5,
					Attributes: map[string]string{
						"android:layout_height": "wrap_content",
					},
				},
				{
					Type:         "ImageButton",
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

	t.Run("themed buttons and unresolvable minHeight are clean, small literal triggers", func(t *testing.T) {
		mk := func(typ, minH string, line int) *android.View {
			attrs := map[string]string{"android:layout_height": "wrap_content"}
			if minH != "" {
				attrs["android:minHeight"] = minH
			}
			return &android.View{Type: typ, LayoutHeight: "wrap_content", Line: line, Attributes: attrs}
		}
		row := func(children ...*android.View) int {
			idx := indexWithLayout("dialog", "res/layout/dialog.xml", &android.View{
				Type:       "androidx.appcompat.widget.LinearLayoutCompat",
				Line:       3,
				Attributes: map[string]string{"android:orientation": "horizontal"},
				Children:   children,
			})
			return len(runResourceRule(r, idx))
		}
		// The theme gives Button and MaterialButton a 48dp minHeight.
		if n := row(mk("Button", "", 5), mk("com.google.android.material.button.MaterialButton", "", 6)); n != 0 {
			t.Errorf("themed buttons without minHeight should be clean, got %d", n)
		}
		if n := row(mk("ImageButton", "@dimen/touch", 5), mk("ImageButton", "?attr/minTouch", 6)); n != 0 {
			t.Errorf("reference minHeight should be clean, got %d", n)
		}
		if n := row(mk("Button", "32dp", 5), mk("com.google.android.material.button.MaterialButton", "0dp", 6)); n != 2 {
			t.Errorf("explicit small minHeight should report, got %d", n)
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

func TestLayoutImportantForAccessibilityNoSkipsTouchInterceptors(t *testing.T) {
	r := findResourceRule(t, "LayoutImportantForAccessibilityNo")
	run := func(attrs map[string]string) int {
		attrs["android:importantForAccessibility"] = "no"
		attrs["android:clickable"] = "true"
		view := &android.View{Type: "View", Line: 3, Attributes: attrs}
		return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view)))
	}
	if n := run(map[string]string{"android:focusable": "false"}); n != 0 {
		t.Errorf("non-focusable interceptor should be clean, got %d", n)
	}
	if n := run(map[string]string{"android:focusable": "false", "android:text": "Go"}); n != 1 {
		t.Errorf("non-focusable view with text should report, got %d", n)
	}
	if n := run(map[string]string{}); n != 1 {
		t.Errorf("clickable view hidden from accessibility should report, got %d", n)
	}
}

func TestAlwaysShowActionResource_JudgesEachMenuAlone(t *testing.T) {
	r := findResourceRule(t, "AlwaysShowActionResource")
	always := func(file string) *android.Layout {
		return &android.Layout{Name: "m", FilePath: file, RootView: &android.View{Type: "menu", Line: 1, Children: []*android.View{
			{Type: "item", Line: 2, Attributes: map[string]string{"app:showAsAction": "always"}},
			{Type: "item", Line: 3, Attributes: map[string]string{"app:showAsAction": "always"}},
		}}}
	}
	ifRoom := &android.Layout{Name: "n", FilePath: "res/menu/other.xml", RootView: &android.View{Type: "menu", Line: 1, Children: []*android.View{
		{Type: "item", Line: 2, Attributes: map[string]string{"app:showAsAction": "ifRoom"}},
	}}}
	idx := indexWithMenu("m", "res/menu/main.xml", always("res/menu/main.xml").RootView)
	idx.MenuVariants = append(idx.MenuVariants, ifRoom)
	if got := runResourceRule(r, idx); len(got) != 1 || got[0].File != "res/menu/main.xml" {
		t.Fatalf("an ifRoom item in another menu must not suppress this one, got %v", got)
	}
}

func TestLayoutEditTextMissingImportanceQualifiedTypes(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")
	for _, typ := range []string{
		"com.google.android.material.textfield.TextInputEditText",
		"androidx.appcompat.widget.AppCompatEditText",
	} {
		view := &android.View{Type: typ, Line: 3, Attributes: map[string]string{"android:inputType": "text"}}
		if n := len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view))); n != 1 {
			t.Errorf("%s: expected 1 finding, got %d", typ, n)
		}
	}
}

func TestMissingContentDescriptionResource_FollowsLint(t *testing.T) {
	r := findResourceRule(t, "MissingContentDescriptionResource")
	img := func(typ string, attrs map[string]string) *android.View {
		return &android.View{Type: typ, Line: 4, Col: 3, Attributes: attrs}
	}
	run := func(children ...*android.View) []string {
		root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: children}
		var msgs []string
		for _, f := range runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)) {
			msgs = append(msgs, f.Message)
		}
		return msgs
	}
	const missing = "Missing `contentDescription` attribute on image"
	if got := run(img("ImageView", map[string]string{"android:importantForAccessibility": "no"})); len(got) != 0 {
		t.Errorf("importantForAccessibility=no should be clean, got %v", got)
	}
	for _, typ := range []string{"ImageView", "ImageButton", "com.google.android.material.floatingactionbutton.FloatingActionButton", "android.support.design.widget.FloatingActionButton"} {
		if got := run(img(typ, map[string]string{})); len(got) != 1 || got[0] != missing {
			t.Errorf("%s: got %v", typ, got)
		}
	}
	for _, typ := range []string{"androidx.appcompat.widget.AppCompatImageView", "com.example.ImageView", "com.google.android.material.imageview.ShapeableImageView"} {
		if got := run(img(typ, map[string]string{})); len(got) != 0 {
			t.Errorf("%s is not checked by Lint, got %v", typ, got)
		}
	}
	hidden := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{"android:importantForAccessibility": "noHideDescendants"},
		Children: []*android.View{img("ImageView", map[string]string{})}}
	if got := run(hidden); len(got) != 1 {
		t.Errorf("Lint ignores ancestor importantForAccessibility, got %v", got)
	}
	if got := run(img("ImageView", map[string]string{"android:importantForAccessibility": "noHideDescendants"})); len(got) != 1 {
		t.Errorf("only importantForAccessibility=no exempts the view, got %v", got)
	}
	for _, desc := range []string{"", "TODO"} {
		got := run(img("ImageView", map[string]string{"android:contentDescription": desc}))
		if len(got) != 1 || got[0] != "Empty `contentDescription` attribute on image" {
			t.Errorf("description %q: got %v", desc, got)
		}
	}
	for _, desc := range []string{"@null", "@string/x", "@{vm.d}", "todo", " "} {
		if got := run(img("ImageView", map[string]string{"android:contentDescription": desc})); len(got) != 0 {
			t.Errorf("description %q should be clean, got %v", desc, got)
		}
	}
	both := img("EditText", map[string]string{"android:contentDescription": "x", "android:hint": "y"})
	if got := run(both); len(got) != 1 || !strings.HasPrefix(got[0], "Do not set both `contentDescription` and `hint`") {
		t.Errorf("contentDescription with hint: got %v", got)
	}
}

func labelForRun(t *testing.T, kids ...*android.View) []string {
	t.Helper()
	return labelForRunAt(t, "res/layout/main.xml", kids...)
}

func labelForRunAt(t *testing.T, path string, kids ...*android.View) []string {
	t.Helper()
	r := findResourceRule(t, "LabelForResource")
	root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: kids}
	var msgs []string
	for _, f := range runResourceRule(r, indexWithLayout("main", path, root)) {
		msgs = append(msgs, fmt.Sprintf("%d:%s", f.Line, strings.TrimPrefix(f.Message, "Missing accessibility label: ")))
	}
	return msgs
}

func labelForEdit(tag, id string, line int, attrs map[string]string) *android.View {
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &android.View{Type: tag, ID: id, Line: line, Attributes: attrs}
}

func TestLabelForResource_LabelAnywhere(t *testing.T) {
	edit := labelForEdit("EditText", "@+id/name", 5, nil)
	label := &android.View{Type: "TextView", Line: 9, Attributes: map[string]string{"android:labelFor": "@id/name", "android:text": "Name"}}
	nested := func(v *android.View) *android.View {
		return &android.View{Type: "LinearLayout", Line: 2, Attributes: map[string]string{}, Children: []*android.View{v}}
	}
	if got := labelForRun(t, nested(edit), nested(label)); len(got) != 0 {
		t.Errorf("label in a different container should satisfy labelFor, got %v", got)
	}
}

// Lint's LabelForDetector has no TextInputLayout, contentDescription or importantForAccessibility exemption.
func TestLabelForResource_OnlyHintAndLabelForCount(t *testing.T) {
	either := "provide either a view with an `android:labelFor` that references this view or provide an `android:hint`"
	til := &android.View{Type: "com.google.android.material.textfield.TextInputLayout", Line: 2,
		Attributes: map[string]string{"android:hint": "Name"},
		Children:   []*android.View{labelForEdit("EditText", "@+id/e", 3, nil)}}
	cases := []struct {
		name string
		kids []*android.View
		want []string
	}{
		{"inside TextInputLayout", []*android.View{til}, []string{"3:" + either}},
		{"contentDescription only", []*android.View{labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:contentDescription": "x"})}, []string{"3:" + either}},
		{"hidden from accessibility", []*android.View{labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:importantForAccessibility": "no"})}, []string{"3:" + either}},
		{"no id", []*android.View{labelForEdit("EditText", "", 3, nil)}, []string{"3:" + either}},
		{"AutoCompleteTextView", []*android.View{labelForEdit("AutoCompleteTextView", "@+id/e", 3, nil)}, []string{"3:" + either}},
		{"AppCompatEditText is not visited", []*android.View{labelForEdit("androidx.appcompat.widget.AppCompatEditText", "@+id/e", 3, nil)}, nil},
		{"hint only", []*android.View{labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:hint": "h"})}, nil},
		{"empty hint", []*android.View{labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:hint": ""})}, []string{"3:Empty `android:hint` attribute"}},
		{"hint and label", []*android.View{
			labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:hint": "h"}),
			labelForEdit("TextView", "", 4, map[string]string{"android:labelFor": "@id/e", "android:text": "t"}),
		}, []string{"3:" + either + ", but not both"}},
		{"label without text", []*android.View{
			labelForEdit("EditText", "@+id/e", 3, nil),
			labelForEdit("TextView", "", 4, map[string]string{"android:labelFor": "@id/e"}),
		}, []string{"4:when using `android:labelFor`, you must also define an `android:text` or an `android:contentDescription`"}},
		{"labelFor on a Button needs no text", []*android.View{
			labelForEdit("EditText", "@+id/e", 3, nil),
			labelForEdit("Button", "", 4, map[string]string{"android:labelFor": "@id/e"}),
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := labelForRun(t, tc.kids...)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Lint's end-of-file loop returns at the first consistent field, so later fields are not checked.
func TestLabelForResource_StopsAtFirstConsistentField(t *testing.T) {
	got := labelForRun(t,
		labelForEdit("EditText", "@+id/a", 3, nil),
		labelForEdit("EditText", "@+id/b", 4, map[string]string{"android:hint": "h"}),
		labelForEdit("EditText", "@+id/c", 5, nil),
		labelForEdit("TextView", "", 6, map[string]string{"android:labelFor": "@id/z", "android:text": "t"}),
	)
	if len(got) != 1 || !strings.HasPrefix(got[0], "3:") {
		t.Errorf("only the field before the first consistent one is reported, got %v", got)
	}
}

func TestLayoutClickableWithoutMinSizeResolvesDimenReferences(t *testing.T) {
	r := findResourceRule(t, "LayoutClickableWithoutMinSize")
	for dimen, want := range map[string]int{"40dp": 1, "56dp": 0} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:         "ImageView",
			LayoutWidth:  "@dimen/icon_size",
			LayoutHeight: "@dimen/icon_size",
			Line:         5,
			Attributes: map[string]string{
				"android:clickable":     "true",
				"android:layout_width":  "@dimen/icon_size",
				"android:layout_height": "@dimen/icon_size",
			},
		})
		idx.Dimensions["icon_size"] = dimen
		if got := len(runResourceRule(r, idx)); got != want {
			t.Errorf("icon_size=%s: expected %d findings, got %d", dimen, want, got)
		}
	}
}

func TestLayoutEditTextMissingImportanceSkipsDeclaredHints(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")
	for name, tc := range map[string]struct {
		attrs map[string]string
		want  int
	}{
		"hint only":                 {map[string]string{"android:hint": "Name"}, 1},
		"with autofillHints":        {map[string]string{"android:hint": "Name", "android:autofillHints": "name"}, 0},
		"with importantForAutofill": {map[string]string{"android:hint": "Name", "android:importantForAutofill": "no"}, 0},
	} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "EditText", Line: 3, Attributes: tc.attrs})
		if got := len(runResourceRule(r, idx)); got != tc.want {
			t.Errorf("%s: expected %d findings, got %d", name, tc.want, got)
		}
	}
}

func TestLayoutRulesReadProjectStyles(t *testing.T) {
	withStyles := func(idx *android.ResourceIndex, styles ...*android.Style) *android.ResourceIndex {
		for _, st := range styles {
			idx.Styles[st.Name] = st
		}
		return idx
	}
	t.Run("autofill hints from a style suppress the missing-hint finding", func(t *testing.T) {
		r := findResourceRule(t, "LayoutAutofillHintMismatch")
		for style, want := range map[string]int{"Base.Password": 0, "Base.Plain": 1} {
			idx := withStyles(indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: "EditText", ID: "@+id/password", Line: 3,
				Attributes: map[string]string{"android:inputType": "textPassword", "style": "@style/" + style},
			}),
				&android.Style{Name: "Base", Items: map[string]string{"android:autofillHints": "password"}},
				&android.Style{Name: "Base.Password", Items: map[string]string{}},
				&android.Style{Name: "Base.Plain", Parent: "Other", Items: map[string]string{}},
				&android.Style{Name: "Other", Items: map[string]string{}},
			)
			if got := len(runResourceRule(r, idx)); got != want {
				t.Errorf("style %s: expected %d findings, got %d", style, want, got)
			}
		}
	})
	t.Run("button minHeight from a style is checked", func(t *testing.T) {
		r := findResourceRule(t, "LayoutMinTouchTargetInButtonRow")
		btn := func(style string, line int) *android.View {
			return &android.View{Type: "ImageButton", LayoutHeight: "wrap_content", Line: line,
				Attributes: map[string]string{"android:layout_height": "wrap_content", "style": "@style/" + style}}
		}
		for style, want := range map[string]int{"Small": 2, "Big": 0, "Unknown": 0} {
			idx := withStyles(indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: "LinearLayout", Line: 1, Attributes: map[string]string{"android:orientation": "horizontal"},
				Children: []*android.View{btn(style, 2), btn(style, 3)},
			}),
				&android.Style{Name: "Small", Items: map[string]string{"android:minHeight": "32dp"}},
				&android.Style{Name: "Big", Items: map[string]string{"android:minHeight": "48dp"}},
			)
			if got := len(runResourceRule(r, idx)); got != want {
				t.Errorf("style %s: expected %d findings, got %d", style, want, got)
			}
		}
	})
}

func TestLayoutEditTextMissingImportanceSkipsNonEditableFields(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")
	for name, attrs := range map[string]map[string]string{
		"disabled":        {"android:inputType": "text", "android:enabled": "false"},
		"not focusable":   {"android:inputType": "text", "android:focusable": "false"},
		"inputType none":  {"android:inputType": "none", "android:hint": "Name"},
		"not editable":    {"android:hint": "Name", "android:editable": "false"},
		"none with flags": {"android:inputType": "none|textNoSuggestions"},
	} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "EditText", Line: 3, Attributes: attrs})
		if got := runResourceRule(r, idx); len(got) != 0 {
			t.Errorf("%s: a read-only field is not filled, got %v", name, got)
		}
	}
}

func TestLayoutEditTextMissingImportanceHonoursExcludingAncestors(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")
	edit := func() *android.View {
		return &android.View{Type: "EditText", Line: 4, Attributes: map[string]string{"android:inputType": "text"}}
	}
	for value, want := range map[string]int{"noExcludeDescendants": 0, "yesExcludeDescendants": 0, "no": 1, "yes": 1} {
		root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{"android:importantForAutofill": value},
			Children: []*android.View{{Type: "FrameLayout", Line: 2, Attributes: map[string]string{}, Children: []*android.View{edit()}}}}
		if got := len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))); got != want {
			t.Errorf("ancestor %s: expected %d findings, got %d", value, want, got)
		}
	}
}

func TestLayoutEditTextMissingImportancePasswordAdvisesAutofillHints(t *testing.T) {
	r := findResourceRule(t, "LayoutEditTextMissingImportance")
	for _, inputType := range []string{"textPassword", "textWebPassword", "textVisiblePassword", "numberPassword"} {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "EditText", Line: 3, Attributes: map[string]string{"android:inputType": inputType}})
		got := runResourceRule(r, idx)
		if len(got) != 1 {
			t.Fatalf("%s: expected 1 finding, got %v", inputType, got)
		}
		if !strings.Contains(got[0].Message, "autofillHints") || strings.Contains(got[0].Message, "Set `yes` or `no`") {
			t.Errorf("%s: password fields need autofillHints, not an importance choice: %s", inputType, got[0].Message)
		}
	}
}

func TestAlwaysShowActionResource_CountsEveryFlagOfCombinedValue(t *testing.T) {
	r := findResourceRule(t, "AlwaysShowActionResource")
	menu := func(values ...string) *android.ResourceIndex {
		root := &android.View{Type: "menu", Line: 1}
		for i, v := range values {
			root.Children = append(root.Children, &android.View{Type: "item", Line: i + 2, Attributes: map[string]string{"app:showAsAction": v}})
		}
		return indexWithMenu("m", "res/menu/main.xml", root)
	}
	// "ifRoom|always" counts as an ifRoom action as well as an always action.
	if got := runResourceRule(r, menu("always", "ifRoom|always")); len(got) != 0 {
		t.Fatalf("two always actions with an ifRoom flag must stay clean, got %d", len(got))
	}
	if got := runResourceRule(r, menu("always", "always|withText")); len(got) != 1 {
		t.Fatalf("always actions and no ifRoom action must report, got %d", len(got))
	}
}

func TestHardcodedValuesResource_FollowsLintAttributesAndSamples(t *testing.T) {
	r := findResourceRule(t, "HardcodedValuesResource")
	view := func(typ string, attrs map[string]string) *android.View {
		return &android.View{Type: typ, Line: 2, Col: 1, Attributes: attrs}
	}
	count := func(idx *android.ResourceIndex) int { return len(runResourceRule(r, idx)) }
	layout := func(v *android.View) *android.ResourceIndex {
		return indexWithLayout("main", "res/layout/main.xml", v)
	}

	got := runResourceRule(r, layout(view("TextView", map[string]string{
		"android:text": "Hello", "android:contentDescription": "Close", "android:title": "T", "android:prompt": "P", "android:summary": "S",
	})))
	if len(got) != 4 {
		t.Fatalf("text, contentDescription, title and prompt must report but not summary: %+v", got)
	}
	messages := map[string]bool{}
	for _, f := range got {
		messages[f.Message] = true
	}
	if !messages["Hardcoded string \"Hello\", should use `@string` resource"] {
		t.Errorf("missing Lint wording in %v", messages)
	}

	for _, value := range []string{"", "@string/x", "@null", "?attr/x", "@{vm.text}", "Hello World!", "Large Text", "Small Text", "Medium Text", "New Text", "New TextView"} {
		if n := count(layout(view("TextView", map[string]string{"android:text": value}))); n != 0 {
			t.Errorf("text=%q: expected 0 findings, got %d", value, n)
		}
	}
	for _, value := range []string{"TODO", "{x}", `\@string/abc`, "New", "New Button"} {
		if n := count(layout(view("TextView", map[string]string{"android:text": value}))); n != 1 {
			t.Errorf("text=%q: expected 1 finding, got %d", value, n)
		}
	}
	if n := count(layout(view("TextView", map[string]string{"tools:text": "Sample", "app:title": "Title"}))); n != 0 {
		t.Errorf("non-android attributes must be ignored, got %d", n)
	}
	menu := indexWithMenu("m", "res/menu/m.xml", view("item", map[string]string{"android:title": "Literal"}))
	if n := count(menu); n != 1 {
		t.Errorf("menu title: expected 1 finding, got %d", n)
	}
}

func TestLabelForResource_MinSdkBelow17(t *testing.T) {
	dir := t.TempDir()
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android"><uses-sdk android:minSdkVersion="14"/></manifest>`
	if err := os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "res", "layout", "main.xml")
	hint := "where minSdk < 17, you should provide an `android:hint`"
	cases := []struct {
		name string
		kids []*android.View
		want []string
	}{
		{"no hint, no label", []*android.View{labelForEdit("EditText", "@+id/e", 3, nil)}, []string{"3:" + hint}},
		{"label without hint", []*android.View{
			labelForEdit("EditText", "@+id/e", 3, nil),
			labelForEdit("TextView", "", 4, map[string]string{"android:labelFor": "@id/e", "android:text": "t"}),
		}, []string{"3:" + hint}},
		{"hint and label are not reported", []*android.View{
			labelForEdit("EditText", "@+id/e", 3, map[string]string{"android:hint": "h"}),
			labelForEdit("TextView", "", 4, map[string]string{"android:labelFor": "@id/e", "android:text": "t"}),
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := labelForRunAt(t, path, tc.kids...)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAlwaysShowActionResource_SuppressedItemSilencesFile(t *testing.T) {
	r := findResourceRule(t, "AlwaysShowActionResource")
	item := func(line int, attrs map[string]string) *android.View {
		return &android.View{Type: "item", Line: line, Attributes: attrs}
	}
	always := map[string]string{"app:showAsAction": "always"}
	menu := func(children ...*android.View) *android.View {
		return &android.View{Type: "menu", Line: 1, Attributes: map[string]string{}, Children: children}
	}
	plain := menu(item(2, always), item(3, always), item(4, always))
	if got := runResourceRule(r, indexWithMenu("m", "res/menu/m.xml", plain)); len(got) != 1 || got[0].Line != 2 {
		t.Fatalf("baseline: want one finding on line 2, got %v", got)
	}
	// Lint sets an ignore-file flag on the first suppressed showAsAction attribute, so the other items count for nothing.
	suppressed := map[string]string{"app:showAsAction": "always", "tools:ignore": "AlwaysShowAction"}
	if got := runResourceRule(r, indexWithMenu("m", "res/menu/m.xml", menu(item(2, always), item(3, always), item(4, always), item(5, suppressed)))); len(got) != 0 {
		t.Fatalf("a suppressed item silences the whole menu, got %v", got)
	}
	ignoredRoot := menu(item(2, always), item(3, always), item(4, always))
	ignoredRoot.Attributes["tools:ignore"] = "AlwaysShowAction"
	if got := runResourceRule(r, indexWithMenu("m", "res/menu/m.xml", ignoredRoot)); len(got) != 0 {
		t.Fatalf("a suppressed root silences the menu, got %v", got)
	}
}
