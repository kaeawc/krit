// Descriptor metadata for internal/rules/di_hygiene.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AnvilContributesBindingWithoutScopeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AnvilContributesBindingWithoutScope",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @ContributesBinding scope mismatches with the @ContributesTo scope on the bound interface.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AnvilMergeComponentEmptyScopeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AnvilMergeComponentEmptyScope",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @MergeComponent scopes with no matching @ContributesTo or @ContributesBinding declarations.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BindsMismatchedArityRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "BindsMismatchedArity",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @Binds functions that do not declare exactly one parameter as required by Dagger.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HiltEntryPointOnNonInterfaceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HiltEntryPointOnNonInterface",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects Hilt @EntryPoint annotations on classes or objects instead of interfaces.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
