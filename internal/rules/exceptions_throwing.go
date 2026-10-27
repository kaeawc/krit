package rules

import (
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ThrowingExceptionFromFinallyRule detects throw inside finally blocks.
type ThrowingExceptionFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionFromFinallyRule) Confidence() float64 { return api.ConfidenceMedium }

// ThrowingExceptionsWithoutMessageOrCauseRule detects throw Exception() with no args.
// Uses tree-sitter dispatch to find call_expression nodes, then checks if the parent
// is a throw (jump_expression) and the exception type is in the configured allowlist.
type ThrowingExceptionsWithoutMessageOrCauseRule struct {
	FlatDispatchBase
	BaseRule
	Exceptions    []string
	allowlistOnce sync.Once
	allowlist     map[string]bool
}

// Default exception types that should have a message.
var defaultExceptionsRequiringMessage = map[string]bool{
	"ArrayIndexOutOfBoundsException": true,
	"Exception":                      true,
	"IllegalArgumentException":       true,
	"IllegalMonitorStateException":   true,
	"IllegalStateException":          true,
	"IndexOutOfBoundsException":      true,
	"NullPointerException":           true,
	"RuntimeException":               true,
	"Throwable":                      true,
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Confidence() float64 {
	return api.ConfidenceMedium
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) exceptionAllowlist() map[string]bool {
	if !experiment.Enabled("exceptions-allowlist-cache") || len(r.Exceptions) == 0 {
		if len(r.Exceptions) == 0 {
			return defaultExceptionsRequiringMessage
		}
		set := make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			set[e] = true
		}
		return set
	}
	r.allowlistOnce.Do(func() {
		r.allowlist = make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			r.allowlist[e] = true
		}
	})
	return r.allowlist
}

func throwingExceptionArgCountFlat(file *scanner.File, idx uint32) int {
	if !experiment.Enabled("exceptions-throw-fastpath") {
		callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
		if callSuffix == 0 {
			return -1
		}
		valueArgs, _ := file.FlatFindChild(callSuffix, "value_arguments")
		if valueArgs == 0 {
			return -1
		}
		argCount := 0
		for i := 0; i < file.FlatChildCount(valueArgs); i++ {
			if file.FlatType(file.FlatChild(valueArgs, i)) == "value_argument" {
				argCount++
			}
		}
		return argCount
	}
	callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return -1
	}
	argList, _ := file.FlatFindChild(callSuffix, "value_arguments")
	if argList == 0 {
		return -1
	}
	return file.FlatNamedChildCount(argList)
}

// ThrowingNewInstanceOfSameExceptionRule detects catch(e: X) { throw X(e) }.
type ThrowingNewInstanceOfSameExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingNewInstanceOfSameExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

// ThrowingExceptionInMainRule detects throw in main function.
type ThrowingExceptionInMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionInMainRule) Confidence() float64 { return api.ConfidenceMedium }

// ErrorUsageWithThrowableRule detects error(throwable) calls.
type ErrorUsageWithThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ErrorUsageWithThrowableRule) Confidence() float64 { return api.ConfidenceMedium }

func errorUsageWithThrowableArgument(ctx *api.Context, idx uint32) (string, bool) {
	if ctx.File == nil || !errorUsageWithThrowableDirectErrorCall(ctx.File, idx) {
		return "", false
	}
	args := errorUsageWithThrowableValueArguments(ctx.File, idx)
	if args == 0 {
		return "", false
	}
	firstArg := uint32(0)
	for arg := ctx.File.FlatFirstChild(args); arg != 0; arg = ctx.File.FlatNextSib(arg) {
		if ctx.File.FlatType(arg) == "value_argument" {
			firstArg = arg
			break
		}
	}
	if firstArg == 0 {
		return "", false
	}
	expr := flatValueArgumentExpression(ctx.File, firstArg)
	if expr == 0 {
		return "", false
	}
	argText := strings.TrimSpace(ctx.File.FlatNodeText(expr))
	if argText == "" || strings.HasPrefix(argText, "\"") {
		return "", false
	}
	if ctx.Resolver != nil {
		typ := ctx.Resolver.ResolveFlatNode(expr, ctx.File)
		if typ != nil && typ.Kind != typeinfer.TypeUnknown {
			return argText, errorUsageWithThrowableTypeMatches(typ)
		}
	}
	if typ, _, ok := flatNullOrEmptyExplicitReceiverType(ctx.File, expr); ok {
		return argText, errorUsageWithThrowableTypeTextMatches(typ)
	}
	return argText, errorUsageWithThrowableTextHeuristic(argText)
}

func errorUsageWithThrowableDirectErrorCall(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		return file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, "error")
	}
	return false
}

func errorUsageWithThrowableValueArguments(file *scanner.File, idx uint32) uint32 {
	_, args := flatCallExpressionParts(file, idx)
	return args
}

func errorUsageWithThrowableTypeMatches(typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	for _, name := range []string{typ.Name, typ.FQN} {
		if name == "Throwable" || name == "kotlin.Throwable" || name == "java.lang.Throwable" ||
			typeinfer.IsSubtypeOfException(name, "Throwable") ||
			typeinfer.IsSubtypeOfException(name, "kotlin.Throwable") ||
			typeinfer.IsSubtypeOfException(name, "java.lang.Throwable") {
			return true
		}
	}
	return typ.IsSubtypeOf("Throwable") || typ.IsSubtypeOf("kotlin.Throwable") || typ.IsSubtypeOf("java.lang.Throwable")
}

