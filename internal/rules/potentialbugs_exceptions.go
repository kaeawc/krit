package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
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
func (r *PrintStackTraceRule) Confidence() float64 { return 0.75 }

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
func (r *TooGenericExceptionThrownRule) Confidence() float64 { return 0.75 }

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

// ---------------------------------------------------------------------------
// UnreachableCatchBlockRule detects catch shadowed by general catch above.
// ---------------------------------------------------------------------------
type UnreachableCatchBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — catch-block
// reachability depends on the thrown-type hierarchy from the resolver;
// heuristic fallback uses name containment. Classified per roadmap/17.
func (r *UnreachableCatchBlockRule) Confidence() float64 { return 0.75 }

func (r *UnreachableCatchBlockRule) checkFlatNode(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	type catchEntry struct {
		typeName string
		line     int
	}
	var catches []catchEntry
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "catch_block" {
			continue
		}
		typeName := extractCatchTypeFlat(file, child)
		if typeName == "" {
			continue
		}
		catches = append(catches, catchEntry{
			typeName: typeName,
			line:     file.FlatRow(child) + 1,
		})
	}
	for i := 0; i < len(catches); i++ {
		for j := i + 1; j < len(catches); j++ {
			parentType := catches[i].typeName
			childType := catches[j].typeName
			childLine := catches[j].line
			if parentType == childType {
				ctx.Emit(r.Finding(file, childLine, 1,
					fmt.Sprintf("Duplicate catch block for '%s'.", childType)))
				continue
			}
			if ctx.Resolver != nil {
				if ctx.Resolver.IsExceptionSubtype(childType, parentType) {
					ctx.Emit(r.Finding(file, childLine, 1,
						fmt.Sprintf("Catch block for '%s' is unreachable because '%s' is caught above.", childType, parentType)))
				}
			} else {
				if typeinfer.IsSubtypeOfException(childType, parentType) {
					ctx.Emit(r.Finding(file, childLine, 1,
						fmt.Sprintf("Catch block for '%s' is unreachable because '%s' is caught above.", childType, parentType)))
				}
			}
		}
	}
}

