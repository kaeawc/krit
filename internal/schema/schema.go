package schema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/jsonschema"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// RuleMeta describes a rule's configurable options for schema generation.
type RuleMeta struct {
	Name            string
	Description     string
	RuleSet         string
	Active          bool // default active state
	Fixable         bool
	FixLevel        string // cosmetic/idiomatic/semantic or ""
	Precision       string // heuristic/ast-backed/project-structure/type-aware/policy
	Capabilities    []string
	LanguageSupport map[string]api.LanguageSupport
	Options         []OptionMeta
}

// OptionMeta describes one configurable option of a rule.
type OptionMeta struct {
	Name        string
	Type        string // "int", "bool", "string", "string[]", "regex"
	Default     interface{}
	Description string
}

// CollectRuleMeta walks api.Registry and builds metadata for every rule,
// reading configurable options from each rule's Meta() descriptor via
// rules.MetaForRule (which falls back to the metaByName index for
// adapter-wrapped rules that drop the concrete struct pointer).
func CollectRuleMeta() []RuleMeta {
	metas := make([]RuleMeta, 0, len(api.Registry))
	for _, r := range api.Registry {
		active := rules.IsDefaultActive(r.ID)

		fixLvl, fixable := rules.GetV2FixLevel(r)
		fixLevel := ""
		if fixable {
			fixLevel = fixLvl.String()
		}

		var opts []OptionMeta
		var languageSupport map[string]api.LanguageSupport
		precision := ""
		if desc, ok := rules.MetaForRule(r); ok {
			opts = descriptorOptions(desc)
			languageSupport = copyLanguageSupport(desc.LanguageSupport)
			precision = desc.Precision.String()
		}

		metas = append(metas, RuleMeta{
			Name:            r.ID,
			Description:     r.Description,
			RuleSet:         r.Category,
			Active:          active,
			Fixable:         fixable,
			FixLevel:        fixLevel,
			Precision:       precision,
			Capabilities:    r.CapabilitiesList(),
			LanguageSupport: languageSupport,
			Options:         opts,
		})
	}
	return metas
}

func copyLanguageSupport(in map[string]api.LanguageSupport) map[string]api.LanguageSupport {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]api.LanguageSupport, len(in))
	for lang, support := range in {
		out[lang] = support
	}
	return out
}

// descriptorOptions converts api.ConfigOption values into the
// schema-facing OptionMeta shape. Each option emits:
//   - one primary entry keyed by ConfigOption.Name
//   - one alias entry per ConfigOption.Aliases[], with
//     `Alias for <primary>.` as the description
//
// This matches the shape the legacy hand-written knownRuleOptions() used
// to emit so the JSON Schema and validator behavior stay identical.
func descriptorOptions(d api.RuleDescriptor) []OptionMeta {
	if len(d.Options) == 0 {
		return nil
	}
	out := make([]OptionMeta, 0, len(d.Options))
	for _, opt := range d.Options {
		out = append(out, OptionMeta{
			Name:        opt.Name,
			Type:        optionTypeString(opt.Type),
			Default:     optionDefault(opt),
			Description: opt.Description,
		})
		for _, alias := range opt.Aliases {
			out = append(out, OptionMeta{
				Name:        alias,
				Type:        optionTypeString(opt.Type),
				Description: fmt.Sprintf("Alias for %s.", opt.Name),
			})
		}
	}
	return out
}

// optionTypeString maps api.OptionType to the schema's string tag.
func optionTypeString(t api.OptionType) string {
	switch t {
	case api.OptInt:
		return "int"
	case api.OptBool:
		return "bool"
	case api.OptString:
		return "string"
	case api.OptRegex:
		return "regex"
	case api.OptStringList:
		return "string[]"
	default:
		return "string"
	}
}

// optionDefault returns the default value suitable for embedding in the
// JSON Schema. String-list options with a nil default are omitted so the
// schema doesn't fabricate a `default: []` field that didn't exist before.
func optionDefault(opt api.ConfigOption) interface{} {
	if opt.Default == nil {
		return nil
	}
	switch opt.Type {
	case api.OptStringList:
		// Only emit when the default is a non-empty concrete slice.
		if s, ok := opt.Default.([]string); ok && len(s) > 0 {
			return s
		}
		return nil
	}
	return opt.Default
}

