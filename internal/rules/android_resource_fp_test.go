package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func fpView(typ string, attrs map[string]string, children ...*android.View) *android.View {
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &android.View{
		Type:               typ,
		ID:                 attrs["android:id"],
		Text:               attrs["android:text"],
		ContentDescription: attrs["android:contentDescription"],
		Background:         attrs["android:background"],
		LayoutWidth:        attrs["android:layout_width"],
		LayoutHeight:       attrs["android:layout_height"],
		Attributes:         attrs,
		Children:           children,
		Line:               2,
	}
}

func fpWithIgnore(attrs map[string]string, ignore string) map[string]string {
	out := map[string]string{}
	for k, v := range attrs {
		out[k] = v
	}
	if ignore != "" {
		out["tools:ignore"] = ignore
	}
	return out
}

func TestLayoutRulesHonourToolsIgnore(t *testing.T) {
	cases := []struct {
		rule, issue, childType string
		attrs                  map[string]string
	}{
		{"SmallSpResource", "SmallSp", "TextView", map[string]string{"android:textSize": "8sp"}},
		{"SpUsageResource", "SpUsage", "TextView", map[string]string{"android:textSize": "14dp"}},
		{"LabelForResource", "LabelFor", "EditText", map[string]string{"android:id": "@+id/name"}},
		{"LayoutEditTextMissingImportance", "Autofill", "EditText", map[string]string{"android:inputType": "text"}},
		{"LayoutAutofillHintMismatch", "Autofill", "EditText", map[string]string{"android:id": "@+id/login_email", "android:inputType": "textEmailAddress"}},
		{"UselessLeafResource", "UselessLeaf", "FrameLayout", nil},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			r := findResourceRule(t, tc.rule)
			build := func(rootIgnore, childIgnore string) int {
				child := fpView(tc.childType, fpWithIgnore(tc.attrs, childIgnore))
				root := fpView("LinearLayout", fpWithIgnore(nil, rootIgnore), child)
				return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)))
			}
			if build("", "") == 0 {
				t.Fatalf("expected a finding without tools:ignore")
			}
			if n := build("", tc.issue); n != 0 {
				t.Errorf("tools:ignore on the element should suppress, got %d", n)
			}
			if n := build(tc.issue, ""); n != 0 {
				t.Errorf("tools:ignore on an ancestor should suppress, got %d", n)
			}
			if n := build("", "SomethingElse"); n == 0 {
				t.Errorf("unrelated tools:ignore must not suppress")
			}
		})
	}
}

func TestOverdrawResourceSkipsPartialOrDecoratedChildren(t *testing.T) {
	r := findResourceRule(t, "OverdrawResource")
	full := func() map[string]string {
		return map[string]string{"android:background": "@color/surface", "android:layout_width": "match_parent", "android:layout_height": "match_parent"}
	}
	run := func(child *android.View) int {
		root := fpView("LinearLayout", map[string]string{"android:background": "@color/surface"}, child)
		return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)))
	}
	if run(fpView("FrameLayout", full())) != 1 {
		t.Fatalf("same-color full-bleed child should be flagged")
	}
	margined := full()
	margined["android:layout_margin"] = "8dp"
	elevated := full()
	elevated["android:elevation"] = "4dp"
	shaped := full()
	shaped["android:background"] = "@drawable/rounded"
	wrap := full()
	wrap["android:layout_height"] = "wrap_content"
	translucent := full()
	translucent["android:background"] = "#80ffffff"
	differentColor := full()
	differentColor["android:background"] = "@color/other"
	for name, attrs := range map[string]map[string]string{
		"margined": margined, "elevated": elevated, "shaped": shaped, "wrap": wrap, "translucent": translucent, "differentColor": differentColor,
	} {
		if n := run(fpView("FrameLayout", attrs)); n != 0 {
			t.Errorf("%s child should not be flagged, got %d", name, n)
		}
	}
}

