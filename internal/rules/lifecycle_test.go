package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestDeprecationFor_RegisteredRule(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	api.Registry = append(append([]*api.Rule{}, saved...), &api.Rule{
		ID:          "TestDeprecatedRule",
		Description: "synthetic deprecated rule",
		Deprecated: &api.Deprecation{
			Since:      "0.7.0",
			ReplacedBy: "TestReplacement",
			Reason:     "rolled into TestReplacement",
		},
		Check: func(*api.Context) {},
	})

	got, ok := DeprecationFor("TestDeprecatedRule")
	if !ok {
		t.Fatal("DeprecationFor missing entry for TestDeprecatedRule")
	}
	if got.Since != "0.7.0" || got.ReplacedBy != "TestReplacement" || got.Reason != "rolled into TestReplacement" {
		t.Errorf("DeprecationFor returned %+v, want Since=0.7.0 ReplacedBy=TestReplacement Reason=...", got)
	}
}

func TestDeprecationFor_NonDeprecatedReturnsFalse(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	api.Registry = append(append([]*api.Rule{}, saved...), &api.Rule{
		ID:          "TestActiveRule",
		Description: "synthetic active rule",
		Check:       func(*api.Context) {},
	})

	if _, ok := DeprecationFor("TestActiveRule"); ok {
		t.Error("DeprecationFor should return false for a rule with Deprecated == nil")
	}
}

func TestDeprecationFor_UnknownRuleReturnsFalse(t *testing.T) {
	if _, ok := DeprecationFor("NoSuchRule_xyz"); ok {
		t.Error("DeprecationFor should return false for an unknown rule ID")
	}
	if _, ok := DeprecationFor(""); ok {
		t.Error("DeprecationFor(\"\") should return false")
	}
}

func TestAllDeprecations_IncludesEveryDeprecated(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	api.Registry = append(append([]*api.Rule{}, saved...),
		&api.Rule{
			ID:          "TestDeprecatedA",
			Description: "synthetic deprecated rule A",
			Deprecated:  &api.Deprecation{Since: "0.5.0", ReplacedBy: "ReplacementA"},
			Check:       func(*api.Context) {},
		},
		&api.Rule{
			ID:          "TestDeprecatedB",
			Description: "synthetic deprecated rule B",
			Deprecated:  &api.Deprecation{Since: "0.6.0"},
			Check:       func(*api.Context) {},
		},
		&api.Rule{
			ID:          "TestActiveRule2",
			Description: "synthetic active rule",
			Check:       func(*api.Context) {},
		},
	)

	got := AllDeprecations()
	if _, ok := got["TestDeprecatedA"]; !ok {
		t.Error("AllDeprecations missing TestDeprecatedA")
	}
	if _, ok := got["TestDeprecatedB"]; !ok {
		t.Error("AllDeprecations missing TestDeprecatedB")
	}
	if _, ok := got["TestActiveRule2"]; ok {
		t.Error("AllDeprecations should not include rules with Deprecated == nil")
	}
}

func TestAllDeprecations_ReturnsNilWhenNoneDeprecated(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	stripped := make([]*api.Rule, 0, len(saved))
	for _, r := range saved {
		if r == nil {
			continue
		}
		clone := *r
		clone.Deprecated = nil
		stripped = append(stripped, &clone)
	}
	api.Registry = stripped

	if got := AllDeprecations(); got != nil {
		t.Errorf("AllDeprecations with no deprecations = %v, want nil", got)
	}
}

func TestAllDeprecations_DefensiveCopy(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	original := &api.Deprecation{Since: "0.5.0", ReplacedBy: "Replacement"}
	api.Registry = append(append([]*api.Rule{}, saved...), &api.Rule{
		ID:          "TestDeprecatedCopy",
		Description: "synthetic rule for defensive-copy test",
		Deprecated:  original,
		Check:       func(*api.Context) {},
	})

	got := AllDeprecations()["TestDeprecatedCopy"]
	got.ReplacedBy = "Mutated"
	if original.ReplacedBy != "Replacement" {
		t.Errorf("AllDeprecations must defensively copy; original ReplacedBy mutated to %q", original.ReplacedBy)
	}
}

func TestMetaForRuleIncludesLifecycleFromRuleField(t *testing.T) {
	r := &api.Rule{
		ID:                    "TestLifecycleMergeFromField",
		Description:           "synthetic rule for lifecycle merge",
		IntroducedIn:          "0.3.0",
		EnabledByDefaultSince: "0.4.0",
		Deprecated:            &api.Deprecation{Since: "0.7.0", ReplacedBy: "TestNewRule"},
		Check:                 func(*api.Context) {},
	}
	meta, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned !ok for non-nil rule")
	}
	if meta.IntroducedIn != "0.3.0" {
		t.Errorf("descriptor IntroducedIn = %q, want 0.3.0", meta.IntroducedIn)
	}
	if meta.EnabledByDefaultSince != "0.4.0" {
		t.Errorf("descriptor EnabledByDefaultSince = %q, want 0.4.0", meta.EnabledByDefaultSince)
	}
	if meta.Deprecated == nil || meta.Deprecated.Since != "0.7.0" || meta.Deprecated.ReplacedBy != "TestNewRule" {
		t.Errorf("descriptor Deprecated = %+v, want {Since:0.7.0 ReplacedBy:TestNewRule}", meta.Deprecated)
	}
}

func TestMetaForRuleDefaultsIntroducedIn(t *testing.T) {
	// A rule with no IntroducedIn declared should still produce a
	// populated descriptor via the api.DefaultIntroducedIn fallback —
	// changelog/explain output should never see an empty version.
	r := &api.Rule{
		ID:          "TestLifecycleDefaultsIntroducedIn",
		Description: "synthetic rule with no IntroducedIn",
		Check:       func(*api.Context) {},
	}
	meta, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned !ok for non-nil rule")
	}
	if meta.IntroducedIn != api.DefaultIntroducedIn {
		t.Errorf("descriptor IntroducedIn = %q, want %q", meta.IntroducedIn, api.DefaultIntroducedIn)
	}
}

// TestEveryRegisteredRuleHasIntroducedIn enforces that every rule in
// the live registry exposes a non-empty IntroducedIn — either declared
// explicitly on the rule literal or filled in by api.Register's default.
// This is the lint gate referenced by issue #191: a new rule cannot ship
// without a recorded introduction version.
func TestEveryRegisteredRuleHasIntroducedIn(t *testing.T) {
	var missing []string
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		meta, ok := MetaForRule(r)
		if !ok {
			continue
		}
		if meta.IntroducedIn == "" {
			missing = append(missing, r.ID)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d registered rule(s) have empty IntroducedIn: %v", len(missing), missing)
	}
}
