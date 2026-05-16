package ruleslinter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

// TestRulesPackageHasNoNewAdHocCaches is the gate against new sync.Map
// memoization in rule files. Existing instances are listed in
// grandfatheredAdHocCaches and shrink as caches migrate to a shared
// per-run facts layer (internal/filefacts/). New ad-hoc caches fail
// here.
func TestRulesPackageHasNoNewAdHocCaches(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Join(filepath.Dir(thisFile), "..", "rules")
	violations, err := AnalyzeAdHocCaches(rulesDir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches(%q): %v", rulesDir, err)
	}
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d ad-hoc cache violation(s):\n%s", len(violations), b.String())
}

// TestRulesPackageHasNoDefensiveContextGuards is the gate against
// reintroducing `if ctx.File == nil { return }` /
// `if ctx.Idx == 0 { return }` boilerplate in rule callbacks. The
// dispatcher guarantees both invariants; the guard is theater that
// obscures the rule's real preconditions. See
// AnalyzeDefensiveContextGuards for the detection contract.
func TestRulesPackageHasNoDefensiveContextGuards(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Join(filepath.Dir(thisFile), "..", "rules")
	violations, err := AnalyzeDefensiveContextGuards(rulesDir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards(%q): %v", rulesDir, err)
	}
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d defensive-context-guard violation(s):\n%s", len(violations), b.String())
}

