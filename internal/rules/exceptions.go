package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ExceptionRaisedInUnexpectedLocationRule detects throw inside equals/hashCode/toString/finalize.
type ExceptionRaisedInUnexpectedLocationRule struct {
	FlatDispatchBase
	BaseRule
	// MethodNames is the list of function names where throwing is treated
	// as a finding. Configurable via the `methodNames` YAML option.
	// Default: equals, hashCode, toString, finalize.
	MethodNames []string
}

// defaultExceptionRaisedInUnexpectedLocationMethods enumerates the default
// `methodNames` set. Used as the fallback when the configured list is empty
// (i.e. zero-value or explicitly cleared).
var defaultExceptionRaisedInUnexpectedLocationMethods = []string{
	"equals", "hashCode", "toString", "finalize",
}

// includesMethod reports whether name is one of the rule's configured
// method names, falling back to the default when the configured list is empty.
func (r *ExceptionRaisedInUnexpectedLocationRule) includesMethod(name string) bool {
	names := r.MethodNames
	if len(names) == 0 {
		names = defaultExceptionRaisedInUnexpectedLocationMethods
	}
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ExceptionRaisedInUnexpectedLocationRule) Confidence() float64 { return api.ConfidenceMedium }

// InstanceOfCheckForExceptionRule detects `is SomeException` inside catch blocks.
type InstanceOfCheckForExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *InstanceOfCheckForExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

// NotImplementedDeclarationRule detects TODO() calls.
type NotImplementedDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *NotImplementedDeclarationRule) Confidence() float64 { return api.ConfidenceMedium }

// isKotlinTODOCall reports whether the call_expression at idx is a call to
// `kotlin.TODO()` — either the unqualified `TODO(...)` (kotlin's TODO is
// imported automatically from the `kotlin` package) or the fully-qualified
// `kotlin.TODO(...)`. Member calls like `foo.TODO()` or `Items.TODO()` on
// non-kotlin receivers are NOT kotlin.TODO and must not be flagged.
func isKotlinTODOCall(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			// Bare `TODO(...)` — kotlin.TODO is auto-imported from the
			// `kotlin` package and is the only realistic referent for an
			// unqualified TODO call at top-level expression position.
			return file.FlatNodeTextEquals(child, "TODO")
		case "navigation_expression":
			// Qualified call. Only `kotlin.TODO(...)` counts as kotlin.TODO.
			segments := flatNavigationChainIdentifiers(file, child)
			return len(segments) == 2 && segments[0] == "kotlin" && segments[1] == "TODO"
		}
	}
	return false
}

// isThrowOfNotImplementedError reports `throw NotImplementedError(...)` and `throw kotlin.NotImplementedError(...)`.
func isThrowOfNotImplementedError(file *scanner.File, idx uint32) bool {
	if first := file.FlatFirstChild(idx); first == 0 || file.FlatType(first) != "throw" {
		return false
	}
	target := throwExpressionTargetFlat(file, idx)
	if target == 0 {
		return false
	}
	for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeTextEquals(child, "NotImplementedError")
		case "navigation_expression":
			segments := flatNavigationChainIdentifiers(file, child)
			return len(segments) == 2 && segments[0] == "kotlin" && segments[1] == "NotImplementedError"
		}
	}
	return false
}

// isProgramEntryMain reports whether a function named main can be a program entry point: a top-level
// Kotlin function, a `@JvmStatic` member, or a static Java method, with no parameter or one array parameter.
func isProgramEntryMain(file *scanner.File, fn uint32) bool {
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok && file.FlatNamedChildCount(params) > 1 {
		return false
	}
	if file.Language == scanner.LangJava {
		return file.FlatHasModifier(fn, "static")
	}
	if _, member := flatEnclosingAncestor(file, fn, "class_body"); member {
		return flatHasAnnotationNamed(file, fn, "JvmStatic")
	}
	return true
}

// RethrowCaughtExceptionRule detects catch { throw e } where e is the caught variable.
type RethrowCaughtExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *RethrowCaughtExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

// ReturnFromFinallyRule detects return statements inside finally blocks.
type ReturnFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLabeled bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ReturnFromFinallyRule) Confidence() float64 { return api.ConfidenceMedium }
