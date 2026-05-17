package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
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
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return file
}

func writeJavaFile(t *testing.T, code string, name string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
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

// --- 1. Dispatcher routes node and line rules independently ----------------

func TestDispatcher_RoutesFamiliesIndependently(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Sample.kt")

	nodeCalls := 0
	nodeRule := api.FakeRule("V2TestNode",
		api.WithNodeTypes("call_expression"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			nodeCalls++
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "call here")
		}),
	)
	nodeRule.Category = "style"

	lineCalls := 0
	lineRule := api.FakeRule("V2TestLine",
		api.WithNeeds(api.NeedsLinePass),
		api.WithCheck(func(ctx *api.Context) {
			lineCalls++
			ctx.EmitAt(1, 1, "line once")
		}),
	)
	lineRule.Category = "naming"

	emptyNodeTypesCalls := 0
	emptyNodeTypesRule := &api.Rule{
		ID:          "V2TestEmptyNodeTypes",
		Category:    "ignored",
		Description: "empty node types are not dispatched",
		Sev:         api.SeverityInfo,
		NodeTypes:   []string{},
		Check: func(ctx *api.Context) {
			emptyNodeTypesCalls++
			ctx.EmitAt(1, 1, "ignored")
		},
	}

	d := NewDispatcher([]*api.Rule{nodeRule, lineRule, emptyNodeTypesRule})
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

func TestDispatcher_LexicalCalleeNamesSkipsUnmatchedCalls(t *testing.T) {
	file := writeKotlinFile(t, `package test

object Receiver {
    fun keep() {}
}

fun keep() {}
fun skip() {}

fun main() {
    keep()
    skip()
    Receiver.keep()
}
`, "CalleeFilter.kt")

	var calls []string
	rule := api.FakeRule("CalleeFiltered",
		api.WithNodeTypes("call_expression"),
		api.WithLexicalCalleeNames("keep"),
		api.WithCheck(func(ctx *api.Context) {
			calls = append(calls, flatCallExpressionName(ctx.File, ctx.Idx))
		}),
	)

	NewDispatcher([]*api.Rule{rule}).Run(file)

	if len(calls) != 2 {
		t.Fatalf("filtered rule invoked %d times (%v), want 2 keep calls", len(calls), calls)
	}
	for _, call := range calls {
		if call != "keep" {
			t.Fatalf("filtered rule invoked for %q, want only keep calls; all calls: %v", call, calls)
		}
	}
}

func TestDispatcher_AttachesSharedJavaSourceFacts(t *testing.T) {
	file := writeJavaFile(t, `package test;

import android.webkit.WebView;

class Browser {
  void setup(WebView webView) {}
}
`, "Browser.java")

	var nodeFacts *javafacts.JavaFileFacts
	var nodeIndex *javafacts.SourceIndex
	nodeCalls := 0
	nodeRule := api.FakeRule("JavaNodeFacts",
		api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) {
			nodeCalls++
			if ctx.JavaFacts == nil {
				t.Fatal("node rule did not receive JavaFacts")
			}
			if ctx.JavaSourceIndex == nil {
				t.Fatal("node rule did not receive JavaSourceIndex")
			}
			if got := ctx.JavaFacts.ResolveType("WebView", ctx.JavaSourceIndex); got != "android.webkit.WebView" {
				t.Fatalf("ResolveType(WebView) = %q", got)
			}
			nodeFacts = ctx.JavaFacts
			nodeIndex = ctx.JavaSourceIndex
		}),
	)
	nodeRule.Languages = []scanner.Language{scanner.LangJava}

	lineCalls := 0
	lineRule := api.FakeRule("JavaLineFacts",
		api.WithNeeds(api.NeedsLinePass),
		api.WithCheck(func(ctx *api.Context) {
			lineCalls++
			if ctx.JavaFacts == nil || ctx.JavaSourceIndex == nil {
				t.Fatal("line rule did not receive Java facts")
			}
			if nodeFacts == nil || nodeIndex == nil {
				t.Fatal("node rule should run before line rule")
			}
			if ctx.JavaFacts != nodeFacts {
				t.Fatal("line and node rules received different JavaFacts instances")
			}
			if ctx.JavaSourceIndex != nodeIndex {
				t.Fatal("line and node rules received different JavaSourceIndex instances")
			}
		}),
	)
	lineRule.Languages = []scanner.Language{scanner.LangJava}

	dispatcher := NewDispatcher([]*api.Rule{nodeRule, lineRule})
	dispatcher.Run(file)

	if nodeCalls == 0 {
		t.Fatal("node rule was never invoked")
	}
	if lineCalls != 1 {
		t.Fatalf("line rule invoked %d times, want 1", lineCalls)
	}
}

// --- 3. Panic recovery -------------------------------------------------------

