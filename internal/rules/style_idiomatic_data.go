package rules

import "github.com/kaeawc/krit/internal/scanner"

type UseArrayLiteralsInAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseArrayLiteralsInAnnotationsRule) Confidence() float64 { return 0.75 }

type UseSumOfInsteadOfFlatMapSizeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseSumOfInsteadOfFlatMapSizeRule) Confidence() float64 { return 0.75 }

var sumOfSourceCalls = map[string]bool{"flatMap": true, "flatten": true, "map": true}

func sumOfNavSelectorFlat(file *scanner.File, nav uint32) string {
	for i := file.FlatChildCount(nav) - 1; i >= 0; i-- {
		child := file.FlatChild(nav, i)
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
		if file.FlatType(child) == "navigation_suffix" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
		}
	}
	return ""
}

type UseLetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseLetRule) Confidence() float64 { return 0.75 }

type UseDataClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowVars bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseDataClassRule) Confidence() float64 { return 0.75 }

type UseIfInsteadOfWhenRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreWhenContainingVariableDeclaration bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfInsteadOfWhenRule) Confidence() float64 { return 0.75 }

type UseIfEmptyOrIfBlankRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfEmptyOrIfBlankRule) Confidence() float64 { return 0.75 }

var ifEmptyOrBlankMethods = map[string]struct {
	replacement string
	negated     bool
}{"isEmpty": {"ifEmpty", false}, "isBlank": {"ifBlank", false}, "isNotEmpty": {"ifEmpty", true}, "isNotBlank": {"ifBlank", true}}

type ExplicitCollectionElementAccessMethodRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *ExplicitCollectionElementAccessMethodRule) Confidence() float64 { return 0.75 }

type AlsoCouldBeApplyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *AlsoCouldBeApplyRule) Confidence() float64 { return 0.75 }

type EqualsNullCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *EqualsNullCallRule) Confidence() float64 { return 0.75 }
