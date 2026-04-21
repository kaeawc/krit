package ruleslinter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRulesPackageHasNoCapabilityDrift is the gate: every rule in
// internal/rules must declare NeedsResolver / NeedsOracle when its
// Check body consumes those capabilities. A new rule that forgets the
// declaration fails this test.
func TestRulesPackageHasNoCapabilityDrift(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Join(filepath.Dir(thisFile), "..", "rules")
	violations, err := Analyze(rulesDir)
	if err != nil {
		t.Fatalf("Analyze(%q): %v", rulesDir, err)
	}
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d capability-declaration violation(s):\n%s", len(violations), b.String())
}

func TestAnalyzeSource_FlagsMissingNeedsResolver(t *testing.T) {
	src := `package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

func init() {
	v2.Register(&v2.Rule{
		ID:          "Buggy",
		Description: "uses resolver without declaring capability",
		Check: func(ctx *v2.Context) {
			if ctx.Resolver != nil {
				_ = ctx.Resolver
			}
		},
	})
}
`
	violations := analyzeSource(t, "buggy.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleID != "Buggy" {
		t.Fatalf("want rule ID Buggy, got %q", violations[0].RuleID)
	}
	if !strings.Contains(violations[0].Message, "NeedsResolver") {
		t.Fatalf("want NeedsResolver in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_FlagsMissingNeedsOracle(t *testing.T) {
	src := `package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "OracleBug",
		Description: "uses oracle without declaring capability",
		Needs:       v2.NeedsResolver,
		Check: func(ctx *v2.Context) {
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				_ = cr.Oracle()
			}
		},
	})
}
`
	violations := analyzeSource(t, "oraclebug.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "NeedsOracle") {
		t.Fatalf("want NeedsOracle in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_AcceptsCorrectDeclaration(t *testing.T) {
	src := `package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "Good",
		Description: "declares what it uses",
		Needs:       v2.NeedsResolver | v2.NeedsOracle,
		Check: func(ctx *v2.Context) {
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				_ = cr.Oracle()
			}
		},
	})
}
`
	violations := analyzeSource(t, "good.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_NeedsTypeInfoSatisfiesBoth(t *testing.T) {
	src := `package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "Unified",
		Description: "uses both resolver and oracle under NeedsTypeInfo",
		Needs:       v2.NeedsTypeInfo,
		Check: func(ctx *v2.Context) {
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				_ = cr.Oracle()
			}
		},
	})
}
`
	violations := analyzeSource(t, "unified.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_ResolvesMethodReference(t *testing.T) {
	src := `package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

type BaseRule struct{ RuleName string }
type FooRule struct{ BaseRule }

func (r *FooRule) check(ctx *v2.Context) {
	if ctx.Resolver != nil {
		_ = ctx.Resolver
	}
}

func init() {
	{
		r := &FooRule{BaseRule: BaseRule{RuleName: "Foo"}}
		v2.Register(&v2.Rule{
			ID:          r.RuleName,
			Description: "bound via method",
			Check:       r.check,
		})
	}
}
`
	violations := analyzeSource(t, "method.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleID != "Foo" {
		t.Fatalf("want rule ID Foo, got %q", violations[0].RuleID)
	}
}

func TestAnalyzeSource_FollowsInPackageHelpers(t *testing.T) {
	// The Check body delegates to an in-package helper that takes the
	// ctx. The linter must follow the call and still detect usage.
	src := `package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

func helper(ctx *v2.Context) {
	_ = ctx.Resolver
}

func init() {
	v2.Register(&v2.Rule{
		ID:          "Delegated",
		Description: "helper pattern",
		Check: func(ctx *v2.Context) {
			helper(ctx)
		},
	})
}
`
	violations := analyzeSource(t, "delegate.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_FlagsMissingNeedsConcurrent_MergeCollectors(t *testing.T) {
	// A rule body that calls scanner.MergeCollectors manages its own
	// worker-local collectors and MUST declare NeedsConcurrent so the
	// dispatcher routes it through the parallel cross-file path.
	src := `package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "MergesSerially",
		Description: "calls MergeCollectors without declaring NeedsConcurrent",
		Needs:       v2.NeedsCrossFile,
		Check: func(ctx *v2.Context) {
			local := scanner.NewFindingCollector(0)
			scanner.MergeCollectors(ctx.Collector, local)
		},
	})
}
`
	violations := analyzeSource(t, "merges.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleID != "MergesSerially" {
		t.Fatalf("want rule ID MergesSerially, got %q", violations[0].RuleID)
	}
	if !strings.Contains(violations[0].Message, "NeedsConcurrent") {
		t.Fatalf("want NeedsConcurrent in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_FlagsMissingNeedsConcurrent_Goroutine(t *testing.T) {
	src := `package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

func init() {
	v2.Register(&v2.Rule{
		ID:          "SpawnsGoroutine",
		Description: "spawns a goroutine without declaring NeedsConcurrent",
		Needs:       v2.NeedsCrossFile,
		Check: func(ctx *v2.Context) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				ctx.EmitAt(1, 1, "hi")
			}()
			<-done
		},
	})
}
`
	violations := analyzeSource(t, "goroutine.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "NeedsConcurrent") {
		t.Fatalf("want NeedsConcurrent in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_FlagsMissingNeedsConcurrent_WaitGroup(t *testing.T) {
	src := `package rules

import (
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "UsesWaitGroup",
		Description: "uses sync.WaitGroup without declaring NeedsConcurrent",
		Needs:       v2.NeedsCrossFile,
		Check: func(ctx *v2.Context) {
			var wg sync.WaitGroup
			wg.Wait()
		},
	})
}
`
	violations := analyzeSource(t, "waitgroup.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "NeedsConcurrent") {
		t.Fatalf("want NeedsConcurrent in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_FlagsDeclaredButUnusedNeedsConcurrent(t *testing.T) {
	// NeedsConcurrent is declared but the body contains none of the
	// concurrent-state signals — the declaration is a lie and must be
	// removed (or the body must actually use the capability).
	src := `package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

func init() {
	v2.Register(&v2.Rule{
		ID:          "FalselyConcurrent",
		Description: "declares NeedsConcurrent without using it",
		Needs:       v2.NeedsCrossFile | v2.NeedsConcurrent,
		Check: func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "hi")
		},
	})
}
`
	violations := analyzeSource(t, "falseconcurrent.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "declares NeedsConcurrent") {
		t.Fatalf("want declared-but-unused message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_AcceptsCorrectNeedsConcurrentDeclaration(t *testing.T) {
	src := `package rules

import (
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func init() {
	v2.Register(&v2.Rule{
		ID:          "HonestConcurrent",
		Description: "declares NeedsConcurrent and actually uses it",
		Needs:       v2.NeedsCrossFile | v2.NeedsConcurrent,
		Check: func(ctx *v2.Context) {
			var wg sync.WaitGroup
			locals := []*scanner.FindingCollector{scanner.NewFindingCollector(0)}
			wg.Add(1)
			go func() {
				defer wg.Done()
				locals[0].Append(scanner.Finding{})
			}()
			wg.Wait()
			scanner.MergeCollectors(ctx.Collector, locals...)
		},
	})
}
`
	violations := analyzeSource(t, "honest.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_ConcurrentSignalsFollowHelpers(t *testing.T) {
	// The concurrent primitive lives inside a same-package helper; the
	// linter must transitively reach it the same way it reaches
	// ctx.Resolver through helpers.
	src := `package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func mergeHelper(dst *scanner.FindingCollector) {
	scanner.MergeCollectors(dst)
}

func init() {
	v2.Register(&v2.Rule{
		ID:          "HelperMerges",
		Description: "delegates MergeCollectors call to an in-package helper",
		Needs:       v2.NeedsCrossFile,
		Check: func(ctx *v2.Context) {
			mergeHelper(ctx.Collector)
		},
	})
}
`
	violations := analyzeSource(t, "helpermerges.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "NeedsConcurrent") {
		t.Fatalf("want NeedsConcurrent in message, got %q", violations[0].Message)
	}
}

func analyzeSource(t *testing.T, name, src string) []Violation {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return analyzeFiles(fset, []*ast.File{f})
}
