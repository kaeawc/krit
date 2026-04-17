package rules

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// TestConfigAdapter_HasKey covers the core presence-vs-absence contract that
// registry.ApplyConfig relies on.
func TestConfigAdapter_HasKey(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "threshold", 80)
	cfg.Set("complexity", "LongMethod", "active", false)

	a := NewConfigAdapter(cfg)

	if !a.HasKey("complexity", "LongMethod", "threshold") {
		t.Errorf("HasKey(threshold) = false, want true")
	}
	if !a.HasKey("complexity", "LongMethod", "active") {
		t.Errorf("HasKey(active) = false, want true")
	}
	if a.HasKey("complexity", "LongMethod", "missing") {
		t.Errorf("HasKey(missing) = true, want false")
	}
	if a.HasKey("complexity", "MissingRule", "threshold") {
		t.Errorf("HasKey on missing rule = true, want false")
	}
	if a.HasKey("missing-set", "LongMethod", "threshold") {
		t.Errorf("HasKey on missing set = true, want false")
	}
}

// TestConfigAdapter_Nil verifies the nil-safety contract.
func TestConfigAdapter_Nil(t *testing.T) {
	var a *ConfigAdapter
	if a.HasKey("a", "b", "c") {
		t.Errorf("nil HasKey = true, want false")
	}
	if a.GetInt("a", "b", "c", 42) != 42 {
		t.Errorf("nil GetInt should return default")
	}
	if a.GetBool("a", "b", "c", true) != true {
		t.Errorf("nil GetBool should return default")
	}
	if a.GetString("a", "b", "c", "d") != "d" {
		t.Errorf("nil GetString should return default")
	}
	if a.GetStringList("a", "b", "c") != nil {
		t.Errorf("nil GetStringList should return nil")
	}
	if a.IsRuleActive("a", "b") != nil {
		t.Errorf("nil IsRuleActive should return nil")
	}
	if a.IsRuleSetActive("a") != nil {
		t.Errorf("nil IsRuleSetActive should return nil")
	}
}

// TestConfigAdapter_Getters covers the GetXxx methods with present and
// absent values across all option types.
func TestConfigAdapter_Getters(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "threshold", 80)
	cfg.Set("complexity", "LongMethod", "ignoreThings", true)
	cfg.Set("complexity", "LongMethod", "pattern", "foo.*bar")
	cfg.Set("complexity", "LongMethod", "names", []interface{}{"a", "b", "c"})

	a := NewConfigAdapter(cfg)

	if got := a.GetInt("complexity", "LongMethod", "threshold", 60); got != 80 {
		t.Errorf("GetInt threshold = %d, want 80", got)
	}
	if got := a.GetInt("complexity", "LongMethod", "missing", 42); got != 42 {
		t.Errorf("GetInt missing = %d, want 42 (default)", got)
	}
	if !a.GetBool("complexity", "LongMethod", "ignoreThings", false) {
		t.Errorf("GetBool ignoreThings = false, want true")
	}
	if got := a.GetString("complexity", "LongMethod", "pattern", ""); got != "foo.*bar" {
		t.Errorf("GetString pattern = %q, want %q", got, "foo.*bar")
	}
	got := a.GetStringList("complexity", "LongMethod", "names")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetStringList names = %v, want %v", got, want)
	}
}

// TestConfigAdapter_IsRuleActive verifies the presence-as-override
// semantics of IsRuleActive / IsRuleSetActive.
func TestConfigAdapter_IsRuleActive(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "active", false)
	cfg.Set("naming", "ClassNaming", "active", true)

	a := NewConfigAdapter(cfg)

	ra := a.IsRuleActive("complexity", "LongMethod")
	if ra == nil || *ra != false {
		t.Errorf("IsRuleActive(LongMethod) = %v, want *false", ra)
	}
	rb := a.IsRuleActive("naming", "ClassNaming")
	if rb == nil || *rb != true {
		t.Errorf("IsRuleActive(ClassNaming) = %v, want *true", rb)
	}
	if a.IsRuleActive("complexity", "SomethingElse") != nil {
		t.Errorf("IsRuleActive on missing rule should be nil")
	}
}

// TestConfigAdapter_IsRuleSetActive verifies ruleset-level overrides, which
// require poking a key directly into the top-level map (Set only nests).
func TestConfigAdapter_IsRuleSetActive(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "threshold", 80)
	// Prime the ruleset-level active flag by reaching into the raw data
	// map (Set always nests under ruleSet.rule.key).
	data := cfg.Data()
	rsMap, ok := data["complexity"].(map[string]interface{})
	if !ok {
		rsMap = make(map[string]interface{})
		data["complexity"] = rsMap
	}
	rsMap["active"] = false

	a := NewConfigAdapter(cfg)
	rsa := a.IsRuleSetActive("complexity")
	if rsa == nil || *rsa != false {
		t.Errorf("IsRuleSetActive(complexity) = %v, want *false", rsa)
	}
	if a.IsRuleSetActive("missing") != nil {
		t.Errorf("IsRuleSetActive(missing) should be nil")
	}
}

// TestConfigAdapter_AliasFallback exercises registry.ApplyConfig's primary-
// wins-over-alias behavior via the adapter: when both `allowedLines` and
// `threshold` are set, the primary key wins.
func TestConfigAdapter_AliasFallback(t *testing.T) {
	// Case 1: only alias set — alias should fire.
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "threshold", 40)
	rule := &LongMethodRule{AllowedLines: 60}
	adapter := NewConfigAdapter(cfg)

	// Build a minimal descriptor matching the generated shape so we can
	// verify registry.ApplyConfig's probe order independently of the
	// generator.
	desc := registry.RuleDescriptor{
		ID:      "LongMethod",
		RuleSet: "complexity",
		Options: []registry.ConfigOption{{
			Name:    "allowedLines",
			Aliases: []string{"threshold"},
			Type:    registry.OptInt,
			Default: 60,
			Apply: func(target interface{}, value interface{}) {
				target.(*LongMethodRule).AllowedLines = value.(int)
			},
		}},
	}
	registry.ApplyConfig(rule, desc, adapter)
	if rule.AllowedLines != 40 {
		t.Errorf("alias-only: AllowedLines = %d, want 40", rule.AllowedLines)
	}

	// Case 2: primary set AND alias set — primary wins.
	cfg2 := config.NewConfig()
	cfg2.Set("complexity", "LongMethod", "allowedLines", 100)
	cfg2.Set("complexity", "LongMethod", "threshold", 40)
	rule2 := &LongMethodRule{AllowedLines: 60}
	registry.ApplyConfig(rule2, desc, NewConfigAdapter(cfg2))
	if rule2.AllowedLines != 100 {
		t.Errorf("primary-wins: AllowedLines = %d, want 100 (primary must win over alias)", rule2.AllowedLines)
	}
}
