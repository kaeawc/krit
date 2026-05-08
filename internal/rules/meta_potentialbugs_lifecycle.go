// Descriptor metadata for internal/rules/potentialbugs_lifecycle.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *ExitOutsideMainRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExitOutsideMain",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}

func (r *ExplicitGarbageCollectionCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExplicitGarbageCollectionCall",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *InvalidRangeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InvalidRange",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *IteratorHasNextCallsNextMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IteratorHasNextCallsNextMethod",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *IteratorNotThrowingNoSuchElementExceptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IteratorNotThrowingNoSuchElementException",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *LateinitUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LateinitUsage",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[LateinitUsageRule]{
				Name:        "ignoreOnClassesPattern",
				Default:     "",
				Description: "Regex for classes to exclude.",
				Apply:       func(r *LateinitUsageRule, v *regexp.Regexp) { r.IgnoreOnClassesPattern = v },
			}),
		},
	}
}

func (r *MissingPackageDeclarationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingPackageDeclaration",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		FixLevel:      "cosmetic",
	}
}

func (r *MissingSuperCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingSuperCall",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MissingSuperCallRule]{
				Name:        "mustInvokeSuperAnnotations",
				Default:     []string{"androidx.annotation.CallSuper", "javax.annotation.OverridingMethodsMustInvokeSuper"},
				Description: "",
				Apply:       func(r *MissingSuperCallRule, v []string) { r.MustInvokeSuperAnnotations = v },
			}),
		},
	}
}

func (r *MissingUseCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingUseCall",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}
