package pipeline

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestSplitConcurrentCrossRules_Partitions checks that only
// cross-file / parsed-files rules flagged NeedsConcurrent move to the
// concurrent bucket. All other rules stay on the serial path or are
// filtered out entirely.
func TestSplitConcurrentCrossRules_Partitions(t *testing.T) {
	serialCross := v2.FakeRule("SerialCross", v2.WithNeeds(v2.NeedsCrossFile))
	concurrentCross := v2.FakeRule("ConcurrentCross", v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent))
	concurrentParsed := v2.FakeRule("ConcurrentParsed", v2.WithNeeds(v2.NeedsParsedFiles|v2.NeedsConcurrent))
	perFile := v2.FakeRule("PerFile", v2.WithNodeTypes("x"))
	module := v2.FakeRule("Module", v2.WithNeeds(v2.NeedsModuleIndex|v2.NeedsConcurrent))
	nilRule := (*v2.Rule)(nil)

	serial, concurrent := splitConcurrentCrossRules([]*v2.Rule{
		serialCross, concurrentCross, perFile, concurrentParsed, module, nilRule,
	})

	if got := ruleIDs(serial); len(got) != 1 || got[0] != "SerialCross" {
		t.Errorf("serial = %v, want [SerialCross]", got)
	}
	if got := ruleIDs(concurrent); len(got) != 2 || got[0] != "ConcurrentCross" || got[1] != "ConcurrentParsed" {
		t.Errorf("concurrent = %v, want [ConcurrentCross ConcurrentParsed]", got)
	}
}

func ruleIDs(rules []*v2.Rule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.ID
	}
	return out
}

func TestBuildCrossRuleContext_IncludesDeclaredSemanticInputs(t *testing.T) {
	kotlinFile := &scanner.File{Path: "src/main/kotlin/Foo.kt"}
	javaFile := &scanner.File{Path: "src/main/java/Foo.java"}
	parsedFiles := crossRuleParsedFiles([]*scanner.File{kotlinFile}, []*scanner.File{javaFile})
	resolver := typeinfer.NewFakeResolver()
	index := scanner.BuildIndexFromData(nil, nil)
	dst := scanner.NewFindingCollector(0)
	rule := v2.FakeRule("SemanticCross", v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsParsedFiles|v2.NeedsResolver))

	ctx := buildCrossRuleContext(rule, index, parsedFiles, resolver, dst)

	if ctx.CodeIndex != index {
		t.Fatal("CodeIndex was not wired for NeedsCrossFile")
	}
	if ctx.Resolver != resolver {
		t.Fatal("Resolver was not wired for NeedsResolver")
	}
	if len(ctx.ParsedFiles) != 2 || ctx.ParsedFiles[0] != kotlinFile || ctx.ParsedFiles[1] != javaFile {
		t.Fatalf("ParsedFiles=%v, want Kotlin and Java files in order", ctx.ParsedFiles)
	}
}

// TestRunConcurrentCrossRules_FindingEquivalence verifies that the
// merged output of the concurrent execution path contains the exact
// same findings as a serial run, independent of worker count. The final
// row order is recovered via SortByFileLine so worker interleavings do
// not leak into user-visible output.
func TestRunConcurrentCrossRules_FindingEquivalence(t *testing.T) {
	rules := makeConcurrentRules(16)
	serialCols := runCrossRulesSerial(rules)

	for _, workers := range []int{1, 2, 4, 8} {
		workers := workers
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			dst := scanner.NewFindingCollector(0)
			runConcurrentCrossRules(context.Background(), rules, nil, nil, nil, dst, workers, nil)
			got := *dst.Columns()

			if got.Len() != serialCols.Len() {
				t.Fatalf("Len=%d want %d", got.Len(), serialCols.Len())
			}
			got.SortByFileLine()
			serialCopy := serialCols.Clone()
			serialCopy.SortByFileLine()
			if !findingColumnsEqual(&got, &serialCopy) {
				t.Errorf("concurrent output diverges from serial after sort")
			}
		})
	}
}

