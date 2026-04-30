package rules

// Hand-written Meta() for ForbiddenImportRule. This file is the sole source
// of truth for the rule's descriptor.
//
// Why hand-written:
//   The forbiddenImports YAML list writes to BOTH ForbiddenImports AND
//   Patterns on the rule struct. Keeping the dual write in one place here
//   keeps descriptor behavior explicit until the Patterns shim is retired.

import "github.com/kaeawc/krit/internal/rules/registry"

// Meta returns the descriptor for ForbiddenImportRule.
func (r *ForbiddenImportRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ForbiddenImport",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects import statements matching configured forbidden patterns.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "allowedImports",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "Allowed import patterns (override forbidden).",
				Apply: func(target interface{}, value interface{}) {
					target.(*ForbiddenImportRule).AllowedImports = value.([]string)
				},
			},
			{
				Name:        "forbiddenImports",
				Type:        registry.OptStringList,
				Default:     []string{"sun.", "jdk.internal."},
				Description: "Forbidden import patterns.",
				// Assign to BOTH ForbiddenImports and Patterns. Patterns
				// is kept for backward-compat; the rule prefers
				// ForbiddenImports when set and falls back to Patterns.
				Apply: func(target interface{}, value interface{}) {
					rule := target.(*ForbiddenImportRule)
					list := value.([]string)
					rule.ForbiddenImports = list
					rule.Patterns = list
				},
			},
		},
	}
}
