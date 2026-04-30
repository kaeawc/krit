package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules" // ensure rules are registered via init()
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

func TestGenerateSchema_ValidJSON(t *testing.T) {
	metas := CollectRuleMeta()
	schema := GenerateSchema(metas)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("GenerateSchema produced invalid JSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GenerateSchema produced empty JSON")
	}

	// Verify it round-trips
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Generated JSON does not round-trip: %v", err)
	}
}

func TestGenerateSchema_HasAllRuleSets(t *testing.T) {
	metas := CollectRuleMeta()
	schema := GenerateSchema(metas)

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing 'properties'")
	}

	// Check that all rulesets from registry are present
	knownSets := KnownRuleSets()
	for set := range knownSets {
		if _, ok := props[set]; !ok {
			t.Errorf("schema missing ruleset '%s'", set)
		}
	}
}

func TestGenerateSchema_HasConfigurableRules(t *testing.T) {
	metas := CollectRuleMeta()
	schema := GenerateSchema(metas)

	props := schema["properties"].(map[string]interface{})

	// Check that complexity.LongMethod has allowedLines
	complexity, ok := props["complexity"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing 'complexity'")
	}
	complexityProps := complexity["properties"].(map[string]interface{})
	longMethod, ok := complexityProps["LongMethod"].(map[string]interface{})
	if !ok {
		t.Fatal("complexity missing 'LongMethod'")
	}
	lmProps := longMethod["properties"].(map[string]interface{})
	if _, ok := lmProps["allowedLines"]; !ok {
		t.Error("LongMethod schema missing 'allowedLines'")
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "active", true)
	cfg.Set("complexity", "LongMethod", "allowedLines", 100)

	errs := ValidateConfig(cfg)
	for _, e := range errs {
		if e.Level == "error" {
			t.Errorf("unexpected error: %s", e)
		}
	}
}

func TestValidateConfig_UnknownRuleSet(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Data()["bogusRuleSet"] = map[string]interface{}{
		"active": true,
	}

	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Path == "bogusRuleSet" && e.Level == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for unknown ruleset 'bogusRuleSet'")
	}
}

func TestValidateConfig_UnknownRule(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "NonExistentRule", "active", true)

	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Path == "complexity.NonExistentRule" && e.Level == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for unknown rule 'NonExistentRule'")
	}
}

func TestValidateConfig_UnknownKey(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("complexity", "LongMethod", "bogusKey", 42)

	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Path == "complexity.LongMethod.bogusKey" && e.Level == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for unknown config key 'bogusKey'")
	}
}

func TestValidateConfig_WrongType(t *testing.T) {
	cfg := config.NewConfig()
	// allowedLines expects int, pass a string
	cfg.Set("complexity", "LongMethod", "allowedLines", "not-a-number")

	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Path == "complexity.LongMethod.allowedLines" && e.Level == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected type error for string where int expected")
	}
}

