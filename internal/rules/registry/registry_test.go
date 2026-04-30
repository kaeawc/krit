package registry

import (
	"regexp"
	"testing"
)

// configurableRule stands in for a real rule struct. It carries the
// fields that each ConfigOption's Apply closure assigns.
type configurableRule struct {
	AllowedLines  int
	Strict        bool
	Prefix        string
	Ignored       []string
	ClassPattern  *regexp.Regexp
	UnchangedFlag bool
	UnchangedList []string
}

// descriptorForConfigurableRule builds the descriptor used by most tests.
// Each Apply closure downcasts target to *configurableRule.
func descriptorForConfigurableRule() RuleDescriptor {
	return RuleDescriptor{
		ID:            "ConfigurableRule",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "test rule with one option of every type",
		DefaultActive: true,
		Options: []ConfigOption{
			{
				Name:    "allowedLines",
				Aliases: []string{"threshold"},
				Type:    OptInt,
				Default: 60,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).AllowedLines = value.(int)
				},
			},
			{
				Name:    "strict",
				Type:    OptBool,
				Default: false,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).Strict = value.(bool)
				},
			},
			{
				Name:    "prefix",
				Type:    OptString,
				Default: "",
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).Prefix = value.(string)
				},
			},
			{
				Name: "ignored",
				Type: OptStringList,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).Ignored = value.([]string)
				},
			},
			{
				Name: "classPattern",
				Type: OptRegex,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).ClassPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name: "unchangedFlag",
				Type: OptBool,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).UnchangedFlag = value.(bool)
				},
			},
			{
				Name: "unchangedList",
				Type: OptStringList,
				Apply: func(target interface{}, value interface{}) {
					target.(*configurableRule).UnchangedList = value.([]string)
				},
			},
		},
	}
}

// Test 1: Basic round-trip — setting int + bool + string + string[]
// values on the fake config mutates the rule struct through Apply.
func TestApplyConfig_1_BasicRoundTrip(t *testing.T) {
	rule := &configurableRule{AllowedLines: 60}
	cfg := NewFakeConfigSource()
	cfg.Set("complexity", "ConfigurableRule", "allowedLines", 120)
	cfg.Set("complexity", "ConfigurableRule", "strict", true)
	cfg.Set("complexity", "ConfigurableRule", "prefix", "foo")
	cfg.Set("complexity", "ConfigurableRule", "ignored", []string{"a", "b"})

	active := ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
	if !active {
		t.Fatalf("expected rule active, got inactive")
	}
	if rule.AllowedLines != 120 {
		t.Errorf("AllowedLines = %d, want 120", rule.AllowedLines)
	}
	if !rule.Strict {
		t.Errorf("Strict = false, want true")
	}
	if rule.Prefix != "foo" {
		t.Errorf("Prefix = %q, want %q", rule.Prefix, "foo")
	}
	if len(rule.Ignored) != 2 || rule.Ignored[0] != "a" || rule.Ignored[1] != "b" {
		t.Errorf("Ignored = %v, want [a b]", rule.Ignored)
	}
}

// Test 2: Alias lookup — an option with Name "allowedLines" and Aliases
// ["threshold"] reads from `threshold` when only that alias is set, and
// prefers `Name` when both are set.
func TestApplyConfig_2_AliasLookup(t *testing.T) {
	t.Run("alias only", func(t *testing.T) {
		rule := &configurableRule{AllowedLines: 60}
		cfg := NewFakeConfigSource()
		cfg.Set("complexity", "ConfigurableRule", "threshold", 200)

		ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
		if rule.AllowedLines != 200 {
			t.Errorf("alias only: AllowedLines = %d, want 200", rule.AllowedLines)
		}
	})
	t.Run("primary name wins over alias", func(t *testing.T) {
		rule := &configurableRule{AllowedLines: 60}
		cfg := NewFakeConfigSource()
		cfg.Set("complexity", "ConfigurableRule", "allowedLines", 300)
		cfg.Set("complexity", "ConfigurableRule", "threshold", 999)

		ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
		if rule.AllowedLines != 300 {
			t.Errorf("primary precedence: AllowedLines = %d, want 300 (primary should win)", rule.AllowedLines)
		}
	})
}

