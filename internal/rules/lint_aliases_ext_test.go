package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestSuppressLintByAndroidLintIDSuppressesRecyclerAdapterRule(t *testing.T) {
	const adapter = `
package test;
import android.annotation.SuppressLint;
import androidx.recyclerview.widget.RecyclerView;
class MyAdapter extends RecyclerView.Adapter<RecyclerView.ViewHolder> {
  %s
  void refresh() {
    notifyDataSetChanged();
  }
}`
	plain := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterWithoutDiffUtil", strings.Replace(adapter, "%s", "", 1))
	if len(plain) != 1 {
		t.Fatalf("expected a finding without the annotation, got %d", len(plain))
	}
	suppressed := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterWithoutDiffUtil", strings.Replace(adapter, "%s", `@SuppressLint("NotifyDataSetChanged")`, 1))
	if len(suppressed) != 0 {
		t.Fatalf("@SuppressLint(\"NotifyDataSetChanged\") must suppress the rule, got %d", len(suppressed))
	}
}

func TestRunResourceHonoursToolsIgnoreOnAncestors(t *testing.T) {
	r := findResourceRule(t, "IdTypePrefixMismatchResource")
	build := func(rootIgnore string) *android.ResourceIndex {
		attrs := map[string]string{}
		if rootIgnore != "" {
			attrs["tools:ignore"] = rootIgnore
		}
		root := &android.View{Type: "FrameLayout", Line: 2, Attributes: attrs, Children: []*android.View{
			{Type: "ImageView", ID: "@+id/tv_name", Line: 4, Attributes: map[string]string{}},
		}}
		return indexWithLayout("main", "res/layout/main.xml", root)
	}
	run := func(idx *android.ResourceIndex) int {
		file := &scanner.File{Path: "res", Language: scanner.LangXML}
		cols, _ := rules.NewDispatcher([]*api.Rule{r}, nil).RunResource(file, idx)
		return cols.Len()
	}
	if n := run(build("")); n != 1 {
		t.Fatalf("expected 1 finding without tools:ignore, got %d", n)
	}
	for _, ignore := range []string{"IdTypePrefixMismatchResource", "CutPasteIdResource", "all", "Foo,IdTypePrefixMismatchResource"} {
		if n := run(build(ignore)); n != 0 {
			t.Errorf("tools:ignore=%q on the ancestor must suppress the finding, got %d", ignore, n)
		}
	}
	if n := run(build("Overdraw")); n != 1 {
		t.Errorf("an unrelated tools:ignore must not suppress, got %d", n)
	}
}
