package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestRedundantNamespaceResource_FollowsLint(t *testing.T) {
	const appNS = "http://schemas.android.com/apk/res-auto"
	redundant := &android.View{Type: "TextView", Line: 6, Attributes: map[string]string{"xmlns:android": lintTestAndroidNS, "xmlns:app": appNS}}
	toolsOnly := &android.View{Type: "TextView", Line: 11, Attributes: map[string]string{"xmlns:tools": "http://schemas.android.com/tools"}}
	other := &android.View{Type: "TextView", Line: 14, Attributes: map[string]string{"xmlns:app": "http://other"}}
	ignored := &android.View{Type: "TextView", Line: 17, Attributes: map[string]string{"xmlns:app": appNS, "tools:ignore": "RedundantNamespace"}}
	nested := &android.View{Type: "TextView", Line: 20, Attributes: map[string]string{"xmlns:app": appNS}}
	wrapper := &android.View{Type: "FrameLayout", Line: 19, Attributes: map[string]string{}, Children: []*android.View{nested}}
	root := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{"xmlns:android": lintTestAndroidNS, "xmlns:app": appNS},
		Children: []*android.View{redundant, toolsOnly, other, ignored, wrapper}}
	got := runResourceRule(findResourceRule(t, "RedundantNamespaceResource"), indexWithLayout("main", "res/layout/main.xml", root))
	var lines []int
	for _, f := range got {
		if f.Message != "This namespace declaration is redundant" {
			t.Errorf("unexpected message %q", f.Message)
		}
		lines = append(lines, f.Line)
	}
	if len(lines) != 3 {
		t.Fatalf("want findings on the repeated android and app declarations and the nested app one, got lines %v", lines)
	}
}