// Test 3: Regex option — an unanchored pattern gets ^...$ wrapping; an
// invalid pattern does not panic and leaves the target field nil.
func TestApplyConfig_3_RegexOption(t *testing.T) {
	t.Run("unanchored pattern is anchored", func(t *testing.T) {
		rule := &configurableRule{}
		cfg := NewFakeConfigSource()
		cfg.Set("complexity", "ConfigurableRule", "classPattern", "[a-z]+")

		ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
		if rule.ClassPattern == nil {
			t.Fatalf("ClassPattern = nil, want compiled regex")
		}
		src := rule.ClassPattern.String()
		if src != "^[a-z]+$" {
			t.Errorf("ClassPattern source = %q, want %q", src, "^[a-z]+$")
		}
		// Sanity: the anchored regex should reject substring matches.
		if rule.ClassPattern.MatchString("abc123") {
			t.Errorf("anchored regex should not match %q", "abc123")
		}
		if !rule.ClassPattern.MatchString("abc") {
			t.Errorf("anchored regex should match %q", "abc")
		}
	})

	t.Run("invalid pattern leaves field untouched", func(t *testing.T) {
		rule := &configurableRule{ClassPattern: regexp.MustCompile("^pre$")}
		cfg := NewFakeConfigSource()
		cfg.Set("complexity", "ConfigurableRule", "classPattern", "[unclosed")

		// Must not panic.
		ApplyConfig(rule, descriptorForConfigurableRule(), cfg)

		// The pre-existing pattern should still be in place.
		if rule.ClassPattern == nil || rule.ClassPattern.String() != "^pre$" {
			t.Errorf("ClassPattern mutated on invalid input: %v", rule.ClassPattern)
		}
	})
}

// Test 5: Ruleset disable — when the ruleset is inactive, the function
// returns false AND no option overrides are applied.
func TestApplyConfig_5_RuleSetDisable(t *testing.T) {
	rule := &configurableRule{AllowedLines: 60}
	cfg := NewFakeConfigSource()
	cfg.SetRuleSetActive("complexity", false)
	// These overrides would normally take effect — they must not when
	// the ruleset is disabled.
	cfg.Set("complexity", "ConfigurableRule", "allowedLines", 999)
	cfg.Set("complexity", "ConfigurableRule", "strict", true)

	active := ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
	if active {
		t.Errorf("active = true, want false when ruleset disabled")
	}
	if rule.AllowedLines != 60 {
		t.Errorf("AllowedLines = %d, want 60 (ruleset disable must skip options)", rule.AllowedLines)
	}
	if rule.Strict {
		t.Errorf("Strict = true, want false (ruleset disable must skip options)")
	}
}

// Test 6: Unknown YAML key — setting a value under a key the descriptor
// does not declare is silently ignored. No panic, no mutation.
func TestApplyConfig_6_UnknownYAMLKey(t *testing.T) {
	rule := &configurableRule{AllowedLines: 60}
	cfg := NewFakeConfigSource()
	cfg.Set("complexity", "ConfigurableRule", "unknownKey", 999)

	active := ApplyConfig(rule, descriptorForConfigurableRule(), cfg)
	if !active {
		t.Errorf("active = false, want true (unknown key should not disable)")
	}
	if rule.AllowedLines != 60 {
		t.Errorf("AllowedLines = %d, want 60 (unknown key must not mutate)", rule.AllowedLines)
	}
}

