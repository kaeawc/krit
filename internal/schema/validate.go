package schema

import (
	"fmt"
	"sort"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ValidationError describes a single config validation problem.
type ValidationError struct {
	Path    string // e.g. "style.MagicNumber.ignoreNumbers"
	Message string
	Level   string // "error" or "warning"
}

func (e ValidationError) String() string {
	return fmt.Sprintf("[%s] %s: %s", e.Level, e.Path, e.Message)
}

// ValidateConfig checks a Config for unknown rulesets, unknown rules,
// unknown config keys, and type mismatches. It returns a slice of errors/warnings.
func ValidateConfig(cfg *config.Config) []ValidationError {
	data := cfg.Data()
	if data == nil {
		return nil
	}

	knownSets := KnownRuleSets()
	rulesBySet := KnownRulesBySet()
	optionsByRule := KnownOptionsByRule()

	var errs []ValidationError

	// Sort top-level keys for deterministic output
	topKeys := make([]string, 0, len(data))
	for k := range data {
		topKeys = append(topKeys, k)
	}
	sort.Strings(topKeys)

	for _, key := range topKeys {
		val := data[key]

		if !knownSets[key] {
			errs = append(errs, ValidationError{
				Path:    key,
				Message: fmt.Sprintf("unknown ruleset '%s'", key),
				Level:   "error",
			})
			continue
		}
		if key == "config" {
			continue
		}
		if key == "module_template" {
			section, ok := val.(map[string]interface{})
			if !ok {
				errs = append(errs, ValidationError{
					Path:    key,
					Message: fmt.Sprintf("expected object, got %T", val),
					Level:   "error",
				})
				continue
			}
			errs = append(errs, validateModuleTemplateFields(section)...)
			continue
		}
		if key == "slos" {
			errs = append(errs, validateSLOs(val)...)
			continue
		}
		if key == "testSourcePaths" || key == "testSourcePathsOverride" {
			if err := checkType(key, val, OptionTypeStringSlice); err != nil {
				errs = append(errs, *err)
			}
			continue
		}
		if key == config.PluginRulesKey {
			errs = append(errs, validatePluginRules(val)...)
			continue
		}

		setMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		knownRules := rulesBySet[key]

		// Sort rule keys for deterministic output
		ruleKeys := make([]string, 0, len(setMap))
		for k := range setMap {
			ruleKeys = append(ruleKeys, k)
		}
		sort.Strings(ruleKeys)

		for _, ruleKey := range ruleKeys {
			ruleVal := setMap[ruleKey]
			if ruleKey == "active" {
				// Ruleset-level active flag
				continue
			}

			if !knownRules[ruleKey] {
				errs = append(errs, ValidationError{
					Path:    fmt.Sprintf("%s.%s", key, ruleKey),
					Message: fmt.Sprintf("unknown rule '%s' in ruleset '%s'", ruleKey, key),
					Level:   "error",
				})
				continue
			}

			ruleMap, ok := ruleVal.(map[string]interface{})
			if !ok {
				continue
			}

			allowedKeys := optionsByRule[ruleKey]
			errs = append(errs, validateRuleFields(key, ruleKey, ruleMap, allowedKeys)...)
		}
	}

	return errs
}

func validateSLOs(raw interface{}) []ValidationError {
	items, ok := raw.([]interface{})
	if !ok {
		return []ValidationError{{
			Path:    "slos",
			Message: fmt.Sprintf("expected array, got %T", raw),
			Level:   "error",
		}}
	}
	var errs []ValidationError
	for i, item := range items {
		path := fmt.Sprintf("slos[%d]", i)
		section, ok := item.(map[string]interface{})
		if !ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("expected object, got %T", item),
				Level:   "error",
			})
			continue
		}
		if _, ok := section["module"]; !ok {
			errs = append(errs, ValidationError{
				Path:    path + ".module",
				Message: "missing required config key 'module'",
				Level:   "error",
			})
		}
		keys := make([]string, 0, len(section))
		for key := range section {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := section[key]
			fieldPath := path + "." + key
			switch key {
			case "module":
				if err := checkType(fieldPath, val, OptionTypeString); err != nil {
					errs = append(errs, *err)
				}
			case "max_warnings_per_kloc", "max_errors_per_kloc":
				if !isNumber(val) {
					errs = append(errs, ValidationError{
						Path:    fieldPath,
						Message: fmt.Sprintf("expected number, got %T", val),
						Level:   "error",
					})
				}
			default:
				errs = append(errs, ValidationError{
					Path:    fieldPath,
					Message: fmt.Sprintf("unknown config key '%s'", key),
					Level:   "error",
				})
			}
		}
	}
	return errs
}

