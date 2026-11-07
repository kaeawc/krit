package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// PrintStackTraceRule detects .printStackTrace() calls.
// ---------------------------------------------------------------------------
type PrintStackTraceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs exceptions rule. Detection matches exception type names
// and throw/catch shapes; name-only fallback fires on project types
// sharing the same simple name. Classified per roadmap/17.
func (r *PrintStackTraceRule) Confidence() float64 { return api.ConfidenceMedium }

func printStackTraceReceiverIsThrowable(ctx *api.Context, call uint32) bool {
	if ctx.File == nil {
		return false
	}
	file := ctx.File
	receiverName := flatReceiverNameFromCall(file, call)
	if receiverName == "" {
		return false
	}
	if catchNode, ok := flatEnclosingAncestor(file, call, "catch_block", "catch_clause"); ok {
		if receiverName == extractCaughtVarNameFlat(file, catchNode) {
			return true
		}
	}
	return false
}

// TooGenericExceptionCaughtRule detects catching Exception/Throwable.
// ---------------------------------------------------------------------------
type TooGenericExceptionCaughtRule struct {
	FlatDispatchBase
	BaseRule
	ExceptionNames            []string       // configurable list of generic exception types
	AllowedExceptionNameRegex *regexp.Regexp // exception var names matching this are allowed
}

var genericCaughtTypes = []string{
	"Exception", "Throwable", "Error", "RuntimeException",
	"NullPointerException", "ArrayIndexOutOfBoundsException",
	"IndexOutOfBoundsException", "IllegalMonitorStateException",
}

func (r *TooGenericExceptionCaughtRule) exceptionNameSet() map[string]bool {
	names := r.ExceptionNames
	if len(names) == 0 {
		names = genericCaughtTypes
	}
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// checkNode ports detekt 1.23.8 TooGenericExceptionCaught: the text of the caught type must be in the
// configured names. Nothing else about the catch body matters.
func (r *TooGenericExceptionCaughtRule) checkNode(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	var param uint32
	typeText := ""
	sawColon := false
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch {
		case file.FlatType(c) == "simple_identifier" && param == 0:
			param = c
		case file.FlatType(c) == ":":
			sawColon = true
		case sawColon && file.FlatIsNamed(c):
			typeText = file.FlatNodeText(c)
			sawColon = false
		}
	}
	if param == 0 || typeText == "" || !r.exceptionNameSet()[typeText] {
		return
	}
	if r.AllowedExceptionNameRegex != nil && r.AllowedExceptionNameRegex.MatchString(file.FlatNodeText(param)) {
		return
	}
	ctx.EmitAtNode(param, "The caught exception is too generic. Prefer catching specific exceptions to the case that is currently handled.")
}

func extractCaughtVarNameFlat(file *scanner.File, catchNode uint32) string {
	if file == nil || catchNode == 0 {
		return ""
	}
	if file.FlatType(catchNode) == "catch_clause" {
		last := ""
		for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "block" {
				break
			}
			file.FlatWalkAllNodes(child, func(idx uint32) {
				if file.FlatType(idx) == "identifier" || file.FlatType(idx) == "simple_identifier" {
					last = file.FlatNodeText(idx)
				}
			})
		}
		return last
	}
	if file.FlatType(catchNode) != "catch_block" {
		return ""
	}
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func catchBodyRethrowsCaughtException(file *scanner.File, catchNode uint32, varName string) bool {
	found := false
	file.FlatWalkNodes(catchNode, "jump_expression", func(idx uint32) {
		if found {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		found = text == "throw "+varName
	})
	return found
}

func extractCaughtTypeNameFlat(file *scanner.File, catchNode uint32) string {
	if file == nil || catchNode == 0 {
		return ""
	}
	if file.FlatType(catchNode) == "catch_clause" {
		last := ""
		for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "block" {
				break
			}
			file.FlatWalkAllNodes(child, func(idx uint32) {
				if file.FlatType(idx) == "type_identifier" || file.FlatType(idx) == "scoped_type_identifier" {
					last = flatTypeLastIdentifier(file, idx)
					if last == "" {
						last = strings.TrimSpace(file.FlatNodeText(idx))
					}
				}
			})
		}
		return last
	}
	if file.FlatType(catchNode) != "catch_block" {
		return ""
	}
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type":
			if name := flatTypeLastIdentifier(file, child); name != "" {
				return name
			}
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func flatTypeLastIdentifier(file *scanner.File, idx uint32) string {
	last := ""
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "type_identifier", "simple_identifier", "identifier":
			last = file.FlatNodeString(candidate, nil)
		}
	})
	return last
}

// ---------------------------------------------------------------------------
// TooGenericExceptionThrownRule detects throwing Exception/Throwable.
// ---------------------------------------------------------------------------
type TooGenericExceptionThrownRule struct {
	FlatDispatchBase
	BaseRule
	ExceptionNames []string // configurable list of generic exception types
}

