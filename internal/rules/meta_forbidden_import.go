package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// Hand-written Meta() for ForbiddenImportRule. This file is the sole source
// of truth for the rule's descriptor.
//
// Why hand-written:
//   The forbiddenImports YAML list writes to BOTH ForbiddenImports AND
//   Patterns on the rule struct. Keeping the dual write in one place here
//   keeps descriptor behavior explicit until the Patterns shim is retired.

// Meta returns the descriptor for ForbiddenImportRule.
func (r *ForbiddenImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenImport",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonRequiresUserConfig,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenImportRule]{
				Name:        "imports",
				Description: "Forbidden imports as globs matched against the whole import path (detekt `imports`).",
				Apply:       func(r *ForbiddenImportRule, v []string) { r.Imports = v },
			}),
			api.StringOption(api.StringOptionSpec[ForbiddenImportRule]{
				Name:        "forbiddenPatterns",
				Description: "Comma-separated regexes searched in the import path (detekt `forbiddenPatterns`).",
				Apply: func(r *ForbiddenImportRule, v string) {
					r.ForbiddenPatterns = nil
					for _, p := range strings.Split(v, ",") {
						if p = strings.TrimSpace(p); p == "" {
							continue
						}
						if re, err := regexp.Compile(p); err == nil {
							r.ForbiddenPatterns = append(r.ForbiddenPatterns, re)
						}
					}
				},
			}),
			api.StringListOption(api.StringListOptionSpec[ForbiddenImportRule]{
				Name:        "allowedImports",
				Description: "Allowed import patterns (override forbidden).",
				Apply:       func(r *ForbiddenImportRule, v []string) { r.AllowedImports = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ForbiddenImportRule]{
				Name:        "forbiddenImports",
				Default:     []string{"sun.", "jdk.internal."},
				Description: "Forbidden import patterns.",
				// Assign to BOTH ForbiddenImports and Patterns. Patterns
				// is kept for backward-compat; the rule prefers
				// ForbiddenImports when set and falls back to Patterns.
				Apply: func(r *ForbiddenImportRule, v []string) {
					r.ForbiddenImports = v
					r.Patterns = v
				},
			}),
		},
	}
}
