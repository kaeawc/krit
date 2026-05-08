// Descriptor metadata for internal/rules/library.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ForbiddenPublicDataClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenPublicDataClass",
		RuleSet:       "libraries",
		DefaultActive: false,
	}
}

func (r *LibraryCodeMustSpecifyReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LibraryCodeMustSpecifyReturnType",
		RuleSet:       "libraries",
		DefaultActive: false,
	}
}

func (r *LibraryEntitiesShouldNotBePublicRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LibraryEntitiesShouldNotBePublic",
		RuleSet:       "libraries",
		DefaultActive: false,
	}
}
