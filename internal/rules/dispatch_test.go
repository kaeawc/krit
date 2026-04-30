package rules

import (
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestDispatcher_DefersCrossFileAndModuleAwareRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	crossInvoked := 0
	cross := v2.FakeRule("Cross",
		v2.WithNeeds(v2.NeedsCrossFile),
		v2.WithCheck(func(ctx *v2.Context) { crossInvoked++ }),
	)
	modInvoked := 0
	mod := v2.FakeRule("Module",
		v2.WithNeeds(v2.NeedsModuleIndex),
		v2.WithCheck(func(ctx *v2.Context) { modInvoked++ }),
	)
	dispatcher := NewDispatcherV2([]*v2.Rule{cross, mod})

	dispatcher.Run(file)

	if crossInvoked != 0 {
		t.Fatalf("expected cross-file rule Check to be skipped, got %d calls", crossInvoked)
	}
	if modInvoked != 0 {
		t.Fatalf("expected module-aware rule Check to be skipped, got %d calls", modInvoked)
	}

	dispatchCount, aggregateCount, lineCount, crossCount, moduleCount := dispatcher.Stats()
	if dispatchCount != 0 || aggregateCount != 0 || lineCount != 0 {
		t.Fatalf("expected no dispatch/aggregate/line rules, got dispatch=%d aggregate=%d line=%d", dispatchCount, aggregateCount, lineCount)
	}
	if crossCount != 1 {
		t.Fatalf("expected 1 cross-file rule, got %d", crossCount)
	}
	if moduleCount != 1 {
		t.Fatalf("expected 1 module-aware rule, got %d", moduleCount)
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

	rule := v2.FakeRule("Dispatch",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, int(ctx.Node.StartCol)+1, "flat finding")
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, stats := dispatcher.RunWithStats(file)
	if columns.Len() == 0 {
		t.Fatal("expected findings from magic number rule")
	}
	if stats.SuppressionIndexMs < 0 || stats.DispatchWalkMs < 0 || stats.AggregateFinalizeMs < 0 ||
		stats.DispatchRuleNs < 0 || stats.AggregateCollectNs < 0 ||
		stats.LineRuleMs < 0 || stats.SuppressionFilterMs < 0 {
		t.Fatalf("expected non-negative stats, got %+v", stats)
	}
	if len(stats.DispatchRuleNsByRule) == 0 {
		t.Fatal("expected per-rule dispatch timings")
	}
	stat := stats.RuleStatsByRule[rule.ID]
	if stat.Rule != rule.ID || stat.Family != "dispatch" || stat.Invocations != 1 || stat.DurationNs <= 0 {
		t.Fatalf("expected dispatch rule timing with one invocation, got %+v", stat)
	}
}

func TestDispatcher_RunWithStats_TracksLineRuleInvocations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("fun example() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := v2.FakeRule("LineTiming",
		v2.WithNeeds(v2.NeedsLinePass),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "line finding")
		}),
	)
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, stats := dispatcher.RunWithStats(file)
	if columns.Len() != 1 {
		t.Fatalf("expected one line-rule finding, got %d", columns.Len())
	}
	stat := stats.RuleStatsByRule[rule.ID]
	if stat.Rule != rule.ID || stat.Family != "line" || stat.Invocations != 1 || stat.DurationNs <= 0 {
		t.Fatalf("expected line rule timing with one invocation, got %+v", stat)
	}
}