func extractCatchTypeFlat(file *scanner.File, catchNode uint32) string {
	for i := 0; i < file.FlatChildCount(catchNode); i++ {
		child := file.FlatChild(catchNode, i)
		if file.FlatType(child) == "user_type" {
			text := strings.TrimSpace(file.FlatNodeText(child))
			if idx := strings.Index(text, "<"); idx >= 0 {
				text = text[:idx]
			}
			if text != "" {
				return text
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// UnreachableCodeRule detects code after return/throw/break/continue,
// Nothing-returning calls like TODO() and error(), exhaustive when expressions,
// if expressions where all branches terminate, and infinite loops (while(true)).
//
// When the oracle provides compiler diagnostics (UNREACHABLE_CODE, USELESS_ELVIS),
// those are used authoritatively — no heuristic false positives.
//
// Limitations of heuristic fallback (without oracle diagnostics):
//   - Cannot detect useless elvis (KaFirDiagnostic.UselessElvis)
//
// ---------------------------------------------------------------------------
type UnreachableCodeRule struct {
	FlatDispatchBase
	BaseRule
}

// oracleDiagnosticFactories lists the Kotlin compiler diagnostic factory names
// that this rule consumes from the oracle.
var oracleDiagnosticFactories = map[string]bool{
	"UNREACHABLE_CODE": true,
	"USELESS_ELVIS":    true,
}

// Confidence reports a tier-2 (medium) base confidence. When the
// oracle provides compiler diagnostics the rule is authoritative
// (the Kotlin compiler already decided the code is unreachable), but
// the heuristic fallback walks sibling nodes looking for
// return/throw/break/continue and can miss cases involving labeled
// jumps, when-exhaustive-with-Nothing returns, or
// compiler-intrinsic functions beyond the small nothingReturningFuncs
// allow-list (TODO, error). Findings produced via the diagnostic
// path could override to 0.95 per-finding; the rule-level base
// reflects the fallback's accuracy.
func (r *UnreachableCodeRule) Confidence() float64 { return 0.75 }

// nothingReturningFuncs lists bare function names that are known to return Nothing.
//
// The heuristic-only callers (blockTerminatesFlat) require this to be a
// conservative allow-list — any name added here is treated as Nothing-returning
// even without resolver evidence. Names that overlap with common user-defined
// helpers (like `fail`) are intentionally omitted; they get picked up via the
// resolver-backed path when the resolver can prove a Nothing return type.
var nothingReturningFuncs = map[string]bool{
	"TODO":        true,
	"error":       true,
	"exitProcess": true,
}

// nothingReturningQualifierPrefixes are the leading navigation segments
// accepted as qualifiers for a Nothing-returning stdlib call. `kotlin.error`
// or `kotlin.system.exitProcess` qualify; arbitrary user-defined receivers
// like `myObject.error` do not.
var nothingReturningQualifierPrefixes = [][]string{
	{"kotlin"},
	{"kotlin", "system"},
	{"kotlin", "test"},
}

func (r *UnreachableCodeRule) checkNode(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// If oracle has compiler diagnostics for this file, use those (authoritative, no false positives).
	var oracleLookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		oracleLookup = cr.Oracle()
	}
	if oracleLookup != nil {
		diags := oracleLookupDiagnosticsForFlatRange(oracleLookup, file, idx)
		if len(diags) > 0 {
			r.checkWithDiagnosticsFlat(ctx, diags)
			return
		}
	}

	// Fall back to heuristic analysis.
	foundJump := false
	skipNext := false
	jumpLine := 0
	skipUntilRow := -1
	childCount := file.FlatChildCount(idx)
	for i := 0; i < childCount; i++ {
		child := file.FlatChild(idx, i)
		if foundJump {
			if unreachableCodeSkipSibling(file, child, skipNext, skipUntilRow) {
				if skipNext && !isFlatCommentNode(file, child) && file.FlatType(child) != "label" {
					skipNext = false
				}
				continue
			}
			r.emitUnreachableFinding(ctx, file, child, i, childCount, idx, jumpLine)
			return
		}
		if newJump, newLine, newSkipNext, newSkipUntilRow := unreachableCodeDetectJump(file, child, i, childCount, idx, skipUntilRow, ctx.Resolver); newJump {
			foundJump = true
			jumpLine = newLine
			skipNext = newSkipNext
			if newSkipUntilRow > skipUntilRow {
				skipUntilRow = newSkipUntilRow
			}
		}
	}
}

func unreachableCodeSkipSibling(file *scanner.File, child uint32, skipNext bool, skipUntilRow int) bool {
	if isFlatCommentNode(file, child) {
		return true
	}
	if file.FlatType(child) == "label" {
		return true
	}
	if skipNext {
		return true
	}
	if skipUntilRow >= 0 && file.FlatRow(child) <= skipUntilRow {
		return true
	}
	return unreachableCodeLooksLikeJumpExpressionContinuation(file, child)
}

func (r *UnreachableCodeRule) emitUnreachableFinding(ctx *api.Context, file *scanner.File, child uint32, i, childCount int, idx uint32, jumpLine int) {
	f := r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
		fmt.Sprintf("Unreachable code detected after jump statement at line %d.", jumpLine))
	startByte := int(file.FlatStartByte(child))
	endByte := startByte
	for j := i; j < childCount; j++ {
		c := file.FlatChild(idx, j)
		if isFlatCommentNode(file, c) {
			continue
		}
		endByte = int(file.FlatEndByte(c))
	}
	for endByte < len(file.Content) && (file.Content[endByte] == '\n' || file.Content[endByte] == '\r') {
		endByte++
	}
	adjustedStart := startByte
	for adjustedStart > 0 && (file.Content[adjustedStart-1] == ' ' || file.Content[adjustedStart-1] == '\t') {
		adjustedStart--
	}
	if adjustedStart > 0 && file.Content[adjustedStart-1] == '\n' {
		adjustedStart--
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   adjustedStart,
		EndByte:     endByte,
		Replacement: "",
	}
	ctx.Emit(f)
}

func unreachableCodeDetectJump(file *scanner.File, child uint32, i, childCount int, idx uint32, skipUntilRow int, resolver typeinfer.TypeResolver) (foundJump bool, jumpLine int, skipNext bool, newSkipUntilRow int) {
	newSkipUntilRow = skipUntilRow
	if file.FlatType(child) == "jump_expression" {
		text := file.FlatNodeText(child)
		if strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw") || strings.HasPrefix(text, "break") || strings.HasPrefix(text, "continue") {
			trimmed := strings.TrimSpace(text)
			isBareReturn := trimmed == "return" || strings.HasPrefix(trimmed, "return@")
			isBareThrow := trimmed == "throw"
			isReturnOrThrow := strings.HasPrefix(trimmed, "return") || strings.HasPrefix(trimmed, "throw")
			if isReturnOrThrow {
				if endRow := unreachableCodeJumpContinuationEndRow(file, file.FlatRow(child)); endRow > newSkipUntilRow {
					newSkipUntilRow = endRow
				}
				for peek := i + 1; peek < childCount; peek++ {
					sibling := file.FlatChild(idx, peek)
					if isFlatCommentNode(file, sibling) {
						continue
					}
					if (isBareReturn || isBareThrow) && file.FlatRow(sibling) == file.FlatRow(child) {
						skipNext = true
					} else if unreachableCodeLooksLikeJumpExpressionContinuation(file, sibling) {
						skipNext = true
					}
					break
				}
			}
			return true, file.FlatRow(child) + 1, skipNext, newSkipUntilRow
		}
	}
	if isNothingReturningCallFlat(file, child, resolver) {
		return true, file.FlatRow(child) + 1, false, newSkipUntilRow
	}
	if file.FlatType(child) == "if_expression" && ifAllBranchesTerminateFlat(file, child) {
		return true, file.FlatRow(child) + 1, false, newSkipUntilRow
	}
	if file.FlatType(child) == "when_expression" && whenIsExhaustiveAndTerminatesFlat(file, child, resolver) {
		return true, file.FlatRow(child) + 1, false, newSkipUntilRow
	}
	if file.FlatType(child) == "while_statement" && isInfiniteLoopFlat(file, child) {
		return true, file.FlatRow(child) + 1, false, newSkipUntilRow
	}
	return false, 0, false, newSkipUntilRow
}

func unreachableCodeLooksLikeJumpExpressionContinuation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	trimmed := strings.TrimSpace(file.FlatNodeText(idx))
	if trimmed == "" {
		return false
	}
	for _, prefix := range []string{"as ", "as? ", "is ", "!is ", ".", "?.", "?:", "!!"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	for _, line := range strings.Split(trimmed, "\n")[1:] {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"as ", "as? ", "is ", "!is ", ".", "?.", "?:", "!!"} {
			if strings.HasPrefix(line, prefix) {
				return true
			}
		}
	}
	return false
}

