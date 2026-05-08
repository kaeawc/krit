package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// These tests exercise findFixableFindings (the core idempotency
// helper) using fake rules with hand-crafted Check functions. They
// verify the filtering layer that decides whether the .expected
// output for a rule is "still fixable" — without depending on any
// real rule's behavior, so a regression in this harness is caught
// independently of rule changes.

const idempotencyTestSource = `package test

fun greet() {
    println("hi")
}
`

// writeIdempotencyFixture writes a small Kotlin file to a temp dir
// and returns the path. We don't reuse writeKotlinFile from
// dispatcher_test.go because that file is in package rules; this
// helper lives in package rules_test.
func writeIdempotencyFixture(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestFindFixableFindings_ReturnsOnlyFixableFindings(t *testing.T) {
	path := writeIdempotencyFixture(t, "Sample.kt", idempotencyTestSource)

	emitsFixable := api.FakeRule("FakeFixable",
		api.WithNodeTypes("call_expression"),
		api.WithFix(api.FixCosmetic),
		api.WithCheck(func(ctx *api.Context) {
			ctx.Emit(scanner.Finding{
				Line: 1,
				Col:  1,
				Fix: &scanner.Fix{
					StartLine:   1,
					EndLine:     1,
					Replacement: "// noop\n",
				},
			})
		}),
	)
	emitsNonFixable := api.FakeRule("FakeNonFixable",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "informational only")
		}),
	)

	leftovers, err := findFixableFindings(path, []*api.Rule{emitsFixable, emitsNonFixable}, "")
	if err != nil {
		t.Fatalf("findFixableFindings: %v", err)
	}
	if len(leftovers) == 0 {
		t.Fatal("expected at least one fixable finding, got none")
	}
	for _, f := range leftovers {
		if f.Fix == nil {
			t.Errorf("returned finding has nil Fix: %+v", f)
		}
		if f.Rule == "FakeNonFixable" {
			t.Errorf("returned finding from non-fixable rule: %+v", f)
		}
	}
}

func TestFindFixableFindings_RuleFilterScopesToOneRule(t *testing.T) {
	path := writeIdempotencyFixture(t, "Sample.kt", idempotencyTestSource)

	ruleA := api.FakeRule("FakeRuleA",
		api.WithNodeTypes("call_expression"),
		api.WithFix(api.FixCosmetic),
		api.WithCheck(func(ctx *api.Context) {
			ctx.Emit(scanner.Finding{
				Line: 1, Col: 1,
				Fix: &scanner.Fix{StartLine: 1, EndLine: 1, Replacement: "a"},
			})
		}),
	)
	ruleB := api.FakeRule("FakeRuleB",
		api.WithNodeTypes("call_expression"),
		api.WithFix(api.FixCosmetic),
		api.WithCheck(func(ctx *api.Context) {
			ctx.Emit(scanner.Finding{
				Line: 1, Col: 1,
				Fix: &scanner.Fix{StartLine: 1, EndLine: 1, Replacement: "b"},
			})
		}),
	)

	scoped, err := findFixableFindings(path, []*api.Rule{ruleA, ruleB}, "FakeRuleA")
	if err != nil {
		t.Fatalf("findFixableFindings: %v", err)
	}
	if len(scoped) == 0 {
		t.Fatal("expected findings for FakeRuleA, got none")
	}
	for _, f := range scoped {
		if f.Rule != "FakeRuleA" {
			t.Errorf("rule filter leaked: got finding from %s, want only FakeRuleA", f.Rule)
		}
	}
}

func TestFindFixableFindings_NoFindingsMeansIdempotent(t *testing.T) {
	path := writeIdempotencyFixture(t, "Sample.kt", idempotencyTestSource)

	silent := api.FakeRule("FakeSilent",
		api.WithNodeTypes("call_expression"),
		api.WithFix(api.FixCosmetic),
		api.WithCheck(func(ctx *api.Context) {}),
	)

	leftovers, err := findFixableFindings(path, []*api.Rule{silent}, "")
	if err != nil {
		t.Fatalf("findFixableFindings: %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("expected zero leftovers for a silent rule, got %d: %+v", len(leftovers), leftovers)
	}
}
