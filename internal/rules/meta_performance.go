// Descriptor metadata for internal/rules/performance.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ArrayPrimitiveRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ArrayPrimitive",
		RuleSet:       "performance",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		KnownLimitations: []string{
			"Lexical type-token match: aliased types via typealias to Array<Int> are not detected.",
			"Identifies the declared type only; runtime allocation patterns (e.g. Array(n) { it.toInt() }) are out of scope.",
		},
	}
}

func (r *BitmapDecodeWithoutOptionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BitmapDecodeWithoutOptions",
		RuleSet:       "performance",
		DefaultActive: true,
	}
}

func (r *CouldBeSequenceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CouldBeSequence",
		RuleSet:       "performance",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"Counts chained calls by lexical name without confirming the receiver is a Collection: receivers exposing similarly named extensions on non-Iterable types may be flagged.",
			"Threshold tuning is project-dependent; small chains on large collections may still benefit from .asSequence().",
		},
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[CouldBeSequenceRule]{
				Name:        "allowedOperations",
				Aliases:     []string{"threshold"},
				Default:     2,
				Description: "Minimum chained collection operations to suggest sequence.",
				Apply:       func(r *CouldBeSequenceRule, v int) { r.AllowedOperations = v },
			}),
		},
	}
}

func (r *ForEachOnRangeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForEachOnRange",
		RuleSet:       "performance",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *SpreadOperatorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpreadOperator",
		RuleSet:       "performance",
		DefaultActive: true,
	}
}

func (r *UnnecessaryInitOnArrayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryInitOnArray",
		RuleSet:       "performance",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryPartOfBinaryExpressionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryPartOfBinaryExpression",
		RuleSet:       "performance",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryTemporaryInstantiationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryTemporaryInstantiation",
		RuleSet:       "performance",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryTypeCastingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryTypeCasting",
		RuleSet:       "performance",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
	}
}
