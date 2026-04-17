package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type testCrossFileRule struct {
	BaseRule
	checkCalls int
}

func (r *testCrossFileRule) Check(_ *scanner.File) []scanner.Finding {
	r.checkCalls++
	return nil
}

func (r *testCrossFileRule) CheckCrossFile(_ *scanner.CodeIndex) []scanner.Finding { return nil }

type testModuleAwareRule struct {
	BaseRule
	checkCalls int
}

func (r *testModuleAwareRule) Check(_ *scanner.File) []scanner.Finding {
	r.checkCalls++
	return nil
}

func (r *testModuleAwareRule) SetModuleIndex(_ *module.PerModuleIndex) {}
func (r *testModuleAwareRule) CheckModuleAware() []scanner.Finding     { return nil }

type testFlatDispatchRule struct{ BaseRule }

func (r *testFlatDispatchRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *testFlatDispatchRule) NodeTypes() []string                     { return []string{"integer_literal"} }
func (r *testFlatDispatchRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "flat finding")}
}

// testMediumConfidenceDispatchRule declares a base Confidence below the
// dispatch default (0.95) so the test can confirm the
// confidence provider override is applied.
type testMediumConfidenceDispatchRule struct{ BaseRule }

func (r *testMediumConfidenceDispatchRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *testMediumConfidenceDispatchRule) NodeTypes() []string                     { return []string{"integer_literal"} }
func (r *testMediumConfidenceDispatchRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "medium finding")}
}
func (r *testMediumConfidenceDispatchRule) Confidence() float64 { return 0.60 }

// testExplicitConfidenceDispatchRule sets Confidence on the finding
// itself so the test can confirm a per-finding override beats the
// rule-level default.
type testExplicitConfidenceDispatchRule struct{ BaseRule }

func (r *testExplicitConfidenceDispatchRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *testExplicitConfidenceDispatchRule) NodeTypes() []string                     { return []string{"integer_literal"} }
func (r *testExplicitConfidenceDispatchRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "explicit finding")
	f.Confidence = 0.42
	return []scanner.Finding{f}
}
func (r *testExplicitConfidenceDispatchRule) Confidence() float64 { return 0.60 }

type testFixingDispatchRule struct{ BaseRule }

func (r *testFixingDispatchRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *testFixingDispatchRule) NodeTypes() []string                     { return []string{"integer_literal"} }
func (r *testFixingDispatchRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return []scanner.Finding{{
		File:     file.Path,
		Line:     file.FlatRow(idx) + 1,
		Col:      file.FlatCol(idx) + 1,
		RuleSet:  r.RuleSet(),
		Rule:     r.Name(),
		Severity: r.Severity(),
		Message:  "fix preserved",
		Fix: &scanner.Fix{
			StartLine:   file.FlatRow(idx) + 1,
			EndLine:     file.FlatRow(idx) + 1,
			Replacement: "0",
		},
	}}
}

func TestDispatcher_DoesNotTreatCrossFileOrModuleAwareAsLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	cross := &testCrossFileRule{BaseRule: BaseRule{RuleName: "Cross", RuleSetName: "test", Sev: "warning"}}
	moduleAware := &testModuleAwareRule{BaseRule: BaseRule{RuleName: "Module", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{cross, moduleAware})

	dispatcher.Run(file)

	if cross.checkCalls != 0 {
		t.Fatalf("expected cross-file rule Check to be skipped, got %d calls", cross.checkCalls)
	}
	if moduleAware.checkCalls != 0 {
		t.Fatalf("expected module-aware rule Check to be skipped, got %d calls", moduleAware.checkCalls)
	}

	dispatchCount, aggregateCount, lineCount, crossCount, moduleCount, legacyCount := dispatcher.Stats()
	if dispatchCount != 0 || aggregateCount != 0 || lineCount != 0 {
		t.Fatalf("expected no dispatch/aggregate/line rules, got dispatch=%d aggregate=%d line=%d", dispatchCount, aggregateCount, lineCount)
	}
	if crossCount != 1 {
		t.Fatalf("expected 1 cross-file rule, got %d", crossCount)
	}
	if moduleCount != 1 {
		t.Fatalf("expected 1 module-aware rule, got %d", moduleCount)
	}
	if legacyCount != 0 {
		t.Fatalf("expected 0 legacy rules, got %d", legacyCount)
	}
}

