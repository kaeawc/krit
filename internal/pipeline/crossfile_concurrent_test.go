package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestSplitConcurrentCrossRules_Partitions checks that only
// cross-file / parsed-files rules flagged NeedsConcurrent move to the
// concurrent bucket. All other rules stay on the serial path or are
// filtered out entirely.
func TestSplitConcurrentCrossRules_Partitions(t *testing.T) {
	serialCross := api.FakeRule("SerialCross", api.WithNeeds(api.NeedsCrossFile))
	concurrentCross := api.FakeRule("ConcurrentCross", api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent))
	concurrentParsed := api.FakeRule("ConcurrentParsed", api.WithNeeds(api.NeedsParsedFiles|api.NeedsConcurrent))
	perFile := api.FakeRule("PerFile", api.WithNodeTypes("x"))
	module := api.FakeRule("Module", api.WithNeeds(api.NeedsModuleIndex|api.NeedsConcurrent))
	nilRule := (*api.Rule)(nil)

	serial, concurrent := splitConcurrentCrossRules([]*api.Rule{
		serialCross, concurrentCross, perFile, concurrentParsed, module, nilRule,
	})

	if got := ruleIDs(serial); len(got) != 1 || got[0] != "SerialCross" {
		t.Errorf("serial = %v, want [SerialCross]", got)
	}
	if got := ruleIDs(concurrent); len(got) != 2 || got[0] != "ConcurrentCross" || got[1] != "ConcurrentParsed" {
		t.Errorf("concurrent = %v, want [ConcurrentCross ConcurrentParsed]", got)
	}
}

func ruleIDs(rules []*api.Rule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.ID
	}
	return out
}

func TestBuildCrossRuleContext_IncludesDeclaredSemanticInputs(t *testing.T) {
	kotlinFile := &scanner.File{Path: "src/main/kotlin/Foo.kt"}
	javaFile := writeParsedJavaFile(t, `package app;

class Foo {}
`, "Foo.java")
	parsedFiles := crossRuleParsedFiles([]*scanner.File{kotlinFile}, []*scanner.File{javaFile})
	resolver := typeinfer.NewFakeResolver()
	index := scanner.BuildIndexFromData(nil, nil)
	dst := scanner.NewFindingCollector(0)
	rule := api.FakeRule("SemanticCross", api.WithNeeds(api.NeedsCrossFile|api.NeedsParsedFiles|api.NeedsResolver))
	javaSourceIndex := javaSourceIndexForParsedFiles(parsedFiles)

	ctx := buildCrossRuleContext(rule, index, parsedFiles, resolver, nil, javaSourceIndex, dst)

	if ctx.CodeIndex != index {
		t.Fatal("CodeIndex was not wired for NeedsCrossFile")
	}
	if ctx.Resolver != resolver {
		t.Fatal("Resolver was not wired for NeedsResolver")
	}
	if len(ctx.ParsedFiles) != 2 || ctx.ParsedFiles[0] != kotlinFile || ctx.ParsedFiles[1] != javaFile {
		t.Fatalf("ParsedFiles=%v, want Kotlin and Java files in order", ctx.ParsedFiles)
	}
	if ctx.JavaSourceIndex == nil {
		t.Fatal("JavaSourceIndex was not wired")
	}
	if ctx.JavaSourceIndex != javaSourceIndex {
		t.Fatal("JavaSourceIndex was rebuilt instead of reusing the phase index")
	}
	if _, ok := ctx.JavaSourceIndex.ClassesByFQN["app.Foo"]; !ok {
		t.Fatalf("JavaSourceIndex missing app.Foo: %#v", ctx.JavaSourceIndex.ClassesByFQN)
	}
}

func writeParsedJavaFile(t *testing.T, code string, name string) *scanner.File {
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
			runConcurrentCrossRules(context.Background(), rules, nil, nil, nil, nil, nil, dst, workers, nil, nil)
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
	good := api.FakeRule("Good", api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
		ctx.EmitAt(1, 1, "ok")
	}))
	bad := api.FakeRule("Bad", api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
		panic("boom")
	}))
	rules := []*api.Rule{good, bad, good, bad}
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(context.Background(), rules, nil, nil, nil, nil, nil, dst, 4, nil, nil)
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
	rules := make([]*api.Rule, 8)
	for i := range rules {
		rules[i] = api.FakeRule(fmt.Sprintf("R%d", i), api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
			ran.Add(1)
		}))
	}
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(ctx, rules, nil, nil, nil, nil, nil, dst, 4, nil, nil)
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
	r := api.FakeRule("Solo", api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
		ctx.EmitAt(1, 1, "solo")
	}))
	dst := scanner.NewFindingCollector(0)
	runConcurrentCrossRules(context.Background(), []*api.Rule{r}, nil, nil, nil, nil, nil, dst, 4, nil, nil)
	if got := dst.Columns().Len(); got != 1 {
		t.Fatalf("Len=%d want 1", got)
	}
}

// TestRunConcurrentCrossRules_PanicRecordedInErrors verifies that a rule
// panic is recovered AND surfaced through the errors slice instead of
// being silently swallowed. Covers both the serial-fallback (1 rule) and
// the parallel-worker (>= concurrentCrossRuleThreshold) paths.
func TestRunConcurrentCrossRules_PanicRecordedInErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{"SerialFallback", 1},
		{"Parallel", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ruleSet := make([]*api.Rule, tc.n)
			for i := range ruleSet {
				id := fmt.Sprintf("Boom%02d", i)
				ruleSet[i] = api.FakeRule(id, api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
					panic("boom: " + ctx.Rule.ID)
				}))
			}
			dst := scanner.NewFindingCollector(0)
			var errs []rules.DispatchError
			runConcurrentCrossRules(context.Background(), ruleSet, nil, nil, nil, nil, nil, dst, 4, nil, &errs)
			if len(errs) != tc.n {
				t.Fatalf("errs=%d want %d (%v)", len(errs), tc.n, errs)
			}
			seen := make(map[string]bool)
			for _, e := range errs {
				seen[e.RuleName] = true
				if e.PanicValue == nil {
					t.Errorf("rule %q: PanicValue is nil", e.RuleName)
				}
			}
			for _, r := range ruleSet {
				if !seen[r.ID] {
					t.Errorf("missing panic record for %q (got %v)", r.ID, errs)
				}
			}
		})
	}
}

func makeConcurrentRules(n int) []*api.Rule {
	rules := make([]*api.Rule, n)
	for i := 0; i < n; i++ {
		i := i // shadow for closure
		id := fmt.Sprintf("Concurrent%02d", i)
		rules[i] = api.FakeRule(id, api.WithNeeds(api.NeedsCrossFile|api.NeedsConcurrent), api.WithCheck(func(ctx *api.Context) {
			// Each rule emits three findings on distinct lines so a
			// stable sort has non-trivial work to do.
			ctx.Emit(scanner.Finding{File: "a.kt", Line: (i % 9) + 1, Col: 1, Message: "a"})
			ctx.Emit(scanner.Finding{File: "b.kt", Line: 5, Col: i + 1, Message: "b"})
			ctx.Emit(scanner.Finding{File: "c.kt", Line: 9, Col: 3, Message: "c"})
		}))
	}
	return rules
}

func runCrossRulesSerial(rules []*api.Rule) scanner.FindingColumns {
	dst := scanner.NewFindingCollector(0)
	for _, r := range rules {
		rctx := buildCrossRuleContext(r, nil, nil, nil, nil, nil, dst)
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