func TestValidateConfig_InvalidNamingRegex(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("naming", "ClassNaming", "classPattern", "[unclosed")

	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Path == "naming.ClassNaming.classPattern" && e.Level == "error" {
			found = true
			if !strings.Contains(e.Message, "invalid regex") {
				t.Fatalf("regex validation message = %q, want invalid regex", e.Message)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected regex validation error for naming.ClassNaming.classPattern")
	}
}

func TestValidateConfig_UnanchoredNamingRegexAccepted(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("naming", "ClassNaming", "classPattern", "[A-Z][a-z]+")

	errs := ValidateConfig(cfg)
	for _, e := range errs {
		if e.Level == "error" {
			t.Fatalf("unexpected validation error: %s", e)
		}
	}
}

func TestKnownRuleSets_NonEmpty(t *testing.T) {
	sets := KnownRuleSets()
	if len(sets) == 0 {
		t.Fatal("KnownRuleSets returned empty map")
	}
	for _, name := range []string{"style", "complexity", "naming"} {
		if !sets[name] {
			t.Errorf("KnownRuleSets missing expected set %q", name)
		}
	}
}

func TestKnownRulesBySet_StyleContainsMagicNumber(t *testing.T) {
	bySet := KnownRulesBySet()
	if len(bySet) == 0 {
		t.Fatal("KnownRulesBySet returned empty map")
	}
	styleRules, ok := bySet["style"]
	if !ok {
		t.Fatal("KnownRulesBySet missing 'style' set")
	}
	if !styleRules["MagicNumber"] {
		t.Error("style set missing 'MagicNumber' rule")
	}
}

func TestKnownRulesBySet_UnknownSetEmpty(t *testing.T) {
	bySet := KnownRulesBySet()
	if rules, ok := bySet["nonexistent-set-xyz"]; ok && len(rules) > 0 {
		t.Errorf("expected no rules for unknown set, got %d", len(rules))
	}
}

func TestKnownOptionsByRule_MagicNumberHasIgnoreNumbers(t *testing.T) {
	opts := KnownOptionsByRule()
	if len(opts) == 0 {
		t.Fatal("KnownOptionsByRule returned empty map")
	}
	mnOpts, ok := opts["MagicNumber"]
	if !ok {
		t.Fatal("KnownOptionsByRule missing 'MagicNumber'")
	}
	// Every rule should have "active" and "excludes"
	if _, ok := mnOpts["active"]; !ok {
		t.Error("MagicNumber missing standard 'active' option")
	}
	if _, ok := mnOpts["excludes"]; !ok {
		t.Error("MagicNumber missing standard 'excludes' option")
	}
	// MagicNumber-specific option
	if _, ok := mnOpts["ignoreNumbers"]; !ok {
		t.Error("MagicNumber missing 'ignoreNumbers' option")
	}
}

func TestKnownOptionsByRule_UnknownRuleEmpty(t *testing.T) {
	opts := KnownOptionsByRule()
	if ruleOpts, ok := opts["NonExistentRuleXYZ"]; ok && len(ruleOpts) > 0 {
		t.Errorf("expected no options for unknown rule, got %d", len(ruleOpts))
	}
}

func TestCollectRuleMeta_NotEmpty(t *testing.T) {
	metas := CollectRuleMeta()
	if len(metas) == 0 {
		t.Fatal("CollectRuleMeta returned no rules")
	}

	// Verify at least one rule has options
	hasOptions := false
	for _, m := range metas {
		if len(m.Options) > 0 {
			hasOptions = true
			break
		}
	}
	if !hasOptions {
		t.Error("no rules have configurable options")
	}
}

func TestCollectRuleMeta_IncludesJavaSupportClassification(t *testing.T) {
	metas := CollectRuleMeta()
	for _, m := range metas {
		if m.Name != "AddJavascriptInterface" {
			continue
		}
		support, ok := m.LanguageSupport[rules.JavaLanguageSupportKey]
		if !ok {
			t.Fatal("AddJavascriptInterface missing Java support classification")
		}
		if support.Status != v2rules.LanguageSupportSupported {
			t.Fatalf("AddJavascriptInterface Java support = %s, want %s", support.Status, v2rules.LanguageSupportSupported)
		}
		return
	}
	t.Fatal("AddJavascriptInterface metadata not found")
}

// TestControversialRulesRegistry parses config/onboarding/controversial-rules.json
// and asserts that every rule name references a registered rule and every
// fixture path exists on disk. This guards the registry against bitrot
// when rules are renamed or fixtures move.
func TestControversialRulesRegistry(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	path := filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading registry: %v", err)
	}

	var registry struct {
		SchemaVersion int `json:"schemaVersion"`
		Questions     []struct {
			ID              string          `json:"id"`
			Question        string          `json:"question"`
			Rationale       string          `json:"rationale"`
			Rules           []string        `json:"rules"`
			CascadeFrom     *string         `json:"cascade_from"`
			CascadesTo      []string        `json:"cascades_to"`
			Defaults        map[string]bool `json:"defaults"`
			PositiveFixture *string         `json:"positive_fixture"`
			NegativeFixture *string         `json:"negative_fixture"`
			Kind            string          `json:"kind"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatalf("parsing registry: %v", err)
	}

	if registry.SchemaVersion != 1 {
		t.Errorf("unexpected schemaVersion %d", registry.SchemaVersion)
	}

	known := make(map[string]bool, len(v2rules.Registry))
	for _, r := range v2rules.Registry {
		known[r.ID] = true
	}

	ids := make(map[string]bool, len(registry.Questions))
	expectedProfiles := []string{"strict", "balanced", "relaxed", "detekt-compat"}

	for _, q := range registry.Questions {
		if q.ID == "" {
			t.Error("question missing id")
			continue
		}
		if ids[q.ID] {
			t.Errorf("%s: duplicate question id", q.ID)
		}
		ids[q.ID] = true

		if q.Question == "" {
			t.Errorf("%s: missing question text", q.ID)
		}
		if q.Rationale == "" {
			t.Errorf("%s: missing rationale", q.ID)
		}
		if q.Kind != "rule" && q.Kind != "parent" {
			t.Errorf("%s: invalid kind %q", q.ID, q.Kind)
		}

		for _, name := range q.Rules {
			if !known[name] {
				t.Errorf("%s: rule %q is not registered", q.ID, name)
			}
		}

		for _, profile := range expectedProfiles {
			if _, ok := q.Defaults[profile]; !ok {
				t.Errorf("%s: missing default for profile %q", q.ID, profile)
			}
		}

		if q.PositiveFixture != nil {
			if _, err := os.Stat(filepath.Join(repoRoot, *q.PositiveFixture)); err != nil {
				t.Errorf("%s: positive fixture missing: %v", q.ID, err)
			}
		}
		if q.NegativeFixture != nil {
			if _, err := os.Stat(filepath.Join(repoRoot, *q.NegativeFixture)); err != nil {
				t.Errorf("%s: negative fixture missing: %v", q.ID, err)
			}
		}
	}

	for _, q := range registry.Questions {
		if q.CascadeFrom != nil && !ids[*q.CascadeFrom] {
			t.Errorf("%s: cascade_from %q is not a known question id", q.ID, *q.CascadeFrom)
		}
		for _, target := range q.CascadesTo {
			if !ids[target] {
				t.Errorf("%s: cascades_to %q is not a known question id", q.ID, target)
			}
		}
	}
}

// TestOnboardingProfilesValidate ensures every profile shipped under
// config/profiles/ loads and validates against the current rule
// registry. Adding a new profile or renaming a ruleset will fail this
// test until the profile is updated in lockstep.
func TestOnboardingProfilesValidate(t *testing.T) {
	profiles := []string{"balanced", "strict", "relaxed", "detekt-compat"}
	repoRoot := filepath.Join("..", "..")

	for _, name := range profiles {
		name := name
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(repoRoot, "config", "profiles", name+".yml")
			cfg, err := config.LoadConfig(path)
			if err != nil {
				t.Fatalf("loading profile %s: %v", name, err)
			}
			errs := ValidateConfig(cfg)
			for _, e := range errs {
				if e.Level == "error" {
					t.Errorf("%s: %s", name, e)
				}
			}
		})
	}
}
