package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runDeadCodeOnSources(t *testing.T, sources map[string]string) []scanner.Finding {
	t.Helper()
	root := t.TempDir()
	var files []*scanner.File
	for name, src := range sources {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	index := scanner.BuildIndex(files, 1)
	registered := buildRuleIndex()["DeadCode"]
	if registered == nil {
		t.Fatal("DeadCode rule not registered")
	}
	ctx := &api.Context{CodeIndex: index, Collector: scanner.NewFindingCollector(0)}
	registered.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestDeadCode_OperatorFunctionsAreNotDead(t *testing.T) {
	findings := runDeadCodeOnSources(t, map[string]string{
		"A.kt": `package test
class Vec { operator fun plus(o: Vec): Vec = this
    operator fun component1(): Int = 1
    fun neverCalled() {} }
`,
		"B.kt": `package test
fun use() {
    val v = Vec() + Vec()
    val (a) = v
    println(a)
}
`,
	})
	var names []string
	for _, f := range findings {
		names = append(names, f.Message)
	}
	joined := strings.Join(names, "\n")
	if strings.Contains(joined, "'plus'") || strings.Contains(joined, "'component1'") {
		t.Errorf("operator functions are called through syntax and must not be reported:\n%s", joined)
	}
	if !strings.Contains(joined, "'neverCalled'") {
		t.Errorf("an ordinary unused function must still be reported:\n%s", joined)
	}
}
