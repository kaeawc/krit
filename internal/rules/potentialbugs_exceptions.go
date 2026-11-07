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

// asyncBoundarySimpleToFQN maps unqualified supertype names to their canonical FQN.
// A "" FQN means the type is always available without an import (java.lang.*).
var asyncBoundarySimpleToFQN = map[string]string{
	"Job":               "kotlinx.coroutines.Job",
	"AbstractCoroutine": "kotlinx.coroutines.AbstractCoroutine",
	"Worker":            "androidx.work.Worker",
	"CoroutineWorker":   "androidx.work.CoroutineWorker",
	"ListenableWorker":  "androidx.work.ListenableWorker",
	"Callable":          "java.util.concurrent.Callable",
	"Runnable":          "java.lang.Runnable",
	"Thread":            "java.lang.Thread",
	"AsyncTask":         "android.os.AsyncTask",
	"TimerTask":         "java.util.TimerTask",
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

// catchBodyPassesVarAsArgFlat returns true if varName appears as a value_argument
// anywhere inside the catch_block, or as the subject of a throw_expression.
// This avoids regexp.MustCompile in the hot dispatch path.
func catchBodyPassesVarAsArgFlat(file *scanner.File, catchNode uint32, varName string) bool {
	found := false
	file.FlatWalkAllNodes(catchNode, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "value_argument":
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "simple_identifier" && file.FlatNodeString(child, nil) == varName {
					found = true
					return
				}
			}
		case "throw_expression":
			file.FlatWalkAllNodes(idx, func(inner uint32) {
				if !found && file.FlatType(inner) == "simple_identifier" && file.FlatNodeString(inner, nil) == varName {
					found = true
				}
			})
		}
	})
	return found
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

func catchBodyCallsOnCaughtVar(file *scanner.File, catchNode uint32, varName string, callee string) bool {
	found := false
	file.FlatWalkNodes(catchNode, "call_expression", func(idx uint32) {
		if found || flatCallNameAny(file, idx) != callee {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		found = strings.HasPrefix(text, varName+".")
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

func isInsideAsyncBoundaryFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "lambda_literal" {
			call := p
			for {
				next, ok := file.FlatParent(call)
				if !ok {
					call = 0
					break
				}
				call = next
				if file.FlatType(call) == "call_expression" {
					break
				}
				if file.FlatType(call) == "function_declaration" {
					call = 0
					break
				}
			}
			if call != 0 && file.FlatType(call) == "call_expression" {
				switch flatCallExpressionName(file, call) {
				case "execute", "submit", "invokeLater", "post", "postDelayed",
					"runOnUiThread", "Thread", "thread", "launch", "async",
					"runInTransaction", "schedule":
					return true
				}
			}
		}
		if file.FlatType(p) != "class_declaration" {
			continue
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			child := file.FlatChild(p, i)
			if file.FlatType(child) != "delegation_specifier" {
				continue
			}
			typeName := viewConstructorSupertypeNameFlat(file, child)
			if typeName == "" {
				continue
			}
			// Match simple name AND verify the file imports the corresponding
			// package. java.lang.* types need no import.
			fqn, known := asyncBoundarySimpleToFQN[typeName]
			if !known {
				continue
			}
			if fqn == "java.lang.Runnable" || fqn == "java.lang.Thread" {
				// java.lang is always available; no import required.
				return true
			}
			if asyncBoundaryHasImport(file, fqn) {
				return true
			}
		}
		return false
	}
	return false
}

// asyncBoundaryHasImport reports whether the file contains an import for fqn.
func asyncBoundaryHasImport(file *scanner.File, fqn string) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		var parts []string
		file.FlatWalkAllNodes(node, func(child uint32) {
			switch file.FlatType(child) {
			case "simple_identifier", "type_identifier":
				parts = append(parts, file.FlatNodeString(child, nil))
			}
		})
		path := strings.Join(parts, ".")
		found = path == fqn
	})
	return found
}

func isCatchPartOfTryExpressionFlat(file *scanner.File, catchNode uint32) bool {
	tryExpr, ok := file.FlatParent(catchNode)
	if !ok || file.FlatType(tryExpr) != "try_expression" {
		return false
	}
	crossedStatements := false
	child := tryExpr
	for cur, ok := file.FlatParent(tryExpr); ok; {
		advance := true
		switch file.FlatType(cur) {
		case "property_declaration", "variable_declaration",
			"assignment", "augmented_assignment",
			"value_argument", "value_arguments",
			"return_expression", "jump_expression",
			"binary_expression", "elvis_expression",
			"parenthesized_expression":
			return true
		case "function_body":
			if file.FlatChildCount(cur) == 0 || file.FlatType(file.FlatChild(cur, 0)) != "=" {
				return false
			}
			return !crossedStatements
		case "statements":
			if cc := file.FlatChildCount(cur); cc > 0 && file.FlatChild(cur, cc-1) != child {
				crossedStatements = true
			}
		case "control_structure_body", "if_expression", "when_entry",
			"when_expression", "lambda_literal":
			advance = true
		case "function_declaration", "class_body", "source_file":
			return false
		}
		if !advance {
			continue
		}
		next, nextOK := file.FlatParent(cur)
		if !nextOK {
			break
		}
		child = cur
		cur = next
		ok = nextOK
	}
	return false
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
