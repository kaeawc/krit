// Descriptor metadata for internal/rules/style_idiomatic.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *UseAnyOrNoneInsteadOfFindRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseAnyOrNoneInsteadOfFind",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects .find {} != null patterns that should use .any {} or .none {} instead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseCheckNotNullRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseCheckNotNull",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects check(x != null) calls that should use checkNotNull(x) instead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseCheckOrErrorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseCheckOrError",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects if (!cond) throw IllegalStateException patterns that should use check().",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseEmptyCounterpartRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseEmptyCounterpart",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects listOf(), setOf(), and similar calls with no arguments that should use emptyList(), emptySet(), etc.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseIsNullOrEmptyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseIsNullOrEmpty",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects x == null || x.isEmpty() patterns that should use isNullOrEmpty().",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseOrEmptyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseOrEmpty",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects x ?: emptyList() patterns that should use .orEmpty() instead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseRequireNotNullRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseRequireNotNull",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects require(x != null) calls that should use requireNotNull(x) instead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseRequireRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseRequire",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects if (!cond) throw IllegalArgumentException patterns that should use require().",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}