func unreachableCodeJumpContinuationEndRow(file *scanner.File, jumpRow int) int {
	if file == nil || jumpRow < 0 || jumpRow >= len(file.Lines) {
		return -1
	}
	depth := unreachableCodeBraceDelta(file.Lines[jumpRow])
	for row := jumpRow + 1; row < len(file.Lines); row++ {
		trimmed := strings.TrimSpace(file.Lines[row])
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if unreachableCodeLineStartsContinuation(trimmed) {
			return row
		}
		if depth <= 0 {
			return -1
		}
		depth += unreachableCodeBraceDelta(file.Lines[row])
	}
	return -1
}

func unreachableCodeLineStartsContinuation(trimmed string) bool {
	for _, prefix := range []string{"as ", "as? ", "is ", "!is ", ".", "?.", "?:", "!!"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func unreachableCodeBraceDelta(line string) int {
	depth := 0
	for _, r := range line {
		switch r {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
		}
	}
	return depth
}

// checkWithDiagnosticsFlat uses compiler diagnostics from the oracle to find unreachable code
// within the given statements node. Only diagnostics whose line falls within the node's
// range and whose factoryName matches a known diagnostic are reported.
func (r *UnreachableCodeRule) checkWithDiagnosticsFlat(ctx *api.Context, diags []oracle.Diagnostic) {
	idx, file := ctx.Idx, ctx.File
	startLine := file.FlatRow(idx) + 1
	endLine := flatEndLine(file, idx)

	for _, d := range diags {
		if !oracleDiagnosticFactories[d.FactoryName] {
			continue
		}
		if d.Line < startLine || d.Line > endLine {
			continue
		}
		msg := fmt.Sprintf("Unreachable code detected (%s).", d.FactoryName)
		if d.Message != "" {
			msg = fmt.Sprintf("Unreachable code: %s", d.Message)
		}
		ctx.Emit(r.Finding(file, d.Line, d.Col, msg))
	}
}

func flatEndLine(file *scanner.File, idx uint32) int {
	if file == nil || idx == 0 {
		return 0
	}
	text := file.FlatNodeText(idx)
	lines := strings.Count(text, "\n")
	return file.FlatRow(idx) + 1 + lines
}

func isFlatCommentNode(file *scanner.File, idx uint32) bool {
	return file != nil && (file.FlatType(idx) == "line_comment" || file.FlatType(idx) == "multiline_comment")
}

func isNothingCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	name := flatCallExpressionName(file, idx)
	if !nothingReturningFuncs[name] {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav == 0 {
		return true
	}
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) == 0 || segments[len(segments)-1] != name {
		return false
	}
	prefix := segments[:len(segments)-1]
	for _, accepted := range nothingReturningQualifierPrefixes {
		if stringSliceEqual(prefix, accepted) {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isNothingReturningCallFlat returns true for any call_expression whose
// callee provably returns Nothing. It first checks the static heuristic
// table (isNothingCallFlat) for stdlib globals, then asks the resolver
// for the call's inferred type when the call has no receiver. The
// resolver-backed path catches workspace functions declared with
// `: Nothing` return type.
//
// The resolver fallback is restricted to bare calls because the
// resolver's call-type inference falls back to top-level stdlib lookup
// when a navigation receiver doesn't yield a matching method, which can
// produce a misleading Nothing for harmless calls like `logger.error()`.
// Limiting to bare calls avoids that false positive while still catching
// the high-value case (workspace `fun X(): Nothing`).
//
// Returns false when the resolver is nil or the inferred type is unknown.
func isNothingReturningCallFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if isNothingCallFlat(file, idx) {
		return true
	}
	if resolver == nil || file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	if nav, _ := flatCallExpressionParts(file, idx); nav != 0 {
		// Has a receiver; the resolver's fallback to top-level stdlib
		// lookup can produce a wrong Nothing here. Stay conservative.
		return false
	}
	t := resolver.ResolveFlatNode(idx, file)
	if t == nil {
		return false
	}
	return t.Name == "Nothing"
}

// ifAllBranchesTerminateFlat checks if an if_expression has both then and else branches
// and each branch ends with a return/throw/Nothing-call.
func ifAllBranchesTerminateFlat(file *scanner.File, idx uint32) bool {
	var thenBlock, elseBlock uint32
	foundElse := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatNodeTextEquals(child, "else") {
			foundElse = true
			continue
		}
		if file.FlatType(child) == "control_structure_body" {
			if !foundElse {
				thenBlock = child
			} else {
				elseBlock = child
			}
		}
		// else branch could be another if_expression (else if)
		if foundElse && file.FlatType(child) == "if_expression" {
			if ifAllBranchesTerminateFlat(file, child) {
				elseBlock = child // mark as terminating
			} else {
				return false
			}
		}
	}
	if thenBlock == 0 || !foundElse {
		return false
	}
	if elseBlock == 0 && !foundElse {
		return false
	}
	// Check then block terminates
	if !blockTerminatesFlat(file, thenBlock) {
		return false
	}
	// If else branch was an if_expression that already passed, it terminates
	if elseBlock != 0 && file.FlatType(elseBlock) == "if_expression" {
		return true // already checked recursively above
	}
	if elseBlock == 0 {
		return false
	}
	return blockTerminatesFlat(file, elseBlock)
}

// blockTerminatesFlat checks if the last statement in a block is a jump or Nothing-call.
func blockTerminatesFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	// Unwrap single control_structure_body → statements
	stmts := idx
	if file.FlatType(idx) == "control_structure_body" {
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "statements" {
				stmts = child
				break
			}
		}
	}
	// Find the last non-comment child
	var last uint32
	for i := file.FlatChildCount(stmts) - 1; i >= 0; i-- {
		child := file.FlatChild(stmts, i)
		if file.FlatType(child) != "line_comment" && file.FlatType(child) != "multiline_comment" &&
			file.FlatType(child) != "{" && file.FlatType(child) != "}" {
			last = child
			break
		}
	}
	if last == 0 {
		return false
	}
	if file.FlatType(last) == "jump_expression" {
		text := file.FlatNodeText(last)
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	}
	if isNothingCallFlat(file, last) {
		return true
	}
	// Recursively check nested if/when
	if file.FlatType(last) == "if_expression" {
		return ifAllBranchesTerminateFlat(file, last)
	}
	return false
}

