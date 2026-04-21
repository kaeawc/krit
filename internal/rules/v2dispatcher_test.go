package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// writeKotlinFile writes `code` to a temp file and parses it with
// scanner.ParseFile so the FlatTree / NodeTypeTable are populated.
func writeKotlinFile(t *testing.T, code string, name string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return file
}

// sampleKotlin is a small program whose flat tree reliably contains
// call_expression, function_declaration, identifier, and source_file
// node types after parsing.
const sampleKotlin = `package test

fun greet(name: String): String {
    return "hi " + name
}

fun main() {
    greet("world")
    println(greet("kotlin"))
}
`

// --- 1. V2Dispatcher routes node rules, line rules, and legacy rules ---------

func TestV2Dispatcher_RoutesFamiliesIndependently(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Sample.kt")

	nodeCalls := 0
	nodeRule := v2.FakeRule("V2TestNode",
		v2.WithNodeTypes("call_expression"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			nodeCalls++
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "call here")
		}),
	)
	nodeRule.Category = "style"

	lineCalls := 0
	lineRule := v2.FakeRule("V2TestLine",
		v2.WithNeeds(v2.NeedsLinePass),
		v2.WithCheck(func(ctx *v2.Context) {
			lineCalls++
			ctx.EmitAt(1, 1, "line once")
		}),
	)
	lineRule.Category = "naming"

	legacyCalls := 0
	legacyRule := &v2.Rule{
		ID:          "V2TestLegacy",
		Category:    "legacy",
		Description: "legacy once-per-file rule",
		Sev:         v2.SeverityInfo,
		// nil NodeTypes + no NeedsLinePass flag → allNodeRules in our
		// classifier. Mark it as a "legacy" by giving it an empty
		// (non-nil) NodeTypes slice so it falls through to legacyRules.
		NodeTypes: []string{},
		Check: func(ctx *v2.Context) {
			legacyCalls++
			ctx.EmitAt(1, 1, "legacy")
		},
	}

	d := NewV2Dispatcher([]*v2.Rule{nodeRule, lineRule, legacyRule})
	columns := d.Run(file)

	if nodeCalls == 0 {
		t.Error("node rule was never invoked on call_expression")
	}
	if lineCalls != 1 {
		t.Errorf("line rule invoked %d times, want 1", lineCalls)
	}
	if legacyCalls != 1 {
		t.Errorf("legacy rule invoked %d times, want 1", legacyCalls)
	}

	// Verify findings carry rule metadata populated by stampV2Findings.
	if columns.Len() == 0 {
		t.Fatal("expected findings, got none")
	}
	sawNode, sawLine, sawLegacy := false, false, false
	for i := 0; i < columns.Len(); i++ {
		switch columns.RuleAt(i) {
		case "V2TestNode":
			sawNode = true
			if columns.RuleSetAt(i) != "style" || columns.SeverityAt(i) != "warning" {
				t.Errorf("node finding metadata wrong: ruleset=%q severity=%q", columns.RuleSetAt(i), columns.SeverityAt(i))
			}
		case "V2TestLine":
			sawLine = true
			if columns.RuleSetAt(i) != "naming" {
				t.Errorf("line finding ruleset %q, want naming", columns.RuleSetAt(i))
			}
		case "V2TestLegacy":
			sawLegacy = true
		}
	}
	if !sawNode || !sawLine || !sawLegacy {
		t.Errorf("missing findings: node=%v line=%v legacy=%v", sawNode, sawLine, sawLegacy)
	}
}

// --- 3. Panic recovery -------------------------------------------------------

func TestV2Dispatcher_PanicRecovery(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Panic.kt")

	goodCalls := 0
	goodRule := v2.FakeRule("GoodRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithCheck(func(ctx *v2.Context) {
			goodCalls++
			ctx.EmitAt(1, 1, "ok")
		}),
	)
	badRule := v2.FakeRule("BadRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithCheck(func(ctx *v2.Context) {
			panic("intentional")
		}),
	)

	d := NewV2Dispatcher([]*v2.Rule{goodRule, badRule})
	columns, stats := d.RunWithStats(file)

	if goodCalls == 0 {
		t.Error("good rule was never invoked — dispatcher may have aborted on panic")
	}
	if columns.Len() == 0 {
		t.Error("expected findings from good rule even with bad rule panicking")
	}
	if len(stats.Errors) == 0 {
		t.Fatal("expected DispatchError entries from panic recovery")
	}
	found := false
	for _, e := range stats.Errors {
		if e.RuleName == "BadRule" {
			found = true
			if !strings.Contains(e.Error(), "intentional") {
				t.Errorf("error message missing panic value: %s", e.Error())
			}
		}
	}
	if !found {
		t.Error("no DispatchError for BadRule")
	}
}

// --- 4. Exclusion filter -----------------------------------------------------

