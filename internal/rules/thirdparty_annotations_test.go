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
