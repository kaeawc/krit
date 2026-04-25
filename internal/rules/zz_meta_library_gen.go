// Descriptor metadata for internal/rules/library.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *ForbiddenPublicDataClassRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ForbiddenPublicDataClass",
		RuleSet:       "libraries",
		Severity:      "warning",
		Description:   "Detects public data classes in library code whose exposed properties make the API hard to evolve.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LibraryCodeMustSpecifyReturnTypeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LibraryCodeMustSpecifyReturnType",
		RuleSet:       "libraries",
		Severity:      "warning",
		Description:   "Detects public functions and properties in library code without explicit return type annotations.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LibraryEntitiesShouldNotBePublicRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LibraryEntitiesShouldNotBePublic",
		RuleSet:       "libraries",
		Severity:      "warning",
		Description:   "Detects public top-level declarations in library code that could be made internal.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