func TestUselessLeafResourceSkipsReferencedLayouts(t *testing.T) {
	r := findResourceRule(t, "UselessLeafResource")
	stubLayout := &android.Layout{Name: "stub_content", FilePath: "res/layout/stub_content.xml", RootView: fpView("FrameLayout", nil)}
	host := &android.Layout{Name: "host", FilePath: "res/layout/host.xml", RootView: fpView("LinearLayout", nil,
		fpView("ViewStub", map[string]string{"android:layout": "@layout/stub_content"}))}
	idx := indexWithLayout("host", host.FilePath, host.RootView)
	idx.Layouts["stub_content"] = stubLayout
	if n := len(runResourceRule(r, idx)); n != 0 {
		t.Fatalf("ViewStub target should not be reported, got %d", n)
	}
	delete(idx.Layouts, "host")
	if n := len(runResourceRule(r, idx)); n != 0 {
		t.Fatalf("Lint never reports a root view, got %d", n)
	}
}

func TestLayoutAutofillHintMismatchOnlyForCredentialFields(t *testing.T) {
	r := findResourceRule(t, "LayoutAutofillHintMismatch")
	run := func(attrs map[string]string) int {
		return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", fpView("EditText", attrs))))
	}
	if run(map[string]string{"android:id": "@+id/sign_in_email", "android:inputType": "textEmailAddress"}) != 1 {
		t.Errorf("sign-in email field without hint should be flagged")
	}
	if run(map[string]string{"android:id": "@+id/invite_email", "android:inputType": "textEmailAddress"}) != 0 {
		t.Errorf("non-credential email field should not be flagged")
	}
	if run(map[string]string{"android:id": "@+id/login_pin", "android:inputType": "numberPassword"}) != 0 {
		t.Errorf("numberPassword PIN should not be flagged")
	}
}

func TestStringRepeatedInContentDescriptionRespectsAccessibilityVisibility(t *testing.T) {
	r := findResourceRule(t, "StringRepeatedInContentDescription")
	run := func(parentAttrs, childAttrs map[string]string) int {
		parentAttrs["android:contentDescription"] = "Save"
		childAttrs["android:text"] = "Save"
		root := fpView("LinearLayout", map[string]string{}, fpView("FrameLayout", parentAttrs, fpView("TextView", childAttrs)))
		return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)))
	}
	if run(map[string]string{}, map[string]string{}) != 1 {
		t.Errorf("reachable duplicate should be flagged")
	}
	if run(map[string]string{}, map[string]string{"android:importantForAccessibility": "no"}) != 0 {
		t.Errorf("child hidden from accessibility should not be flagged")
	}
	if run(map[string]string{"android:importantForAccessibility": "noHideDescendants"}, map[string]string{}) != 0 {
		t.Errorf("child under noHideDescendants parent should not be flagged")
	}
}

func TestDuplicateIncludedIDsResourceIncludeIDDoesNotOverrideRootID(t *testing.T) {
	r := findResourceRule(t, "DuplicateIncludedIdsResource")
	build := func(includeID string) int {
		incAttrs := map[string]string{"layout": "@layout/row"}
		if includeID != "" {
			incAttrs["android:id"] = includeID
		}
		host := fpView("LinearLayout", map[string]string{"android:id": "@+id/row_root"}, fpView("include", incAttrs))
		row := fpView("FrameLayout", map[string]string{"android:id": "@+id/row_root"})
		idx := indexWithLayout("host", "res/layout/host.xml", host)
		idx.Layouts["row"] = &android.Layout{Name: "row", FilePath: "res/layout/row.xml", RootView: row}
		return len(runResourceRule(r, idx))
	}
	if build("") != 1 {
		t.Errorf("root id collision without include id should be flagged")
	}
	// Lint 32.4.1 ignores the include's own android:id, so the collision is still reported.
	if build("@+id/first_row") != 1 {
		t.Errorf("include id does not override the included root id, so the collision stays")
	}
}

func TestStringRepeatedInContentDescriptionIgnoresNullReference(t *testing.T) {
	r := findResourceRule(t, "StringRepeatedInContentDescription")
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
		Children: []*android.View{
			{Type: "ImageView", Line: 2, ContentDescription: "@null", Attributes: map[string]string{}},
			{Type: "TextView", Line: 3, Text: "@null", Attributes: map[string]string{}},
		},
	})
	if findings := runResourceRule(r, idx); len(findings) != 0 {
		t.Fatalf("@null is not text, got %d findings", len(findings))
	}
}
