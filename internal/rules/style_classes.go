package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// AbstractClassCanBeConcreteClassRule detects abstract classes with no abstract members.
// With type inference: also checks inherited abstract members via ClassHierarchy
// to avoid false positives when abstract members come from a supertype.
type AbstractClassCanBeConcreteClassRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence — classifying
// abstractness requires knowing all concrete method bodies;
// resolver-assisted but falls back to structural heuristic. Classified per
// roadmap/17.
func (r *AbstractClassCanBeConcreteClassRule) Confidence() float64 { return 0.75 }

// AbstractClassCanBeInterfaceRule detects abstract classes with no state.
type AbstractClassCanBeInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *AbstractClassCanBeInterfaceRule) Confidence() float64 { return 0.75 }

// DataClassShouldBeImmutableRule detects var properties in data classes.
// With type inference: also flags val properties whose types are mutable collections
// (MutableList, MutableMap, MutableSet) since they undermine data class immutability.
type DataClassShouldBeImmutableRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence — detecting
// mutable properties in data classes needs type-aware var detection;
// fallback uses keyword matching. Classified per roadmap/17.
func (r *DataClassShouldBeImmutableRule) Confidence() float64 { return 0.75 }

// mutableCollectionTypes lists types that are mutable collections.
var mutableCollectionTypes = map[string]bool{
	"MutableList": true, "MutableSet": true, "MutableMap": true,
	"MutableCollection": true, "MutableIterable": true,
}

// DataClassContainsFunctionsRule detects data classes with function members.
type DataClassContainsFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	ConversionFunctionPrefix []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *DataClassContainsFunctionsRule) Confidence() float64 { return 0.75 }

// ProtectedMemberInFinalClassRule detects protected members in final classes.
// With type inference: verifies via ClassHierarchy that no subclass exists,
// confirming the class is truly final even if it appears in a different module.
type ProtectedMemberInFinalClassRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence — flags protected
// members on non-open classes; class-openness detection depends on declared
// modifiers plus resolver for inherited finality. Classified per
// roadmap/17.
func (r *ProtectedMemberInFinalClassRule) Confidence() float64 { return 0.75 }

func forEachDirectClassMemberFlat(file *scanner.File, body uint32, fn func(uint32)) {
	if file == nil || body == 0 {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(body); i++ {
		child := file.FlatNamedChild(body, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "class_member_declarations" {
			for j := 0; j < file.FlatNamedChildCount(child); j++ {
				member := file.FlatNamedChild(child, j)
				if member != 0 {
					fn(member)
				}
			}
			continue
		}
		fn(child)
	}
}

// NestedClassesVisibilityRule reports nested classes/objects/interfaces that use an explicit
// public modifier inside an internal parent class. The public keyword is misleading because
// the nested class still has internal visibility. Matches detekt's NestedClassesVisibility.
type NestedClassesVisibilityRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *NestedClassesVisibilityRule) Confidence() float64 { return 0.75 }

// UtilityClassWithPublicConstructorRule detects utility classes with public constructors.
type UtilityClassWithPublicConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *UtilityClassWithPublicConstructorRule) Confidence() float64 { return 0.75 }


// OptionalAbstractKeywordRule detects abstract keyword on interface members.
type OptionalAbstractKeywordRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *OptionalAbstractKeywordRule) Confidence() float64 { return 0.75 }

// ClassOrderingRule checks that class members are in the conventional order.
type ClassOrderingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *ClassOrderingRule) Confidence() float64 { return 0.75 }

// ObjectLiteralToLambdaRule detects object literals that could be lambdas.
// With type inference: uses ClassHierarchy to detect SAM interfaces from
// dependencies and other project files beyond the hardcoded allowlist.
type ObjectLiteralToLambdaRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence. SAM
// conversion eligibility depends on whether the supertype is a
// single-abstract-method interface, which needs either a class
// hierarchy from the resolver or a hardcoded allow-list — both
// paths miss project-defined SAM interfaces. The rule also can't
// easily detect when the object literal captures state or has
// side-effect init blocks that prevent conversion.
func (r *ObjectLiteralToLambdaRule) Confidence() float64 { return 0.75 }

