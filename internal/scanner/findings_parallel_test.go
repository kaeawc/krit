package scanner

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// TestFindingCollectorWorkerLocalMerge_RaceFree models the CrossRuleConcurrency
// hot path: N worker goroutines each build a worker-local collector, and the
// phase owner serially merges them into a single destination. Run with
// `go test -race` to verify no concurrent writes to shared state.
func TestFindingCollectorWorkerLocalMerge_RaceFree(t *testing.T) {
	const workers = 8
	const perWorker = 64

	locals := make([]*FindingCollector, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			local := NewFindingCollector(perWorker)
			for i := 0; i < perWorker; i++ {
				local.Append(Finding{
					File:     fmt.Sprintf("src/Worker%02d.kt", w),
					Line:     i + 1,
					Col:      1,
					RuleSet:  "style",
					Rule:     fmt.Sprintf("Rule%02d", w),
					Severity: "warning",
					Message:  fmt.Sprintf("w=%d i=%d", w, i),
				})
			}
			locals[w] = local
		}()
	}
	wg.Wait()

	merged := MergeCollectors(nil, locals...)
	columns := merged.Columns()

	if got, want := columns.Len(), workers*perWorker; got != want {
		t.Fatalf("merged row count = %d, want %d", got, want)
	}

	// Every per-worker pair (file,rule) should appear exactly perWorker times
	// with line numbers 1..perWorker in insertion order.
	seen := make(map[string]int, workers)
	for row := 0; row < columns.Len(); row++ {
		key := columns.FileAt(row) + "|" + columns.RuleAt(row)
		expectedLine := seen[key] + 1
		if got := columns.LineAt(row); got != expectedLine {
			t.Fatalf("row %d: %s line = %d, want %d (per-worker insertion order must survive merge)", row, key, got, expectedLine)
		}
		seen[key] = expectedLine
	}
	if len(seen) != workers {
		t.Fatalf("expected %d unique worker streams, saw %d", workers, len(seen))
	}
	for key, count := range seen {
		if count != perWorker {
			t.Fatalf("worker %s contributed %d rows, want %d", key, count, perWorker)
		}
	}
}

// TestFindingCollectorMergeOrder_IsDeterministicAfterSort asserts that the
// phase-end SortByFileLine step yields identical output regardless of the
// order in which worker-local collectors are merged. This is the guarantee
// CrossRuleConcurrency depends on: worker count / scheduling must not leak
// into the sorted output.
func TestFindingCollectorMergeOrder_IsDeterministicAfterSort(t *testing.T) {
	makeWorkers := func() []*FindingCollector {
		a := NewFindingCollector(0)
		a.Append(Finding{File: "b.kt", Line: 5, Col: 1, RuleSet: "style", Rule: "Alpha", Severity: "warning", Message: "b5"})
		a.Append(Finding{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "Alpha", Severity: "warning", Message: "a20"})

		b := NewFindingCollector(0)
		b.Append(Finding{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "Beta", Severity: "warning", Message: "a10"})
		b.Append(Finding{File: "c.kt", Line: 1, Col: 1, RuleSet: "style", Rule: "Beta", Severity: "warning", Message: "c1"})

		c := NewFindingCollector(0)
		c.Append(Finding{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "Gamma", Severity: "warning", Message: "a20g"})
		c.Append(Finding{File: "b.kt", Line: 5, Col: 1, RuleSet: "style", Rule: "Gamma", Severity: "warning", Message: "b5g"})

		return []*FindingCollector{a, b, c}
	}

	var baseline []Finding
	permutations := [][]int{
		{0, 1, 2},
		{2, 1, 0},
		{1, 0, 2},
		{1, 2, 0},
		{0, 2, 1},
		{2, 0, 1},
	}
	for idx, order := range permutations {
		workers := makeWorkers()
		permuted := make([]*FindingCollector, len(order))
		for i, pos := range order {
			permuted[i] = workers[pos]
		}

		merged := MergeCollectors(nil, permuted...)
		columns := merged.Columns()
		columns.SortByFileLine()
		got := columns.Findings()

		if idx == 0 {
			baseline = got
			continue
		}
		if !reflect.DeepEqual(got, baseline) {
			t.Fatalf("permutation %v changed sorted output:\nbaseline: %#v\ngot:      %#v", order, baseline, got)
		}
	}
}
