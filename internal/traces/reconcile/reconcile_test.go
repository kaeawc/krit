package reconcile

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/traces"
)

func TestReconcileExactByFQN(t *testing.T) {
	idx := BuildIndex([]scanner.Symbol{
		{Name: "bar", FQN: "com.acme.Foo.bar", File: "Foo.kt"},
	})
	results := Reconcile(idx, []traces.RuntimeState{
		{Fingerprint: "fp1", TopSymbol: "com.acme.Foo.bar"},
	})
	if len(results) != 1 {
		t.Fatalf("want 1 result")
	}
	if results[0].Resolution != traces.ResolvedExact {
		t.Fatalf("want exact, got %s", results[0].Resolution)
	}
	if results[0].Match.FQN != "com.acme.Foo.bar" {
		t.Fatalf("match: %v", results[0].Match)
	}
}

func TestReconcileExactByUniqueSimpleName(t *testing.T) {
	idx := BuildIndex([]scanner.Symbol{
		{Name: "uniqueFn", FQN: "com.acme.X.uniqueFn"},
		{Name: "other", FQN: "com.acme.Y.other"},
	})
	results := Reconcile(idx, []traces.RuntimeState{
		{Fingerprint: "fp1", TopSymbol: "uniqueFn"},
	})
	if results[0].Resolution != traces.ResolvedExact {
		t.Fatalf("want exact, got %s", results[0].Resolution)
	}
}

func TestReconcileFuzzyProducesRankedSuggestions(t *testing.T) {
	idx := BuildIndex([]scanner.Symbol{
		{Name: "compute", FQN: "com.acme.runner.A.compute"},
		{Name: "compute", FQN: "com.acme.runner.B.compute"},
	})
	// Top symbol is the simple name only — multiple candidates share it,
	// so the resolver should fall through to fuzzy ranking by suffix
	// similarity (which is the same for both candidates here).
	results := Reconcile(idx, []traces.RuntimeState{
		{Fingerprint: "fp1", TopSymbol: "compute"},
	})
	if results[0].Resolution != traces.ResolvedFuzzy {
		t.Fatalf("want fuzzy, got %s", results[0].Resolution)
	}
	if len(results[0].Suggestions) != 2 {
		t.Fatalf("want 2 suggestions, got %d", len(results[0].Suggestions))
	}
}

// TestReconcileFuzzyTopCandidateMatchesActualImpl: when two impls of
// `process()` exist and runtime captured only the simple name plus
// caller chain, the top-1 candidate is the impl whose package sits
// inside the caller chain.
func TestReconcileFuzzyTopCandidateMatchesActualImpl(t *testing.T) {
	idx := BuildIndex([]scanner.Symbol{
		{Name: "process", FQN: "com.acme.runner.A.process", Package: "com.acme.runner"},
		{Name: "process", FQN: "com.acme.alt.B.process", Package: "com.acme.alt"},
	})
	results := Reconcile(idx, []traces.RuntimeState{
		{
			Fingerprint: "fp1",
			TopSymbol:   "process",
			CallerFrames: []string{
				"com.acme.runner.Pipeline.run",
				"com.acme.runner.Main.main",
			},
		},
	})
	if results[0].Resolution != traces.ResolvedFuzzy {
		t.Fatalf("want fuzzy, got %s", results[0].Resolution)
	}
	if got := results[0].Suggestions[0].CandidateSymbol; got != "com.acme.runner.A.process" {
		t.Fatalf("top-1 candidate: want com.acme.runner.A.process, got %s", got)
	}
}

func TestReconcileUnresolvedWhenNoCandidates(t *testing.T) {
	idx := BuildIndex([]scanner.Symbol{
		{Name: "known", FQN: "com.acme.X.known"},
	})
	results := Reconcile(idx, []traces.RuntimeState{
		{Fingerprint: "fp1", TopSymbol: "unknown.Symbol.frobnicate"},
	})
	if results[0].Resolution != traces.Unresolved {
		t.Fatalf("want unresolved, got %s", results[0].Resolution)
	}
}
