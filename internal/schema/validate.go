package schema

import (
	"fmt"
	"sort"

	"github.com/kaeawc/krit/internal/config"
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