// validatePluginRules checks the top-level pluginRules section.
// Shape: pluginRules: { <ruleID>: { active: bool, options: { ... } } }.
// Rule IDs are accepted as-is — they belong to user-supplied jars and
// the validator has no registry to check them against.
func validatePluginRules(raw interface{}) []ValidationError {
	section, ok := raw.(map[string]interface{})
	if !ok {
		return []ValidationError{{
			Path:    config.PluginRulesKey,
			Message: fmt.Sprintf("expected object, got %T", raw),
			Level:   "error",
		}}
	}
	var errs []ValidationError
	ruleIDs := make([]string, 0, len(section))
	for id := range section {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, ruleID := range ruleIDs {
		ruleVal := section[ruleID]
		path := config.PluginRulesKey + "." + ruleID
		ruleMap, ok := ruleVal.(map[string]interface{})
		if !ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("expected object, got %T", ruleVal),
				Level:   "error",
			})
			continue
		}
		keys := make([]string, 0, len(ruleMap))
		for k := range ruleMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fieldPath := path + "." + key
			switch key {
			case "active":
				if err := checkType(fieldPath, ruleMap[key], OptionTypeBool); err != nil {
					errs = append(errs, *err)
				}
			case "options":
				if _, ok := ruleMap[key].(map[string]interface{}); !ok {
					errs = append(errs, ValidationError{
						Path:    fieldPath,
						Message: fmt.Sprintf("expected object, got %T", ruleMap[key]),
						Level:   "error",
					})
				}
			default:
				errs = append(errs, ValidationError{
					Path:    fieldPath,
					Message: fmt.Sprintf("unknown config key '%s' (allowed: active, options)", key),
					Level:   "error",
				})
			}
		}
	}
	return errs
}

func validateModuleTemplateFields(section map[string]interface{}) []ValidationError {
	allowed := map[string]OptionType{
		"feature_root":        OptionTypeString,
		"required_submodules": OptionTypeStringSlice,
		"required_plugins":    OptionTypeStringSlice,
	}
	var errs []ValidationError
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		expectedType, ok := allowed[key]
		path := "module_template." + key
		if !ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("unknown config key '%s'", key),
				Level:   "error",
			})
			continue
		}
		if err := checkType(path, section[key], expectedType); err != nil {
			errs = append(errs, *err)
		}
	}
	return errs
}

func isNumber(v interface{}) bool {
	switch v.(type) {
	case int, int64, float64, float32:
		return true
	default:
		return false
	}
}

func validateRuleFields(setName, ruleName string, ruleMap map[string]interface{}, allowedKeys map[string]OptionType) []ValidationError {
	var errs []ValidationError

	// Sort keys for deterministic output
	keys := make([]string, 0, len(ruleMap))
	for k := range ruleMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := ruleMap[key]
		path := fmt.Sprintf("%s.%s.%s", setName, ruleName, key)

		expectedType, known := allowedKeys[key]
		if !known {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("unknown config key '%s'", key),
				Level:   "error",
			})
			continue
		}

		if err := checkType(path, val, expectedType); err != nil {
			errs = append(errs, *err)
		}
	}
	return errs
}

func checkType(path string, val interface{}, expected OptionType) *ValidationError {
	switch expected {
	case OptionTypeInt:
		switch val.(type) {
		case int, int64, float64:
			return nil
		}
		return &ValidationError{Path: path, Message: fmt.Sprintf("expected integer, got %T", val), Level: "error"}
	case OptionTypeBool:
		if _, ok := val.(bool); !ok {
			return &ValidationError{Path: path, Message: fmt.Sprintf("expected boolean, got %T", val), Level: "error"}
		}
	case OptionTypeString:
		if _, ok := val.(string); !ok {
			return &ValidationError{Path: path, Message: fmt.Sprintf("expected string, got %T", val), Level: "error"}
		}
	case OptionTypeRegex:
		s, ok := val.(string)
		if !ok {
			return &ValidationError{Path: path, Message: fmt.Sprintf("expected string, got %T", val), Level: "error"}
		}
		if err := api.ValidateAnchoredPattern(s); err != nil {
			return &ValidationError{Path: path, Message: fmt.Sprintf("invalid regex: %v", err), Level: "error"}
		}
	case OptionTypeStringSlice:
		if _, ok := val.([]interface{}); !ok {
			if _, ok := val.([]string); !ok {
				// Also accept a single string as a one-element list
				if _, ok := val.(string); !ok {
					return &ValidationError{Path: path, Message: fmt.Sprintf("expected array of strings, got %T", val), Level: "error"}
				}
			}
		}
	}
	return nil
}
