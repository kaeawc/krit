package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// A View subclass declared in another file counts through that file's imports; a subclass of a project class
// named View in another package does not, even though the resolver's Supertypes names that class (HOM-31).
func TestDenyListedApiCrossFileViewSubclass(t *testing.T) {
	views := `package com.example.views

import android.content.Context
import android.view.View

class BadgeView(context: Context) : View(context)
`
	lookalike := `package com.example.fake

open class View {
  fun setOnClickListener(listener: Any?) {}
}

class FakeView : View()
`
	caller := `package com.example

import com.example.fake.FakeView
import com.example.views.BadgeView

fun clear(badge: BadgeView, fake: FakeView) {
  badge.setOnClickListener(null)
  fake.setOnClickListener(null)
}
`
	got := runRuleOnSourcesWithResolver(t, "DenyListedApi", map[string]string{
		"views/BadgeView.kt": views, "fake/Fake.kt": lookalike, "Caller.kt": caller,
	}, "Caller.kt")
	if len(got) != 1 || got[0].Line != 7 {
		t.Errorf("want one finding, on badge.setOnClickListener(null), got %v", got)
	}
}

// runRuleOnSourcesWithResolver indexes every source with the resolver and runs rule on the file named target.
func runRuleOnSourcesWithResolver(t *testing.T, ruleID string, sources map[string]string, target string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	var targetFile *scanner.File
	for name, src := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		parse := scanner.ParseFile
		if filepath.Ext(path) == ".java" {
			parse = scanner.ParseJavaFile
		}
		file, err := parse(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
		if name == target {
			targetFile = file
		}
	}
	resolver := typeinfer.NewResolver()
	// Like IndexPhase, index Kotlin and Java sources (#1885).
	resolver.IndexFilesParallel(files, 1)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(targetFile)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not registered", ruleID)
	return nil
}

// runRuleOnSourcesWithResolverAll is runRuleOnSourcesWithResolver over every source file.
func runRuleOnSourcesWithResolverAll(t *testing.T, ruleID string, sources map[string]string) []scanner.Finding {
	t.Helper()
	var out []scanner.Finding
	for name := range sources {
		out = append(out, runRuleOnSourcesWithResolver(t, ruleID, sources, name)...)
	}
	return out
}
