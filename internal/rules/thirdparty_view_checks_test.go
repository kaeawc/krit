package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestSlackLintResolverParity is TestSlackLintParity for ports that need the source resolver's class index: each
// case under testdata/slack_lint_resolver (except stubs) is indexed as one project, the way the pipeline builds the
// resolver, before its checks run. The case layout and divergences.txt are as in testdata/slack_lint/README.md.
func TestSlackLintResolverParity(t *testing.T) {
	root := filepath.Join("testdata", "slack_lint_resolver")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, c := range cases {
		if !c.IsDir() || c.Name() == "stubs" {
			continue
		}
		ran++
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			want := slackParityExpected(t, dir)
			got := runResolverParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from slack-lints 0.11.1 (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no resolver parity cases found")
	}
}

func runResolverParityChecks(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*scanner.File
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var file *scanner.File
		switch filepath.Ext(path) {
		case ".kt":
			file, err = scanner.ParseFile(context.Background(), path)
		case ".java":
			file, err = scanner.ParseJavaFile(context.Background(), path)
		default:
			return nil
		}
		if err == nil {
			files = append(files, file)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 1)
	var out []string
	for _, check := range checks {
		var rule *api.Rule
		for _, r := range api.Registry {
			if r.ID == check {
				rule = r
			}
		}
		if rule == nil {
			t.Fatalf("rule %q not registered", check)
		}
		dispatcher := rules.NewDispatcher([]*api.Rule{rule}, resolver)
		for _, file := range files {
			cols := dispatcher.Run(file)
			for _, f := range cols.Findings() {
				rel, err := filepath.Rel(abs, f.File)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
			}
		}
	}
	sort.Strings(out)
	return out
}

// The override's class is found across files, through the declaring file's imports: a `View` imported from another
// package there is not android.view.View.
func TestDoNotCallViewToStringCrossFileHierarchy(t *testing.T) {
	views := `package com.example.views

import android.content.Context
import android.view.View

open class BadgeView(context: Context) : View(context) {
  override fun toString(): String = "BadgeView"
}
`
	lookalike := `package com.example.fake

class View {
  open override fun toString(): String = "fake"
}

class FakeView : View() {
  override fun toString(): String = "FakeView"
}
`
	caller := `package com.example

import com.example.fake.FakeView
import com.example.views.BadgeView

fun log(badge: BadgeView, fake: FakeView, nullable: BadgeView?) {
  badge.toString()
  fake.toString()
  nullable?.toString()
  nullable.toString()
}
`
	got := runRuleOnSourcesWithResolver(t, "DoNotCallViewToString", map[string]string{
		"views/BadgeView.kt": views, "fake/Fake.kt": lookalike, "Caller.kt": caller,
	}, "Caller.kt")
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	if fmt.Sprint(lines) != "[7 9]" {
		t.Errorf("want findings on badge.toString() and nullable?.toString(), got %v", got)
	}
}

// An inflate() in a class nested in an ItemDecoration, or on an unrelated receiver, is not reported; one in an
// anonymous ItemDecoration is.
func TestInflationInItemDecorationScopes(t *testing.T) {
	const src = `package com.example

import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView.ItemDecoration

class Inflater { fun inflate(id: Int) = id }

val decoration = object : ItemDecoration() {
  fun draw(parent: ViewGroup, own: Inflater) {
    LayoutInflater.from(parent.context).inflate(1, parent, false)
    own.inflate(1)
  }
}

class Outer : ItemDecoration() {
  class Helper(private val inflater: LayoutInflater) {
    fun make() = inflater.inflate(1, null)
  }
}
`
	got := runRuleOnSourcesWithResolver(t, "InflationInItemDecoration", map[string]string{"Deco.kt": src}, "Deco.kt")
	if len(got) != 1 || got[0].Line != 11 {
		t.Errorf("want one finding, in the anonymous ItemDecoration, got %v", got)
	}
}