// Test 7: Missing key leaves field alone — the rule's AllowedLines
// defaults to 60, the config source has no override, and the field is
// untouched after ApplyConfig.
func TestApplyConfig_7_MissingKeyLeavesFieldAlone(t *testing.T) {
	rule := &configurableRule{
		AllowedLines:  60,
		UnchangedFlag: true,
		UnchangedList: []string{"x"},
	}
	cfg := NewFakeConfigSource()

	ApplyConfig(rule, descriptorForConfigurableRule(), cfg)

	if rule.AllowedLines != 60 {
		t.Errorf("AllowedLines = %d, want 60 (no override → no mutation)", rule.AllowedLines)
	}
	if !rule.UnchangedFlag {
		t.Errorf("UnchangedFlag = false, want true (no override → no mutation)")
	}
	if len(rule.UnchangedList) != 1 || rule.UnchangedList[0] != "x" {
		t.Errorf("UnchangedList = %v, want [x]", rule.UnchangedList)
	}
}

// Supplementary: rule-level active override flips active regardless of
// the DefaultActive in the descriptor. Covers the "active override
// applies but options still apply" path in ApplyConfig.
func TestApplyConfig_RuleLevelActiveOverride(t *testing.T) {
	d := descriptorForConfigurableRule()
	d.DefaultActive = false

	rule := &configurableRule{AllowedLines: 60}
	cfg := NewFakeConfigSource()
	cfg.SetRuleActive("complexity", "ConfigurableRule", true)
	cfg.Set("complexity", "ConfigurableRule", "allowedLines", 42)

	active := ApplyConfig(rule, d, cfg)
	if !active {
		t.Errorf("active = false, want true after rule-level enable")
	}
	if rule.AllowedLines != 42 {
		t.Errorf("AllowedLines = %d, want 42 (options apply even with rule-level override)", rule.AllowedLines)
	}
}

// TestApplyConfig_CustomApply exercises the CustomApply escape hatch. A
// fake rule with a flag + a non-trivial option verifies that (a) the hook
// runs after the Options loop (so it can override option-applied fields),
// (b) it's invoked even when the rule has zero options, and (c) it's not
// invoked when the ruleset is disabled (because ApplyConfig short-circuits
// before options and the hook).
func TestApplyConfig_CustomApply(t *testing.T) {
	type customRule struct {
		Flag          bool
		CustomRanWith ConfigSource
		AllowedLines  int
	}

	t.Run("hook runs after options and can override them", func(t *testing.T) {
		rule := &customRule{AllowedLines: 60}
		d := RuleDescriptor{
			ID:            "CustomRule",
			RuleSet:       "complexity",
			DefaultActive: true,
			Options: []ConfigOption{
				{
					Name: "allowedLines",
					Type: OptInt,
					Apply: func(target interface{}, value interface{}) {
						target.(*customRule).AllowedLines = value.(int)
					},
				},
			},
			CustomApply: func(target interface{}, cfg ConfigSource) {
				r := target.(*customRule)
				r.Flag = true
				r.CustomRanWith = cfg
				// Override the option-applied value to prove run order.
				r.AllowedLines = 999
			},
		}
		cfg := NewFakeConfigSource()
		cfg.Set("complexity", "CustomRule", "allowedLines", 120)

		active := ApplyConfig(rule, d, cfg)
		if !active {
			t.Fatalf("expected active=true")
		}
		if !rule.Flag {
			t.Errorf("CustomApply did not flip Flag")
		}
		if rule.CustomRanWith != cfg {
			t.Errorf("CustomApply received cfg=%v, want %v", rule.CustomRanWith, cfg)
		}
		if rule.AllowedLines != 999 {
			t.Errorf("AllowedLines=%d, want 999 (CustomApply runs after Options)", rule.AllowedLines)
		}
	})

	t.Run("hook runs when rule has no options", func(t *testing.T) {
		rule := &customRule{}
		d := RuleDescriptor{
			ID:            "NoOptionsRule",
			RuleSet:       "complexity",
			DefaultActive: true,
			CustomApply: func(target interface{}, cfg ConfigSource) {
				target.(*customRule).Flag = true
			},
		}
		ApplyConfig(rule, d, NewFakeConfigSource())
		if !rule.Flag {
			t.Errorf("CustomApply did not run with zero Options")
		}
	})

	t.Run("hook is skipped when ruleset is disabled", func(t *testing.T) {
		rule := &customRule{}
		d := RuleDescriptor{
			ID:            "CustomRule",
			RuleSet:       "complexity",
			DefaultActive: true,
			CustomApply: func(target interface{}, cfg ConfigSource) {
				target.(*customRule).Flag = true
			},
		}
		cfg := NewFakeConfigSource()
		cfg.SetRuleSetActive("complexity", false)
		active := ApplyConfig(rule, d, cfg)
		if active {
			t.Errorf("expected active=false")
		}
		if rule.Flag {
			t.Errorf("CustomApply ran despite ruleset disable (should be skipped)")
		}
	})

	t.Run("nil CustomApply is a no-op", func(t *testing.T) {
		rule := &customRule{AllowedLines: 60}
		d := RuleDescriptor{
			ID:            "CustomRule",
			RuleSet:       "complexity",
			DefaultActive: true,
			// CustomApply intentionally nil.
		}
		ApplyConfig(rule, d, NewFakeConfigSource())
		if rule.Flag {
			t.Errorf("Flag should remain false when CustomApply is nil")
		}
	})
}

