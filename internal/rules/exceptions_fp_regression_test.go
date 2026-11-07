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

// ruleByIDForExceptionsTest returns the registered rule with the given ID.
func ruleByIDForExceptionsTest(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %q not registered", id)
	return nil
}

// runExceptionsRuleOnSource parses an inline Kotlin/Java source and returns the
// findings produced by the named rule.
func runExceptionsRuleOnSource(t *testing.T, id, ext, src string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "Fixture"+ext)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var (
		file *scanner.File
		err  error
	)
	if ext == ".java" {
		file, err = scanner.ParseJavaFile(context.Background(), p)
	} else {
		file, err = scanner.ParseFile(context.Background(), p)
	}
	if err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	rule := ruleByIDForExceptionsTest(t, id)
	if rule.Needs.Has(api.NeedsResolver) {
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		cols := rules.NewDispatcher([]*api.Rule{rule}, resolver).Run(file)
		return cols.Findings()
	}
	cols := rules.NewDispatcher([]*api.Rule{rule}, nil).Run(file)
	return cols.Findings()
}
