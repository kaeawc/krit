// Descriptor metadata for internal/rules/performance.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *ArrayPrimitiveRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ArrayPrimitive",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects Array<Int> and similar boxed primitive arrays that should use IntArray, LongArray, etc.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *BitmapDecodeWithoutOptionsRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "BitmapDecodeWithoutOptions",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects BitmapFactory.decode* calls without BitmapFactory.Options, which may decode full-size bitmaps.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CouldBeSequenceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CouldBeSequence",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects collection operation chains that could use sequences to avoid intermediate allocations.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "allowedOperations",
				Aliases:     []string{"threshold"},
				Type:        registry.OptInt,
				Default:     2,
				Description: "Minimum chained collection operations to suggest sequence.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CouldBeSequenceRule).AllowedOperations = value.(int)
				},
			},
		},
	}
}

func (r *ForEachOnRangeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ForEachOnRange",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects (range).forEach patterns that should use a simple for loop to avoid lambda overhead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *SpreadOperatorRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "SpreadOperator",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects the spread operator (*array) in function calls which creates an array copy.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryInitOnArrayRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnnecessaryInitOnArray",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects IntArray(n) { 0 } and similar array initializations where the init value is already the default.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryPartOfBinaryExpressionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnnecessaryPartOfBinaryExpression",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects redundant parts of binary expressions like x && true or x || false.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryTemporaryInstantiationRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnnecessaryTemporaryInstantiation",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects temporary wrapper instantiations like Integer.valueOf(x).toString() that can be simplified.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryTypeCastingRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnnecessaryTypeCasting",
		RuleSet:       "performance",
		Severity:      "warning",
		Description:   "Detects safe casts immediately compared with null and redundant casts to an already-known type.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