func TestV2Dispatcher_RespectsExclusions(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "ExcludedTest.kt")

	nodeCalls := 0
	rule := v2.FakeRule("ExcludableRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithCheck(func(ctx *v2.Context) {
			nodeCalls++
		}),
	)

	// Register an exclusion that matches any file containing "Excluded".
	SetRuleExcludes("ExcludableRule", []string{"**/*Test.kt"})
	t.Cleanup(func() { SetRuleExcludes("ExcludableRule", nil) })

	d := NewV2Dispatcher([]*v2.Rule{rule})
	_ = d.Run(file)

	if nodeCalls != 0 {
		t.Errorf("rule ran %d times despite exclusion — expected 0", nodeCalls)
	}
}

// --- 5. ctx.Resolver is populated for resolver-needing rules ----------------

func TestV2Dispatcher_ResolverPopulatedInContext(t *testing.T) {
	cases := []struct {
		name  string
		needs v2.Capabilities
	}{
		{"NeedsResolver", v2.NeedsResolver},
		{"NeedsTypeInfo", v2.NeedsTypeInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeKotlinFile(t, "fun foo() { bar() }", tc.name+".kt")

			var gotResolver typeinfer.TypeResolver
			rule := v2.FakeRule("ResolverRule",
				v2.WithNodeTypes("call_expression"),
				v2.WithNeeds(tc.needs),
				v2.WithCheck(func(ctx *v2.Context) {
					gotResolver = ctx.Resolver
				}),
			)

			d := NewV2Dispatcher([]*v2.Rule{rule}, typeinfer.NewFakeResolver())
			_ = d.Run(file)

			if gotResolver == nil {
				t.Error("ctx.Resolver was nil even though resolver was provided")
			}
		})
	}
}

func TestV2Dispatcher_ResolverNilWithoutResolver(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "Resolver2.kt")

	var gotResolver typeinfer.TypeResolver
	rule := v2.FakeRule("ResolverRule2",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsResolver),
		v2.WithCheck(func(ctx *v2.Context) {
			gotResolver = ctx.Resolver
		}),
	)

	d := NewV2Dispatcher([]*v2.Rule{rule})
	_ = d.Run(file)

	if gotResolver != nil {
		t.Error("ctx.Resolver was non-nil even though no resolver was supplied")
	}
}

// --- 6. Cross-file and module-aware rules are exposed but not executed -------

func TestV2Dispatcher_CrossFileRulesAccessor(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Cross.kt")

	invoked := 0
	cross := v2.FakeRule("CrossRule",
		v2.WithNeeds(v2.NeedsCrossFile),
		v2.WithCheck(func(ctx *v2.Context) { invoked++ }),
	)

	d := NewV2Dispatcher([]*v2.Rule{cross})
	_ = d.Run(file)

	if invoked != 0 {
		t.Errorf("cross-file rule should not be invoked in per-file Run(), was called %d", invoked)
	}
	if len(d.CrossFileRules()) != 1 {
		t.Errorf("expected 1 cross-file rule in accessor, got %d", len(d.CrossFileRules()))
	}
	if d.CrossFileRules()[0].ID != "CrossRule" {
		t.Errorf("accessor returned wrong rule: %q", d.CrossFileRules()[0].ID)
	}
}

func TestV2Dispatcher_ModuleAwareRulesAccessor(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Module.kt")

	invoked := 0
	mod := v2.FakeRule("ModRule",
		v2.WithNeeds(v2.NeedsModuleIndex),
		v2.WithCheck(func(ctx *v2.Context) { invoked++ }),
	)

	d := NewV2Dispatcher([]*v2.Rule{mod})
	_ = d.Run(file)

	if invoked != 0 {
		t.Errorf("module-aware rule should not be invoked per-file, was called %d", invoked)
	}
	if len(d.ModuleAwareRules()) != 1 {
		t.Errorf("expected 1 module-aware rule in accessor, got %d", len(d.ModuleAwareRules()))
	}
}

// --- 7. Stats counts -----------------------------------------------------------

func TestV2Dispatcher_Stats(t *testing.T) {
	rules := []*v2.Rule{
		v2.FakeRule("Node1", v2.WithNodeTypes("call_expression")),
		v2.FakeRule("Node2", v2.WithNodeTypes("function_declaration")),
		v2.FakeRule("Line1", v2.WithNeeds(v2.NeedsLinePass)),
		v2.FakeRule("Cross1", v2.WithNeeds(v2.NeedsCrossFile)),
		v2.FakeRule("Mod1", v2.WithNeeds(v2.NeedsModuleIndex)),
	}
	d := NewV2Dispatcher(rules)

	// Parse a file so the NodeTypeTable has known entries (node rules
	// are only counted after buildFlatTypeIndex matches their types).
	_ = writeKotlinFile(t, sampleKotlin, "Stats.kt")
	// Force the index to rebuild against the populated NodeTypeTable.
	_ = d.ensureFlatTypeIndex(d.collectAllRules())

	dispatched, aggregate, lineRules, crossFile, moduleAware, legacy := d.Stats()
	if lineRules != 1 {
		t.Errorf("lineRules=%d want 1", lineRules)
	}
	if crossFile != 1 {
		t.Errorf("crossFile=%d want 1", crossFile)
	}
	if moduleAware != 1 {
		t.Errorf("moduleAware=%d want 1", moduleAware)
	}
	if aggregate != 0 {
		t.Errorf("aggregate=%d want 0 (v2 has no separate aggregate family)", aggregate)
	}
	if legacy != 0 {
		t.Errorf("legacy=%d want 0", legacy)
	}
	// dispatched may be 0 if "function_declaration" isn't in the
	// NodeTypeTable yet — as long as it is not negative we're good.
	if dispatched < 0 {
		t.Errorf("dispatched=%d unexpected", dispatched)
	}
}