// Confidence reports a tier-2 (medium) base confidence — matches on
// thrown-exception names; name-only fallback false-positives on
// project-defined exceptions with the same name. Classified per roadmap/17.
func (r *TooGenericExceptionThrownRule) Confidence() float64 { return api.ConfidenceMedium }

var defaultGenericThrownNames = []string{"Exception", "Throwable", "Error", "RuntimeException"}

var genericThrownFQNs = map[string]bool{
	"java.lang.Error":            true,
	"java.lang.Exception":        true,
	"java.lang.RuntimeException": true,
	"java.lang.Throwable":        true,
}

// checkKotlinThrow ports detekt 1.23.8 TooGenericExceptionThrown: the callee or name of the thrown
// expression must be in the configured names. Qualified names such as java.lang.Exception(...) are skipped,
// as in detekt.
func (r *TooGenericExceptionThrownRule) checkKotlinThrow(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "throw" {
		return
	}
	var thrown uint32
	for c := file.FlatNextSib(first); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			thrown = c
			break
		}
	}
	if thrown == 0 {
		return
	}
	name := ""
	switch file.FlatType(thrown) {
	case "simple_identifier":
		name = file.FlatNodeText(thrown)
	case "call_expression":
		cur := thrown
		for file.FlatType(file.FlatFirstChild(cur)) == "call_expression" {
			cur = file.FlatFirstChild(cur)
		}
		if callee := file.FlatFirstChild(cur); file.FlatType(callee) == "simple_identifier" {
			name = file.FlatNodeText(callee)
		}
	}
	names := r.ExceptionNames
	if len(names) == 0 {
		names = defaultGenericThrownNames
	}
	for _, n := range names {
		if n == name {
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				name+" is a too generic Exception. Prefer throwing specific exceptions that indicate a specific error case.")
			return
		}
	}
}

func (r *TooGenericExceptionThrownRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) == "jump_expression" {
		r.checkKotlinThrow(ctx)
		return
	}
	if scanner.IsTestFile(file.Path) {
		return
	}
	names := r.ExceptionNames
	if len(names) == 0 {
		names = defaultGenericThrownNames
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	thrownType, call := thrownExceptionConstructorNameFlat(file, idx)
	if thrownType == "" || call == 0 {
		return
	}
	// Fakes throw generic exceptions on purpose to simulate failures.
	if enclosedInTestDouble(file, idx) {
		return
	}
	if thrownConstructorWrapsCaughtException(file, call, idx) {
		return
	}
	if !thrownGenericExceptionMatches(ctx, thrownType, nameSet) {
		return
	}
	// Point at the constructor call, which can sit on a later line than the `throw`.
	ctx.EmitAtNode(call, fmt.Sprintf("Too-generic exception type '%s' thrown.", thrownType))
}

func thrownExceptionConstructorNameFlat(file *scanner.File, throwNode uint32) (string, uint32) {
	if file == nil || throwNode == 0 {
		return "", 0
	}
	// jump_expression also covers return/break/continue; only a throw counts.
	if file.FlatType(throwNode) == "jump_expression" && !strings.HasPrefix(file.FlatNodeText(throwNode), "throw") {
		return "", 0
	}
	// Only the constructor call directly under the throw is the thrown type.
	for child := file.FlatFirstChild(throwNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "call_expression":
			return flatCallExpressionName(file, child), child
		case "object_creation_expression":
			return flatLastIdentifierInNode(file, child), child
		}
	}
	return "", 0
}

func thrownConstructorWrapsCaughtException(file *scanner.File, call uint32, throwNode uint32) bool {
	catchNode, ok := flatEnclosingAncestor(file, throwNode, "catch_block", "catch_clause")
	if !ok {
		return false
	}
	caughtVar := extractCaughtVarNameFlat(file, catchNode)
	if caughtVar == "" {
		return false
	}
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr != 0 && file.FlatType(expr) == "simple_identifier" && file.FlatNodeString(expr, nil) == caughtVar {
			return true
		}
	}
	return false
}

func thrownGenericExceptionMatches(ctx *api.Context, thrownType string, nameSet map[string]bool) bool {
	if !nameSet[thrownType] {
		return false
	}
	if ctx.File == nil {
		return true
	}
	// A same-file class with the same simple name shadows java.lang.Exception,
	// RuntimeException, etc. Skip rather than guessing.
	if flatFindSameFileClassLikeDeclaration(ctx.File, thrownType) != 0 {
		return false
	}
	if ctx.Resolver == nil {
		return true
	}
	if fqn := ctx.Resolver.ResolveImport(thrownType, ctx.File); fqn != "" {
		return genericThrownFQNs[fqn]
	}
	if info := ctx.Resolver.ClassHierarchy(thrownType); info != nil && info.FQN != "" {
		return genericThrownFQNs[info.FQN]
	}
	return true
}
