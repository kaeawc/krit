package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// stubFlatRule is a minimal flat-dispatch rule for testing WrapAsV2.
type stubFlatRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *stubFlatRule) NodeTypes() []string { return []string{"call_expression"} }
func (r *stubFlatRule) Confidence() float64 { return 0.85 }
func (r *stubFlatRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return []scanner.Finding{{
		File: file.Path, Line: int(idx) + 1, Col: 1,
		Rule: r.RuleName, RuleSet: r.RuleSetName, Severity: r.Sev,
		Message: "stub finding",
	}}
}

func TestWrapAsV2_FlatDispatch(t *testing.T) {
	r := &stubFlatRule{BaseRule: BaseRule{
		RuleName: "StubFlat", RuleSetName: "test", Sev: "warning",
		Desc: "stub flat rule",
	}}

	v2r := WrapAsV2(r)

	if v2r.ID != "StubFlat" {
		t.Errorf("ID = %q, want StubFlat", v2r.ID)
	}
	if v2r.Category != "test" {
		t.Errorf("Category = %q, want test", v2r.Category)
	}
	if v2r.Sev != "warning" {
		t.Errorf("Sev = %q, want warning", v2r.Sev)
	}
	if len(v2r.NodeTypes) != 1 || v2r.NodeTypes[0] != "call_expression" {
		t.Errorf("NodeTypes = %v, want [call_expression]", v2r.NodeTypes)
	}
	if v2r.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", v2r.Confidence)
	}
	if v2r.Needs.Has(v2.NeedsLinePass) {
		t.Error("flat dispatch should not have NeedsLinePass")
	}
	if !v2r.Needs.IsPerFile() {
		t.Error("flat dispatch should be per-file")
	}

	// Test the Check function
	file := &scanner.File{Path: "test.kt"}
	ctx := &v2.Context{File: file, Idx: 5}
	v2r.Check(ctx)

	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
	if ctx.Findings[0].Line != 6 {
		t.Errorf("finding line = %d, want 6", ctx.Findings[0].Line)
	}
	if ctx.Findings[0].Message != "stub finding" {
		t.Errorf("finding message = %q, want 'stub finding'", ctx.Findings[0].Message)
	}
}

// stubLineRule is a minimal line rule for testing.
type stubLineRule struct {
	LineBase
	BaseRule
}

func (r *stubLineRule) Confidence() float64 { return 0.75 }
func (r *stubLineRule) CheckLines(file *scanner.File) []scanner.Finding {
	return []scanner.Finding{{
		File: file.Path, Line: 1, Col: 1,
		Rule: r.RuleName, RuleSet: r.RuleSetName, Severity: r.Sev,
		Message: "line finding",
	}}
}

func TestWrapAsV2_Line(t *testing.T) {
	r := &stubLineRule{BaseRule: BaseRule{
		RuleName: "StubLine", RuleSetName: "test", Sev: "info",
		Desc: "stub line rule",
	}}

	v2r := WrapAsV2(r)

	if !v2r.Needs.Has(v2.NeedsLinePass) {
		t.Error("line rule should have NeedsLinePass")
	}
	if !v2r.Needs.IsPerFile() {
		t.Error("line rule should be per-file")
	}
	if v2r.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", v2r.Confidence)
	}

	file := &scanner.File{Path: "test.kt"}
	ctx := &v2.Context{File: file}
	v2r.Check(ctx)

	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
	if ctx.Findings[0].Rule != "StubLine" {
		t.Errorf("finding.Rule = %q, want StubLine", ctx.Findings[0].Rule)
	}
}

// stubCrossFileRule is a minimal cross-file rule for testing.
type stubCrossFileRule struct {
	BaseRule
}

func (r *stubCrossFileRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *stubCrossFileRule) CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding {
	return []scanner.Finding{{
		File: "cross.kt", Line: 1, Col: 1,
		Rule: r.RuleName, Message: "cross finding",
	}}
}

func TestWrapAsV2_CrossFile(t *testing.T) {
	r := &stubCrossFileRule{BaseRule: BaseRule{
		RuleName: "StubCross", RuleSetName: "test", Sev: "warning",
		Desc: "stub cross-file rule",
	}}

	v2r := WrapAsV2(r)

	if !v2r.Needs.Has(v2.NeedsCrossFile) {
		t.Error("cross-file rule should have NeedsCrossFile")
	}
	if v2r.Needs.IsPerFile() {
		t.Error("cross-file rule should not be per-file")
	}

	ctx := &v2.Context{CodeIndex: &scanner.CodeIndex{}}
	v2r.Check(ctx)

	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
}