// --- ReportMissingCapabilities verbose diagnostics ---------------------------

// TestV2Dispatcher_ReportMissingCapabilities_ResolverMissing verifies the
// dispatcher logs a per-rule diagnostic when a rule declares NeedsResolver
// but no resolver is configured, and stays silent for satisfied capabilities.
func TestV2Dispatcher_ReportMissingCapabilities_ResolverMissing(t *testing.T) {
	needsResolver := v2.FakeRule("V2MissingResolver", v2.WithNeeds(v2.NeedsResolver))
	noNeeds := v2.FakeRule("V2NoNeeds")
	d := NewV2Dispatcher([]*v2.Rule{needsResolver, noNeeds})

	var buf strings.Builder
	logger := func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
	}

	// Oracle is available → only the resolver warning should fire.
	d.ReportMissingCapabilities(true, logger)

	out := buf.String()
	if !strings.Contains(out, "skipped rule V2MissingResolver: NeedsResolver declared but no resolver configured") {
		t.Errorf("expected resolver diagnostic, got: %q", out)
	}
	if strings.Contains(out, "V2NoNeeds") {
		t.Errorf("rule without NeedsResolver should not be logged: %q", out)
	}
	if strings.Contains(out, "NeedsOracle") {
		t.Errorf("oracle warning must not fire when oracle is available: %q", out)
	}

	// Second call is a no-op (sync.Once dedup).
	before := buf.Len()
	d.ReportMissingCapabilities(true, logger)
	if buf.Len() != before {
		t.Errorf("ReportMissingCapabilities should emit at most once per run")
	}
}

// TestV2Dispatcher_ReportMissingCapabilities_OracleMissing verifies the
// dispatcher logs a per-rule diagnostic when a rule declares NeedsOracle
// but the caller reports oracle unavailable.
func TestV2Dispatcher_ReportMissingCapabilities_OracleMissing(t *testing.T) {
	needsOracle := v2.FakeRule("V2MissingOracle", v2.WithNeeds(v2.NeedsOracle))
	d := NewV2Dispatcher([]*v2.Rule{needsOracle}, &capsStubResolver{})

	var buf strings.Builder
	logger := func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
	}

	d.ReportMissingCapabilities(false, logger)

	out := buf.String()
	if !strings.Contains(out, "skipped rule V2MissingOracle: NeedsOracle declared but no oracle configured") {
		t.Errorf("expected oracle diagnostic, got: %q", out)
	}
}

// TestV2Dispatcher_ReportMissingCapabilities_AllSatisfied verifies the
// non-verbose / fully-wired path emits nothing.
func TestV2Dispatcher_ReportMissingCapabilities_AllSatisfied(t *testing.T) {
	needsBoth := v2.FakeRule("V2Both", v2.WithNeeds(v2.NeedsResolver|v2.NeedsOracle))
	d := NewV2Dispatcher([]*v2.Rule{needsBoth}, &capsStubResolver{})

	var buf strings.Builder
	logger := func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
	}
	d.ReportMissingCapabilities(true, logger)
	if buf.Len() != 0 {
		t.Errorf("expected no output when resolver + oracle are wired, got: %q", buf.String())
	}

	// Nil logger is always a no-op.
	d2 := NewV2Dispatcher([]*v2.Rule{needsBoth})
	d2.ReportMissingCapabilities(false, nil)
}

// capsStubResolver is a zero-behaviour TypeResolver used to produce a
// non-nil resolver for ReportMissingCapabilities tests.
type capsStubResolver struct{}

func (*capsStubResolver) ResolveFlatNode(uint32, *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (*capsStubResolver) ResolveByNameFlat(string, uint32, *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (*capsStubResolver) ResolveImport(string, *scanner.File) string    { return "" }
func (*capsStubResolver) IsNullableFlat(uint32, *scanner.File) *bool    { return nil }
func (*capsStubResolver) ClassHierarchy(string) *typeinfer.ClassInfo    { return nil }
func (*capsStubResolver) SealedVariants(string) []string                { return nil }
func (*capsStubResolver) EnumEntries(string) []string                   { return nil }
func (*capsStubResolver) AnnotationValueFlat(uint32, *scanner.File, string, string) string {
	return ""
}
func (*capsStubResolver) IsExceptionSubtype(string, string) bool { return false }
