package output

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// BenchmarkFormatJSONColumns_LargeCorpus measures the per-finding cost
// of the JSON formatter against a synthetic 87,000-finding payload,
// matching the warm-bundle output size on ~/github/kotlin. Reflection-
// based json.Marshal dominates wall-time here, so a hand-written
// finding encoder would show up immediately as a delta on this bench.
func BenchmarkFormatJSONColumns_LargeCorpus(b *testing.B) {
	cols := buildSyntheticColumns(87_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := FormatJSONColumnsCompact(
			&buf,
			cols,
			"dev",
			18_494,
			387,
			time.Now(),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		); err != nil {
			b.Fatalf("FormatJSONColumnsCompact: %v", err)
		}
	}
}

// BenchmarkFormatJSONColumns_Discard isolates the encode cost from the
// in-memory buffer growth by writing into io.Discard.
func BenchmarkFormatJSONColumns_Discard(b *testing.B) {
	cols := buildSyntheticColumns(87_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := FormatJSONColumnsCompact(
			io.Discard,
			cols,
			"dev",
			18_494,
			387,
			time.Now(),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		); err != nil {
			b.Fatalf("FormatJSONColumnsCompact: %v", err)
		}
	}
}

func buildSyntheticColumns(n int) *scanner.FindingColumns {
	collector := scanner.NewFindingCollector(n)
	for i := 0; i < n; i++ {
		f := scanner.Finding{
			File:       fmt.Sprintf("/Users/jason/github/kotlin/some/dir/File%d.kt", i%2000),
			Line:       i%500 + 1,
			Col:        i%80 + 1,
			RuleSet:    pickRuleSet(i),
			Rule:       pickRule(i),
			Severity:   "warning",
			Message:    "Synthetic finding message for benchmarking JSON output throughput.",
			Confidence: 0.85,
		}
		collector.Append(f)
	}
	cols := collector.Columns()
	cols.SortByFileLine()
	return cols
}

func pickRuleSet(i int) string {
	sets := []string{"style", "potential-bugs", "performance", "complexity", "compose"}
	return sets[i%len(sets)]
}

func pickRule(i int) string {
	rules := []string{
		"MaxLineLength", "UnusedVariable", "UselessElvisOnNonNull",
		"UnsafeCallOnNullableType", "LongParameterList", "SpreadOperator",
		"CyclomaticComplexMethod", "WildcardImport", "ReturnCount",
	}
	return rules[i%len(rules)]
}

// BenchmarkBuildJSONFindings_PreSorted measures just the byte
// concatenation cost when the FindingColumns is already in sorted
// order — VisitSortedByFileLine still touches the radix-sort scratch
// allocator (cheap) but the sort itself is effectively a no-op on
// pre-sorted data. The delta vs LargeCorpus pins the sort cost.
func BenchmarkBuildJSONFindings_PreSorted(b *testing.B) {
	cols := buildSyntheticColumns(87_000)
	cols.SortByFileLine() // baseline already calls this in buildSyntheticColumns
	fixLevels := map[string]string{}
	efforts := map[string]string{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		byRuleSet := make(map[string]int)
		byRule := make(map[string]int)
		fixableCount := 0
		_ = buildJSONFindings(cols, fixLevels, efforts, byRuleSet, byRule, &fixableCount, false)
	}
}
