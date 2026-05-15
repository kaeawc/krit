package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestOwnersValidation enforces the issue-197 evaluation criterion:
// every default-active rule in the registry must publish at least one
// owner. MetaForRule falls back to api.DefaultRuleOwners when the rule
// did not declare its own, so this test fails only if the project-wide
// fallback ever gets cleared.
func TestOwnersValidation(t *testing.T) {
	for _, r := range api.Registry {
		if !IsDefaultActive(r.ID) {
			continue
		}
		desc, ok := MetaForRule(r)
		if !ok {
			t.Fatalf("MetaForRule(%s) returned ok=false", r.ID)
		}
		if len(desc.Owners) == 0 {
			t.Fatalf("default-active rule %s has no owners; either declare Rule.Owners or keep api.DefaultRuleOwners populated", r.ID)
		}
	}
}

// TestMetaForRule_OwnersFallback verifies the fallback to
// api.DefaultRuleOwners when a rule declares no Owners.
func TestMetaForRule_OwnersFallback(t *testing.T) {
	r := api.FakeRule("OwnersFallbackProbe")
	desc, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned ok=false")
	}
	if len(desc.Owners) == 0 {
		t.Fatal("expected owners fallback, got empty slice")
	}
	if desc.Owners[0] != api.DefaultRuleOwners[0] {
		t.Fatalf("expected first owner %q, got %q",
			api.DefaultRuleOwners[0], desc.Owners[0])
	}
}

// TestMetaForRule_OwnersExplicit verifies that an explicit Rule.Owners
// is preserved verbatim and not overridden by the fallback.
func TestMetaForRule_OwnersExplicit(t *testing.T) {
	r := api.FakeRule("OwnersExplicitProbe")
	r.Owners = []string{"@kaeawc/android", "@kaeawc/perf"}

	desc, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned ok=false")
	}
	if len(desc.Owners) != 2 {
		t.Fatalf("expected 2 owners, got %d: %v", len(desc.Owners), desc.Owners)
	}
	if desc.Owners[0] != "@kaeawc/android" || desc.Owners[1] != "@kaeawc/perf" {
		t.Fatalf("explicit owners not preserved: %v", desc.Owners)
	}
}