func errorUsageWithThrowableTypeTextMatches(typ string) bool {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSuffix(typ, "?")
	if idx := strings.Index(typ, "<"); idx >= 0 {
		typ = typ[:idx]
	}
	if idx := strings.LastIndex(typ, "."); idx >= 0 {
		typ = typ[idx+1:]
	}
	return typ == "Throwable" || typeinfer.IsSubtypeOfException(typ, "Throwable")
}

func errorUsageWithThrowableTextHeuristic(argText string) bool {
	lower := strings.ToLower(strings.TrimSpace(argText))
	return strings.Contains(lower, "exception") || strings.Contains(lower, "throwable") ||
		strings.Contains(lower, "error") || lower == "e" || lower == "ex" || lower == "err" || lower == "t"
}

// ObjectExtendsThrowableRule detects object : Exception/Throwable/Error.
type ObjectExtendsThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — relies on
// resolver to determine supertypes; falls back to name-based heuristics on
// the `Throwable` identifier. Classified per roadmap/17.
func (r *ObjectExtendsThrowableRule) Confidence() float64 { return api.ConfidenceMedium }

var throwableBaseTypes = []string{"Throwable", "Exception", "Error", "RuntimeException",
	"IllegalStateException", "IllegalArgumentException", "IOException",
	"UnsupportedOperationException"}

func walkThrowExpressionsFlat(file *scanner.File, idx uint32, fn func(throwNode uint32)) {
	file.FlatWalkNodes(idx, "jump_expression", func(n uint32) {
		text := file.FlatNodeText(n)
		if strings.HasPrefix(text, "throw ") || text == "throw" {
			fn(n)
		}
	})
	file.FlatWalkNodes(idx, "throw_statement", fn)
}

// throwExpressionTargetFlat returns the call_expression (Kotlin) or
// object_creation_expression (Java) used as the thrown instance, or 0
// if the throw is bare (`throw` with no operand) or rethrows a simple
// identifier (`throw e`).
func throwExpressionTargetFlat(file *scanner.File, throwNode uint32) uint32 {
	for child := file.FlatFirstChild(throwNode); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "call_expression", "object_creation_expression":
			return child
		}
	}
	return 0
}

// throwTargetTypeNameFlat returns the last identifier of the type
// constructed by a throw target — `IOException` for both
// `IOException(msg)` and `java.io.IOException(msg)`.
func throwTargetTypeNameFlat(file *scanner.File, target uint32) string {
	switch file.FlatType(target) {
	case "call_expression":
		return flatCallExpressionName(file, target)
	case "object_creation_expression":
		for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier":
				return file.FlatNodeText(child)
			case "scoped_type_identifier", "scoped_identifier":
				if name := flatTypeLastIdentifier(file, child); name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// throwTargetArgUsageFlat reports the number of positional arguments
// in the throw target and whether any of them references varName as
// an identifier. String literals and comments are skipped because
// they are not identifier AST nodes.
func throwTargetArgUsageFlat(file *scanner.File, target uint32, varName string) (argCount int, hasVar bool) {
	args := throwTargetArgsFlat(file, target)
	if args == 0 {
		return 0, false
	}
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		argCount++
		if hasVar {
			continue
		}
		file.FlatWalkAllNodes(child, func(id uint32) {
			if hasVar {
				return
			}
			switch file.FlatType(id) {
			case "simple_identifier", "identifier":
				if file.FlatNodeTextEquals(id, varName) {
					hasVar = true
				}
			}
		})
	}
	return argCount, hasVar
}

func throwTargetArgsFlat(file *scanner.File, target uint32) uint32 {
	switch file.FlatType(target) {
	case "call_expression":
		return flatCallKeyArguments(file, target)
	case "object_creation_expression":
		for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "argument_list" {
				return child
			}
		}
	}
	return 0
}

func javaCatchOnlyRethrowsVar(file *scanner.File, catchNode uint32, varName string) bool {
	block, ok := file.FlatFindChild(catchNode, "block")
	if !ok || block == 0 {
		return false
	}
	statementCount := 0
	rethrows := false
	for child := file.FlatFirstChild(block); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "{", "}", "line_comment", "block_comment", "comment":
			continue
		case "throw_statement":
			statementCount++
			rethrows = strings.TrimSpace(file.FlatNodeText(child)) == "throw "+varName+";"
		default:
			if file.FlatIsNamed(child) {
				statementCount++
			}
		}
	}
	return statementCount == 1 && rethrows
}

// hasLaterCatchSibling reports whether catchNode (a Kotlin catch_block or a
// Java catch_clause) is followed by another catch of the same try.
//
// RethrowCaughtException uses this to skip rethrow-only catches that guard a
// later handler. A later catch can only intercept the rethrown exception when
// its type is a supertype of (or overlaps with) the earlier type. Proving two
// types unrelated needs a type hierarchy, and a later catch of the identical
// type is unreachable (a compile error in Java), so any later catch is treated
// as one that would otherwise catch the exception. This keeps the rule
// precise without type information; it only gives up findings in the rare
// case where the earlier rethrow-only catch is followed by an unrelated type.
func hasLaterCatchSibling(file *scanner.File, catchNode uint32) bool {
	kind := file.FlatType(catchNode)
	for sib := file.FlatNextSib(catchNode); sib != 0; sib = file.FlatNextSib(sib) {
		if file.FlatType(sib) == kind {
			return true
		}
	}
	return false
}
