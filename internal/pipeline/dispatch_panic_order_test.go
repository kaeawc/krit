package pipeline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/rules"
)

// TestEmitPanicDiagnostics_StableOrder asserts that
// `(DispatchPhase).emitPanicDiagnostics` writes panic warnings in a
// stable order regardless of the input slice ordering coming out of
// the parallel dispatch loop. Regression for #28: previously
// `acc.Errors` was emitted in goroutine-completion order, polluting
// CI logs and making snapshot tests over Reporter output flaky.
func TestEmitPanicDiagnostics_StableOrder(t *testing.T) {
	canonicalErrs := []rules.DispatchError{
		{FilePath: "/a.kt", Line: 10, RuleName: "rule-alpha", PanicValue: "boom-1"},
		{FilePath: "/a.kt", Line: 20, RuleName: "rule-beta", PanicValue: "boom-2"},
		{FilePath: "/b.kt", Line: 5, RuleName: "rule-gamma", PanicValue: "boom-3"},
		{FilePath: "/c.kt", Line: 1, RuleName: "rule-delta", PanicValue: "boom-4"},
	}

	// Run several permutations of the same set of errors. Output must
	// be byte-identical across permutations.
	permutations := [][]int{
		{0, 1, 2, 3},
		{3, 2, 1, 0},
		{1, 3, 0, 2},
		{2, 0, 3, 1},
	}

	var refOutput string
	for k, perm := range permutations {
		errs := make([]rules.DispatchError, len(perm))
		for i, p := range perm {
			errs[i] = canonicalErrs[p]
		}

		buf := &bytes.Buffer{}
		acc := rules.RunStats{Errors: errs}
		var p DispatchPhase
		p.emitPanicDiagnostics(IndexResult{Reporter: &diag.Reporter{Warning: buf}}, acc)

		got := buf.String()
		if k == 0 {
			refOutput = got
			// Sanity: panic messages must appear in canonical (sorted) order.
			canonOrder := []string{"/a.kt", "/a.kt", "/b.kt", "/c.kt"}
			lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
			if len(lines) != 5 { // 4 panic lines + summary
				t.Fatalf("expected 5 lines, got %d:\n%s", len(lines), got)
			}
			for i, want := range canonOrder {
				if !strings.Contains(lines[i], want) {
					t.Fatalf("line %d missing %q:\n%s", i, want, got)
				}
			}
			continue
		}
		if got != refOutput {
			t.Fatalf("perm %d: output differs\n  ref: %q\n  got: %q", k, refOutput, got)
		}
	}
}

// TestEmitPanicDiagnostics_NoErrorsIsSilent confirms that with zero
// errors no warnings are written (avoids spurious "0 panic(s)" lines
// in CI logs).
func TestEmitPanicDiagnostics_NoErrorsIsSilent(t *testing.T) {
	buf := &bytes.Buffer{}
	var p DispatchPhase
	p.emitPanicDiagnostics(IndexResult{Reporter: &diag.Reporter{Warning: buf}}, rules.RunStats{})
	if buf.Len() != 0 {
		t.Fatalf("expected silent, got %q", buf.String())
	}
}
