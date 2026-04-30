// Descriptor metadata for internal/rules/di_hygiene.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AnvilContributesBindingWithoutScopeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AnvilContributesBindingWithoutScope",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @ContributesBinding scope mismatches with the @ContributesTo scope on the bound interface.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AnvilMergeComponentEmptyScopeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AnvilMergeComponentEmptyScope",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @MergeComponent scopes with no matching @ContributesTo or @ContributesBinding declarations.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BindsMismatchedArityRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BindsMismatchedArity",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @Binds functions that do not declare exactly one parameter as required by Dagger.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeadBindingsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeadBindings",
		RuleSet:       "di-hygiene",
		Severity:      "info",
		Description:   "Detects @Provides/@Binds functions whose return type is not requested by any @Inject site or component exposure in the project.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.5,
	}
}

func (r *HiltEntryPointOnNonInterfaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HiltEntryPointOnNonInterface",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects Hilt @EntryPoint annotations on classes or objects instead of interfaces.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
