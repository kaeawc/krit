package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

type UseArrayLiteralsInAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseArrayLiteralsInAnnotationsRule) Confidence() float64 { return api.ConfidenceMedium }

type UseSumOfInsteadOfFlatMapSizeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseSumOfInsteadOfFlatMapSizeRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UseLetRule) Confidence() float64 { return api.ConfidenceMedium }

type UseDataClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowVars bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseDataClassRule) Confidence() float64 { return api.ConfidenceMedium }

type UseIfInsteadOfWhenRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreWhenContainingVariableDeclaration bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfInsteadOfWhenRule) Confidence() float64 { return api.ConfidenceMedium }

type UseIfEmptyOrIfBlankRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfEmptyOrIfBlankRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ExplicitCollectionElementAccessMethodRule) Confidence() float64 { return api.ConfidenceMedium }

func explicitCollectionAccessReceiverSupported(ctx *api.Context, receiver uint32, method string) bool {
	if ctx.File == nil || receiver == 0 {
		return false
	}
	if typ, ok := semantics.ExpressionType(ctx, receiver); ok && explicitCollectionAccessResolvedTypeSupports(typ, method) {
		return true
	}
	if typ, _, ok := flatNullOrEmptyExplicitReceiverType(ctx.File, receiver); ok && explicitCollectionAccessTypeSupports(ctx.File, typ, method) {
		return true
	}
	return explicitCollectionAccessInitializerSupports(ctx.File, receiver, method)
}

func explicitCollectionAccessResolvedTypeSupports(typ semantics.TypeInfo, method string) bool {
	if explicitCollectionAccessTypeNameSupports(typ.Name, method) || explicitCollectionAccessTypeNameSupports(typ.FQN, method) {
		return true
	}
	if typ.Type == nil {
		return false
	}
	for _, name := range explicitCollectionAccessSupportedTypes(method) {
		if typ.Type.IsSubtypeOf(name) {
			return true
		}
	}
	return false
}

func explicitCollectionAccessTypeSupports(file *scanner.File, typ string, method string) bool {
	name := explicitCollectionAccessSimpleTypeName(typ)
	if explicitCollectionAccessTypeNameSupports(name, method) || explicitCollectionAccessTypeNameSupports(typ, method) {
		return true
	}
	for _, fqn := range explicitCollectionAccessImportedTypes(name, method) {
		if fileImportsFQN(file, fqn) {
			return true
		}
	}
	return false
}

func explicitCollectionAccessTypeNameSupports(typ string, method string) bool {
	name := explicitCollectionAccessSimpleTypeName(typ)
	for _, supported := range explicitCollectionAccessSupportedTypes(method) {
		if name == explicitCollectionAccessSimpleTypeName(supported) || typ == supported {
			return true
		}
	}
	return false
}

func explicitCollectionAccessSupportedTypes(method string) []string {
	getTypes := []string{
		"kotlin.String", "String",
		"kotlin.Array", "Array",
		"kotlin.collections.List", "List",
		"kotlin.collections.MutableList", "MutableList",
		"kotlin.collections.Map", "Map",
		"kotlin.collections.MutableMap", "MutableMap",
		"java.util.List", "java.util.Map",
		"ByteArray", "ShortArray", "IntArray", "LongArray", "FloatArray", "DoubleArray", "BooleanArray", "CharArray",
		"ImmutableList", "PersistentList", "ImmutableMap", "PersistentMap",
	}
	if method == "set" {
		return []string{
			"kotlin.Array", "Array",
			"kotlin.collections.MutableList", "MutableList",
			"kotlin.collections.MutableMap", "MutableMap",
			"java.util.List", "java.util.Map",
			"ByteArray", "ShortArray", "IntArray", "LongArray", "FloatArray", "DoubleArray", "BooleanArray", "CharArray",
		}
	}
	return getTypes
}

func explicitCollectionAccessImportedTypes(simple string, method string) []string {
	if method != "get" {
		return nil
	}
	switch simple {
	case "ImmutableList":
		return []string{"kotlinx.collections.immutable.ImmutableList"}
	case "PersistentList":
		return []string{"kotlinx.collections.immutable.PersistentList"}
	case "ImmutableMap":
		return []string{"kotlinx.collections.immutable.ImmutableMap"}
	case "PersistentMap":
		return []string{"kotlinx.collections.immutable.PersistentMap"}
	}
	return nil
}

func explicitCollectionAccessSimpleTypeName(typ string) string {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSuffix(typ, "?")
	if idx := strings.Index(typ, "<"); idx >= 0 {
		typ = typ[:idx]
	}
	if idx := strings.LastIndex(typ, "."); idx >= 0 {
		typ = typ[idx+1:]
	}
	return strings.TrimSpace(typ)
}

func explicitCollectionAccessInitializerSupports(file *scanner.File, receiver uint32, method string) bool {
	path := flatNullOrEmptyReferencePath(file, receiver)
	if len(path) != 1 {
		return false
	}
	name := path[0]
	var supported bool
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if supported || file.FlatType(candidate) != "property_declaration" || flatNullOrEmptyDeclarationName(file, candidate) != name {
			return
		}
		if !semantics.SameEnclosingOwner(file, candidate, receiver) {
			return
		}
		firstCall := uint32(0)
		file.FlatWalkNodes(candidate, "call_expression", func(call uint32) {
			if firstCall == 0 {
				firstCall = call
			}
		})
		supported = explicitCollectionAccessFactorySupports(flatCallExpressionName(file, firstCall), method)
	})
	return supported
}

func explicitCollectionAccessFactorySupports(name string, method string) bool {
	switch name {
	case "listOf", "mapOf", "arrayOf":
		return method == "get"
	case "mutableListOf", "mutableMapOf",
		"byteArrayOf", "shortArrayOf", "intArrayOf", "longArrayOf", "floatArrayOf", "doubleArrayOf", "booleanArrayOf", "charArrayOf":
		return true
	}
	return false
}

type AlsoCouldBeApplyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *AlsoCouldBeApplyRule) Confidence() float64 { return api.ConfidenceMedium }

type EqualsNullCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *EqualsNullCallRule) Confidence() float64 { return api.ConfidenceMedium }
