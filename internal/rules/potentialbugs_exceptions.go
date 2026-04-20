package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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



// ---------------------------------------------------------------------------
// asyncBoundaryBaseClasses are base classes where catching generic exceptions
// is a best practice because they run on background threads and uncaught
// exceptions would crash the app.
var asyncBoundaryBaseClasses = map[string]bool{
	"Job":             true,
	"BaseJob":         true,
	"Worker":          true,
	"CoroutineWorker": true,
	"Runnable":        true,
	"Callable":        true,
	"Thread":          true,
	"AsyncTask":       true,
	"TimerTask":       true,
}

// TooGenericExceptionCaughtRule detects catching Exception/Throwable.
// ---------------------------------------------------------------------------
type TooGenericExceptionCaughtRule struct {
	FlatDispatchBase
	BaseRule
	resolver                  typeinfer.TypeResolver
	ExceptionNames            []string       // configurable list of generic exception types
	AllowedExceptionNameRegex *regexp.Regexp // exception var names matching this are allowed
}

func (r *TooGenericExceptionCaughtRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — matches on
// exception-type names; with the resolver it can respect custom
// hierarchies, without it flags by name only. Classified per roadmap/17.
func (r *TooGenericExceptionCaughtRule) Confidence() float64 { return 0.75 }

var genericCaughtTypes = []string{
	"Exception", "Throwable", "Error", "RuntimeException",
	"NullPointerException", "ArrayIndexOutOfBoundsException",
	"IndexOutOfBoundsException", "IllegalMonitorStateException",
}

// genericExceptionSet allows O(1) lookups for the generic exception list.
var genericExceptionSet = func() map[string]bool {
	m := make(map[string]bool, len(genericCaughtTypes))
	for _, t := range genericCaughtTypes {
		m[t] = true
	}
	return m
}()


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

func (r *TooGenericExceptionCaughtRule) checkNode(ctx *v2.Context) {
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
	if caughtVar != "" {
		text := file.FlatNodeText(idx)
		braceIdx := strings.Index(text, "{")
		if braceIdx >= 0 {
			body := text[braceIdx+1:]
			// Look for the variable being passed as an argument: `(...e...)`
			// or `, e,` or `, e)` patterns.
			argPattern := regexp.MustCompile(`[,(]\s*` + regexp.QuoteMeta(caughtVar) + `\s*[,)]`)
			if argPattern.MatchString(body) {
				return
			}
		}
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
		// Fallback: scan node text for generic types
		text := file.FlatNodeText(idx)
		for _, t := range exNames {
			if strings.Contains(text, ": "+t) || strings.Contains(text, ":"+t) {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Caught too-generic exception type '%s'.", t)))
				return
			}
		}
		return
	}

	// Direct match against the configured list
	exSet := r.exceptionNameSet()
	if exSet[caughtType] {
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Caught too-generic exception type '%s'.", caughtType)))
		return
	}

	// With resolver, check if the caught type is a known subtype of a generic exception
	if r.resolver != nil {
		for _, generic := range exNames {
			if r.resolver.IsExceptionSubtype(generic, caughtType) {
				// generic IS-A caughtType means caughtType is more general
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Caught too-generic exception type '%s' (catches subtypes like '%s').", caughtType, generic)))
				return
			}
		}
	} else {
		// Fallback without resolver: use global table
		for _, generic := range exNames {
			if typeinfer.IsSubtypeOfException(generic, caughtType) {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Caught too-generic exception type '%s' (catches subtypes like '%s').", caughtType, generic)))
				return
			}
		}
	}
}

func extractCaughtVarNameFlat(file *scanner.File, catchNode uint32) string {
	text := file.FlatNodeText(catchNode)
	re := regexp.MustCompile(`catch\s*\(\s*(\w+)\s*:`)
	m := re.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func extractCaughtTypeNameFlat(file *scanner.File, catchNode uint32) string {
	text := file.FlatNodeText(catchNode)
	re := regexp.MustCompile(`catch\s*\(\s*\w+\s*:\s*(\w+)`)
	m := re.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
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
			if asyncBoundaryBaseClasses[typeName] {
				return true
			}
			if strings.HasSuffix(typeName, "Job") ||
				strings.HasSuffix(typeName, "Worker") ||
				strings.HasSuffix(typeName, "Task") {
				return true
			}
		}
		return false
	}
	return false
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
	resolver       typeinfer.TypeResolver
	ExceptionNames []string // configurable list of generic exception types
}

func (r *TooGenericExceptionThrownRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — matches on
// thrown-exception names; name-only fallback false-positives on
// project-defined exceptions with the same name. Classified per roadmap/17.
func (r *TooGenericExceptionThrownRule) Confidence() float64 { return 0.75 }

var defaultGenericThrownNames = []string{"Exception", "Throwable", "Error", "RuntimeException"}

var genericThrownRe = regexp.MustCompile(`\bthrow\s+(\w+)\s*\(([^)]*)\)`)

// isLikelyCaughtException returns true if args looks like a single identifier
// that could be a caught exception variable (single word, starts lowercase).
// Used to exempt the exception-chaining idiom `throw RuntimeException(e)`.
func isLikelyCaughtException(args string) bool {
	// Must be a single identifier, no commas, no string literals, no dots.
	if strings.ContainsAny(args, `,".(`) {
		return false
	}
	if args == "" {
		return false
	}
	c := args[0]
	if !(c >= 'a' && c <= 'z') && c != '_' {
		return false
	}
	// Common caught-exception names.
	switch args {
	case "e", "ex", "exc", "t", "throwable", "cause", "err":
		return true
	}
	// Single lowercase identifier — likely a local variable.
	for i := 0; i < len(args); i++ {
		c := args[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (r *TooGenericExceptionThrownRule) check(ctx *v2.Context) {
	file := ctx.File
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
	for i, line := range file.Lines {
		if m := genericThrownRe.FindStringSubmatch(line); m != nil {
			thrownType := m[1]
			args := strings.TrimSpace(m[2])
			// Skip exception-chaining idiom: `throw RuntimeException(e)` where
			// `e` is a caught Throwable being wrapped.
			if args != "" && isLikelyCaughtException(args) {
				continue
			}
			// Direct match against known generic types
			if nameSet[thrownType] {
				ctx.Emit(r.Finding(file, i+1, 1,
					fmt.Sprintf("Too-generic exception type '%s' thrown.", thrownType)))
				continue
			}
			// With resolver, check if the thrown type resolves to a known generic exception
			// (e.g., imported under an alias). Do NOT flag subtypes — only the generic types themselves.
			if r.resolver != nil {
				info := r.resolver.ClassHierarchy(thrownType)
				if info != nil && nameSet[info.Name] {
					ctx.Emit(r.Finding(file, i+1, 1,
						fmt.Sprintf("Too-generic exception type '%s' thrown.", info.Name)))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// UnreachableCatchBlockRule detects catch shadowed by general catch above.
// ---------------------------------------------------------------------------
type UnreachableCatchBlockRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UnreachableCatchBlockRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — catch-block
// reachability depends on the thrown-type hierarchy from the resolver;
// heuristic fallback uses name containment. Classified per roadmap/17.
func (r *UnreachableCatchBlockRule) Confidence() float64 { return 0.75 }

var catchTypeRe = regexp.MustCompile(`catch\s*\(\s*\w+\s*:\s*(\w+)`)

func (r *UnreachableCatchBlockRule) checkFlatNode(ctx *v2.Context) {
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
			if r.resolver != nil {
				if r.resolver.IsExceptionSubtype(childType, parentType) {
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
	text := file.FlatNodeText(catchNode)
	if m := catchTypeRe.FindStringSubmatch(text); m != nil {
		return m[1]
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
	resolver     typeinfer.TypeResolver
	oracleLookup oracle.Lookup
}

// oracleDiagnosticFactories lists the Kotlin compiler diagnostic factory names
// that this rule consumes from the oracle.
var oracleDiagnosticFactories = map[string]bool{
	"UNREACHABLE_CODE": true,
	"USELESS_ELVIS":    true,
}

func (r *UnreachableCodeRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
	if cr, ok := res.(*oracle.CompositeResolver); ok {
		r.oracleLookup = cr.Oracle()
	}
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
var nothingReturningFuncs = map[string]bool{
	"TODO":  true,
	"error": true,
}

func (r *UnreachableCodeRule) checkNode(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	// If oracle has compiler diagnostics for this file, use those (authoritative, no false positives).
	if r.oracleLookup != nil {
		diags := r.oracleLookup.LookupDiagnostics(file.Path)
		if len(diags) > 0 {
			r.checkWithDiagnosticsFlat(ctx, diags)
			return
		}
	}

	// Fall back to heuristic analysis.
	foundJump := false
	skipNext := false // true when tree-sitter split return/throw value into a separate sibling
	jumpLine := 0
	childCount := file.FlatChildCount(idx)
	for i := 0; i < childCount; i++ {
		child := file.FlatChild(idx, i)
		if foundJump {
			// Skip blank/comment nodes
			if isFlatCommentNode(file, child) {
				continue
			}
			// Skip label nodes (targets for labeled break/continue)
			if file.FlatType(child) == "label" {
				continue
			}
			// When tree-sitter splits "return expr" or "throw expr" into two siblings,
			// the next non-comment sibling is the return/throw value, not unreachable code.
			if skipNext {
				skipNext = false
				continue
			}
			f := r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
				fmt.Sprintf("Unreachable code detected after jump statement at line %d.", jumpLine))
			// Fix: delete all unreachable statements from this point to the end of the block
			startByte := int(file.FlatStartByte(child))
			endByte := startByte
			// Walk remaining children to find the last unreachable statement
			for j := i; j < childCount; j++ {
				c := file.FlatChild(idx, j)
				if isFlatCommentNode(file, c) {
					continue
				}
				endByte = int(file.FlatEndByte(c))
			}
			// Also consume trailing whitespace/newline after the last unreachable node
			for endByte < len(file.Content) && (file.Content[endByte] == '\n' || file.Content[endByte] == '\r') {
				endByte++
			}
			// Also remove leading newline before the unreachable code
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
			return
		}
		// Detect jump_expression: return, throw, break, continue
		if file.FlatType(child) == "jump_expression" {
			text := file.FlatNodeText(child)
			if strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw") || strings.HasPrefix(text, "break") || strings.HasPrefix(text, "continue") {
				foundJump = true
				jumpLine = file.FlatRow(child) + 1
				// Tree-sitter may split "return expr" or "throw expr" into two sibling nodes:
				// the jump_expression contains only the keyword, and the value is a separate sibling.
				// Detect this by checking if the text is just the bare keyword (possibly with a label).
				trimmed := strings.TrimSpace(text)
				isBareReturn := trimmed == "return" || strings.HasPrefix(trimmed, "return@")
				isBareThrow := trimmed == "throw"
				if isBareReturn || isBareThrow {
					// Check if the next non-comment sibling is on the same line (i.e., it's the value).
					jumpRow := file.FlatRow(child)
					for peek := i + 1; peek < childCount; peek++ {
						sibling := file.FlatChild(idx, peek)
						if isFlatCommentNode(file, sibling) {
							continue
						}
						if file.FlatRow(sibling) == jumpRow {
							// Same line — this is the return/throw value, skip it.
							skipNext = true
						}
						break
					}
				}
			}
		}
		// Detect Nothing-returning calls: TODO(), error()
		if !foundJump && isNothingCallFlat(file, child) {
			foundJump = true
			jumpLine = file.FlatRow(child) + 1
		}
		// Detect if expressions where all branches terminate
		if !foundJump && file.FlatType(child) == "if_expression" && ifAllBranchesTerminateFlat(file, child) {
			foundJump = true
			jumpLine = file.FlatRow(child) + 1
		}
		// Detect exhaustive when expressions (all branches terminate)
		if !foundJump && file.FlatType(child) == "when_expression" && whenIsExhaustiveAndTerminatesFlat(file, child, r.resolver) {
			foundJump = true
			jumpLine = file.FlatRow(child) + 1
		}
		// Detect infinite loops: while(true) with no break
		if !foundJump && file.FlatType(child) == "while_statement" && isInfiniteLoopFlat(file, child) {
			foundJump = true
			jumpLine = file.FlatRow(child) + 1
		}
	}
}

// checkWithDiagnosticsFlat uses compiler diagnostics from the oracle to find unreachable code
// within the given statements node. Only diagnostics whose line falls within the node's
// range and whose factoryName matches a known diagnostic are reported.
func (r *UnreachableCodeRule) checkWithDiagnosticsFlat(ctx *v2.Context, diags []oracle.OracleDiagnostic) {
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
	return nothingReturningFuncs[flatCallExpressionName(file, idx)]
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
		body := file.FlatFindChild(child, "control_structure_body")
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
