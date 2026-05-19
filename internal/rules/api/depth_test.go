package api

import (
	"testing"
)

func TestResolveActiveRulesAtDepth_NotThoroughIsIdentity(t *testing.T) {
	rules := []*Rule{
		{ID: "A", Needs: NeedsResolver},
		{ID: "B", Needs: NeedsResolver, ThoroughOnlyNeeds: NeedsOracleExprType},
	}
	got := ResolveActiveRulesAtDepth(rules, false)
	if &got[0] != &rules[0] || len(got) != len(rules) {
		t.Fatalf("expected the original slice header back; got %v", got)
	}
	if got[1].Needs != NeedsResolver {
		t.Fatalf("expected Needs untouched at non-thorough depth; got %v", got[1].Needs)
	}
}

func TestResolveActiveRulesAtDepth_ThoroughButNoOptIn(t *testing.T) {
	rules := []*Rule{
		{ID: "A", Needs: NeedsResolver},
		{ID: "B", Needs: NeedsResolver},
	}
	got := ResolveActiveRulesAtDepth(rules, true)
	if &got[0] != &rules[0] {
		t.Fatalf("expected original slice when no rule opts in")
	}
}

func TestResolveActiveRulesAtDepth_ThoroughClonesOptedInRules(t *testing.T) {
	a := &Rule{ID: "A", Needs: NeedsResolver}
	b := &Rule{ID: "B", Needs: NeedsResolver, ThoroughOnlyNeeds: NeedsOracleExprType}
	c := &Rule{ID: "C", Needs: NeedsCrossFile, ThoroughOnlyNeeds: NeedsOracleSupertypes | NeedsOracleClassAnnotations}

	got := ResolveActiveRulesAtDepth([]*Rule{a, b, c}, true)

	if got[0] != a {
		t.Errorf("rule without ThoroughOnlyNeeds should keep its pointer; got %p, want %p", got[0], a)
	}
	if got[1] == b {
		t.Errorf("rule with ThoroughOnlyNeeds must be cloned, not mutated in place")
	}
	if got[1].Needs != NeedsResolver|NeedsOracleExprType {
		t.Errorf("expected Needs=%v; got %v", NeedsResolver|NeedsOracleExprType, got[1].Needs)
	}
	wantC := NeedsCrossFile | NeedsOracleSupertypes | NeedsOracleClassAnnotations
	if got[2].Needs != wantC {
		t.Errorf("expected Needs=%v; got %v", wantC, got[2].Needs)
	}
}

// Daemon-mode invariant: the helper must not mutate Registry pointers.
func TestResolveActiveRulesAtDepth_RegistryRulesNotMutated(t *testing.T) {
	b := &Rule{ID: "B", Needs: NeedsResolver, ThoroughOnlyNeeds: NeedsOracleExprType}
	originalNeeds := b.Needs
	originalThorough := b.ThoroughOnlyNeeds

	_ = ResolveActiveRulesAtDepth([]*Rule{b}, true)

	if b.Needs != originalNeeds || b.ThoroughOnlyNeeds != originalThorough {
		t.Fatalf("input rule mutated: Needs %v→%v, ThoroughOnlyNeeds %v→%v",
			originalNeeds, b.Needs, originalThorough, b.ThoroughOnlyNeeds)
	}
}

// Idempotency: calling the helper on its own output must be free —
// both pipeline entry points (RunProjectStreaming and RunProjectAnalysis)
// project rules, and the streaming path runs the helper before passing
// args downstream to the analysis path. The second call must observe
// already-projected rules (ThoroughOnlyNeeds==0 on the clones) and
// return the input slice unchanged.
func TestResolveActiveRulesAtDepth_IsIdempotent(t *testing.T) {
	rules := []*Rule{{ID: "B", Needs: NeedsResolver, ThoroughOnlyNeeds: NeedsOracleExprType}}

	first := ResolveActiveRulesAtDepth(rules, true)
	if first[0].ThoroughOnlyNeeds != 0 {
		t.Fatalf("clone should have ThoroughOnlyNeeds zeroed; got %v", first[0].ThoroughOnlyNeeds)
	}
	second := ResolveActiveRulesAtDepth(first, true)
	if &second[0] != &first[0] {
		t.Fatalf("second call should return the same slice; allocations are wasted work")
	}
}

func TestResolveActiveRulesAtDepth_NilEntriesPassThrough(t *testing.T) {
	b := &Rule{ID: "B", Needs: NeedsResolver, ThoroughOnlyNeeds: NeedsOracleExprType}
	got := ResolveActiveRulesAtDepth([]*Rule{nil, b, nil}, true)
	if got[0] != nil || got[2] != nil {
		t.Fatalf("nil entries should pass through unchanged; got %v", got)
	}
	if got[1].Needs != NeedsResolver|NeedsOracleExprType {
		t.Fatalf("expected ORed Needs on opted-in rule; got %v", got[1].Needs)
	}
}

func TestResolveActiveRulesAtDepth_EmptyInput(t *testing.T) {
	if got := ResolveActiveRulesAtDepth(nil, true); got != nil {
		t.Fatalf("nil input should return nil; got %v", got)
	}
	if got := ResolveActiveRulesAtDepth([]*Rule{}, true); len(got) != 0 {
		t.Fatalf("empty input should return empty slice; got %v", got)
	}
}
