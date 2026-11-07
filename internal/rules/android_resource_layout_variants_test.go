package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// Layout rules must inspect every qualifier variant and report its own file.
func TestLayoutRulesAnalyseEveryQualifierVariant(t *testing.T) {
	r := findResourceRule(t, "TextFieldsResource")
	idx := &android.ResourceIndex{
		Layouts:       map[string]*android.Layout{},
		LayoutConfigs: map[string]map[string]*android.Layout{},
	}
	for _, p := range []string{"res/layout/main.xml", "res/layout-land/main.xml", "res/layout-sw600dp/main.xml"} {
		l := &android.Layout{Name: "main", FilePath: p, RootView: &android.View{
			Type: "EditText", Line: 3,
			Attributes: map[string]string{},
		}}
		idx.LayoutVariants = append(idx.LayoutVariants, l)
		if idx.Layouts["main"] == nil {
			idx.Layouts["main"] = l
		}
	}
	files := map[string]bool{}
	for _, f := range runResourceRule(r, idx) {
		files[f.File] = true
	}
	if len(files) != 3 {
		t.Fatalf("findings cover files %v, want all three variants", files)
	}
}