func TestAnalyzeDefensiveContextGuards_FlagsRuleCallback(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

type FooRule struct{}

func (r *FooRule) check(ctx *api.Context) {
	if ctx.File == nil || ctx.Idx == 0 {
		return
	}
	_ = ctx
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeDefensiveContextGuards(dir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "dispatcher") {
		t.Fatalf("want dispatcher hint in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeDefensiveContextGuards_FlagsFileOnly(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func check(ctx *api.Context) {
	if ctx.File == nil {
		return
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeDefensiveContextGuards(dir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeDefensiveContextGuards_IgnoresHelperReturningValue(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Helpers that return values are responsible for their own nil-safety;
// they may be called from tests or other helpers that build Contexts
// outside the dispatcher.
func helper(ctx *api.Context) bool {
	if ctx.File == nil {
		return false
	}
	return true
}

func helperStr(ctx *api.Context) string {
	if ctx.File == nil {
		return ""
	}
	return ctx.File.Path
}
`
	if err := os.WriteFile(filepath.Join(dir, "helper.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeDefensiveContextGuards(dir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations from value-returning helpers, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeDefensiveContextGuards_IgnoresCompoundGuardWithRealCheck(t *testing.T) {
	// A condition that mixes the dispatcher-guaranteed check with a real
	// precondition is still flagged: the guard is still doing theater for
	// the ctx.File / ctx.Idx half. The rule author should split the check
	// or drop the dispatcher half.
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func check(ctx *api.Context) {
	if ctx.File == nil || ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeDefensiveContextGuards(dir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation from compound guard, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeDefensiveContextGuards_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func check(ctx *api.Context) {
	if ctx.File == nil {
		return
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeDefensiveContextGuards(dir)
	if err != nil {
		t.Fatalf("AnalyzeDefensiveContextGuards: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations from test files, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeAdHocCaches_FlagsNewSyncMap(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

var newCache sync.Map
`
	if err := os.WriteFile(filepath.Join(dir, "newrule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "filefacts") {
		t.Fatalf("want filefacts hint, got %q", violations[0].Message)
	}
}

func TestAnalyzeAdHocCaches_AcceptsGrandfathered(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

var allowedCache sync.Map
`
	if err := os.WriteFile(filepath.Join(dir, "exempt_test_fixture.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	saved := grandfatheredAdHocCaches
	defer func() { grandfatheredAdHocCaches = saved }()
	grandfatheredAdHocCaches = map[AdHocCacheException]bool{
		{File: "exempt_test_fixture.go", Name: "allowedCache"}: true,
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeAdHocCaches_FlagsStructField(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

type MyRule struct {
	cache sync.Map
}
`
	if err := os.WriteFile(filepath.Join(dir, "myrule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeAdHocCaches_FlagsMutexMapPair(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

var (
	fooCacheMu sync.RWMutex
	fooCache   = map[string]int{}
)
`
	if err := os.WriteFile(filepath.Join(dir, "newrule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "filefacts") {
		t.Fatalf("want filefacts hint, got %q", violations[0].Message)
	}
}

func TestAnalyzeAdHocCaches_DoesNotFlagLoneMutex(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

var dispatcherMu sync.RWMutex
`
	if err := os.WriteFile(filepath.Join(dir, "newrule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations from lone mutex, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeAdHocCaches_SkipsInfraFiles(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import "sync"

var dispatcherInternal sync.Map
`
	if err := os.WriteFile(filepath.Join(dir, "dispatcher.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeAdHocCaches(dir)
	if err != nil {
		t.Fatalf("AnalyzeAdHocCaches: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations from infra file, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_FlagsMissingNeedsResolver(t *testing.T) {
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:          "Buggy",
		Description: "uses resolver without declaring capability",
		Check: func(ctx *api.Context) {
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
	api "github.com/kaeawc/krit/internal/rules/api"
)

func init() {
	api.Register(&api.Rule{
		ID:          "OracleBug",
		Description: "uses oracle without declaring capability",
		Needs:       api.NeedsResolver,
		Check: func(ctx *api.Context) {
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
	api "github.com/kaeawc/krit/internal/rules/api"
)

func init() {
	api.Register(&api.Rule{
		ID:          "Good",
		Description: "declares what it uses",
		Needs:       api.NeedsResolver | api.NeedsOracle,
		Check: func(ctx *api.Context) {
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
	api "github.com/kaeawc/krit/internal/rules/api"
)

func init() {
	api.Register(&api.Rule{
		ID:          "Unified",
		Description: "uses both resolver and oracle under NeedsTypeInfo",
		Needs:       api.NeedsTypeInfo,
		Check: func(ctx *api.Context) {
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

import api "github.com/kaeawc/krit/internal/rules/api"

type BaseRule struct{ RuleName string }
type FooRule struct{ BaseRule }

func (r *FooRule) check(ctx *api.Context) {
	if ctx.Resolver != nil {
		_ = ctx.Resolver
	}
}

func init() {
	{
		r := &FooRule{BaseRule: BaseRule{RuleName: "Foo"}}
		api.Register(&api.Rule{
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

import api "github.com/kaeawc/krit/internal/rules/api"

func helper(ctx *api.Context) {
	_ = ctx.Resolver
}

func init() {
	api.Register(&api.Rule{
		ID:          "Delegated",
		Description: "helper pattern",
		Check: func(ctx *api.Context) {
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
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func init() {
	api.Register(&api.Rule{
		ID:          "MergesSerially",
		Description: "calls MergeCollectors without declaring NeedsConcurrent",
		Needs:       api.NeedsCrossFile,
		Check: func(ctx *api.Context) {
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

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:          "SpawnsGoroutine",
		Description: "spawns a goroutine without declaring NeedsConcurrent",
		Needs:       api.NeedsCrossFile,
		Check: func(ctx *api.Context) {
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

	api "github.com/kaeawc/krit/internal/rules/api"
)

func init() {
	api.Register(&api.Rule{
		ID:          "UsesWaitGroup",
		Description: "uses sync.WaitGroup without declaring NeedsConcurrent",
		Needs:       api.NeedsCrossFile,
		Check: func(ctx *api.Context) {
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

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:          "FalselyConcurrent",
		Description: "declares NeedsConcurrent without using it",
		Needs:       api.NeedsCrossFile | api.NeedsConcurrent,
		Check: func(ctx *api.Context) {
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

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func init() {
	api.Register(&api.Rule{
		ID:          "HonestConcurrent",
		Description: "declares NeedsConcurrent and actually uses it",
		Needs:       api.NeedsCrossFile | api.NeedsConcurrent,
		Check: func(ctx *api.Context) {
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
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func mergeHelper(dst *scanner.FindingCollector) {
	scanner.MergeCollectors(dst)
}

func init() {
	api.Register(&api.Rule{
		ID:          "HelperMerges",
		Description: "delegates MergeCollectors call to an in-package helper",
		Needs:       api.NeedsCrossFile,
		Check: func(ctx *api.Context) {
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

func TestAnalyzeSource_FlagsDeclaredFixWithoutAssignment(t *testing.T) {
	// A rule that declares Fix: api.FixSemantic but whose Check body
	// never assigns f.Fix is lying to --fix UX and SARIF metadata.
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:          "EmptyFixRule",
		Description: "advertises a fix but does not produce one",
		Fix:         api.FixSemantic,
		Check: func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "no fix attached")
		},
	})
}
`
	violations := analyzeSource(t, "emptyfix.go", src)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "FixSemantic") {
		t.Fatalf("want FixSemantic in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeSource_AcceptsFixAssignmentInCheckBody(t *testing.T) {
	src := `package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func init() {
	api.Register(&api.Rule{
		ID:          "InlineFixRule",
		Description: "sets f.Fix directly in the check body",
		Fix:         api.FixIdiomatic,
		Check: func(ctx *api.Context) {
			f := ctx.Finding(1, 1, "msg")
			f.Fix = &scanner.Fix{ByteMode: true}
			ctx.Emit(f)
		},
	})
}
`
	violations := analyzeSource(t, "inlinefix.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_AcceptsFixAssignmentInRuleHelperMethod(t *testing.T) {
	// The fix lives on a helper method bound to the rule receiver.
	// The linter must thread the method's receiver through call
	// resolution so it can follow r.helper() into the helper body.
	src := `package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type DelegatedFixRule struct{}

func (r *DelegatedFixRule) emit(ctx *api.Context) {
	f := ctx.Finding(1, 1, "msg")
	f.Fix = &scanner.Fix{ByteMode: true}
	ctx.Emit(f)
}

func (r *DelegatedFixRule) check(ctx *api.Context) {
	r.emit(ctx)
}

func init() {
	r := &DelegatedFixRule{}
	api.Register(&api.Rule{
		ID:          "DelegatedFixRule",
		Description: "fix is set in a helper method on the rule receiver",
		Fix:         api.FixIdiomatic,
		Check:       r.check,
	})
}
`
	violations := analyzeSource(t, "delegatedfix.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeSource_FixNoneSkipsGate(t *testing.T) {
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:          "NoFixRule",
		Description: "explicitly declares no fix",
		Fix:         api.FixNone,
		Check: func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "msg")
		},
	})
}
`
	violations := analyzeSource(t, "nofix.go", src)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

// TestRulesPackageHasOptInReasons is the gate: every default-inactive
// rule (DefaultActive: false, or DefaultActive omitted) in internal/rules
// must declare an OptInReason in the same composite literal so the
// registry can be audited by category. Default-active rules must NOT
// carry a reason.
func TestRulesPackageHasOptInReasons(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Join(filepath.Dir(thisFile), "..", "rules")
	violations, err := AnalyzeOptInReason(rulesDir)
	if err != nil {
		t.Fatalf("AnalyzeOptInReason(%q): %v", rulesDir, err)
	}
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d OptInReason violation(s):\n%s", len(violations), b.String())
}

func TestAnalyzeOptInReason_FlagsMissingReason(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:            "OptInNoReason",
		DefaultActive: false,
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "rule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeOptInReason(dir)
	if err != nil {
		t.Fatalf("AnalyzeOptInReason: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleID != "OptInNoReason" {
		t.Fatalf("want rule ID OptInNoReason, got %q", violations[0].RuleID)
	}
	if !strings.Contains(violations[0].Message, "OptInReason") {
		t.Fatalf("want OptInReason in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeOptInReason_AcceptsReason(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:            "OptInWithReason",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "rule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeOptInReason(dir)
	if err != nil {
		t.Fatalf("AnalyzeOptInReason: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestAnalyzeOptInReason_FlagsReasonOnActiveRule(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func init() {
	api.Register(&api.Rule{
		ID:            "ActiveButHasReason",
		DefaultActive: true,
		OptInReason:   api.OptInReasonOpinionated,
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "rule.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeOptInReason(dir)
	if err != nil {
		t.Fatalf("AnalyzeOptInReason: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "must not carry") {
		t.Fatalf("want 'must not carry' in message, got %q", violations[0].Message)
	}
}

func TestAnalyzeOptInReason_HandlesRuleDescriptor(t *testing.T) {
	dir := t.TempDir()
	src := `package rules

import api "github.com/kaeawc/krit/internal/rules/api"

type FooRule struct{}

func (r *FooRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Foo",
		DefaultActive: false,
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "meta.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	violations, err := AnalyzeOptInReason(dir)
	if err != nil {
		t.Fatalf("AnalyzeOptInReason: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleID != "Foo" {
		t.Fatalf("want rule ID Foo, got %q", violations[0].RuleID)
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