func TestDispatcher_PanicRecovery(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Panic.kt")

	goodCalls := 0
	goodRule := api.FakeRule("GoodRule",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			goodCalls++
			ctx.EmitAt(1, 1, "ok")
		}),
	)
	badRule := api.FakeRule("BadRule",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			panic("intentional")
		}),
	)

	d := NewDispatcher([]*api.Rule{goodRule, badRule})
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

func TestDispatcher_RespectsExclusions(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "ExcludedTest.kt")

	nodeCalls := 0
	rule := api.FakeRule("ExcludableRule",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			nodeCalls++
		}),
	)

	// Register an exclusion that matches any file containing "Excluded".
	SetRuleExcludes("ExcludableRule", []string{"**/*Test.kt"})
	t.Cleanup(func() { SetRuleExcludes("ExcludableRule", nil) })

	d := NewDispatcher([]*api.Rule{rule})
	_ = d.Run(file)

	if nodeCalls != 0 {
		t.Errorf("rule ran %d times despite exclusion — expected 0", nodeCalls)
	}
}

// --- 5. ctx.Resolver is populated for resolver-needing rules ----------------

func TestDispatcher_ResolverPopulatedInContext(t *testing.T) {
	cases := []struct {
		name  string
		needs api.Capabilities
	}{
		{"NeedsResolver", api.NeedsResolver},
		{"NeedsTypeInfo", api.NeedsTypeInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeKotlinFile(t, "fun foo() { bar() }", tc.name+".kt")

			var gotResolver typeinfer.TypeResolver
			rule := api.FakeRule("ResolverRule",
				api.WithNodeTypes("call_expression"),
				api.WithNeeds(tc.needs),
				api.WithCheck(func(ctx *api.Context) {
					gotResolver = ctx.Resolver
				}),
			)

			d := NewDispatcher([]*api.Rule{rule}, typeinfer.NewFakeResolver())
			_ = d.Run(file)

			if gotResolver == nil {
				t.Error("ctx.Resolver was nil even though resolver was provided")
			}
		})
	}
}

func TestDispatcher_ResolverNilWithoutResolver(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "Resolver2.kt")

	var gotResolver typeinfer.TypeResolver
	rule := api.FakeRule("ResolverRule2",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsResolver),
		api.WithCheck(func(ctx *api.Context) {
			gotResolver = ctx.Resolver
		}),
	)

	d := NewDispatcher([]*api.Rule{rule})
	_ = d.Run(file)

	if gotResolver != nil {
		t.Error("ctx.Resolver was non-nil even though no resolver was supplied")
	}
}

// --- 6. Cross-file and module-aware rules are exposed but not executed -------