// extractSupertypeName gets the simple type name from a delegation_specifier node,
// handling dotted names (e.g., "Foo.Bar") and generics (e.g., "Callable<Int>").
// For dotted names like "MenuItem.OnActionExpandListener", returns the last segment.
func extractSupertypeNameFlat(file *scanner.File, specNode uint32) string {
	ut, _ := file.FlatFindChild(specNode, "user_type")
	if ut == 0 {
		return ""
	}
	var lastIdent string
	for i := 0; i < file.FlatChildCount(ut); i++ {
		child := file.FlatChild(ut, i)
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return lastIdent
}

// knownSAMInterfaces is a set of well-known Java SAM interfaces and Kotlin fun
// interfaces from common libraries that support SAM conversion.
var knownSAMInterfaces = map[string]bool{
	// Java standard library
	"Runnable":          true,
	"Callable":          true,
	"Comparator":        true,
	"Predicate":         true,
	"Function":          true,
	"Supplier":          true,
	"Consumer":          true,
	"BiFunction":        true,
	"BiConsumer":        true,
	"BiPredicate":       true,
	"UnaryOperator":     true,
	"BinaryOperator":    true,
	"FileFilter":        true,
	"FilenameFilter":    true,
	"InvocationHandler": true,
	"ThreadFactory":     true,
	"PrivilegedAction":  true,
	"Executor":          true,
	// Java AWT/Swing
	"ActionListener": true,
	// Kotlin coroutines
	"ChildHandle": true,
	// Compose runtime — DisposableEffectResult is a regular Kotlin interface, NOT fun interface
	// Android
	"OnClickListener":     true,
	"OnLongClickListener": true,
	"OnTouchListener":     true,
	"OnScrollListener":    true,
}

// isSAMConvertible checks whether the given supertype name is a SAM-convertible interface.
// It checks (1) known Java SAM interfaces, (2) "fun interface" in the same file,
// and (3) oracle ClassHierarchy for interfaces with exactly one abstract method.
func isSAMConvertible(name string, file *scanner.File, resolver typeinfer.TypeResolver) bool {
	// Check known SAM interfaces (Java SAM + common Kotlin fun interfaces)
	if knownSAMInterfaces[name] {
		return true
	}

	// Check for "fun interface <Name>" in the same file (Kotlin fun interfaces)
	fileText := string(file.Content)
	// Look for "fun interface Name" pattern — handles the common case
	target := "fun interface " + name
	if strings.Contains(fileText, target) {
		return true
	}

	// With type inference: check oracle for project-defined and dependency interfaces
	// that have exactly one abstract function member (SAM interfaces).
	// SAM conversion only works on Java interfaces or Kotlin "fun interface" —
	// regular Kotlin interfaces with a single abstract method are NOT SAM convertible.
	if resolver != nil {
		info := resolver.ClassHierarchy(name)
		if info != nil && info.Kind == "interface" {
			abstractFunCount := 0
			for _, m := range info.Members {
				if m.IsAbstract && m.Kind == "function" {
					abstractFunCount++
				}
			}
			if abstractFunCount == 1 {
				// Check if this is a Kotlin "fun interface" or a Java interface.
				// Java source files always support SAM. Kotlin interfaces only
				// support SAM if declared with "fun interface".
				if info.File != "" {
					if strings.HasSuffix(info.File, ".kt") || strings.HasSuffix(info.File, ".kts") {
						return false // Kotlin source without "fun" — not SAM convertible
					}
					return true // Java source — SAM convertible
				}
				// No file info — check if we're in a Kotlin project
				// by conservatively assuming non-SAM unless proven otherwise
				return false
			}
		}
	}

	return false
}

// objectBodyContainsBareThis walks the AST looking for unlabeled 'this' expressions.
// It does NOT descend into nested object_literal nodes (they have their own 'this').
func objectBodyContainsBareThisFlat(file *scanner.File, node uint32) bool {
	if file.FlatType(node) == "this_expression" {
		hasLabel := false
		for i := 0; i < file.FlatChildCount(node); i++ {
			if file.FlatType(file.FlatChild(node, i)) == "type_identifier" {
				hasLabel = true
				break
			}
		}
		return !hasLabel
	}
	if file.FlatType(node) == "object_literal" {
		return false
	}
	for i := 0; i < file.FlatChildCount(node); i++ {
		if objectBodyContainsBareThisFlat(file, file.FlatChild(node, i)) {
			return true
		}
	}
	return false
}

// SerialVersionUIDInSerializableClassRule detects missing serialVersionUID.
// With type inference: uses ClassHierarchy to verify the class actually implements
// java.io.Serializable (could be transitive through a supertype).
type SerialVersionUIDInSerializableClassRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence — Serializable
// detection uses supertype names; without resolver, falls back to matching
// `Serializable` in the delegation list. Classified per roadmap/17.
func (r *SerialVersionUIDInSerializableClassRule) Confidence() float64 { return 0.75 }

func flatDirectChildrenOfType(file *scanner.File, idx uint32, nodeType string) []uint32 {
	var out []uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == nodeType {
			out = append(out, child)
		}
	}
	return out
}

// checksSerializable walks the class hierarchy to find java.io.Serializable.
func checksSerializable(resolver typeinfer.TypeResolver, info *typeinfer.ClassInfo) bool {
	visited := make(map[string]bool)
	return checksSerializableRec(resolver, info, visited)
}

func checksSerializableRec(resolver typeinfer.TypeResolver, info *typeinfer.ClassInfo, visited map[string]bool) bool {
	if visited[info.FQN] || visited[info.Name] {
		return false
	}
	visited[info.FQN] = true
	visited[info.Name] = true

	for _, st := range info.Supertypes {
		if st == "java.io.Serializable" || strings.HasSuffix(st, ".Serializable") || st == "Serializable" {
			return true
		}
		// Check transitively
		parts := strings.Split(st, ".")
		stName := parts[len(parts)-1]
		stInfo := resolver.ClassHierarchy(stName)
		if stInfo == nil {
			stInfo = resolver.ClassHierarchy(st)
		}
		if stInfo != nil && checksSerializableRec(resolver, stInfo, visited) {
			return true
		}
	}
	return false
}