// stubLegacyRule has no specialized interface.
type stubLegacyRule struct {
	BaseRule
}

func (r *stubLegacyRule) Check(file *scanner.File) []scanner.Finding {
	return []scanner.Finding{{
		File: file.Path, Line: 1, Col: 1,
		Rule: r.RuleName, Message: "legacy finding",
	}}
}

func TestWrapAsV2_Legacy(t *testing.T) {
	r := &stubLegacyRule{BaseRule: BaseRule{
		RuleName: "StubLegacy", RuleSetName: "test", Sev: "warning",
		Desc: "stub legacy rule",
	}}

	v2r := WrapAsV2(r)

	// Legacy rules have no special capabilities
	if v2r.Needs != 0 {
		t.Errorf("legacy rule Needs = %d, want 0", v2r.Needs)
	}

	file := &scanner.File{Path: "test.kt"}
	ctx := &v2.Context{File: file}
	v2r.Check(ctx)

	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
	if ctx.Findings[0].Message != "legacy finding" {
		t.Errorf("finding message = %q, want 'legacy finding'", ctx.Findings[0].Message)
	}
}

func TestWrapAllAsV2(t *testing.T) {
	rules := []Rule{
		&stubFlatRule{BaseRule: BaseRule{RuleName: "A", RuleSetName: "t", Sev: "w", Desc: "a"}},
		&stubLineRule{BaseRule: BaseRule{RuleName: "B", RuleSetName: "t", Sev: "w", Desc: "b"}},
		&stubLegacyRule{BaseRule: BaseRule{RuleName: "C", RuleSetName: "t", Sev: "w", Desc: "c"}},
	}

	v2rules := WrapAllAsV2(rules)

	if len(v2rules) != 3 {
		t.Fatalf("expected 3 v2 rules, got %d", len(v2rules))
	}
	if v2rules[0].ID != "A" {
		t.Errorf("v2rules[0].ID = %q, want A", v2rules[0].ID)
	}
	if v2rules[1].ID != "B" {
		t.Errorf("v2rules[1].ID = %q, want B", v2rules[1].ID)
	}
	if v2rules[2].ID != "C" {
		t.Errorf("v2rules[2].ID = %q, want C", v2rules[2].ID)
	}

	// Verify different capabilities
	if v2rules[0].Needs.Has(v2.NeedsLinePass) {
		t.Error("flat rule should not have NeedsLinePass")
	}
	if !v2rules[1].Needs.Has(v2.NeedsLinePass) {
		t.Error("line rule should have NeedsLinePass")
	}
}

func TestWrapAsV2_OracleFilter(t *testing.T) {
	r := &stubFlatRuleWithOracle{BaseRule: BaseRule{
		RuleName: "WithOracle", RuleSetName: "test", Sev: "warning",
		Desc: "rule with oracle filter",
	}}

	v2r := WrapAsV2(r)

	if v2r.Oracle == nil {
		t.Fatal("expected oracle filter, got nil")
	}
	if len(v2r.Oracle.Identifiers) != 1 || v2r.Oracle.Identifiers[0] != "suspend" {
		t.Errorf("oracle identifiers = %v, want [suspend]", v2r.Oracle.Identifiers)
	}
}

type stubFlatRuleWithOracle struct {
	FlatDispatchBase
	BaseRule
}

func (r *stubFlatRuleWithOracle) NodeTypes() []string { return []string{"call_expression"} }
func (r *stubFlatRuleWithOracle) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return nil
}
func (r *stubFlatRuleWithOracle) OracleFilter() *OracleFilter {
	return &OracleFilter{Identifiers: []string{"suspend"}}
}

func TestWrapAsV2_FixLevel(t *testing.T) {
	r := &stubFlatRuleWithFixLevel{BaseRule: BaseRule{
		RuleName: "WithFixLevel", RuleSetName: "test", Sev: "warning",
		Desc: "rule with fix level",
	}}

	v2r := WrapAsV2(r)

	if v2r.Fix != v2.FixCosmetic {
		t.Errorf("Fix = %v, want FixCosmetic", v2r.Fix)
	}
}

type stubFlatRuleWithFixLevel struct {
	FlatDispatchBase
	BaseRule
}

func (r *stubFlatRuleWithFixLevel) NodeTypes() []string { return []string{"call_expression"} }
func (r *stubFlatRuleWithFixLevel) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return nil
}
func (r *stubFlatRuleWithFixLevel) IsFixable() bool     { return true }
func (r *stubFlatRuleWithFixLevel) FixLevel() FixLevel { return FixCosmetic }