// Supplementary: nil ConfigSource simply returns DefaultActive and leaves
// the rule untouched. This guards the ApplyConfig contract against callers
// that have no config yet.
func TestApplyConfig_NilConfig(t *testing.T) {
	rule := &configurableRule{AllowedLines: 60}
	d := descriptorForConfigurableRule()
	d.DefaultActive = true

	active := ApplyConfig(rule, d, nil)
	if !active {
		t.Errorf("active = false, want true when cfg is nil and DefaultActive true")
	}
	if rule.AllowedLines != 60 {
		t.Errorf("AllowedLines = %d, want 60 (nil cfg must not mutate)", rule.AllowedLines)
	}
}

// TestApplyConfigActiveOnly covers the reduced variant used when the caller
// has Meta() but no concrete struct pointer. Options must never be applied
// (if they were, the nil target would panic). Ruleset and rule-level
// active overrides must still be honored.
func TestApplyConfigActiveOnly(t *testing.T) {
	descDefault := func(active bool) RuleDescriptor {
		d := descriptorForConfigurableRule()
		d.DefaultActive = active
		return d
	}

	t.Run("NilConfigReturnsDefault", func(t *testing.T) {
		if got := ApplyConfigActiveOnly(descDefault(true), nil); !got {
			t.Errorf("got false, want true for DefaultActive=true")
		}
		if got := ApplyConfigActiveOnly(descDefault(false), nil); got {
			t.Errorf("got true, want false for DefaultActive=false")
		}
	})

	t.Run("RulesetDisableShortCircuits", func(t *testing.T) {
		fcs := NewFakeConfigSource()
		fcs.SetRuleSetActive("complexity", false)
		if got := ApplyConfigActiveOnly(descDefault(true), fcs); got {
			t.Errorf("ruleset disabled: got true, want false")
		}
	})

	t.Run("RuleEnableOverride", func(t *testing.T) {
		fcs := NewFakeConfigSource()
		fcs.SetRuleActive("complexity", "ConfigurableRule", true)
		if got := ApplyConfigActiveOnly(descDefault(false), fcs); !got {
			t.Errorf("rule enabled via config: got false, want true")
		}
	})

	t.Run("RuleDisableOverride", func(t *testing.T) {
		fcs := NewFakeConfigSource()
		fcs.SetRuleActive("complexity", "ConfigurableRule", false)
		if got := ApplyConfigActiveOnly(descDefault(true), fcs); got {
			t.Errorf("rule disabled via config: got true, want false")
		}
	})

	t.Run("OptionsAreIgnored", func(t *testing.T) {
		// Even when config has option values, ApplyConfigActiveOnly must
		// not attempt to invoke Apply (no target pointer to downcast).
		// Sanity check: this path should not panic.
		fcs := NewFakeConfigSource()
		fcs.Set("complexity", "ConfigurableRule", "allowedLines", 999)
		_ = ApplyConfigActiveOnly(descDefault(true), fcs)
	})
}
