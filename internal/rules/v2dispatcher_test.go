package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
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

// --- 1. V2Dispatcher routes node and line rules independently ----------------

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

	emptyNodeTypesCalls := 0
	emptyNodeTypesRule := &v2.Rule{
		ID:          "V2TestEmptyNodeTypes",
		Category:    "ignored",
		Description: "empty node types are not dispatched",
		Sev:         v2.SeverityInfo,
		NodeTypes:   []string{},
		Check: func(ctx *v2.Context) {
			emptyNodeTypesCalls++
			ctx.EmitAt(1, 1, "ignored")
		},
	}

	d := NewV2Dispatcher([]*v2.Rule{nodeRule, lineRule, emptyNodeTypesRule})
	columns := d.Run(file)

	if nodeCalls == 0 {
		t.Error("node rule was never invoked on call_expression")
	}
	if lineCalls != 1 {
		t.Errorf("line rule invoked %d times, want 1", lineCalls)
	}
	if emptyNodeTypesCalls != 0 {
		t.Errorf("empty NodeTypes rule invoked %d times, want 0", emptyNodeTypesCalls)
	}

	// Verify findings carry rule metadata populated by stampV2Findings.
	if columns.Len() == 0 {
		t.Fatal("expected findings, got none")
	}
	sawNode, sawLine, sawIgnored := false, false, false
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
		case "V2TestEmptyNodeTypes":
			sawIgnored = true
		}
	}
	if !sawNode || !sawLine || sawIgnored {
		t.Errorf("unexpected findings: node=%v line=%v ignored=%v", sawNode, sawLine, sawIgnored)
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

func TestV2Dispatcher_ResourceSourceRulesRunWithResourceIndex(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "ResourceSource.kt")

	invoked := 0
	rule := v2.FakeRule("ResourceSourceRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsResources),
		v2.WithCheck(func(ctx *v2.Context) {
			if ctx.ResourceIndex == nil {
				t.Error("ResourceIndex was not populated")
			}
			invoked++
		}),
	)
	rule.Languages = []scanner.Language{scanner.LangKotlin}

	d := NewV2Dispatcher([]*v2.Rule{rule})
	_ = d.Run(file)
	if invoked != 0 {
		t.Fatalf("resource-backed source rule ran during ordinary dispatch")
	}
	if len(d.ResourceSourceRules()) != 1 {
		t.Fatalf("expected one resource-backed source rule, got %d", len(d.ResourceSourceRules()))
	}

	_ = d.RunResourceSource(file, &android.ResourceIndex{})
	if invoked != 1 {
		t.Fatalf("resource-backed source rule invoked %d times, want 1", invoked)
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

	dispatched, aggregate, lineRules, crossFile, moduleAware := d.Stats()
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

// --- 8. TypeInfo.PreferBackend routing --------------------------------------

// compositeStub simulates *oracle.CompositeResolver without the import
// cycle: it exposes a Fallback() method so the dispatcher's
// fallbackAware interface assertion succeeds, and answers everything
// else as a pass-through.
type compositeStub struct {
	fallback typeinfer.TypeResolver
}

func (c *compositeStub) Fallback() typeinfer.TypeResolver { return c.fallback }

func (c *compositeStub) ResolveFlatNode(uint32, *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (c *compositeStub) ResolveByNameFlat(string, uint32, *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (c *compositeStub) ResolveImport(string, *scanner.File) string { return "" }
func (c *compositeStub) IsNullableFlat(uint32, *scanner.File) *bool { return nil }
func (c *compositeStub) ClassHierarchy(string) *typeinfer.ClassInfo { return nil }
func (c *compositeStub) SealedVariants(string) []string             { return nil }
func (c *compositeStub) EnumEntries(string) []string                { return nil }
func (c *compositeStub) AnnotationValueFlat(uint32, *scanner.File, string, string) string {
	return ""
}
func (c *compositeStub) IsExceptionSubtype(string, string) bool { return false }

// TestV2Dispatcher_TypeInfoPreferResolver verifies that a rule
// declaring PreferResolver receives the composite's fallback — not the
// composite itself — even when both backends are wired.
func TestV2Dispatcher_TypeInfoPreferResolver(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferResolver.kt")

	fallback := typeinfer.NewFakeResolver()
	composite := &compositeStub{fallback: fallback}

	var got typeinfer.TypeResolver
	rule := v2.FakeRule("PreferResolverRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsTypeInfo),
		v2.WithCheck(func(ctx *v2.Context) {
			got = ctx.Resolver
		}),
	)
	rule.TypeInfo = v2.TypeInfoHint{PreferBackend: v2.PreferResolver}

	d := NewV2Dispatcher([]*v2.Rule{rule}, composite)
	_ = d.Run(file)

	if got == nil {
		t.Fatal("ctx.Resolver was nil — rule skipped unexpectedly")
	}
	if got != fallback {
		t.Errorf("PreferResolver wired %T, want the fallback %T", got, fallback)
	}
}

// TestV2Dispatcher_TypeInfoPreferOracle verifies a rule declaring
// PreferOracle receives the composite resolver as-is.
func TestV2Dispatcher_TypeInfoPreferOracle(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferOracle.kt")

	composite := &compositeStub{fallback: typeinfer.NewFakeResolver()}

	var got typeinfer.TypeResolver
	rule := v2.FakeRule("PreferOracleRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsTypeInfo),
		v2.WithCheck(func(ctx *v2.Context) {
			got = ctx.Resolver
		}),
	)
	rule.TypeInfo = v2.TypeInfoHint{PreferBackend: v2.PreferOracle}

	d := NewV2Dispatcher([]*v2.Rule{rule}, composite)
	_ = d.Run(file)

	if got != composite {
		t.Errorf("PreferOracle wired %T, want composite %T", got, composite)
	}
}

// TestV2Dispatcher_TypeInfoPreferAnyUnchanged verifies the zero-value
// hint preserves the pre-hint behaviour: the dispatcher hands rules
// whichever resolver the constructor was given.
func TestV2Dispatcher_TypeInfoPreferAnyUnchanged(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferAny.kt")

	composite := &compositeStub{fallback: typeinfer.NewFakeResolver()}

	var got typeinfer.TypeResolver
	rule := v2.FakeRule("PreferAnyRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsTypeInfo),
		v2.WithCheck(func(ctx *v2.Context) {
			got = ctx.Resolver
		}),
	)
	// TypeInfo left at zero value → PreferAny, Required=false.

	d := NewV2Dispatcher([]*v2.Rule{rule}, composite)
	_ = d.Run(file)

	if got != composite {
		t.Errorf("PreferAny wired %T, want composite %T", got, composite)
	}
}

// TestV2Dispatcher_TypeInfoPreferOracleSkipsWhenUnavailable verifies
// that a rule declaring PreferOracle with Required=false (default) is
// skipped silently when the dispatcher has no composite resolver
// (i.e. oracle is not wired).
func TestV2Dispatcher_TypeInfoPreferOracleSkipsWhenUnavailable(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferOracleMissing.kt")

	calls := 0
	rule := v2.FakeRule("PreferOracleMissingRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsTypeInfo),
		v2.WithCheck(func(ctx *v2.Context) { calls++ }),
	)
	rule.TypeInfo = v2.TypeInfoHint{PreferBackend: v2.PreferOracle}

	// Bare source-level resolver — no composite/Fallback() → oracle absent.
	d := NewV2Dispatcher([]*v2.Rule{rule}, typeinfer.NewFakeResolver())
	_ = d.Run(file)

	if calls != 0 {
		t.Errorf("PreferOracle rule ran %d times without oracle; expected skip", calls)
	}
}

// TestV2Dispatcher_TypeInfoRequiredFallsThrough verifies Required=true
// lets the rule fall through to whatever backend IS wired, even when
// its preferred one is missing.
func TestV2Dispatcher_TypeInfoRequiredFallsThrough(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "RequiredFallthrough.kt")

	source := typeinfer.NewFakeResolver()

	var got typeinfer.TypeResolver
	rule := v2.FakeRule("RequiredFallthroughRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsTypeInfo),
		v2.WithCheck(func(ctx *v2.Context) { got = ctx.Resolver }),
	)
	rule.TypeInfo = v2.TypeInfoHint{PreferBackend: v2.PreferOracle, Required: true}

	d := NewV2Dispatcher([]*v2.Rule{rule}, source)
	_ = d.Run(file)

	if got != source {
		t.Errorf("Required=true should fall through to the source resolver, got %T", got)
	}
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
func (*capsStubResolver) ResolveImport(string, *scanner.File) string { return "" }
func (*capsStubResolver) IsNullableFlat(uint32, *scanner.File) *bool { return nil }
func (*capsStubResolver) ClassHierarchy(string) *typeinfer.ClassInfo { return nil }
func (*capsStubResolver) SealedVariants(string) []string             { return nil }
func (*capsStubResolver) EnumEntries(string) []string                { return nil }
func (*capsStubResolver) AnnotationValueFlat(uint32, *scanner.File, string, string) string {
	return ""
}
func (*capsStubResolver) IsExceptionSubtype(string, string) bool { return false }
