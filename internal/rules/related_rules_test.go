package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func relatedFakeRule(id string, related ...string) *api.Rule {
	r := fakeRule(id, api.MaturityStable)
	r.RelatedRules = related
	return r
}

func TestExpandWithRelated_AddsOneHop(t *testing.T) {
	registry := []*api.Rule{
		relatedFakeRule("A", "B", "C"),
		relatedFakeRule("B", "D"), // not followed: only one hop
		relatedFakeRule("C"),
		relatedFakeRule("D"),
	}

	disabled := map[string]bool{"A": true}
	ExpandWithRelated(disabled, registry)

	for _, id := range []string{"A", "B", "C"} {
		if !disabled[id] {
			t.Errorf("expected %s to be disabled after expansion; set = %v", id, disabled)
		}
	}
	if disabled["D"] {
		t.Errorf("expansion should be non-transitive; D was reachable only through B and must not be disabled. set = %v", disabled)
	}
}

func TestExpandWithRelated_EmptySetIsNoop(t *testing.T) {
	registry := []*api.Rule{relatedFakeRule("A", "B"), relatedFakeRule("B")}
	disabled := map[string]bool{}
	ExpandWithRelated(disabled, registry)
	if len(disabled) != 0 {
		t.Fatalf("ExpandWithRelated on empty set should be a no-op; got %v", disabled)
	}
}

func TestExpandWithRelated_NoOpWhenNoneDisabledMatch(t *testing.T) {
	registry := []*api.Rule{relatedFakeRule("A", "B"), relatedFakeRule("B")}
	disabled := map[string]bool{"X": true}
	ExpandWithRelated(disabled, registry)
	if disabled["A"] || disabled["B"] {
		t.Fatalf("rules unrelated to disabled set must not be touched; got %v", disabled)
	}
}

func TestActiveRulesV2_RespectsExpandedDisabledSet(t *testing.T) {
	registry := []*api.Rule{
		relatedFakeRule("A", "B"),
		relatedFakeRule("B"),
		relatedFakeRule("C"),
	}
	disabled := map[string]bool{"A": true}
	ExpandWithRelated(disabled, registry)

	got := selectActiveRules(registry, disabled, nil, false, false, nil, nil, nil)
	ids := ruleIDs(got)
	if containsID(ids, "A") || containsID(ids, "B") {
		t.Fatalf("A and its related rule B should be filtered out; got %v", ids)
	}
	if !containsID(ids, "C") {
		t.Fatalf("unrelated rule C should remain active; got %v", ids)
	}
}
