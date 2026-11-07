package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func button(line int, attrs map[string]string) *android.View {
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &android.View{Type: "Button", Line: line, Col: 3, Attributes: attrs, Text: attrs["android:text"]}
}

func buttonBar(parentType string, parentAttrs map[string]string, children ...*android.View) *android.View {
	if parentAttrs == nil {
		parentAttrs = map[string]string{}
	}
	return &android.View{Type: parentType, Line: 1, Col: 1, Attributes: parentAttrs, Children: children}
}

func withStrings(idx *android.ResourceIndex, values map[string]string) *android.ResourceIndex {
	idx.StringSources = map[string]android.StringSource{}
	line := 2
	for name, value := range values {
		idx.StringSources[name] = android.StringSource{Value: value, FilePath: "res/values/strings.xml", Line: line}
		line++
	}
	return idx
}

func TestButtonOrderResource(t *testing.T) {
	r := findResourceRule(t, "ButtonOrderResource")
	text := func(s string) map[string]string { return map[string]string{"android:text": s} }

	t.Run("literal OK before Cancel reports once with the reordered labels", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("OK")), button(3, text("Cancel")))
		got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
		if len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
		want := `OK button should be on the right (was "OK | Cancel", should be "Cancel | OK")`
		if got[0].Message != want || got[0].Line != 2 {
			t.Fatalf("unexpected finding: %+v", got[0])
		}
	})

	t.Run("platform string references count as OK and Cancel", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("@android:string/ok")), button(3, text("@android:string/cancel")))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})

	t.Run("app string resources resolve to their label", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("@string/confirm")), button(3, text("@string/dismiss")))
		idx := withStrings(indexWithLayout("main", "res/layout/main.xml", root), map[string]string{"confirm": "Ok", "dismiss": "cancel"})
		got := runResourceRule(r, idx)
		if len(got) != 1 || !strings.Contains(got[0].Message, `was "Ok | cancel", should be "cancel | Ok"`) {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})

	t.Run("Cancel before OK is clean", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("Cancel")), button(3, text("OK")))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("Cancel after another button reports the Cancel position", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("Later")), button(3, text("Cancel")))
		got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
		if len(got) != 1 || !strings.HasPrefix(got[0].Message, "Cancel button should be on the left") {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})

	t.Run("vertical layouts are not judged", func(t *testing.T) {
		root := buttonBar("LinearLayout", map[string]string{"android:orientation": "vertical"}, button(2, text("OK")), button(3, text("Cancel")))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("relative layout alignParentRight on Cancel reports without a suffix", func(t *testing.T) {
		root := buttonBar("RelativeLayout", nil,
			button(2, map[string]string{"android:text": "OK"}),
			button(3, map[string]string{"android:text": "Cancel", "android:layout_alignParentRight": "true"}))
		got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
		if len(got) != 1 || got[0].Message != "Cancel button should be on the left" {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})

	t.Run("tools:ignore on the button silences the finding", func(t *testing.T) {
		ignored := button(2, map[string]string{"android:text": "OK", "tools:ignore": "ButtonOrder"})
		root := buttonBar("LinearLayout", nil, ignored, button(3, text("Cancel")))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}

func TestButtonStyleResource(t *testing.T) {
	r := findResourceRule(t, "ButtonStyleResource")
	text := func(s string) map[string]string { return map[string]string{"android:text": s} }

	t.Run("every button of a plain horizontal bar reports", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("A")), button(3, text("B")))
		got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
		if len(got) != 2 || !strings.HasPrefix(got[0].Message, "Buttons in button bars should be borderless") {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})

	t.Run("the layout name does not matter", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, text("A")), button(3, text("B")))
		if got := runResourceRule(r, indexWithLayout("anything", "res/layout/anything.xml", root)); len(got) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(got))
		}
	})

	t.Run("a style attribute or selectable background is clean", func(t *testing.T) {
		styled := button(2, map[string]string{"style": "?android:attr/buttonBarButtonStyle"})
		selectable := button(3, map[string]string{"android:background": "?android:attr/selectableItemBackground"})
		root := buttonBar("LinearLayout", nil, styled, selectable)
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("a buttonBarStyle parent is a bar even with other children", func(t *testing.T) {
		root := buttonBar("FrameLayout", map[string]string{"style": "?android:attr/buttonBarStyle"}, button(2, nil))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})

	t.Run("a single button, vertical bars and mixed children are clean", func(t *testing.T) {
		cases := []*android.View{
			buttonBar("LinearLayout", nil, button(2, nil)),
			buttonBar("LinearLayout", map[string]string{"android:orientation": "vertical"}, button(2, nil), button(3, nil)),
			buttonBar("LinearLayout", nil, button(2, nil), &android.View{Type: "TextView", Line: 3, Attributes: map[string]string{}}),
			buttonBar("RelativeLayout", nil, button(2, nil), button(3, nil)),
		}
		for i, root := range cases {
			if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
				t.Errorf("case %d: expected 0 findings, got %d", i, len(got))
			}
		}
	})

	t.Run("only plain Button tags are judged", func(t *testing.T) {
		mat := &android.View{Type: "com.google.android.material.button.MaterialButton", Line: 2, Attributes: map[string]string{}}
		root := buttonBar("LinearLayout", nil, mat, button(3, nil))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}

func TestButtonCaseResource(t *testing.T) {
	r := findResourceRule(t, "ButtonCaseResource")

	t.Run("OK and Cancel strings with other capitalization report", func(t *testing.T) {
		idx := withStrings(emptyIndex(), map[string]string{"ok": "Ok", "cancel": "cancel", "fine": "OK", "unrelated": "Cancellation"})
		got := runResourceRule(r, idx)
		if len(got) != 2 {
			t.Fatalf("expected 2 findings, got %d: %+v", len(got), got)
		}
		msgs := got[0].Message + "\n" + got[1].Message
		for _, want := range []string{
			`The standard Android way to capitalize cancel is "Cancel" (tip: use ` + "`@android:string/cancel`" + ` instead)`,
			`The standard Android way to capitalize Ok is "OK" (tip: use ` + "`@android:string/ok`" + ` instead)`,
		} {
			if !strings.Contains(msgs, want) {
				t.Errorf("missing message %q in %q", want, msgs)
			}
		}
	})

	t.Run("quoted labels are compared without the quotes", func(t *testing.T) {
		idx := withStrings(emptyIndex(), map[string]string{"ok": `"OK"`})
		if got := runResourceRule(r, idx); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("literal layout text is not judged", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, map[string]string{"android:text": "ok"}))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("platform yes and no strings report", func(t *testing.T) {
		root := buttonBar("LinearLayout", nil, button(2, map[string]string{"android:text": "@android:string/yes"}))
		got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
		want := `@android:string/yes actually returns "OK", not "Yes"; use @android:string/ok instead or create a local string resource for Yes`
		if len(got) != 1 || got[0].Message != want {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})
}

func TestBackButtonResource(t *testing.T) {
	r := findResourceRule(t, "BackButtonResource")

	t.Run("button using a Back string resource reports", func(t *testing.T) {
		root := buttonBar("FrameLayout", nil, button(2, map[string]string{"android:text": "@string/back"}))
		idx := withStrings(indexWithLayout("main", "res/layout/main.xml", root), map[string]string{"back": "Back"})
		got := runResourceRule(r, idx)
		if len(got) != 1 || got[0].Message != "Back buttons are not standard on Android; see design guide's navigation section" {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})

	t.Run("literal Back text is not judged", func(t *testing.T) {
		root := buttonBar("FrameLayout", nil, button(2, map[string]string{"android:text": "Back"}))
		if got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("a longer label is not a back button", func(t *testing.T) {
		root := buttonBar("FrameLayout", nil, button(2, map[string]string{"android:text": "@string/back"}))
		idx := withStrings(indexWithLayout("main", "res/layout/main.xml", root), map[string]string{"back": "Back to start"})
		if got := runResourceRule(r, idx); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}
