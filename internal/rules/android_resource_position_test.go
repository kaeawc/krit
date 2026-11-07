package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestSuspicious0dpAnchorsAtAttribute(t *testing.T) {
	r := findResourceRule(t, "Suspicious0dpResource")
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type:       "LinearLayout",
		Line:       1,
		Col:        1,
		Attributes: map[string]string{"android:orientation": "vertical"},
		Children: []*android.View{{
			Type: "TextView",
			Line: 3,
			Col:  5,
			AttrPos: map[string]android.XMLPos{
				"android:layout_height": {Line: 5, Col: 9},
			},
			Attributes: map[string]string{
				"android:layout_width":  "wrap_content",
				"android:layout_height": "0dp",
			},
			LayoutWidth:  "wrap_content",
			LayoutHeight: "0dp",
		}},
	})
	findings := runResourceRule(r, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Line != 5 || findings[0].Col != 9 {
		t.Fatalf("got %d:%d, want attribute position 5:9", findings[0].Line, findings[0].Col)
	}
}

func TestMissingAttributeAnchorsAtElementStart(t *testing.T) {
	r := findResourceRule(t, "MissingContentDescriptionResource")
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type:       "ImageView",
		Line:       4,
		Col:        7,
		Attributes: map[string]string{},
	})
	findings := runResourceRule(r, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Line != 4 || findings[0].Col != 7 {
		t.Fatalf("got %d:%d, want element start 4:7", findings[0].Line, findings[0].Col)
	}
}

func TestAttributeRulesAnchorAtTheAttribute(t *testing.T) {
	cases := []struct {
		rule  string
		attrs map[string]string
		attr  string
	}{
		{"SpUsageResource", map[string]string{"android:textSize": "14dp"}, "android:textSize"},
		{"PxUsageResource", map[string]string{"android:layout_width": "12px"}, "android:layout_width"},
		{"RtlHardcodedResource", map[string]string{"android:layout_marginRight": "4dp"}, "android:layout_marginRight"},
		{"RtlSymmetryResource", map[string]string{"android:paddingLeft": "4dp"}, "android:paddingLeft"},
		{"InvalidIdResource", map[string]string{"android:id": "bad id"}, "android:id"},
		{"OnClickResource", map[string]string{"android:onClick": "go"}, "android:onClick"},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			r := findResourceRule(t, tc.rule)
			attrs := map[string]string{"android:layout_width": "wrap_content", "android:layout_height": "wrap_content"}
			for k, v := range tc.attrs {
				attrs[k] = v
			}
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type:         "TextView",
				Line:         3,
				Col:          1,
				AttrPos:      map[string]android.XMLPos{tc.attr: {Line: 11, Col: 5}},
				Attributes:   attrs,
				LayoutWidth:  attrs["android:layout_width"],
				LayoutHeight: attrs["android:layout_height"],
				ID:           attrs["android:id"],
			})
			findings := runResourceRule(r, idx)
			if len(findings) == 0 {
				t.Fatal("expected a finding")
			}
			for _, f := range findings {
				if f.Line != 11 || f.Col != 5 {
					t.Fatalf("got %d:%d, want attribute position 11:5", f.Line, f.Col)
				}
			}
		})
	}
}

func variantIndex(layouts ...*android.Layout) *android.ResourceIndex {
	idx := indexWithLayout("seed", "res/layout/seed.xml", &android.View{Type: "FrameLayout"})
	idx.Layouts = map[string]*android.Layout{}
	idx.LayoutVariants = layouts
	return idx
}

func TestDuplicateIncludedIdsStaysWithinOneQualifierSet(t *testing.T) {
	r := findResourceRule(t, "DuplicateIncludedIdsResource")
	include := func() *android.View {
		return &android.View{Type: "include", Attributes: map[string]string{"layout": "@layout/header"}}
	}
	view := func(id string, children ...*android.View) *android.View {
		return &android.View{Type: "FrameLayout", ID: id, Line: 2, Col: 5, Attributes: map[string]string{}, Children: children}
	}
	portraitScreen := &android.Layout{Name: "screen", FilePath: "res/layout/screen.xml",
		RootView: view("@+id/screen_root", view("@+id/title"), include())}
	portraitHeader := &android.Layout{Name: "header", FilePath: "res/layout/header.xml", RootView: view("@+id/header_root")}
	landHeader := &android.Layout{Name: "header", FilePath: "res/layout-land/header.xml",
		RootView: view("@+id/header_root", view("@+id/title"))}

	if got := runResourceRule(r, variantIndex(portraitScreen, portraitHeader, landHeader)); len(got) != 0 {
		t.Fatalf("portrait screen must not collide with ids defined only in the landscape header: %v", got)
	}

	landScreen := &android.Layout{Name: "screen", FilePath: "res/layout-land/screen.xml",
		RootView: view("@+id/screen_root", view("@+id/title"), include())}
	got := runResourceRule(r, variantIndex(landScreen, portraitHeader, landHeader))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for the landscape collision, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "in layout-land") {
		t.Fatalf("message must name the qualifier variant: %q", got[0].Message)
	}
}