// whenIsExhaustiveAndTerminates checks if a when expression has an else branch
// (or covers all sealed/enum variants) and all branches terminate.
func whenIsExhaustiveAndTerminatesFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	hasElse := false
	allTerminate := true
	branchCount := 0

	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "when_entry" {
			continue
		}
		branchCount++

		// Check for else branch
		entryText := file.FlatNodeText(child)
		if strings.HasPrefix(strings.TrimSpace(entryText), "else") {
			hasElse = true
		}

		// Check if the body of this entry terminates
		body, _ := file.FlatFindChild(child, "control_structure_body")
		if body == 0 {
			// Could be a single expression entry — check the last child
			var lastChild uint32
			for j := file.FlatChildCount(child) - 1; j >= 0; j-- {
				c := file.FlatChild(child, j)
				if !isFlatCommentNode(file, c) {
					lastChild = c
					break
				}
			}
			if lastChild != 0 && (file.FlatType(lastChild) == "jump_expression" || isNothingCallFlat(file, lastChild)) {
				continue
			}
			allTerminate = false
			continue
		}
		if !blockTerminatesFlat(file, body) {
			allTerminate = false
		}
	}

	if branchCount == 0 {
		return false
	}

	// If there's an else branch and all branches terminate, code after is unreachable
	if hasElse && allTerminate {
		return true
	}

	// Check if when subject is a sealed class/enum and all variants are covered
	if !hasElse && allTerminate && resolver != nil {
		subjectName := whenSubjectTypeNameFlat(file, idx, resolver)
		if subjectName != "" {
			variants := resolver.SealedVariants(subjectName)
			if len(variants) > 0 && branchCount >= len(variants) {
				return true
			}
			entries := resolver.EnumEntries(subjectName)
			if len(entries) > 0 && branchCount >= len(entries) {
				return true
			}
		}
	}

	return false
}

