package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
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
