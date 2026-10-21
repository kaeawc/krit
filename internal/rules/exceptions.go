package rules

import (
	"strings"

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

// instanceOfCheckTypeName returns the trailing simple name of the type the
// `is`/`instanceof` check tests against (e.g. `IOException` for both
// `e is java.io.IOException` and `e instanceof IOException`), reading the
// `user_type` (Kotlin) or `type_identifier`/`scoped_type_identifier` (Java)
// operand structurally rather than by splitting raw node text.
func instanceOfCheckTypeName(file *scanner.File, checkNode uint32) string {
	for child := file.FlatFirstChild(checkNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "type_identifier", "scoped_type_identifier", "scoped_identifier", "nullable_type", "generic_type":
			if name := flatTypeLastIdentifier(file, child); name != "" {
				return name
			}
			return lastDotSegment(file.FlatNodeText(child))
		}
	}
	return ""
}

// instanceOfCheckOperandName returns the identifier name of the value being
// type-checked (the left operand of `x is Type` / `x instanceof Type`), or ""
// when the operand is not a bare identifier (e.g. a `when (subject) { is X }`
// condition, where the operand lives on the `when_subject`, or a more complex
// receiver expression). The operand is read as the first named child that is a
// plain identifier, before the type operand.
func instanceOfCheckOperandName(file *scanner.File, checkNode uint32) string {
	for child := file.FlatFirstChild(checkNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "identifier":
			return file.FlatNodeString(child, nil)
		case "user_type", "type_identifier", "scoped_type_identifier",
			"scoped_identifier", "nullable_type", "generic_type":
			// Reached the type operand without seeing a bare identifier
			// operand (e.g. a when-condition `is X`).
			return ""
		}
	}
	return ""
}

// instanceOfWhenSubjectOperand returns the bare-identifier subject of the
// `when` enclosing a subjectful when-condition `is X`, or "" when the nearest
// enclosing when has no bare-identifier subject (a binding subject like
// `when (val cause = e.cause)`, a complex expression, or a subjectless when).
// It stops at function / lambda / class boundaries so it never escapes the
// catch body's expression scope.
func instanceOfWhenSubjectOperand(file *scanner.File, isNode uint32) string {
	for p, ok := file.FlatParent(isNode); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "when_expression":
			subj, found := file.FlatFindChild(p, "when_subject")
			if !found || subj == 0 {
				return ""
			}
			// A binding subject (`when (val x = ...)`) has a
			// variable_declaration child — not a plain dispatch identifier.
			if _, ok := file.FlatFindChild(subj, "variable_declaration"); ok {
				return ""
			}
			ident, ok := file.FlatFindChild(subj, "simple_identifier")
			if !ok || ident == 0 {
				return ""
			}
			return file.FlatNodeString(ident, nil)
		case "function_declaration", "lambda_literal", "class_declaration",
			"object_declaration", "catch_block", "catch_clause", "source_file":
			return ""
		}
	}
	return ""
}

// lastDotSegment returns the trailing `.`-separated segment of a dotted name,
// trimming a trailing `?` nullability marker.
func lastDotSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "?")
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(s)
}

// nameLooksLikeExceptionType reports whether a simple type name denotes an
// exception/throwable type by its `Exception` / `Error` / `Throwable` suffix.
// This is the structural equivalent of the old `\w*Exception` text match but
// operates on the resolved simple type name rather than raw node text.
func nameLooksLikeExceptionType(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(name, "Exception") ||
		strings.HasSuffix(name, "Error") ||
		name == "Throwable"
}

// caughtNarrowingOperands returns the set of identifier names whose `is`/
// `instanceof` checks inside a catch body are narrowing the *already-caught*
// exception (and therefore legitimate, not the "type-check instead of
// polymorphism" smell). That set is the catch parameter itself plus any local
// variable declared in the catch body whose initializer reads the caught
// variable — e.g. `val cause = e.cause` / `Throwable cause = e.getCause()`,
// the idiomatic cause-chain unwrap. The initializer relationship is matched
// structurally by walking the declaration's initializer subtree for an
// identifier reference to a known caught-derived name.
func caughtNarrowingOperands(file *scanner.File, catchNode uint32, caughtVar string) map[string]bool {
	out := map[string]bool{caughtVar: true}
	if caughtVar == "" {
		return out
	}
	// Iterate to a fixpoint so chained derivations
	// (`val a = e.cause; val b = a.cause`) are all captured.
	changed := true
	for changed {
		changed = false
		file.FlatWalkNodes(catchNode, "property_declaration", func(decl uint32) {
			name := caughtNarrowingDeclName(file, decl)
			if name == "" || out[name] {
				return
			}
			if caughtNarrowingInitReferences(file, decl, out) {
				out[name] = true
				changed = true
			}
		})
		file.FlatWalkNodes(catchNode, "variable_declarator", func(decl uint32) {
			name := ""
			if ident, ok := file.FlatFindChild(decl, "identifier"); ok {
				name = file.FlatNodeString(ident, nil)
			}
			if name == "" || out[name] {
				return
			}
			if caughtNarrowingInitReferences(file, decl, out) {
				out[name] = true
				changed = true
			}
		})
	}
	return out
}

// caughtNarrowingDeclName returns the declared name of a Kotlin
// `property_declaration`.
func caughtNarrowingDeclName(file *scanner.File, decl uint32) string {
	if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		if ident, ok := file.FlatFindChild(vd, "simple_identifier"); ok {
			return file.FlatNodeString(ident, nil)
		}
	}
	return ""
}

// caughtNarrowingInitReferences reports whether the initializer side of a
// variable declaration reads any name in `known` (the caught variable or a
// caught-derived alias). It walks only the portion of the declaration after
// the `=` so the declared name itself is not mistaken for a reference.
func caughtNarrowingInitReferences(file *scanner.File, decl uint32, known map[string]bool) bool {
	afterEquals := false
	found := false
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			afterEquals = true
			continue
		}
		if !afterEquals || !file.FlatIsNamed(child) {
			continue
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			if found {
				return
			}
			switch file.FlatType(n) {
			case "simple_identifier", "identifier":
				if known[file.FlatNodeString(n, nil)] {
					found = true
				}
			}
		})
	}
	return found
}

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