// whenSubjectTypeName extracts the type name of the when subject expression.
func whenSubjectTypeNameFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) string {
	// when(expr) — look for the subject expression
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "when_subject" {
			// The actual expression is inside when_subject
			for j := 0; j < file.FlatChildCount(child); j++ {
				inner := file.FlatChild(child, j)
				if file.FlatType(inner) == "simple_identifier" {
					resolved := resolver.ResolveFlatNode(inner, file)
					if resolved != nil && resolved.Name != "" {
						return resolved.Name
					}
				}
			}
		}
	}
	return ""
}

// isInfiniteLoop checks if a while_statement is while(true) with no break.
func isInfiniteLoopFlat(file *scanner.File, idx uint32) bool {
	// Check condition is literal "true"
	condFound := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		text := strings.TrimSpace(file.FlatNodeText(child))
		if file.FlatType(child) == "boolean_literal" && text == "true" {
			condFound = true
			break
		}
		// Condition may be wrapped in parentheses
		if text == "(true)" || text == "( true )" {
			condFound = true
			break
		}
	}
	if !condFound {
		return false
	}
	// Check no break statement exists in the loop body
	hasBreak := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if hasBreak {
			return
		}
		if file.FlatType(child) == "jump_expression" {
			text := file.FlatNodeText(child)
			if strings.HasPrefix(text, "break") {
				hasBreak = true
			}
		}
	})
	return !hasBreak
}

