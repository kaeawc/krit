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

// TestSlackLintCrossFileParity checks slack-lints ports that need the cross-file index and source types against the
// Lint CLI, like TestSlackLintParity: each directory under testdata/slack_lint_crossfile (except stubs) holds
// checks.txt, Kotlin/Java sources under src/main/java and lint-0.11.1.txt (regenerated with
// testdata/slack_lint/run-lint.sh <case> ../slack_lint_crossfile/stubs/<stub>...). Files are indexed and typed the way
// the pipeline does it: Kotlin sources and Java files through the Java declaration pass, with a source type resolver
// over all of them.
func TestSlackLintCrossFileParity(t *testing.T) {
	root := filepath.Join("testdata", "slack_lint_crossfile")
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
			got := runSlackCrossFileChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from slack-lints 0.11.1 (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no cross-file parity cases found")
	}
}

func runSlackCrossFileChecks(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var kotlin, java, all []*scanner.File
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".kt":
			f, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				return err
			}
			kotlin = append(kotlin, f)
			all = append(all, f)
		case ".java":
			f, err := scanner.ParseJavaFile(context.Background(), path)
			if err != nil {
				return err
			}
			java = append(java, f)
			all = append(all, f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := scanner.BuildIndex(kotlin, 1, java...)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(all, 1)
	var out []string
	add := func(f scanner.Finding) {
		rel, err := filepath.Rel(abs, f.File)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
	}
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
		if rule.Needs.IsPerFile() {
			d := rules.NewDispatcher([]*api.Rule{rule}, resolver)
			for _, file := range all {
				cols := d.Run(file)
				for _, f := range cols.Findings() {
					f.File = file.Path
					add(f)
				}
			}
			continue
		}
		collector := scanner.NewFindingCollector(0)
		rule.Check(&api.Context{CodeIndex: index, ParsedFiles: all, Resolver: resolver, Collector: collector, Rule: rule})
		for _, f := range collector.Columns().Findings() {
			add(f)
		}
	}
	sort.Strings(out)
	return out
}

// runSlackAnnotationRule writes the Kotlin sources under a temporary project and runs the cross-file rule id over
// them, returning "file:line" for each finding, sorted.
func runSlackAnnotationRule(t *testing.T, id string, sources map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	for name, src := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	for _, r := range api.Registry {
		if r.ID == id {
			var out []string
			for _, f := range runCrossFileRuleOnFiles(r, files...) {
				out = append(out, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
			sort.Strings(out)
			return out
		}
	}
	t.Fatalf("rule %s not registered", id)
	return nil
}

const slackNamedParamsAPI = `package com.example

import slack.lint.annotations.MustUseNamedParams

@MustUseNamedParams
fun overloaded(a: Int, b: Int) = a + b

fun overloaded(s: String) = s

@MustUseNamedParams
fun shadowed(a: Int, b: Int) = a + b

class Builder @MustUseNamedParams constructor(val a: Int, val b: Int) {
  constructor(s: String) : this(1, 2)
}

class Outer {
  class Inner @MustUseNamedParams constructor(val a: Int, val b: Int)
}
`

// Review of #1812 (P2): a call that fits only an unannotated overload resolves to it; (P2) a member or local
// function of the same name wins over the imported top-level one; (P3) `Outer.Inner(...)` with Outer imported.
func TestMustUseNamedParamsOverloadsShadowingAndNestedClasses(t *testing.T) {
	got := runSlackAnnotationRule(t, "MustUseNamedParams", map[string]string{
		"Api.kt": slackNamedParamsAPI,
		"Caller.kt": `package other

import com.example.Builder
import com.example.Outer
import com.example.overloaded
import com.example.shadowed

open class Base {
  fun shadowed(a: Int, b: Int) = a - b
}

class Shadow {
  fun shadowed(a: Int, b: Int) = a * b
  fun m() = shadowed(1, 2)
}

class Inherits : Base() {
  fun m() = shadowed(1, 2)
}

fun local(): Int {
  fun shadowed(a: Int, b: Int) = a
  return shadowed(1, 2)
}

fun calls() {
  overloaded("x")
  Builder("s")
  overloaded(1, 2)
  Builder(1, 2)
  Outer.Inner(1, 2)
  shadowed(1, 2)
}
`,
	})
	want := []string{"Caller.kt:29", "Caller.kt:30", "Caller.kt:31", "Caller.kt:32"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v, want %v", got, want)
	}
}
