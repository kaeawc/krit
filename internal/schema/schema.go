package schema

import (
	"fmt"
	"sort"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// RuleMeta describes a rule's configurable options for schema generation.
type RuleMeta struct {
	Name        string
	Description string
	RuleSet     string
	Active      bool // default active state
	Fixable     bool
	FixLevel    string // cosmetic/idiomatic/semantic or ""
	Options     []OptionMeta
}

// OptionMeta describes one configurable option of a rule.
type OptionMeta struct {
	Name        string
	Type        string // "int", "bool", "string", "string[]"
	Default     interface{}
	Description string
}

// CollectRuleMeta walks the rules.Registry and builds metadata for every rule,
// reading configurable options from each rule's Meta() descriptor via
// rules.MetaForRule (which falls back to the generated metaByName index
// for adapter-wrapped rules that drop the concrete struct pointer).
func CollectRuleMeta() []RuleMeta {
	rules.RegisterV2Rules()

	metas := make([]RuleMeta, 0, len(rules.Registry))
	for _, r := range rules.Registry {
		active := rules.IsDefaultActive(r.Name())

		fixable := false
		fixLevel := ""
		if _, ok := r.(rules.FixableRule); ok {
			fixable = true
			fixLevel = rules.GetFixLevel(r).String()
		}

		var opts []OptionMeta
		if desc, ok := rules.MetaForRule(r); ok {
			opts = descriptorOptions(desc)
		}

		metas = append(metas, RuleMeta{
			Name:        r.Name(),
			Description: r.Description(),
			RuleSet:     r.RuleSet(),
			Active:      active,
			Fixable:     fixable,
			FixLevel:    fixLevel,
			Options:     opts,
		})
	}
	return metas
}

// descriptorOptions converts registry.ConfigOption values into the
// schema-facing OptionMeta shape. Each option emits:
//   - one primary entry keyed by ConfigOption.Name
//   - one alias entry per ConfigOption.Aliases[], with
//     `Alias for <primary>.` as the description
//
// This matches the shape the legacy hand-written knownRuleOptions() used
// to emit so the JSON Schema and validator behavior stay identical.
func descriptorOptions(d registry.RuleDescriptor) []OptionMeta {
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

// optionTypeString maps registry.OptionType to the schema's string tag.
func optionTypeString(t registry.OptionType) string {
	switch t {
	case registry.OptInt:
		return "int"
	case registry.OptBool:
		return "bool"
	case registry.OptString, registry.OptRegex:
		return "string"
	case registry.OptStringList:
		return "string[]"
	default:
		return "string"
	}
}

// optionDefault returns the default value suitable for embedding in the
// JSON Schema. String-list options with a nil default are omitted so the
// schema doesn't fabricate a `default: []` field that didn't exist before.
func optionDefault(opt registry.ConfigOption) interface{} {
	if opt.Default == nil {
		return nil
	}
	switch opt.Type {
	case registry.OptStringList:
		// Only emit when the default is a non-empty concrete slice.
		if s, ok := opt.Default.([]string); ok && len(s) > 0 {
			return s
		}
		return nil
	}
	return opt.Default
}

// GenerateSchema produces a JSON Schema (as nested maps) for krit.yml configuration.
func GenerateSchema(metas []RuleMeta) map[string]interface{} {
	// Group by ruleset
	bySet := map[string][]RuleMeta{}
	for _, m := range metas {
		bySet[m.RuleSet] = append(bySet[m.RuleSet], m)
	}

	props := map[string]interface{}{}

	// Top-level config section
	props["config"] = map[string]interface{}{
		"type":        "object",
		"description": "Global krit configuration.",
		"properties": map[string]interface{}{
			"validation":       map[string]interface{}{"type": "boolean", "default": true},
			"warningsAsErrors": map[string]interface{}{"type": "boolean", "default": false},
		},
		"additionalProperties": false,
	}

	// Each ruleset
	setNames := sortedKeys(bySet)
	for _, setName := range setNames {
		setMetas := bySet[setName]
		ruleProps := map[string]interface{}{
			"active": map[string]interface{}{
				"type":        "boolean",
				"description": fmt.Sprintf("Enable or disable the entire %s ruleset.", setName),
				"default":     true,
			},
		}
		for _, m := range setMetas {
			ruleProps[m.Name] = ruleSchema(m)
		}
		props[setName] = map[string]interface{}{
			"type":                 "object",
			"description":          fmt.Sprintf("Configuration for the %s ruleset.", setName),
			"properties":           ruleProps,
			"additionalProperties": false,
		}
	}

	schema := map[string]interface{}{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://raw.githubusercontent.com/kaeawc/krit/main/schemas/krit-config.schema.json",
		"title":                "Krit Configuration",
		"description":          "Schema for krit.yml Kotlin static analysis configuration.",
		"type":                 "object",
		"additionalProperties": false,
		"properties":           props,
	}
	return schema
}

func ruleSchema(m RuleMeta) map[string]interface{} {
	ruleProps := map[string]interface{}{
		"active": map[string]interface{}{
			"type":        "boolean",
			"description": fmt.Sprintf("Enable %s (default: %v).", m.Name, m.Active),
			"default":     m.Active,
		},
		"excludes": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Glob patterns for paths to exclude from this rule.",
		},
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

	return map[string]interface{}{
		"type":                 "object",
		"description":          desc,
		"properties":           ruleProps,
		"additionalProperties": false,
	}
}

func optionSchema(opt OptionMeta) map[string]interface{} {
	s := map[string]interface{}{}
	if opt.Description != "" {
		s["description"] = opt.Description
	}
	switch opt.Type {
	case "int":
		s["type"] = "integer"
	case "bool":
		s["type"] = "boolean"
	case "string":
		s["type"] = "string"
	case "string[]":
		s["type"] = "array"
		s["items"] = map[string]interface{}{"type": "string"}
	}
	if opt.Default != nil {
		s["default"] = opt.Default
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
	rules.RegisterV2Rules()
	sets := map[string]bool{"config": true}
	for _, r := range rules.Registry {
		sets[r.RuleSet()] = true
	}
	return sets
}

// KnownRulesBySet returns a map of ruleset -> set of rule names.
func KnownRulesBySet() map[string]map[string]bool {
	rules.RegisterV2Rules()
	bySet := map[string]map[string]bool{}
	for _, r := range rules.Registry {
		rs := r.RuleSet()
		if bySet[rs] == nil {
			bySet[rs] = map[string]bool{}
		}
		bySet[rs][r.Name()] = true
	}
	return bySet
}

// KnownOptionsByRule returns a map of rule name -> set of allowed config keys.
// Always includes "active" and "excludes" as standard keys. Option keys are
// read from each rule's Meta() descriptor via rules.MetaForRule.
func KnownOptionsByRule() map[string]map[string]OptionType {
	rules.RegisterV2Rules()
	result := map[string]map[string]OptionType{}
	for _, r := range rules.Registry {
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
		result[r.Name()] = keys
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
	default:
		return OptionTypeString
	}
}