func TestDispatcher_RunWithStats_TracksRuleBuckets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	dispatcher := NewDispatcher([]Rule{&testFlatDispatchRule{BaseRule: BaseRule{RuleName: "Dispatch", RuleSetName: "test", Sev: "warning"}}})
	findings, stats := dispatcher.RunWithStats(file)
	if len(findings) == 0 {
		t.Fatal("expected findings from magic number rule")
	}
	if stats.SuppressionIndexMs < 0 || stats.DispatchWalkMs < 0 || stats.AggregateFinalizeMs < 0 ||
		stats.DispatchRuleNs < 0 || stats.AggregateCollectNs < 0 ||
		stats.LineRuleMs < 0 || stats.LegacyRuleMs < 0 || stats.SuppressionFilterMs < 0 {
		t.Fatalf("expected non-negative stats, got %+v", stats)
	}
	if len(stats.DispatchRuleNsByRule) == 0 {
		t.Fatal("expected per-rule dispatch timings")
	}
}

func TestDispatcher_FlatDispatchRuleRunsOnFlatTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &testFlatDispatchRule{BaseRule: BaseRule{RuleName: "FlatDispatch", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{rule})
	findings, stats := dispatcher.RunWithStats(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from flat dispatch rule, got %d", len(findings))
	}
	if findings[0].Line != 1 {
		t.Fatalf("expected flat finding on line 1, got %d", findings[0].Line)
	}
	if stats.DispatchRuleNsByRule[rule.Name()] <= 0 {
		t.Fatalf("expected flat rule timing to be recorded, got %+v", stats.DispatchRuleNsByRule)
	}

	dispatchCount, aggregateCount, lineCount, crossCount, moduleCount, legacyCount := dispatcher.Stats()
	if dispatchCount != 1 || aggregateCount != 0 || lineCount != 0 || crossCount != 0 || moduleCount != 0 || legacyCount != 0 {
		t.Fatalf("unexpected dispatcher stats: dispatch=%d aggregate=%d line=%d cross=%d module=%d legacy=%d",
			dispatchCount, aggregateCount, lineCount, crossCount, moduleCount, legacyCount)
	}
}

