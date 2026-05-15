package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// fakeRule returns a minimal *api.Rule fixture with just enough fields for
// selectActiveRules to inspect. It avoids registering with the global
// api.Registry so tests stay hermetic.
func fakeRule(id string, m api.Maturity) *api.Rule {
	return &api.Rule{
		ID:          id,
		Description: "fake",
		Maturity:    m,
		Check:       func(*api.Context) {},
	}
}

func ruleIDs(rs []*api.Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func containsID(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// TestSelectActiveRules_StableDefault: a stable rule that is not in
// defaultInactive runs by default.
func TestSelectActiveRules_StableDefault(t *testing.T) {
	reg := []*api.Rule{fakeRule("Stable", api.MaturityStable)}
	got := selectActiveRules(reg, nil, nil, false, false, false, nil, nil, nil, nil)
	if !containsID(ruleIDs(got), "Stable") {
		t.Fatalf("stable rule should be active by default; got %v", ruleIDs(got))
	}
}

// TestSelectActiveRules_ExperimentalOffByDefault: an experimental rule is
// excluded unless experimental=true (or it is named explicitly).
func TestSelectActiveRules_ExperimentalOffByDefault(t *testing.T) {
	reg := []*api.Rule{fakeRule("Exp", api.MaturityExperimental)}
	exp := map[string]bool{"Exp": true}
	inactive := map[string]bool{"Exp": true}

	got := selectActiveRules(reg, nil, nil, false, false, false, exp, nil, nil, inactive)
	if containsID(ruleIDs(got), "Exp") {
		t.Fatalf("experimental rule should be off by default; got %v", ruleIDs(got))
	}

	got = selectActiveRules(reg, nil, nil, false, true, false, exp, nil, nil, inactive)
	if !containsID(ruleIDs(got), "Exp") {
		t.Fatalf("--experimental should re-enable experimental rules; got %v", ruleIDs(got))
	}
}

// TestSelectActiveRules_DeprecatedNeverImplicitlyOn: a deprecated rule must
// not be re-enabled by --experimental or --all-rules. Only an explicit
// enabledSet entry runs it.
func TestSelectActiveRules_DeprecatedNeverImplicitlyOn(t *testing.T) {
	reg := []*api.Rule{fakeRule("Old", api.MaturityDeprecated)}
	dep := map[string]bool{"Old": true}
	inactive := map[string]bool{"Old": true}

	cases := []struct {
		name             string
		allRules, expFlg bool
	}{
		{"plain", false, false},
		{"experimental", false, true},
		{"all-rules", true, false},
		{"all + experimental", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := selectActiveRules(reg, nil, nil, c.allRules, c.expFlg, false, nil, dep, nil, inactive)
			if containsID(ruleIDs(got), "Old") {
				t.Errorf("deprecated rule must not be implicitly enabled by %s; got %v", c.name, ruleIDs(got))
			}
		})
	}

	en := map[string]bool{"Old": true}
	got := selectActiveRules(reg, nil, en, false, false, false, nil, dep, nil, inactive)
	if !containsID(ruleIDs(got), "Old") {
		t.Fatalf("explicit enable should run a deprecated rule; got %v", ruleIDs(got))
	}
}

// TestSelectActiveRules_DisabledAlwaysWins: disabledSet wins over every
// other opt-in path including explicit enable.
func TestSelectActiveRules_DisabledAlwaysWins(t *testing.T) {
	reg := []*api.Rule{fakeRule("X", api.MaturityStable)}
	dis := map[string]bool{"X": true}
	en := map[string]bool{"X": true}

	got := selectActiveRules(reg, dis, en, true, true, false, nil, nil, nil, nil)
	if containsID(ruleIDs(got), "X") {
		t.Fatalf("disabledSet must override every other path; got %v", ruleIDs(got))
	}
}

// TestSelectActiveRules_AllRulesIncludesExperimental: --all-rules opts in
// experimental rules but still excludes deprecated ones.
func TestSelectActiveRules_AllRulesIncludesExperimental(t *testing.T) {
	reg := []*api.Rule{
		fakeRule("Stable", api.MaturityStable),
		fakeRule("Exp", api.MaturityExperimental),
		fakeRule("Old", api.MaturityDeprecated),
	}
	exp := map[string]bool{"Exp": true}
	dep := map[string]bool{"Old": true}
	inactive := map[string]bool{"Exp": true, "Old": true}

	got := ruleIDs(selectActiveRules(reg, nil, nil, true, false, false, exp, dep, nil, inactive))
	if !containsID(got, "Stable") || !containsID(got, "Exp") {
		t.Errorf("--all-rules should include stable and experimental; got %v", got)
	}
	if containsID(got, "Old") {
		t.Errorf("--all-rules must not include deprecated rules; got %v", got)
	}
}

// TestSelectActiveRules_OptInDefaultInactiveStillRunsViaEnableSet: a stable
// rule that is default-inactive (DefaultActive=false in its descriptor)
// must still run when explicitly enabled.
func TestSelectActiveRules_OptInStillRunsViaEnableSet(t *testing.T) {
	reg := []*api.Rule{fakeRule("OptIn", api.MaturityStable)}
	inactive := map[string]bool{"OptIn": true}
	en := map[string]bool{"OptIn": true}

	got := selectActiveRules(reg, nil, en, false, false, false, nil, nil, nil, inactive)
	if !containsID(ruleIDs(got), "OptIn") {
		t.Fatalf("explicit enable should run an opt-in rule; got %v", ruleIDs(got))
	}
}
