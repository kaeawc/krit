package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestNoisinessFromPrecision verifies the derivation table from Precision
// to default Noisiness: heuristic/text-backed maps to Noisy, every other
// tier maps to Normal.
func TestNoisinessFromPrecision(t *testing.T) {
	cases := []struct {
		precision api.Precision
		want      api.Noisiness
	}{
		{api.PrecisionHeuristicTextBacked, api.NoisinessNoisy},
		{api.PrecisionASTBacked, api.NoisinessNormal},
		{api.PrecisionProjectStructure, api.NoisinessNormal},
		{api.PrecisionTypeAware, api.NoisinessNormal},
		{api.PrecisionPolicy, api.NoisinessNormal},
		{api.PrecisionUnset, api.NoisinessNormal},
	}
	for _, c := range cases {
		if got := NoisinessFromPrecision(c.precision); got != c.want {
			t.Errorf("NoisinessFromPrecision(%q) = %q, want %q",
				c.precision, got, c.want)
		}
	}
}

// TestV2RuleNoisiness_Override verifies explicit Rule.Noisiness wins
// over the derived value.
func TestV2RuleNoisiness_Override(t *testing.T) {
	r := &api.Rule{
		ID:          "Override",
		Category:    "test",
		Description: "override",
		Sev:         api.SeverityWarning,
		NodeTypes:   []string{"call_expression"}, // derives PrecisionASTBacked -> NoisinessNormal
		Check:       func(*api.Context) {},
		Noisiness:   api.NoisinessQuiet,
	}
	if got := V2RuleNoisiness(r); got != api.NoisinessQuiet {
		t.Fatalf("explicit Noisiness override ignored: got %q", got)
	}
}

// TestV2RuleNoisiness_DerivedFromHeuristic verifies that a rule without
// an explicit Noisiness inherits NoisinessNoisy when its derived
// Precision is heuristic/text-backed.
func TestV2RuleNoisiness_DerivedFromHeuristic(t *testing.T) {
	// MagicNumber is in heuristicRuleNames so V2RulePrecision returns
	// PrecisionHeuristicTextBacked; we expect NoisinessNoisy to derive.
	r := &api.Rule{
		ID:          "MagicNumber",
		Category:    "style",
		Description: "magic",
		Sev:         api.SeverityWarning,
		NodeTypes:   []string{"integer_literal"},
		Check:       func(*api.Context) {},
	}
	if got := V2RuleNoisiness(r); got != api.NoisinessNoisy {
		t.Fatalf("heuristic rule should derive NoisinessNoisy; got %q", got)
	}
}

// TestStrictPresetExcludesNoisy verifies the strict flag drops rules
// whose effective Noisiness is NoisinessNoisy unless they are named
// explicitly in enabledSet.
func TestStrictPresetExcludesNoisy(t *testing.T) {
	reg := []*api.Rule{
		fakeRule("Quiet", api.MaturityStable),
		fakeRule("Loud", api.MaturityStable),
	}
	noisy := map[string]bool{"Loud": true}

	got := ruleIDs(selectActiveRules(reg, nil, nil, false, false, true, nil, nil, noisy, nil))
	if containsID(got, "Loud") {
		t.Fatalf("strict should exclude noisy rule; got %v", got)
	}
	if !containsID(got, "Quiet") {
		t.Fatalf("strict should keep non-noisy rule; got %v", got)
	}

	// Explicit enable wins over strict.
	en := map[string]bool{"Loud": true}
	got = ruleIDs(selectActiveRules(reg, nil, en, false, false, true, nil, nil, noisy, nil))
	if !containsID(got, "Loud") {
		t.Fatalf("explicit enable should override strict; got %v", got)
	}

	// Without strict, noisy rules run normally.
	got = ruleIDs(selectActiveRules(reg, nil, nil, false, false, false, nil, nil, noisy, nil))
	if !containsID(got, "Loud") {
		t.Fatalf("noisy rule should run when strict=false; got %v", got)
	}
}

// TestMetaForRule_NoisinessAlwaysSet verifies every registered rule has
// a non-zero Noisiness after MetaForRule resolves it.
func TestMetaForRule_NoisinessAlwaysSet(t *testing.T) {
	for _, r := range api.Registry {
		desc, ok := MetaForRule(r)
		if !ok {
			t.Fatalf("MetaForRule(%s) returned ok=false", r.ID)
		}
		if desc.Noisiness == api.NoisinessUnset {
			t.Fatalf("rule %s: descriptor noisiness is NoisinessUnset", r.ID)
		}
	}
}

// TestMetaForRule_KnownLimitationsSurfaced verifies that KnownLimitations
// declared on the rule's Meta() descriptor flow through MetaForRule.
func TestMetaForRule_KnownLimitationsSurfaced(t *testing.T) {
	var found *api.Rule
	for _, r := range api.Registry {
		if r.ID == "MagicNumber" {
			found = r
			break
		}
	}
	if found == nil {
		t.Skip("MagicNumber rule not registered in this build")
	}
	desc, ok := MetaForRule(found)
	if !ok {
		t.Fatal("MetaForRule(MagicNumber) returned ok=false")
	}
	if len(desc.KnownLimitations) == 0 {
		t.Fatalf("MagicNumber should publish KnownLimitations bullets")
	}
}
