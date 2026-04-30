package registry

import (
	"regexp"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

type ConfigSource = v2.ConfigSource

func ApplyConfig(rule interface{}, d RuleDescriptor, cfg ConfigSource) (active bool) {
	return v2.ApplyConfig(rule, d, cfg)
}

func ApplyConfigActiveOnly(d RuleDescriptor, cfg ConfigSource) (active bool) {
	return v2.ApplyConfigActiveOnly(d, cfg)
}

func DefaultInactiveSet(descs []RuleDescriptor) map[string]bool {
	return v2.DefaultInactiveSet(descs)
}

func CompileAnchoredPattern(ruleName, field, pattern string) *regexp.Regexp {
	return v2.CompileAnchoredPattern(ruleName, field, pattern)
}

func ValidateAnchoredPattern(pattern string) error {
	return v2.ValidateAnchoredPattern(pattern)
}
