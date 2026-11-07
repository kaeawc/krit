package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// viewConstructorLayoutIndex builds a layout index whose root names each class as an element tag.
func viewConstructorLayoutIndex(classes ...string) *android.ResourceIndex {
	idx := emptyIndex()
	root := &android.View{Type: "LinearLayout", Attributes: map[string]string{}}
	for _, c := range classes {
		root.Children = append(root.Children, &android.View{Type: c, Attributes: map[string]string{}})
	}
	layout := &android.Layout{Name: "screen", FilePath: "res/layout/screen.xml", RootView: root}
	idx.Layouts["screen"] = layout
	idx.LayoutConfigs["screen"] = map[string]*android.Layout{"": layout}
	return idx
}

func runViewConstructorWithIndex(t *testing.T, path, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	full := filepath.Join(t.TempDir(), path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), full)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range api.Registry {
		if r.ID == "ViewConstructor" {
			cols := rules.NewDispatcher([]*api.Rule{r}, nil).RunResourceSource(file, idx)
			return cols.Findings()
		}
	}
	t.Fatal("ViewConstructor not found in registry")
	return nil
}

// runViewConstructorRule runs the rule with a layout that inflates every class the shared snippets declare.
func runViewConstructorRule(t *testing.T, code string) []scanner.Finding {
	t.Helper()
	return runViewConstructorWithIndex(t, "Sample.kt", code, viewConstructorLayoutIndex("test.MyView", "test.PlainView", "test.BaseView", "test.FakeScreen"))
}

func runViewConstructorRuleOnPath(t *testing.T, path, code string) []scanner.Finding {
	t.Helper()
	return runViewConstructorWithIndex(t, path, code, viewConstructorLayoutIndex("test.MyView"))
}

func TestViewConstructorRule_LayoutReferences(t *testing.T) {
	const missingAttrs = `
package test
class MyView(context: Context) : View(context)
`
	t.Run("not named in any layout is skipped", func(t *testing.T) {
		if got := runViewConstructorWithIndex(t, "Sample.kt", missingAttrs, viewConstructorLayoutIndex("test.Other")); len(got) != 0 {
			t.Fatalf("expected no findings for a view no layout inflates, got %d", len(got))
		}
	})
	t.Run("no resource index is skipped", func(t *testing.T) {
		file := parseInline(t, missingAttrs)
		for _, r := range api.Registry {
			if r.ID == "ViewConstructor" {
				cols := rules.NewDispatcher([]*api.Rule{r}, nil).RunResourceSource(file, nil)
				if got := cols.Findings(); len(got) != 0 {
					t.Fatalf("expected no findings without an index, got %d", len(got))
				}
			}
		}
	})
	t.Run("named by tag still fires", func(t *testing.T) {
		if got := runViewConstructorWithIndex(t, "Sample.kt", missingAttrs, viewConstructorLayoutIndex("test.MyView")); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("named by view class attribute still fires", func(t *testing.T) {
		idx := viewConstructorLayoutIndex()
		idx.Layouts["screen"].RootView.Children = []*android.View{{Type: "view", Attributes: map[string]string{"class": "test.MyView"}}}
		if got := runViewConstructorWithIndex(t, "Sample.kt", missingAttrs, idx); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("named by nested dollar name still fires", func(t *testing.T) {
		code := "package test\nclass Outer {\n    class MyView(context: Context) : View(context)\n}\n"
		if got := runViewConstructorWithIndex(t, "Sample.kt", code, viewConstructorLayoutIndex("test.Outer$MyView")); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("secondary context constructor but not in layout is skipped", func(t *testing.T) {
		code := "package test\nclass PinLayout : FrameLayout {\n    constructor(context: Context) : super(context)\n}\n"
		if got := runViewConstructorWithIndex(t, "Sample.kt", code, viewConstructorLayoutIndex()); len(got) != 0 {
			t.Fatalf("expected no findings, got %d", len(got))
		}
	})
	t.Run("private class is skipped", func(t *testing.T) {
		code := "package test\nprivate class MyView(context: Context) : View(context)\n"
		if got := runViewConstructorWithIndex(t, "Sample.kt", code, viewConstructorLayoutIndex("test.MyView")); len(got) != 0 {
			t.Fatalf("expected no findings, got %d", len(got))
		}
	})
	t.Run("required extra constructor parameter is skipped", func(t *testing.T) {
		code := "package test\nclass MyView(context: Context, val title: String) : View(context)\n"
		if got := runViewConstructorWithIndex(t, "Sample.kt", code, viewConstructorLayoutIndex("test.MyView")); len(got) != 0 {
			t.Fatalf("expected no findings, got %d", len(got))
		}
	})
}
