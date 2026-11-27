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

	got := selectActiveRules(registry, disabled, nil, false, false, false, nil, nil, nil, nil)
	ids := ruleIDs(got)
	if containsID(ids, "A") || containsID(ids, "B") {
		t.Fatalf("A and its related rule B should be filtered out; got %v", ids)
	}
	if !containsID(ids, "C") {
		t.Fatalf("unrelated rule C should remain active; got %v", ids)
	}
}

// The built-in rules declare their RelatedRules in Meta() descriptors, not on
// api.Rule, so expanding over the registry must read the descriptors, or
// --disable-related disables nothing (#1956).
func TestExpandWithRelated_ReadsRuleDescriptors(t *testing.T) {
	disabled := map[string]bool{"KaptUsage": true}
	ExpandWithRelated(disabled, api.Registry)
	if !disabled["KotlinAndroidPluginWithBuiltInKotlin"] {
		t.Fatalf("KaptUsage's related rule KotlinAndroidPluginWithBuiltInKotlin was not added: %v", disabled)
	}
	if len(disabled) != 2 {
		t.Errorf("expansion should add only KaptUsage's one related rule; got %v", disabled)
	}
}

// ValidateRelations checks api.Rule.RelatedRules at dispatcher construction;
// relations declared in Meta() descriptors must name registered rules too.
func TestRuleDescriptorRelatedRulesAreRegistered(t *testing.T) {
	registered := make(map[string]bool, len(api.Registry))
	for _, r := range api.Registry {
		registered[r.ID] = true
	}
	for _, r := range api.Registry {
		meta, _ := MetaForRule(r)
		for _, related := range meta.RelatedRules {
			if !registered[related] {
				t.Errorf("rule %s lists unknown related rule %s", r.ID, related)
			}
		}
	}
}