// TestRunConcurrentCrossRules_RecoversFromPanics ensures a panicking
// rule does not take down sibling workers.
func TestRunConcurrentCrossRules_RecoversFromPanics(t *testing.T) {
	good := v2.FakeRule("Good", v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent), v2.WithCheck(func(ctx *v2.Context) {
		ctx.EmitAt(1, 1, "ok")
	}))
	bad := v2.FakeRule("Bad", v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent), v2.WithCheck(func(ctx *v2.Context) {
		panic("boom")
	}))
	rules := []*v2.Rule{good, bad, good, bad}
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(context.Background(), rules, nil, nil, nil, dst, 4, nil)
	if got := dst.Columns().Len(); got != 2 {
		t.Fatalf("Len=%d want 2 (two Good invocations survive two Bad panics)", got)
	}
}

// TestRunConcurrentCrossRules_HonoursContextCancel validates that a
// cancelled context prevents further rule dispatch.
func TestRunConcurrentCrossRules_HonoursContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var ran atomic.Int32
	rules := make([]*v2.Rule, 8)
	for i := range rules {
		rules[i] = v2.FakeRule(fmt.Sprintf("R%d", i), v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent), v2.WithCheck(func(ctx *v2.Context) {
			ran.Add(1)
		}))
	}
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(ctx, rules, nil, nil, nil, dst, 4, nil)
	// We allow up to one rule per worker to observe the cancelled ctx
	// after dispatch; the critical property is that not all rules run.
	if int(ran.Load()) >= len(rules) {
		t.Fatalf("ran=%d; cancelled context should short-circuit dispatch", ran.Load())
	}
}

// TestRunConcurrentCrossRules_SingleRuleFallsBackToSerial checks the
// threshold: a single concurrent rule should still run (on the shared
// collector, no goroutine spin-up).
func TestRunConcurrentCrossRules_SingleRuleFallsBackToSerial(t *testing.T) {
	r := v2.FakeRule("Solo", v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent), v2.WithCheck(func(ctx *v2.Context) {
		ctx.EmitAt(1, 1, "solo")
	}))
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(context.Background(), []*v2.Rule{r}, nil, nil, nil, dst, 4, nil)
	if got := dst.Columns().Len(); got != 1 {
		t.Fatalf("Len=%d want 1", got)
	}
}

func makeConcurrentRules(n int) []*v2.Rule {
	rules := make([]*v2.Rule, n)
	for i := 0; i < n; i++ {
		i := i // shadow for closure
		id := fmt.Sprintf("Concurrent%02d", i)
		rules[i] = v2.FakeRule(id, v2.WithNeeds(v2.NeedsCrossFile|v2.NeedsConcurrent), v2.WithCheck(func(ctx *v2.Context) {
			// Each rule emits three findings on distinct lines so a
			// stable sort has non-trivial work to do.
			ctx.Emit(scanner.Finding{File: "a.kt", Line: (i % 9) + 1, Col: 1, Message: "a"})
			ctx.Emit(scanner.Finding{File: "b.kt", Line: 5, Col: i + 1, Message: "b"})
			ctx.Emit(scanner.Finding{File: "c.kt", Line: 9, Col: 3, Message: "c"})
		}))
	}
	return rules
}

func runCrossRulesSerial(rules []*v2.Rule) scanner.FindingColumns {
	dst := scanner.NewFindingCollector(0)
	for _, r := range rules {
		rctx := buildCrossRuleContext(r, nil, nil, nil, dst)
		r.Check(rctx)
	}
	return *dst.Columns()
}

// findingColumnsEqual compares two column sets row-by-row, reconstructing
// canonical Finding values so intern-table differences do not matter.
func findingColumnsEqual(a, b *scanner.FindingColumns) bool {
	if a.Len() != b.Len() {
		return false
	}
	af := a.Findings()
	bf := b.Findings()
	sort.Slice(af, func(i, j int) bool { return fingerprint(af[i]) < fingerprint(af[j]) })
	sort.Slice(bf, func(i, j int) bool { return fingerprint(bf[i]) < fingerprint(bf[j]) })
	for i := range af {
		if fingerprint(af[i]) != fingerprint(bf[i]) {
			return false
		}
	}
	return true
}

func fingerprint(f scanner.Finding) string {
	return fmt.Sprintf("%s|%d|%d|%s|%s|%s", f.File, f.Line, f.Col, f.Rule, f.RuleSet, f.Message)
}