// GenerateSchema produces the krit.yml JSON Schema as a typed
// *jsonschema.Schema. The result implements json.Marshaler, so callers
// that emit JSON can hand it directly to json.NewEncoder.Encode or
// json.MarshalIndent.
func GenerateSchema(metas []RuleMeta) *jsonschema.Schema {
	bySet := map[string][]RuleMeta{}
	for _, m := range metas {
		bySet[m.RuleSet] = append(bySet[m.RuleSet], m)
	}

	props := map[string]*jsonschema.Schema{}

	props["config"] = jsonschema.Object(map[string]*jsonschema.Schema{
		"validation":       jsonschema.Boolean("").WithDefault(true),
		"warningsAsErrors": jsonschema.Boolean("").WithDefault(false),
	}).WithDescription("Global krit configuration.").AdditionalPropertiesFalse()

	props["maxCost"] = jsonschema.StringEnum(
		[]string{"trivial", "line", "ast", "crossfile", "oracle", "fir", "fast", "balanced", "thorough"},
		"Maximum rule weight class to run. Filters the active rule set so higher-cost rules are skipped. Presets: fast≡ast, balanced≡crossfile, thorough≡fir.",
	)

	props["module_template"] = jsonschema.Object(map[string]*jsonschema.Schema{
		"feature_root":        jsonschema.String("Gradle path glob for feature root modules, for example feature:*."),
		"required_submodules": jsonschema.Array(jsonschema.String(""), "Child module names required below each matching feature root."),
		"required_plugins":    jsonschema.Array(jsonschema.String(""), "Gradle plugin IDs required on each matching feature root build script."),
	}).WithDescription("Feature-module template conformance settings.").AdditionalPropertiesFalse()

	sloItem := jsonschema.Object(map[string]*jsonschema.Schema{
		"module":                jsonschema.String("Gradle module path, for example :core."),
		"max_warnings_per_kloc": jsonschema.Number("Maximum warning findings per 1,000 main-source lines."),
		"max_errors_per_kloc":   jsonschema.Number("Maximum error findings per 1,000 main-source lines."),
	}).AdditionalPropertiesFalse().WithRequired("module")
	props["slos"] = jsonschema.Array(sloItem, "Per-module finding-density service-level objectives.")

	props["testSourcePaths"] = jsonschema.Array(jsonschema.String(""), "Additional path substrings treated as test sources.")
	props["testSourcePathsOverride"] = jsonschema.Array(jsonschema.String(""), "Replacement path substrings treated as test sources; wins over testSourcePaths and defaults.")

	customRulesItem := jsonschema.Object(map[string]*jsonschema.Schema{
		"id":                 jsonschema.String("Rule ID used in findings and config toggles."),
		"ruleset":            jsonschema.String("Ruleset/category for the rule. Defaults to custom."),
		"description":        jsonschema.String("Human-readable rule description."),
		"severity":           jsonschema.StringEnum([]string{"error", "warning", "info"}, "Finding severity. Defaults to warning."),
		"defaultActive":      jsonschema.Boolean("Whether the rule is active by default. Defaults to true."),
		"nodeTypes":          jsonschema.Array(jsonschema.String(""), "Tree-sitter node types to inspect."),
		"languages":          jsonschema.Array(jsonschema.StringEnum([]string{"kotlin", "java", "xml", "gradle", "version-catalog"}, ""), "Languages the rule applies to. Omit to use dispatcher defaults."),
		"match":              jsonschema.StringEnum([]string{"nodeText", "stringLiteralBody"}, "Text to match. stringLiteralBody unwraps Java/Kotlin string literals."),
		"pattern":            jsonschema.String("Go regular expression matched against the selected text."),
		"message":            jsonschema.String("Finding message emitted when the pattern matches."),
		"confidence":         jsonschema.Number("Finding confidence. Defaults to 0.75."),
		"ignorePlaceholders": jsonschema.Boolean("Ignore values containing common placeholder markers."),
	}).AdditionalPropertiesFalse().WithRequired("id", "pattern", "message")
	props["customRules"] = jsonschema.Array(customRulesItem, "Config-defined pattern rules registered at runtime.")

	for _, setName := range sortedKeys(bySet) {
		setMetas := bySet[setName]
		ruleProps := map[string]*jsonschema.Schema{
			"active": jsonschema.Boolean(fmt.Sprintf("Enable or disable the entire %s ruleset.", setName)).WithDefault(true),
		}
		for _, m := range setMetas {
			ruleProps[m.Name] = ruleSchema(m)
		}
		props[setName] = jsonschema.Object(ruleProps).
			WithDescription(fmt.Sprintf("Configuration for the %s ruleset.", setName)).
			AdditionalPropertiesFalse()
	}

	return jsonschema.Object(props).
		WithSchemaURI("https://json-schema.org/draft/2020-12/schema").
		WithID("https://raw.githubusercontent.com/kaeawc/krit/main/schemas/krit-config.schema.json").
		WithTitle("Krit Configuration").
		WithDescription("Schema for krit.yml Kotlin static analysis configuration.").
		AdditionalPropertiesFalse()
}