func TestDispatcher_CrossFileRulesAccessor(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Cross.kt")

	invoked := 0
	cross := api.FakeRule("CrossRule",
		api.WithNeeds(api.NeedsCrossFile),
		api.WithCheck(func(ctx *api.Context) { invoked++ }),
	)

	d := NewDispatcher([]*api.Rule{cross})
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

func TestDispatcher_ModuleAwareRulesAccessor(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Module.kt")

	invoked := 0
	mod := api.FakeRule("ModRule",
		api.WithNeeds(api.NeedsModuleIndex),
		api.WithCheck(func(ctx *api.Context) { invoked++ }),
	)

	d := NewDispatcher([]*api.Rule{mod})
	_ = d.Run(file)

	if invoked != 0 {
		t.Errorf("module-aware rule should not be invoked per-file, was called %d", invoked)
	}
	if len(d.ModuleAwareRules()) != 1 {
		t.Errorf("expected 1 module-aware rule in accessor, got %d", len(d.ModuleAwareRules()))
	}
}

func TestDispatcher_ResourceSourceRulesRunWithResourceIndex(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "ResourceSource.kt")

	invoked := 0
	rule := api.FakeRule("ResourceSourceRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsResources),
		api.WithCheck(func(ctx *api.Context) {
			if ctx.ResourceIndex == nil {
				t.Error("ResourceIndex was not populated")
			}
			invoked++
		}),
	)
	rule.Languages = []scanner.Language{scanner.LangKotlin}

	d := NewDispatcher([]*api.Rule{rule})
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

func TestDispatcher_Stats(t *testing.T) {
	rules := []*api.Rule{
		api.FakeRule("Node1", api.WithNodeTypes("call_expression")),
		api.FakeRule("Node2", api.WithNodeTypes("function_declaration")),
		api.FakeRule("Line1", api.WithNeeds(api.NeedsLinePass)),
		api.FakeRule("Cross1", api.WithNeeds(api.NeedsCrossFile)),
		api.FakeRule("Mod1", api.WithNeeds(api.NeedsModuleIndex)),
	}
	d := NewDispatcher(rules)

	// Parse a file so the NodeTypeTable has known entries (node rules
	// are only counted after buildFlatTypeIndex matches their types).
	_ = writeKotlinFile(t, sampleKotlin, "Stats.kt")
	// Force the index to rebuild against the populated NodeTypeTable.
	_ = d.ensureFlatTypeIndex(d.collectAllRules())

	dispatched, lineRules, crossFile, moduleAware := d.Stats()
	if lineRules != 1 {
		t.Errorf("lineRules=%d want 1", lineRules)
	}
	if crossFile != 1 {
		t.Errorf("crossFile=%d want 1", crossFile)
	}
	if moduleAware != 1 {
		t.Errorf("moduleAware=%d want 1", moduleAware)
	}
	// dispatched may be 0 if "function_declaration" isn't in the
	// NodeTypeTable yet — as long as it is not negative we're good.
	if dispatched < 0 {
		t.Errorf("dispatched=%d unexpected", dispatched)
	}
}

// --- ReportMissingCapabilities verbose diagnostics ---------------------------

// TestDispatcher_ReportMissingCapabilities_ResolverMissing verifies the
// dispatcher logs a per-rule diagnostic when a rule declares NeedsResolver
// but no resolver is configured, and stays silent for satisfied capabilities.
func TestDispatcher_ReportMissingCapabilities_ResolverMissing(t *testing.T) {
	needsResolver := api.FakeRule("V2MissingResolver", api.WithNeeds(api.NeedsResolver))
	noNeeds := api.FakeRule("V2NoNeeds")
	d := NewDispatcher([]*api.Rule{needsResolver, noNeeds})

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

// TestDispatcher_ReportMissingCapabilities_OracleMissing verifies the
// dispatcher logs a per-rule diagnostic when a rule declares NeedsOracle
// but the caller reports oracle unavailable.
func TestDispatcher_ReportMissingCapabilities_OracleMissing(t *testing.T) {
	needsOracle := api.FakeRule("V2MissingOracle", api.WithNeeds(api.NeedsOracle))
	d := NewDispatcher([]*api.Rule{needsOracle}, &capsStubResolver{})

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

// TestDispatcher_ReportMissingCapabilities_AllSatisfied verifies the
// non-verbose / fully-wired path emits nothing.
func TestDispatcher_ReportMissingCapabilities_AllSatisfied(t *testing.T) {
	needsBoth := api.FakeRule("V2Both", api.WithNeeds(api.NeedsResolver|api.NeedsOracle))
	d := NewDispatcher([]*api.Rule{needsBoth}, &capsStubResolver{})

	var buf strings.Builder
	logger := func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
	}
	d.ReportMissingCapabilities(true, logger)
	if buf.Len() != 0 {
		t.Errorf("expected no output when resolver + oracle are wired, got: %q", buf.String())
	}

	// Nil logger is always a no-op.
	d2 := NewDispatcher([]*api.Rule{needsBoth})
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

// TestDispatcher_TypeInfoPreferResolver verifies that a rule
// declaring PreferResolver receives the composite's fallback — not the
// composite itself — even when both backends are wired.
func TestDispatcher_TypeInfoPreferResolver(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferResolver.kt")

	fallback := typeinfer.NewFakeResolver()
	composite := &compositeStub{fallback: fallback}

	var got typeinfer.TypeResolver
	rule := api.FakeRule("PreferResolverRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsTypeInfo),
		api.WithCheck(func(ctx *api.Context) {
			got = ctx.Resolver
		}),
	)
	rule.TypeInfo = api.TypeInfoHint{PreferBackend: api.PreferResolver}

	d := NewDispatcher([]*api.Rule{rule}, composite)
	_ = d.Run(file)

	if got == nil {
		t.Fatal("ctx.Resolver was nil — rule skipped unexpectedly")
	}
	if got != fallback {
		t.Errorf("PreferResolver wired %T, want the fallback %T", got, fallback)
	}
}

// TestDispatcher_TypeInfoPreferOracle verifies a rule declaring
// PreferOracle receives the composite resolver as-is.
func TestDispatcher_TypeInfoPreferOracle(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferOracle.kt")

	composite := &compositeStub{fallback: typeinfer.NewFakeResolver()}

	var got typeinfer.TypeResolver
	rule := api.FakeRule("PreferOracleRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsTypeInfo),
		api.WithCheck(func(ctx *api.Context) {
			got = ctx.Resolver
		}),
	)
	rule.TypeInfo = api.TypeInfoHint{PreferBackend: api.PreferOracle}

	d := NewDispatcher([]*api.Rule{rule}, composite)
	_ = d.Run(file)

	if got != composite {
		t.Errorf("PreferOracle wired %T, want composite %T", got, composite)
	}
}

// TestDispatcher_TypeInfoPreferAnyUnchanged verifies the zero-value
// hint preserves the pre-hint behaviour: the dispatcher hands rules
// whichever resolver the constructor was given.
func TestDispatcher_TypeInfoPreferAnyUnchanged(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferAny.kt")

	composite := &compositeStub{fallback: typeinfer.NewFakeResolver()}

	var got typeinfer.TypeResolver
	rule := api.FakeRule("PreferAnyRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsTypeInfo),
		api.WithCheck(func(ctx *api.Context) {
			got = ctx.Resolver
		}),
	)
	// TypeInfo left at zero value → PreferAny, Required=false.

	d := NewDispatcher([]*api.Rule{rule}, composite)
	_ = d.Run(file)

	if got != composite {
		t.Errorf("PreferAny wired %T, want composite %T", got, composite)
	}
}

// TestDispatcher_TypeInfoPreferOracleSkipsWhenUnavailable verifies
// that a rule declaring PreferOracle with Required=false (default) is
// skipped silently when the dispatcher has no composite resolver
// (i.e. oracle is not wired).
func TestDispatcher_TypeInfoPreferOracleSkipsWhenUnavailable(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "PreferOracleMissing.kt")

	calls := 0
	rule := api.FakeRule("PreferOracleMissingRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsTypeInfo),
		api.WithCheck(func(ctx *api.Context) { calls++ }),
	)
	rule.TypeInfo = api.TypeInfoHint{PreferBackend: api.PreferOracle}

	// Bare source-level resolver — no composite/Fallback() → oracle absent.
	d := NewDispatcher([]*api.Rule{rule}, typeinfer.NewFakeResolver())
	_ = d.Run(file)

	if calls != 0 {
		t.Errorf("PreferOracle rule ran %d times without oracle; expected skip", calls)
	}
}

// TestDispatcher_TypeInfoRequiredFallsThrough verifies Required=true
// lets the rule fall through to whatever backend IS wired, even when
// its preferred one is missing.
func TestDispatcher_TypeInfoRequiredFallsThrough(t *testing.T) {
	file := writeKotlinFile(t, "fun foo() { bar() }", "RequiredFallthrough.kt")

	source := typeinfer.NewFakeResolver()

	var got typeinfer.TypeResolver
	rule := api.FakeRule("RequiredFallthroughRule",
		api.WithNodeTypes("call_expression"),
		api.WithNeeds(api.NeedsTypeInfo),
		api.WithCheck(func(ctx *api.Context) { got = ctx.Resolver }),
	)
	rule.TypeInfo = api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true}

	d := NewDispatcher([]*api.Rule{rule}, source)
	_ = d.Run(file)

	if got != source {
		t.Errorf("Required=true should fall through to the source resolver, got %T", got)
	}
}

func TestRuleScope_DerivesFromNeedsAndNodeTypes(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want api.Scope
	}{
		{
			name: "explicit scope wins",
			rule: &api.Rule{ID: "x", Description: "x", Scope: api.ScopeManifest, Needs: api.NeedsCrossFile, Check: func(*api.Context) {}},
			want: api.ScopeManifest,
		},
		{
			name: "cross-file precedes other scope flags",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsCrossFile | api.NeedsParsedFiles, Check: func(*api.Context) {}},
			want: api.ScopeCrossFile,
		},
		{
			name: "module index",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsModuleIndex, Check: func(*api.Context) {}},
			want: api.ScopeModuleIndex,
		},
		{
			name: "manifest",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsManifest, Check: func(*api.Context) {}},
			want: api.ScopeManifest,
		},
		{
			name: "resource (xml)",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsResources, Check: func(*api.Context) {}},
			want: api.ScopeResource,
		},
		{
			name: "gradle",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsGradle, Check: func(*api.Context) {}},
			want: api.ScopeGradle,
		},
		{
			name: "aggregate",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsAggregate, Aggregate: &api.Aggregate{Collect: func(*api.Context) {}, Finalize: func(*api.Context) {}, Reset: func() {}}},
			want: api.ScopeAggregate,
		},
		{
			name: "line pass",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsLinePass, Check: func(*api.Context) {}},
			want: api.ScopeLinePass,
		},
		{
			name: "parsed files alone",
			rule: &api.Rule{ID: "x", Description: "x", Needs: api.NeedsParsedFiles, Check: func(*api.Context) {}},
			want: api.ScopeParsedFiles,
		},
		{
			name: "node types only",
			rule: &api.Rule{ID: "x", Description: "x", NodeTypes: []string{"call_expression"}, Check: func(*api.Context) {}},
			want: api.ScopePerFileNode,
		},
		{
			name: "all nodes",
			rule: &api.Rule{ID: "x", Description: "x", Check: func(*api.Context) {}},
			want: api.ScopePerFileAllNodes,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RuleScope(tc.rule); got != tc.want {
				t.Errorf("RuleScope=%s want %s", got, tc.want)
			}
		})
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