func TestSortedRuleExecutionStats_DerivesAveragesAndShare(t *testing.T) {
	stats := RunStats{RuleStatsByRule: map[string]RuleExecutionStat{
		"Slow": {Rule: "Slow", Family: "dispatch", Invocations: 2, DurationNs: 80},
		"Fast": {Rule: "Fast", Family: "line", Invocations: 1, DurationNs: 20},
	}}

	got := SortedRuleExecutionStats(stats)
	if len(got) != 2 {
		t.Fatalf("expected two stats, got %d", len(got))
	}
	if got[0].Rule != "Slow" || got[0].AvgNs != 40 || got[0].SharePct != 80 {
		t.Fatalf("unexpected slow stat: %+v", got[0])
	}
	if got[1].Rule != "Fast" || got[1].AvgNs != 20 || got[1].SharePct != 20 {
		t.Fatalf("unexpected fast stat: %+v", got[1])
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

	rule := v2.FakeRule("FlatDispatch",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, int(ctx.Node.StartCol)+1, "flat finding")
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, stats := dispatcher.RunWithStats(file)

	if columns.Len() != 1 {
		t.Fatalf("expected 1 finding from flat dispatch rule, got %d", columns.Len())
	}
	if columns.LineAt(0) != 1 {
		t.Fatalf("expected flat finding on line 1, got %d", columns.LineAt(0))
	}
	if stats.DispatchRuleNsByRule[rule.ID] <= 0 {
		t.Fatalf("expected flat rule timing to be recorded, got %+v", stats.DispatchRuleNsByRule)
	}

	dispatchCount, aggregateCount, lineCount, crossCount, moduleCount := dispatcher.Stats()
	if dispatchCount != 1 || aggregateCount != 0 || lineCount != 0 || crossCount != 0 || moduleCount != 0 {
		t.Fatalf("unexpected dispatcher stats: dispatch=%d aggregate=%d line=%d cross=%d module=%d",
			dispatchCount, aggregateCount, lineCount, crossCount, moduleCount)
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

	rule := v2.FakeRule("KeepFix",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			row := int(ctx.Node.StartRow) + 1
			col := int(ctx.Node.StartCol) + 1
			ctx.Emit(scanner.Finding{
				File:     ctx.File.Path,
				Line:     row,
				Col:      col,
				RuleSet:  "test",
				Rule:     "KeepFix",
				Severity: "warning",
				Message:  "fix preserved",
				Fix: &scanner.Fix{
					StartLine:   row,
					EndLine:     row,
					Replacement: "0",
				},
			})
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, _ := dispatcher.RunWithStats(file)

	if columns.Len() != 1 {
		t.Fatalf("expected 1 unsuppressed finding, got %d", columns.Len())
	}
	if columns.LineAt(0) != 7 {
		t.Fatalf("expected unsuppressed finding on line 7, got line %d", columns.LineAt(0))
	}
	fix := columns.FixAt(0)
	if fix == nil {
		t.Fatal("expected fix to survive columnar round-trip")
	}
	if fix.Replacement != "0" {
		t.Fatalf("expected fix replacement to survive round-trip, got %q", fix.Replacement)
	}
	if columns.ConfidenceAt(0) != 0.95 {
		t.Fatalf("expected default confidence to survive round-trip, got %v", columns.ConfidenceAt(0))
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

	rule := v2.FakeRule("KeepFix",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			row := int(ctx.Node.StartRow) + 1
			col := int(ctx.Node.StartCol) + 1
			ctx.Emit(scanner.Finding{
				File:     ctx.File.Path,
				Line:     row,
				Col:      col,
				RuleSet:  "test",
				Rule:     "KeepFix",
				Severity: "warning",
				Message:  "fix preserved",
				Fix: &scanner.Fix{
					StartLine:   row,
					EndLine:     row,
					Replacement: "0",
				},
			})
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})

	columns, columnStats := dispatcher.RunColumnsWithStats(file)
	columns2, sliceStats := dispatcher.RunWithStats(file)

	if columns.Len() != columns2.Len() {
		t.Fatalf("columnar run length mismatch: RunColumnsWithStats=%d RunWithStats=%d", columns.Len(), columns2.Len())
	}
	if len(columnStats.DispatchRuleNsByRule) != len(sliceStats.DispatchRuleNsByRule) {
		t.Fatalf("expected same number of per-rule timing entries, got columns=%+v slice=%+v", columnStats.DispatchRuleNsByRule, sliceStats.DispatchRuleNsByRule)
	}
	if len(columnStats.DispatchRuleNsByRule) != 1 || columnStats.DispatchRuleNsByRule[rule.ID] <= 0 {
		t.Fatalf("expected recorded timing for %s, got %+v", rule.ID, columnStats.DispatchRuleNsByRule)
	}
	if sliceStats.DispatchRuleNsByRule[rule.ID] <= 0 {
		t.Fatalf("expected RunWithStats timing for %s, got %+v", rule.ID, sliceStats.DispatchRuleNsByRule)
	}
	if columnStats.AggregateCollectNs != sliceStats.AggregateCollectNs ||
		columnStats.AggregateFinalizeMs != sliceStats.AggregateFinalizeMs ||
		columnStats.LineRuleMs != sliceStats.LineRuleMs ||
		columnStats.SuppressionFilterMs != sliceStats.SuppressionFilterMs {
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

	rule := v2.FakeRule("Medium",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithConfidence(0.60),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, int(ctx.Node.StartCol)+1, "medium finding")
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, _ := dispatcher.RunWithStats(file)

	if columns.Len() != 1 {
		t.Fatalf("expected 1 finding, got %d", columns.Len())
	}
	if columns.ConfidenceAt(0) != 0.60 {
		t.Fatalf("expected rule-declared confidence 0.60, got %v", columns.ConfidenceAt(0))
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

	rule := v2.FakeRule("Explicit",
		v2.WithNodeTypes("integer_literal"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithConfidence(0.60),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.Emit(scanner.Finding{
				File:       ctx.File.Path,
				Line:       int(ctx.Node.StartRow) + 1,
				Col:        int(ctx.Node.StartCol) + 1,
				RuleSet:    "test",
				Rule:       "Explicit",
				Severity:   "warning",
				Message:    "explicit finding",
				Confidence: 0.42,
			})
		}),
	)
	rule.Category = "test"
	dispatcher := NewDispatcherV2([]*v2.Rule{rule})
	columns, _ := dispatcher.RunWithStats(file)

	if columns.Len() != 1 {
		t.Fatalf("expected 1 finding, got %d", columns.Len())
	}
	if columns.ConfidenceAt(0) != 0.42 {
		t.Fatalf("expected per-finding confidence 0.42 to beat rule default 0.60, got %v", columns.ConfidenceAt(0))
	}
}

func TestHasV2Implementation_DetectsFlatDispatchRule(t *testing.T) {
	rule := v2.FakeRule("Flat",
		v2.WithNodeTypes("call_expression"),
		v2.WithCheck(func(ctx *v2.Context) {}),
	)
	if !HasV2Implementation(rule) {
		t.Fatal("expected a flat-dispatch rule to be classified as implemented")
	}
}

func TestHasV2Implementation_RejectsMissingCheck(t *testing.T) {
	rule := &v2.Rule{ID: "MissingCheck", Category: "test", NodeTypes: []string{"call_expression"}}
	if HasV2Implementation(rule) {
		t.Fatal("expected a rule with no executable callback to be rejected")
	}
}

func TestV2RegistryHasRunnableImplementations(t *testing.T) {
	var missing []string
	for _, r := range v2.Registry {
		if !HasV2Implementation(r) {
			missing = append(missing, r.ID)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("v2 registry contains rules without runnable implementations: %v", missing)
	}
}
