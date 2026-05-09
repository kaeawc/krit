package fixer

import (
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestApplyAllFixesColumns_StableErrorOrder asserts that the slice of
// errors returned from ApplyAllFixesColumns is deterministic across
// runs, regardless of Go's map iteration randomization. Regression for
// issue #27: map iteration over `byFile` previously surfaced errors in
// non-deterministic order, causing CI log diff noise and flaky tests
// over Reporter output.
func TestApplyAllFixesColumns_StableErrorOrder(t *testing.T) {
	// Use ten distinct non-existent paths so applyFixesDetailedColumns
	// errors uniformly on os.ReadFile. Lexically interleaved names make
	// it obvious in test output if the sort order is wrong.
	paths := []string{
		"/nonexistent/krit-determinism/zzz.kt",
		"/nonexistent/krit-determinism/aaa.kt",
		"/nonexistent/krit-determinism/mmm.kt",
		"/nonexistent/krit-determinism/bbb.kt",
		"/nonexistent/krit-determinism/yyy.kt",
		"/nonexistent/krit-determinism/ccc.kt",
		"/nonexistent/krit-determinism/qqq.kt",
		"/nonexistent/krit-determinism/ddd.kt",
		"/nonexistent/krit-determinism/rrr.kt",
		"/nonexistent/krit-determinism/eee.kt",
	}

	want := append([]string(nil), paths...)
	sort.Strings(want)

	var findings []scanner.Finding
	for _, p := range paths {
		findings = append(findings, findingWithRule(p, "rule-A", &scanner.Fix{
			StartByte:   0,
			EndByte:     1,
			Replacement: "X",
			ByteMode:    true,
		}))
	}

	// 200 iterations to amplify any scheduler-driven non-determinism.
	for i := 0; i < 200; i++ {
		columns := scanner.CollectFindings(findings)
		_, _, errs := ApplyAllFixesColumns(&columns, "")
		if len(errs) != len(want) {
			t.Fatalf("iter %d: got %d errors, want %d", i, len(errs), len(want))
		}
		for k, e := range errs {
			if !strings.HasPrefix(e.Error(), want[k]+":") {
				t.Fatalf("iter %d: errs[%d] = %q, want path %q first", i, k, e.Error(), want[k])
			}
		}
	}
}