func TestDispatcher_RunWithStats_PreservesFixesAcrossColumnarSuppressionFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	src := `@Suppress("KeepFix")
fun suppressed() {
    val a = 1
}

fun live() {
    val b = 2
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &testFixingDispatchRule{BaseRule: BaseRule{RuleName: "KeepFix", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{rule})
	findings, _ := dispatcher.RunWithStats(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 unsuppressed finding, got %d", len(findings))
	}
	finding := findings[0]
	if finding.Line != 7 {
		t.Fatalf("expected unsuppressed finding on line 7, got line %d", finding.Line)
	}
	if finding.Fix == nil {
		t.Fatal("expected fix to survive columnar round-trip")
	}
	if finding.Fix.Replacement != "0" {
		t.Fatalf("expected fix replacement to survive round-trip, got %q", finding.Fix.Replacement)
	}
	if finding.Confidence != 0.95 {
		t.Fatalf("expected default confidence to survive round-trip, got %v", finding.Confidence)
	}
}

func TestDispatcher_RunColumnsWithStats_MatchesRunWithStats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	src := `fun first() = 1
fun second() = 2
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &testFixingDispatchRule{BaseRule: BaseRule{RuleName: "KeepFix", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{rule})

	columns, columnStats := dispatcher.RunColumnsWithStats(file)
	findings, sliceStats := dispatcher.RunWithStats(file)

	if !reflect.DeepEqual(columns.Findings(), findings) {
		t.Fatalf("columnar run mismatch:\nwant: %#v\ngot:  %#v", findings, columns.Findings())
	}
	if len(columnStats.DispatchRuleNsByRule) != len(sliceStats.DispatchRuleNsByRule) {
		t.Fatalf("expected same number of per-rule timing entries, got columns=%+v slice=%+v", columnStats.DispatchRuleNsByRule, sliceStats.DispatchRuleNsByRule)
	}
	if len(columnStats.DispatchRuleNsByRule) != 1 || columnStats.DispatchRuleNsByRule[rule.Name()] <= 0 {
		t.Fatalf("expected recorded timing for %s, got %+v", rule.Name(), columnStats.DispatchRuleNsByRule)
	}
	if sliceStats.DispatchRuleNsByRule[rule.Name()] <= 0 {
		t.Fatalf("expected slice run timing for %s, got %+v", rule.Name(), sliceStats.DispatchRuleNsByRule)
	}
	if columnStats.AggregateCollectNs != sliceStats.AggregateCollectNs ||
		columnStats.AggregateFinalizeMs != sliceStats.AggregateFinalizeMs ||
		columnStats.LineRuleMs != sliceStats.LineRuleMs ||
		columnStats.LegacyRuleMs != sliceStats.LegacyRuleMs {
		t.Fatalf("expected stable empty buckets, got columns=%+v slice=%+v", columnStats, sliceStats)
	}
}

func TestDispatcher_ConfidenceProviderOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &testMediumConfidenceDispatchRule{BaseRule: BaseRule{RuleName: "Medium", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{rule})
	findings, _ := dispatcher.RunWithStats(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Confidence != 0.60 {
		t.Fatalf("expected rule-declared confidence 0.60, got %v", findings[0].Confidence)
	}
}

func TestDispatcher_ExplicitFindingConfidenceBeatsRuleDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &testExplicitConfidenceDispatchRule{BaseRule: BaseRule{RuleName: "Explicit", RuleSetName: "test", Sev: "warning"}}
	dispatcher := NewDispatcher([]Rule{rule})
	findings, _ := dispatcher.RunWithStats(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Confidence != 0.42 {
		t.Fatalf("expected per-finding confidence 0.42 to beat rule default 0.60, got %v", findings[0].Confidence)
	}
}

// stubOnlyRule implements only the base Rule interface (Check returns
// nil) and none of the family-specific interfaces. IsImplemented
// should report false for this rule.
type stubOnlyRule struct{ BaseRule }

func (r *stubOnlyRule) Check(_ *scanner.File) []scanner.Finding { return nil }

func TestIsImplemented_DetectsFlatDispatchRule(t *testing.T) {
	rule := &testFlatDispatchRule{BaseRule: BaseRule{RuleName: "Flat", RuleSetName: "test", Sev: "warning"}}
	if !IsImplemented(rule) {
		t.Fatal("expected a flat-dispatch rule to be classified as implemented")
	}
}

func TestIsImplemented_DetectsStub(t *testing.T) {
	rule := &stubOnlyRule{BaseRule: BaseRule{RuleName: "Stub", RuleSetName: "test", Sev: "warning"}}
	if IsImplemented(rule) {
		t.Fatal("expected a rule implementing only the base Rule interface to be classified as stub")
	}
}

func TestIsImplemented_RegistryHasFewStubs(t *testing.T) {
	stubs := 0
	for _, r := range Registry {
		if !IsImplemented(r) {
			stubs++
		}
	}
	// Guard against accidental regressions of the stub classifier. The
	// exact number is free to change as rules are implemented, but a
	// large jump would indicate either a new stub got in or the
	// classifier broke.
	if stubs > 25 {
		t.Fatalf("stub count grew unexpectedly: %d stubs in registry of %d rules; if this is intentional, raise the threshold", stubs, len(Registry))
	}
}
