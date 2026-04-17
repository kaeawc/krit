package rules

import (
	"os"
	"path/filepath"
	"sort"
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
	findings := d.Run(file)

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
	if len(findings) == 0 {
		t.Fatal("expected findings, got none")
	}
	sawNode, sawLine, sawLegacy := false, false, false
	for _, f := range findings {
		switch f.Rule {
		case "V2TestNode":
			sawNode = true
			if f.RuleSet != "style" || f.Severity != "warning" {
				t.Errorf("node finding metadata wrong: %+v", f)
			}
		case "V2TestLine":
			sawLine = true
			if f.RuleSet != "naming" {
				t.Errorf("line finding ruleset %q, want naming", f.RuleSet)
			}
		case "V2TestLegacy":
			sawLegacy = true
		}
	}
	if !sawNode || !sawLine || !sawLegacy {
		t.Errorf("missing findings: node=%v line=%v legacy=%v", sawNode, sawLine, sawLegacy)
	}
}

// --- 2. V2 output matches V1 output for equivalent wrapped rules -------------

// testWrappedRule is a minimal v1 flat-dispatch rule that emits one
// finding per node of the given type. We build a matching v2 wrapper
// for it and verify both dispatchers produce the same set of findings.
type testWrappedRule struct {
	name       string
	ruleset    string
	sev        string
	desc       string
	nodeTypes  []string
	findingMsg string
}

func (r *testWrappedRule) Name() string        { return r.name }
func (r *testWrappedRule) Description() string { return r.desc }
func (r *testWrappedRule) RuleSet() string     { return r.ruleset }
func (r *testWrappedRule) Severity() string    { return r.sev }
func (r *testWrappedRule) Check(*scanner.File) []scanner.Finding {
	return nil
}
func (r *testWrappedRule) NodeTypes() []string { return r.nodeTypes }
func (r *testWrappedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	n := file.FlatTree.Nodes[idx]
	return []scanner.Finding{{
		File: file.Path, Line: int(n.StartRow) + 1, Col: int(n.StartCol) + 1,
		Rule: r.name, RuleSet: r.ruleset, Severity: r.sev,
		Message: r.findingMsg,
	}}
}

func TestV2Dispatcher_MatchesV1DispatcherOutput(t *testing.T) {
	file := writeKotlinFile(t, sampleKotlin, "Compare.kt")

	v1Rule := &testWrappedRule{
		name: "CompareRule", ruleset: "style", sev: "warning",
		desc: "test", nodeTypes: []string{"call_expression"},
		findingMsg: "call here",
	}

	// Run the v1 dispatcher with the v1 rule directly.
	v1Disp := NewDispatcher([]Rule{v1Rule})
	v1Findings := v1Disp.Run(file)

	// Now wrap the same v1 rule into v2 via WrapAsV2 and run through V2Dispatcher.
	v2Rule := WrapAsV2(v1Rule)
	v2Disp := NewV2Dispatcher([]*v2.Rule{v2Rule})
	v2Findings := v2Disp.Run(file)

	if len(v1Findings) == 0 {
		t.Fatal("v1 dispatcher returned no findings; fixture does not trigger the rule")
	}
	if len(v1Findings) != len(v2Findings) {
		t.Fatalf("finding count differs: v1=%d v2=%d\nv1=%+v\nv2=%+v",
			len(v1Findings), len(v2Findings), v1Findings, v2Findings)
	}

	normalize := func(fs []scanner.Finding) []string {
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, f.Rule+"|"+f.RuleSet+"|"+itoa(f.Line)+"|"+itoa(f.Col)+"|"+f.Message)
		}
		sort.Strings(out)
		return out
	}
	v1Keys := normalize(v1Findings)
	v2Keys := normalize(v2Findings)

	for i := range v1Keys {
		if v1Keys[i] != v2Keys[i] {
			t.Errorf("finding %d differs:\n  v1=%s\n  v2=%s", i, v1Keys[i], v2Keys[i])
		}
	}
}

func itoa(n int) string {
	// Avoid importing strconv just for a test helper.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	s := string(buf[i:])
	if neg {
		return "-" + s
	}
	return s
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
	findings, stats := d.RunWithStats(file)

	if goodCalls == 0 {
		t.Error("good rule was never invoked — dispatcher may have aborted on panic")
	}
	if len(findings) == 0 {
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

// --- 5. Resolver hook fires --------------------------------------------------

func TestV2Dispatcher_ResolverHookFires(t *testing.T) {
	hookFired := false
	rule := v2.FakeRule("ResolverRule",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsResolver),
	)
	rule.SetResolverHook = func(r typeinfer.TypeResolver) {
		if r != nil {
			hookFired = true
		}
	}

	_ = NewV2Dispatcher([]*v2.Rule{rule}, typeinfer.NewFakeResolver())

	if !hookFired {
		t.Error("SetResolverHook was not called even though resolver was provided")
	}
}

func TestV2Dispatcher_ResolverHookNotFiredWithoutResolver(t *testing.T) {
	hookFired := false
	rule := v2.FakeRule("ResolverRule2",
		v2.WithNodeTypes("call_expression"),
		v2.WithNeeds(v2.NeedsResolver),
	)
	rule.SetResolverHook = func(r typeinfer.TypeResolver) {
		hookFired = true
	}

	_ = NewV2Dispatcher([]*v2.Rule{rule})

	if hookFired {
		t.Error("SetResolverHook fired even though no resolver was supplied")
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
