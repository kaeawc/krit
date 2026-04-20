package rules

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// Phase 2C of the CodegenRegistry migration: validate that the Meta()
// descriptors on the three licensing rules produce the SAME observable
// effect as the legacy switch in config.go#applyRuleConfig. These tests
// prove the registry contract against real rule structs — the legacy path
// is deliberately kept; only Meta() is added.

// newCopyrightYearOutdatedRule returns a freshly constructed rule with the
// same defaults used in init(). Tests use a fresh instance per case so the
// fields aren't leaked across subtests.
func newCopyrightYearOutdatedRule() *CopyrightYearOutdatedRule {
	return &CopyrightYearOutdatedRule{
		BaseRule:         BaseRule{RuleName: "CopyrightYearOutdated", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects stale copyright years in file header comments."},
		RecentYearCutoff: recentCopyrightYearCutoff,
	}
}

func newMissingSpdxIdentifierRule() *MissingSpdxIdentifierRule {
	return &MissingSpdxIdentifierRule{
		BaseRule:       BaseRule{RuleName: "MissingSpdxIdentifier", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects file header comments that are missing a SPDX license identifier."},
		RequiredPrefix: spdxIdentifierPrefix,
	}
}

func newDependencyLicenseUnknownRule() *DependencyLicenseUnknownRule {
	return &DependencyLicenseUnknownRule{
		BaseRule: BaseRule{RuleName: "DependencyLicenseUnknown", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects external dependencies not present in the embedded license registry."},
	}
}

// TestLicensing_Meta_Descriptors asserts that each licensing rule's Meta()
// returns the expected descriptor. This covers identity, severity,
// default-inactive mapping from defaults.go, and the option shape for
// DependencyLicenseUnknown.
func TestLicensing_Meta_Descriptors(t *testing.T) {
	t.Run("CopyrightYearOutdated", func(t *testing.T) {
		d := newCopyrightYearOutdatedRule().Meta()
		if d.ID != "CopyrightYearOutdated" {
			t.Errorf("ID = %q, want %q", d.ID, "CopyrightYearOutdated")
		}
		if d.RuleSet != licensingRuleSet {
			t.Errorf("RuleSet = %q, want %q", d.RuleSet, licensingRuleSet)
		}
		if d.Severity != "info" {
			t.Errorf("Severity = %q, want %q", d.Severity, "info")
		}
		if d.DefaultActive {
			t.Errorf("DefaultActive = true, want false (rule is opt-in per defaults.go)")
		}
		if d.Confidence != 0.75 {
			t.Errorf("Confidence = %v, want 0.75", d.Confidence)
		}
		if len(d.Options) != 0 {
			t.Errorf("Options = %d, want 0", len(d.Options))
		}
	})

	t.Run("MissingSpdxIdentifier", func(t *testing.T) {
		d := newMissingSpdxIdentifierRule().Meta()
		if d.ID != "MissingSpdxIdentifier" {
			t.Errorf("ID = %q, want %q", d.ID, "MissingSpdxIdentifier")
		}
		if d.RuleSet != licensingRuleSet {
			t.Errorf("RuleSet = %q, want %q", d.RuleSet, licensingRuleSet)
		}
		if d.DefaultActive {
			t.Errorf("DefaultActive = true, want false (rule is opt-in per defaults.go)")
		}
		if len(d.Options) != 0 {
			t.Errorf("Options = %d, want 0", len(d.Options))
		}
	})

	t.Run("DependencyLicenseUnknown", func(t *testing.T) {
		d := newDependencyLicenseUnknownRule().Meta()
		if d.ID != "DependencyLicenseUnknown" {
			t.Errorf("ID = %q, want %q", d.ID, "DependencyLicenseUnknown")
		}
		if d.RuleSet != licensingRuleSet {
			t.Errorf("RuleSet = %q, want %q", d.RuleSet, licensingRuleSet)
		}
		if d.DefaultActive {
			t.Errorf("DefaultActive = true, want false (rule is opt-in per defaults.go)")
		}
		if len(d.Options) != 1 {
			t.Fatalf("Options = %d, want 1", len(d.Options))
		}
		opt := d.Options[0]
		if opt.Name != "requireVerification" {
			t.Errorf("opt.Name = %q, want %q", opt.Name, "requireVerification")
		}
		if opt.Type != registry.OptBool {
			t.Errorf("opt.Type = %v, want OptBool", opt.Type)
		}
		if opt.Default != false {
			t.Errorf("opt.Default = %v, want false", opt.Default)
		}
		if opt.Apply == nil {
			t.Fatalf("opt.Apply = nil")
		}

		// Exercise the Apply closure directly: it must mutate the field
		// on the target rule.
		rule := newDependencyLicenseUnknownRule()
		opt.Apply(rule, true)
		if !rule.RequireVerification {
			t.Errorf("Apply(true) did not mutate RequireVerification")
		}
	})
}

// TestLicensing_Meta_RegistryApply exercises registry.ApplyConfig against a
// FakeConfigSource: after Set("requireVerification", true), the field is
// updated and the rule reports active when the config explicitly enables
// it.
func TestLicensing_Meta_RegistryApply(t *testing.T) {
	rule := newDependencyLicenseUnknownRule()
	cfg := registry.NewFakeConfigSource()
	cfg.SetRuleActive(licensingRuleSet, "DependencyLicenseUnknown", true)
	cfg.Set(licensingRuleSet, "DependencyLicenseUnknown", "requireVerification", true)

	active := registry.ApplyConfig(rule, rule.Meta(), cfg)
	if !active {
		t.Errorf("active = false, want true (rule-level enable via config)")
	}
	if !rule.RequireVerification {
		t.Errorf("RequireVerification = false, want true")
	}

	// With no override present, the field should remain unchanged.
	rule2 := newDependencyLicenseUnknownRule()
	active2 := registry.ApplyConfig(rule2, rule2.Meta(), registry.NewFakeConfigSource())
	if active2 {
		t.Errorf("active2 = true, want false (no override → DefaultActive)")
	}
	if rule2.RequireVerification {
		t.Errorf("RequireVerification = true, want false (no override must not mutate)")
	}
}

// licensingParityCase drives the parity test: the same YAML-equivalent
// configuration is applied via both the legacy switch (rules.ApplyConfig
// with a *config.Config) and the new registry (registry.ApplyConfig with a
// FakeConfigSource). We assert the observable rule state is identical.
type licensingParityCase struct {
	name string

	// ruleActive is the value for the "active" key under the rule, if any.
	// A nil pointer means "no override"; a non-nil value sets it.
	ruleActive *bool

	// ruleSetActive is the value for the ruleset-level "active" key.
	ruleSetActive *bool

	// requireVerification is the value for the
	// licensing.DependencyLicenseUnknown.requireVerification option.
	// A nil pointer means "omit the key".
	requireVerification *bool
}

func boolPtr(b bool) *bool { return &b }

// TestLicensing_Parity_DependencyLicenseUnknown verifies legacy vs
// registry application produce the same rule state for the single rule
// with configurable options.
func TestLicensing_Parity_DependencyLicenseUnknown(t *testing.T) {
	cases := []licensingParityCase{
		{
			name: "defaults (no overrides)",
		},
		{
			name:                "requireVerification = true",
			requireVerification: boolPtr(true),
		},
		{
			name:                "requireVerification = false explicit",
			requireVerification: boolPtr(false),
		},
		{
			name:       "rule-level enable",
			ruleActive: boolPtr(true),
		},
		{
			name:                "rule-level enable + requireVerification true",
			ruleActive:          boolPtr(true),
			requireVerification: boolPtr(true),
		},
		{
			name:          "ruleset disable (options must NOT apply)",
			ruleSetActive: boolPtr(false),
			// Even though an override is present, ruleset-disable
			// short-circuits the option apply in both code paths.
			requireVerification: boolPtr(true),
		},
		{
			name:       "rule-level disable",
			ruleActive: boolPtr(false),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// --- legacy path -----------------------------------------
			// The legacy code path mutates the global DefaultInactive
			// map. Snapshot and restore so parallel/subsequent tests
			// aren't affected.
			prevActive, prevHad := DefaultInactive["DependencyLicenseUnknown"]
			t.Cleanup(func() {
				if prevHad {
					DefaultInactive["DependencyLicenseUnknown"] = prevActive
				} else {
					delete(DefaultInactive, "DependencyLicenseUnknown")
				}
			})
			// Start from the declared default.
			DefaultInactive["DependencyLicenseUnknown"] = true

			legacyRule := newDependencyLicenseUnknownRule()
			legacyCfg := buildLegacyConfig(tc)
			// We invoke the legacy switch directly to target only our
			// rule, bypassing the Registry walk (which would try to
			// apply to other rules too).
			legacyActive := applyLegacyLicensing(legacyRule, legacyCfg)

			// --- registry path ---------------------------------------
			registryRule := newDependencyLicenseUnknownRule()
			registryCfg := buildRegistryConfig(tc)
			registryActive := registry.ApplyConfig(registryRule, registryRule.Meta(), registryCfg)

			// --- compare ---------------------------------------------
			if legacyActive != registryActive {
				t.Errorf("active mismatch: legacy=%v, registry=%v", legacyActive, registryActive)
			}
			if !reflect.DeepEqual(legacyRule, registryRule) {
				t.Errorf("rule state mismatch:\n  legacy:   %+v\n  registry: %+v", legacyRule, registryRule)
			}
		})
	}
}

// applyLegacyLicensing drives the existing rules.ApplyConfig semantics for
// the single DependencyLicenseUnknown rule. This mirrors the logic in
// config.go#ApplyConfig (ruleset gate → rule-level active → option apply)
// so the parity test can target one rule in isolation without mutating
// state for every other rule in the Registry.
func applyLegacyLicensing(rule *DependencyLicenseUnknownRule, cfg *config.Config) bool {
	ruleSet := licensingRuleSet
	ruleName := "DependencyLicenseUnknown"

	if rsActive := cfg.IsRuleSetActive(ruleSet); rsActive != nil && !*rsActive {
		// Legacy marks the rule as inactive and skips option apply.
		DefaultInactive[ruleName] = true
		return false
	}

	// Start from the declared default (opt-in ⇒ inactive).
	active := IsDefaultActive(ruleName)

	if ra := cfg.IsRuleActive(ruleSet, ruleName); ra != nil {
		if *ra {
			delete(DefaultInactive, ruleName)
			active = true
		} else {
			DefaultInactive[ruleName] = true
			active = false
		}
	}

	// Apply option overrides (same call the legacy switch makes).
	rule.RequireVerification = cfg.GetBool(ruleSet, ruleName, "requireVerification", rule.RequireVerification)

	return active
}

// buildLegacyConfig builds a *config.Config matching the test case.
//
// config.Config has no public helper for writing the ruleset-level
// "active" key (Set() always nests under ruleSet → rule → key), so we
// prime the structure via Set() and then rewrite the ruleset map to
// install the flag where IsRuleSetActive expects it.
func buildLegacyConfig(tc licensingParityCase) *config.Config {
	cfg := config.NewConfig()
	if tc.ruleActive != nil {
		cfg.Set(licensingRuleSet, "DependencyLicenseUnknown", "active", *tc.ruleActive)
	}
	if tc.requireVerification != nil {
		cfg.Set(licensingRuleSet, "DependencyLicenseUnknown", "requireVerification", *tc.requireVerification)
	}
	if tc.ruleSetActive != nil {
		// Ensure the ruleset map exists (Set always creates it), then
		// write the ruleset-level active flag alongside the rule map.
		data := cfg.Data()
		rsMap, ok := data[licensingRuleSet].(map[string]interface{})
		if !ok {
			rsMap = make(map[string]interface{})
			data[licensingRuleSet] = rsMap
		}
		rsMap["active"] = *tc.ruleSetActive
	}
	return cfg
}

// buildRegistryConfig builds a registry.FakeConfigSource matching the
// same test case.
func buildRegistryConfig(tc licensingParityCase) *registry.FakeConfigSource {
	cfg := registry.NewFakeConfigSource()
	if tc.ruleSetActive != nil {
		cfg.SetRuleSetActive(licensingRuleSet, *tc.ruleSetActive)
	}
	if tc.ruleActive != nil {
		cfg.SetRuleActive(licensingRuleSet, "DependencyLicenseUnknown", *tc.ruleActive)
	}
	if tc.requireVerification != nil {
		cfg.Set(licensingRuleSet, "DependencyLicenseUnknown", "requireVerification", *tc.requireVerification)
	}
	return cfg
}

// TestLicensing_Parity_NoOptionRules covers the two rules without options.
// For them the only observable state change is the active flag; the rule
// struct itself has no configurable fields beyond RecentYearCutoff /
// RequiredPrefix, which neither path touches.
func TestLicensing_Parity_NoOptionRules(t *testing.T) {
	type noOptionCase struct {
		name          string
		ruleActive    *bool
		ruleSetActive *bool
	}
	cases := []noOptionCase{
		{name: "defaults"},
		{name: "rule enable", ruleActive: boolPtr(true)},
		{name: "rule disable", ruleActive: boolPtr(false)},
		{name: "ruleset disable", ruleSetActive: boolPtr(false)},
		{name: "ruleset disable overrides rule enable", ruleSetActive: boolPtr(false), ruleActive: boolPtr(true)},
	}

	ruleIDs := []struct {
		id   string
		ctor func() interface{}
	}{
		{id: "CopyrightYearOutdated", ctor: func() interface{} { return newCopyrightYearOutdatedRule() }},
		{id: "MissingSpdxIdentifier", ctor: func() interface{} { return newMissingSpdxIdentifierRule() }},
	}

	for _, rr := range ruleIDs {
		rr := rr
		t.Run(rr.id, func(t *testing.T) {
			for _, tc := range cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					// Snapshot/restore DefaultInactive for this rule.
					prevActive, prevHad := DefaultInactive[rr.id]
					t.Cleanup(func() {
						if prevHad {
							DefaultInactive[rr.id] = prevActive
						} else {
							delete(DefaultInactive, rr.id)
						}
					})
					DefaultInactive[rr.id] = true

					// Build legacy config.
					legacyCfg := config.NewConfig()
					if tc.ruleActive != nil {
						legacyCfg.Set(licensingRuleSet, rr.id, "active", *tc.ruleActive)
					}
					if tc.ruleSetActive != nil {
						data := legacyCfg.Data()
						rsMap, ok := data[licensingRuleSet].(map[string]interface{})
						if !ok {
							rsMap = make(map[string]interface{})
							data[licensingRuleSet] = rsMap
						}
						rsMap["active"] = *tc.ruleSetActive
					}
					legacyActive := legacyNoOptionActive(rr.id, legacyCfg)

					// Build registry config.
					regCfg := registry.NewFakeConfigSource()
					if tc.ruleSetActive != nil {
						regCfg.SetRuleSetActive(licensingRuleSet, *tc.ruleSetActive)
					}
					if tc.ruleActive != nil {
						regCfg.SetRuleActive(licensingRuleSet, rr.id, *tc.ruleActive)
					}

					ruleInstance := rr.ctor()
					type metaProvider interface {
						Meta() registry.RuleDescriptor
					}
					registryActive := registry.ApplyConfig(ruleInstance, ruleInstance.(metaProvider).Meta(), regCfg)

					if legacyActive != registryActive {
						t.Errorf("active mismatch for %s case %q: legacy=%v, registry=%v", rr.id, tc.name, legacyActive, registryActive)
					}
				})
			}
		})
	}
}

// legacyNoOptionActive reproduces the legacy active-state logic for a
// rule with no options.
func legacyNoOptionActive(ruleName string, cfg *config.Config) bool {
	if rsActive := cfg.IsRuleSetActive(licensingRuleSet); rsActive != nil && !*rsActive {
		DefaultInactive[ruleName] = true
		return false
	}
	active := IsDefaultActive(ruleName)
	if ra := cfg.IsRuleActive(licensingRuleSet, ruleName); ra != nil {
		if *ra {
			delete(DefaultInactive, ruleName)
			active = true
		} else {
			DefaultInactive[ruleName] = true
			active = false
		}
	}
	return active
}