// ---------------------------------------------------------------------------
// MissingReturnRule — flags block-bodied functions with a non-Unit/non-Nothing
// declared return type whose body cannot guarantee a returned value on every
// path. Mimics kotlinc's NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY diagnostic
// using AST + the same termination helpers UnreachableCode uses.
//
// Rule scope is intentionally narrow:
//   - block-bodied functions only (`{ ... }`); expression bodies (`= expr`)
//     always return their expression value
//   - functions with an explicit declared return type only; the implicit
//     Unit case is silent
//   - return type "Unit"/"Nothing" (and the kotlin.X qualified forms) is
//     skipped because Unit doesn't need a return and Nothing-returning
//     functions can't return normally
//   - abstract / interface / expect / external functions have no body and
//     are skipped
//
// ---------------------------------------------------------------------------
type MissingReturnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MissingReturnRule) Confidence() float64 { return 0.85 }

func missingReturnDeclaredType(file *scanner.File, fn uint32) (typeIdx uint32, body uint32) {
	foundParams := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters":
			foundParams = true
		case "function_body":
			body = child
		case "user_type", "nullable_type", "type_identifier":
			if foundParams && typeIdx == 0 {
				typeIdx = child
			}
		}
	}
	return typeIdx, body
}

func missingReturnTypeIsUnitOrNothing(text string) bool {
	switch strings.TrimSpace(text) {
	case "Unit", "Nothing", "kotlin.Unit", "kotlin.Nothing":
		return true
	}
	return false
}

func missingReturnIsExpressionBody(file *scanner.File, body uint32) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			return true
		}
		if file.FlatIsNamed(child) {
			// Reached a named child (e.g. statements) before any `=`. It's a
			// block body.
			return false
		}
	}
	return false
}

// missingReturnFunctionTerminates returns true when the block body is
// guaranteed to terminate on every path with a return, throw,
// Nothing-returning call, exhaustive-if, or exhaustive-when.
func missingReturnFunctionTerminates(file *scanner.File, body uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || body == 0 {
		return false
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 {
		return false
	}
	var last uint32
	for i := file.FlatChildCount(stmts) - 1; i >= 0; i-- {
		c := file.FlatChild(stmts, i)
		t := file.FlatType(c)
		if t == "line_comment" || t == "multiline_comment" || t == "{" || t == "}" {
			continue
		}
		last = c
		break
	}
	if last == 0 {
		return false
	}
	switch file.FlatType(last) {
	case "jump_expression":
		text := file.FlatNodeText(last)
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	case "if_expression":
		return ifAllBranchesTerminateFlat(file, last)
	case "when_expression":
		return whenIsExhaustiveAndTerminatesFlat(file, last, resolver)
	}
	return isNothingReturningCallFlat(file, last, resolver)
}

func (r *MissingReturnRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "function_declaration" {
		return
	}
	if file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "expect") || file.FlatHasModifier(idx, "external") {
		return
	}
	retType, body := missingReturnDeclaredType(file, idx)
	if body == 0 || retType == 0 {
		return
	}
	if missingReturnTypeIsUnitOrNothing(file.FlatNodeText(retType)) {
		return
	}
	if missingReturnIsExpressionBody(file, body) {
		return
	}
	if missingReturnFunctionTerminates(file, body, ctx.Resolver) {
		return
	}
	name := flatDeclarationNameLocal(file, idx)
	typeText := strings.TrimSpace(file.FlatNodeText(retType))
	msg := fmt.Sprintf("Function '%s' declares return type '%s' but its body may not return on every path. Add a `return` or terminate every branch.", name, typeText)
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
}

// flatDeclarationNameLocal returns the function declaration's
// simple_identifier name. Local helper to avoid pulling in the
// resolver package's flatDeclarationName.
func flatDeclarationNameLocal(file *scanner.File, fn uint32) string {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return "<anonymous>"
}