func ruleSchema(m RuleMeta) *jsonschema.Schema {
	ruleProps := map[string]*jsonschema.Schema{
		"active":   jsonschema.Boolean(fmt.Sprintf("Enable %s (default: %v).", m.Name, m.Active)).WithDefault(m.Active),
		"excludes": jsonschema.Array(jsonschema.String(""), "Glob patterns for paths to exclude from this rule."),
	}

	for _, opt := range m.Options {
		ruleProps[opt.Name] = optionSchema(opt)
	}

	desc := m.Description
	if desc == "" {
		desc = fmt.Sprintf("Configuration for the %s rule.", m.Name)
	}
	if m.Fixable {
		desc += fmt.Sprintf(" [fixable: %s]", m.FixLevel)
	}
	if m.Precision != "" && m.Precision != "unset" {
		desc += fmt.Sprintf(" [precision: %s]", m.Precision)
	}
	if len(m.Capabilities) > 0 {
		desc += fmt.Sprintf(" [capabilities: %s]", strings.Join(m.Capabilities, ", "))
	}

	return jsonschema.Object(ruleProps).
		WithDescription(desc).
		AdditionalPropertiesFalse()
}

func optionSchema(opt OptionMeta) *jsonschema.Schema {
	var s *jsonschema.Schema
	switch opt.Type {
	case "int":
		s = jsonschema.Integer(opt.Description)
	case "bool":
		s = jsonschema.Boolean(opt.Description)
	case "string":
		s = jsonschema.String(opt.Description)
	case "regex":
		s = &jsonschema.Schema{Type: "string", Format: "regex", Description: opt.Description}
	case "string[]":
		s = jsonschema.Array(jsonschema.String(""), opt.Description)
	default:
		s = &jsonschema.Schema{Description: opt.Description}
	}
	if opt.Default != nil {
		s.Default = opt.Default
	}
	return s
}

func sortedKeys(m map[string][]RuleMeta) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// KnownRuleSets returns the set of known ruleset names from the registry.
func KnownRuleSets() map[string]bool {
	sets := map[string]bool{"config": true, "module_template": true, "slos": true, "testSourcePaths": true, "testSourcePathsOverride": true}
	for _, r := range api.Registry {
		sets[r.Category] = true
	}
	return sets
}

// KnownRulesBySet returns a map of ruleset -> set of rule names.
func KnownRulesBySet() map[string]map[string]bool {
	bySet := map[string]map[string]bool{}
	for _, r := range api.Registry {
		rs := r.Category
		if bySet[rs] == nil {
			bySet[rs] = map[string]bool{}
		}
		bySet[rs][r.ID] = true
	}
	return bySet
}

// KnownOptionsByRule returns a map of rule name -> set of allowed config keys.
// Always includes "active" and "excludes" as standard keys. Option keys are
// read from each rule's Meta() descriptor via rules.MetaForRule.
func KnownOptionsByRule() map[string]map[string]OptionType {
	result := map[string]map[string]OptionType{}
	for _, r := range api.Registry {
		keys := map[string]OptionType{
			"active":   OptionTypeBool,
			"excludes": OptionTypeStringSlice,
		}
		if desc, ok := rules.MetaForRule(r); ok {
			for _, opt := range desc.Options {
				t := parseOptionType(optionTypeString(opt.Type))
				keys[opt.Name] = t
				for _, alias := range opt.Aliases {
					keys[alias] = t
				}
			}
		}
		result[r.ID] = keys
	}
	return result
}

// OptionType represents the expected type of a config option.
type OptionType int

const (
	OptionTypeBool OptionType = iota
	OptionTypeInt
	OptionTypeString
	OptionTypeStringSlice
	OptionTypeRegex
)

func parseOptionType(s string) OptionType {
	switch s {
	case "int":
		return OptionTypeInt
	case "bool":
		return OptionTypeBool
	case "string":
		return OptionTypeString
	case "string[]":
		return OptionTypeStringSlice
	case "regex":
		return OptionTypeRegex
	default:
		return OptionTypeString
	}
}
