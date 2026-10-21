package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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

func (r *TooGenericExceptionCaughtRule) checkNode(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Dedup with EmptyCatchBlock: if the catch body is literally empty,
	// EmptyCatchBlock already flags it and offers a fix.
	catchText := file.FlatNodeText(idx)
	if braceIdx := strings.Index(catchText, "{"); braceIdx >= 0 {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(catchText[braceIdx+1:]), "}"))
		if inner == "" {
			return
		}
	}
	// Check allowed exception variable name regex
	caughtVar := extractCaughtVarNameFlat(file, idx)
	if r.AllowedExceptionNameRegex != nil {
		if caughtVar != "" && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
			return
		}
	}
	if caughtVar != "" && catchBodyRethrowsCaughtException(file, idx, caughtVar) {
		return
	}
	if caughtVar != "" && catchBodyCallsOnCaughtVar(file, idx, caughtVar, "printStackTrace") {
		return
	}
	// Skip catches inside Job/Worker/Runnable classes — these are async
	// execution boundaries where catching generic Exception is a best practice
	// to prevent uncaught exception crashes.
	if isInsideAsyncBoundaryFlat(file, idx) {
		return
	}
	// Skip when the caught variable is passed as an argument to any function
	// call in the catch body. This covers:
	//   catch (e: Exception) { Log.w(TAG, "msg", e) }  — logs with exception
	//   catch (e: Throwable) { Result.Failure(e) }     — wraps into result
	//   catch (e: Exception) { throw Foo(e) }          — rewraps
	// The exception is NOT being swallowed; information is preserved.
	if caughtVar != "" && catchBodyPassesVarAsArgFlat(file, idx, caughtVar) {
		return
	}
	// Skip catches inside try expressions (return value is semantic fallback).
	if isCatchPartOfTryExpressionFlat(file, idx) {
		return
	}
	caughtType := extractCaughtTypeNameFlat(file, idx)
	exNames := r.ExceptionNames
	if len(exNames) == 0 {
		exNames = genericCaughtTypes
	}
	if caughtType == "" {
		// Fallback: scan node text for generic types — name-only heuristic.
		text := file.FlatNodeText(idx)
		for _, t := range exNames {
			if strings.Contains(text, ": "+t) || strings.Contains(text, ":"+t) {
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Caught too-generic exception type '%s'.", t))
				f.Confidence = 0.8
				ctx.Emit(f)
				return
			}
		}
		return
	}

	// Direct match against the configured list — AST name match, no type resolution.
	exSet := r.exceptionNameSet()
	if exSet[caughtType] {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Caught too-generic exception type '%s'.", caughtType))
		f.Confidence = 0.8
		ctx.Emit(f)
		return
	}

	// Use the built-in subtype table for known exception names. Project-specific
	// inheritance is intentionally out of scope for this AST/import-only rule.
	for _, generic := range exNames {
		if typeinfer.IsSubtypeOfException(generic, caughtType) {
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Caught too-generic exception type '%s' (catches subtypes like '%s').", caughtType, generic))
			f.Confidence = 0.8
			ctx.Emit(f)
			return
		}
	}
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

func (r *TooGenericExceptionThrownRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}
	// Skip Gradle build scripts — they legitimately throw RuntimeException
	// for build errors where the user's primary observability is stderr.
	if strings.HasSuffix(file.Path, ".gradle.kts") {
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
	if thrownConstructorWrapsCaughtException(file, call, idx) {
		return
	}
	if !thrownGenericExceptionMatches(ctx, thrownType, nameSet) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Too-generic exception type '%s' thrown.", thrownType)))
}

func thrownExceptionConstructorNameFlat(file *scanner.File, throwNode uint32) (string, uint32) {
	if file == nil || throwNode == 0 {
		return "", 0
	}
	var call uint32
	file.FlatWalkNodes(throwNode, "call_expression", func(candidate uint32) {
		if call == 0 {
			call = candidate
		}
	})
	if call != 0 {
		return flatCallExpressionName(file, call), call
	}
	file.FlatWalkNodes(throwNode, "object_creation_expression", func(candidate uint32) {
		if call == 0 {
			call = candidate
		}
	})
	if call != 0 {
		return flatLastIdentifierInNode(file, call), call
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
