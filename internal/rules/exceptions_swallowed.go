package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SwallowedExceptionRule detects catch blocks that either never use the exception
// variable or that throw a new exception without passing the original as the cause.
// Referencing only e.message or e.toString() (directly or via a variable) in a
// throw counts as swallowed.
type SwallowedExceptionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be swallowed
	IgnoredExceptionTypes     []string       // exception types that are allowed to be swallowed
	LoggingCountsAsHandling   bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *SwallowedExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SwallowedExceptionRule) makeUnusedFindingFlat(ctx *api.Context, caughtVar string) {
	idx, file := ctx.Idx, ctx.File
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Exception '%s' is caught but never used. Either log/handle it or rethrow.", caughtVar))
	endByte := int(file.FlatEndByte(idx))
	if endByte > 0 && file.Content[endByte-1] == '}' {
		catchLine := file.Lines[file.FlatRow(idx)]
		indent := ""
		for _, ch := range catchLine {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		insertion := indent + "    throw " + caughtVar + "\n" + indent
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   endByte - 1,
			EndByte:     endByte,
			Replacement: insertion + "}",
		}
	}
	ctx.Emit(f)
}

func (r *SwallowedExceptionRule) checkSwallowedException(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}
	if swallowedCatchSuppressesRule(file, idx) {
		return
	}
	caughtVar := extractCaughtVarNameFlat(file, idx)
	if caughtVar == "" || caughtVar == "_" {
		return
	}
	if r.AllowedExceptionNameRegex != nil && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
		return
	}
	caughtType := extractCaughtTypeNameFlat(file, idx)
	if len(r.IgnoredExceptionTypes) > 0 && caughtType != "" {
		lowerType := strings.ToLower(caughtType)
		for _, ignored := range r.IgnoredExceptionTypes {
			if strings.Contains(lowerType, strings.ToLower(ignored)) {
				return
			}
		}
	}
	if isCatchPartOfTryExpressionFlat(file, idx) {
		return
	}
	if caughtType == "EOFException" {
		return
	}

	result := r.analyzeSwallowedCatchBody(ctx, idx, caughtVar)
	if result.handled {
		return
	}
	if result.indirectOnly || result.anyCaughtReference {
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Exception '%s' is caught but not meaningfully handled. Pass it directly to logging, handling, or rethrowing code.", caughtVar)))
		return
	}
	// The caught variable is unused. Distinguish a genuine swallow
	// (empty-but-for-a-comment, or log-the-string-and-drop-the-cause) from a
	// catch that performs real recovery work without touching the exception
	// (a fallback assignment, an early return, a toast, …). Recovery is not a
	// swallow, and a truly empty body is already owned by EmptyCatchBlock.
	switch swallowedClassifyNeverUsed(file, idx) {
	case swallowedNeverUsedRecovery, swallowedNeverUsedEmpty:
		return
	}
	r.makeUnusedFindingFlat(ctx, caughtVar)
}

func swallowedCatchSuppressesRule(file *scanner.File, catchNode uint32) bool {
	if file == nil || catchNode == 0 {
		return false
	}
	text := file.FlatNodeText(catchNode)
	if !strings.Contains(text, "@Suppress") {
		return false
	}
	return strings.Contains(text, "\"SwallowedException\"") ||
		strings.Contains(text, "\"exceptions:SwallowedException\"") ||
		strings.Contains(text, "\"all\"")
}

type swallowedCatchResult struct {
	handled            bool
	indirectOnly       bool
	anyCaughtReference bool
}

func (r *SwallowedExceptionRule) analyzeSwallowedCatchBody(ctx *api.Context, catchNode uint32, caughtVar string) swallowedCatchResult {
	file := ctx.File
	body := swallowedCatchStatements(file, catchNode)
	if body == 0 || !swallowedHasNamedStatement(file, body) {
		return swallowedCatchResult{}
	}

	directAliases := map[string]bool{caughtVar: true}
	derivedAliases := map[string]bool{}
	swallowedWalkCatchBody(file, body, func(node uint32) bool {
		// Both `val x = <init>` property declarations and `when (val x =
		// <init>)` subject bindings introduce an alias of the caught
		// exception when their initializer reads it.
		var name string
		var init uint32
		switch file.FlatType(node) {
		case "property_declaration":
			name = swallowedPropertyDeclarationName(file, node)
			init = swallowedPropertyInitializer(file, node)
		case "when_subject":
			name = swallowedPropertyDeclarationName(file, node)
			init = swallowedPropertyInitializer(file, node)
		default:
			return true
		}
		if name == "" || init == 0 {
			return true
		}
		// `val cause = e.cause` (or `e.getCause()`) carries the original
		// throwable forward — using `cause` afterwards is meaningful handling
		// that preserves the chain, unlike a `.message`/`.toString()` String
		// projection. Treat such a variable as a *direct* alias of the caught
		// exception so later uses count as handling.
		if swallowedExpressionIsCauseUnwrap(file, init, directAliases) {
			directAliases[name] = true
			return true
		}
		switch swallowedExpressionAliasKind(file, init, directAliases, derivedAliases) {
		case swallowedAliasDirect:
			directAliases[name] = true
		case swallowedAliasDerived:
			derivedAliases[name] = true
		}
		return true
	})

	result := swallowedCatchResult{}
	swallowedWalkCatchBody(file, body, func(node uint32) bool {
		if result.handled {
			return false
		}
		if file.FlatType(node) == "simple_identifier" && swallowedIdentifierReferencesCaught(file, node, directAliases, derivedAliases) {
			result.anyCaughtReference = true
		}
		switch file.FlatType(node) {
		case "jump_expression":
			jumpResult := swallowedAnalyzeJump(file, node, directAliases, derivedAliases)
			if jumpResult.handled {
				result.handled = true
				return false
			}
			result.indirectOnly = result.indirectOnly || jumpResult.indirectOnly
		case "call_expression":
			callResult := r.swallowedAnalyzeCall(ctx, catchNode, node, directAliases, derivedAliases)
			if callResult.handled {
				result.handled = true
				return false
			}
			result.indirectOnly = result.indirectOnly || callResult.indirectOnly
		case "assignment", "augmented_assignment":
			if swallowedSubtreeReferencesCaught(file, node, directAliases, derivedAliases) {
				result.handled = true
				return false
			}
		case "return_expression":
			result.handled = true
			return false
		}
		return true
	})
	return result
}

type swallowedEvidence struct {
	handled      bool
	indirectOnly bool
}

type swallowedAliasKind uint8

const (
	swallowedAliasNone swallowedAliasKind = iota
	swallowedAliasDirect
	swallowedAliasDerived
)

func swallowedCatchStatements(file *scanner.File, catchNode uint32) uint32 {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "statements" {
			return child
		}
	}
	return 0
}

// swallowedNeverUsedKind classifies a catch body whose caught variable is
// *unused* (no reference, no proven handling) into one of three buckets used
// to decide whether the "caught but never used" finding is warranted.
type swallowedNeverUsedKind uint8

const (
	// swallowedNeverUsedEmpty: a truly empty body (no statements, no comment).
	// EmptyCatchBlock already owns this, so SwallowedException must not also
	// report it.
	swallowedNeverUsedEmpty swallowedNeverUsedKind = iota
	// swallowedNeverUsedLogDrop: the body only logs (and/or holds comments)
	// and drops the cause — the genuine swallow this rule should flag.
	swallowedNeverUsedLogDrop
	// swallowedNeverUsedRecovery: the body performs real recovery work
	// (control-flow jump, assignment, fallback call, …) without referencing
	// the exception — not a swallow; suppressed.
	swallowedNeverUsedRecovery
)

// swallowedClassifyNeverUsed inspects the direct statements of a catch body
// (Kotlin `statements` or Java `block`) and returns whether the body is empty,
// a pure log-and-drop swallow, or genuine recovery. It is only consulted when
// the caught variable is unused.
func swallowedClassifyNeverUsed(file *scanner.File, catchNode uint32) swallowedNeverUsedKind {
	body := swallowedNeverUsedBody(file, catchNode)
	if body == 0 {
		// No statements/block node. The catch is empty save for any comments
		// that hang directly off the catch node (a comment-only swallow that
		// EmptyCatchBlock skips). Treat a comment-only catch as a log-drop
		// swallow; a truly empty one is deferred to EmptyCatchBlock.
		if swallowedCatchHasDirectComment(file, catchNode) {
			return swallowedNeverUsedLogDrop
		}
		return swallowedNeverUsedEmpty
	}
	sawLog := false
	sawComment := false
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "{", "}":
			continue
		case "line_comment", "multiline_comment", "block_comment", "comment", "shebang_line":
			sawComment = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if swallowedStatementIsLoggingCall(file, child) {
			sawLog = true
			continue
		}
		// A bare scope-function call (`run { … }`, `let { … }`, …) is not real
		// recovery — it is just a block wrapper. The analysis intentionally
		// does not credit handling that happens inside the nested lambda, so a
		// catch whose only statement wraps a logging/handling call in a scope
		// function is still treated as a (lambda-buried) swallow.
		if swallowedStatementIsScopeFunctionCall(file, child) {
			sawLog = true
			continue
		}
		// Any other substantive statement — assignment, return/break/continue,
		// throw, non-logging fallback call, control flow, local declaration —
		// is real recovery work, not a silent swallow.
		return swallowedNeverUsedRecovery
	}
	if sawLog || sawComment {
		return swallowedNeverUsedLogDrop
	}
	return swallowedNeverUsedEmpty
}

// swallowedStatementIsScopeFunctionCall reports whether a catch-body statement
// is a bare call to a Kotlin scope function (`run`, `let`, `also`, `apply`,
// `with`, `runCatching`). These wrap a lambda rather than performing recovery.
func swallowedStatementIsScopeFunctionCall(file *scanner.File, stmt uint32) bool {
	call := stmt
	if file.FlatType(call) != "call_expression" {
		inner := uint32(0)
		count := 0
		for c := file.FlatFirstChild(stmt); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				inner = c
				count++
			}
		}
		if count != 1 {
			return false
		}
		call = inner
	}
	if file.FlatType(call) != "call_expression" {
		return false
	}
	callee, _ := swallowedCallTarget(file, call)
	switch callee {
	case "run", "let", "also", "apply", "with", "runCatching":
		return true
	default:
		return false
	}
}

// swallowedNeverUsedBody returns the statement container of a catch body:
// the Kotlin `statements` node or the Java `block` node.
func swallowedNeverUsedBody(file *scanner.File, catchNode uint32) uint32 {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "statements":
			return child
		case "block":
			return child
		}
	}
	return 0
}

// swallowedCatchHasDirectComment reports whether a comment node hangs directly
// off the catch node — the shape of a comment-only catch body that has no
// `statements`/`block` node.
func swallowedCatchHasDirectComment(file *scanner.File, catchNode uint32) bool {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "line_comment", "multiline_comment", "block_comment", "comment":
			return true
		}
	}
	return false
}

// swallowedStatementIsLoggingCall reports whether a catch-body statement is a
// bare logging/println call (possibly the only thing in a "log-the-string-but-
// drop-the-cause" catch). A statement may be the call_expression directly or
// wrap one (e.g. Java's expression_statement).
func swallowedStatementIsLoggingCall(file *scanner.File, stmt uint32) bool {
	call := stmt
	if file.FlatType(call) != "call_expression" && file.FlatType(call) != "method_invocation" {
		// Unwrap a single named child (e.g. Java expression_statement → call).
		inner := uint32(0)
		count := 0
		for c := file.FlatFirstChild(stmt); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				inner = c
				count++
			}
		}
		if count != 1 {
			return false
		}
		call = inner
	}
	switch file.FlatType(call) {
	case "call_expression", "method_invocation":
		callee, _ := swallowedCallTarget(file, call)
		return callee != "" && swallowedLoggingCallee(callee)
	}
	return false
}

// swallowedCallResultIsConsumed reports whether the result of the call at node
// flows somewhere — it is an argument, a returned/thrown value, an assignment
// or property initializer right-hand side, a receiver in a navigation chain,
// etc. — rather than being a bare expression statement whose value is
// discarded. A bare `ignored(e)` statement is *not* consumed.
func swallowedCallResultIsConsumed(file *scanner.File, node uint32) bool {
	parent, ok := file.FlatParent(node)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "statements", "control_structure_body", "block":
		// Direct child of a statement container. Such a value is discarded
		// *unless* it is the tail expression of a catch body whose enclosing
		// `try` is itself in a consumed (expression) position — then the
		// wrapped exception flows out as the try's value (e.g. a `catch`
		// branch whose body is `Result.Failure(e)` used as a `when`/return
		// arm). Only the *last* named statement can be such a tail.
		if file.FlatType(parent) == "statements" && swallowedIsLastNamedStatement(file, parent, node) {
			if swallowedCatchTryIsConsumed(file, parent) {
				return true
			}
			// A lambda's last expression supplies its value. Count it only
			// when the lambda expression itself is consumed by its caller.
			owner, ok := file.FlatParent(parent)
			if ok && file.FlatType(owner) == "lambda_literal" {
				return swallowedCallResultIsConsumed(file, owner)
			}
		}
		return false
	case "function_body":
		// Kotlin expression-body functions return their body expression. A
		// block-bodied function_body is a statement scope and is not value use.
		if !flatFunctionBodyHasBlock(file, parent) {
			return true
		}
	}
	return true
}

func flatFunctionBodyHasBlock(file *scanner.File, body uint32) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "block" || file.FlatType(child) == "statements" {
			return true
		}
	}
	return false
}

// swallowedIsLastNamedStatement reports whether node is the final named child
// of the statements container.
func swallowedIsLastNamedStatement(file *scanner.File, statements, node uint32) bool {
	last := uint32(0)
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			last = child
		}
	}
	return last == node
}

// swallowedCatchTryIsConsumed reports whether the `statements` node is the body
// of a catch whose enclosing `try_expression` is in a consumed position (its
// value flows somewhere rather than being a discarded statement).
func swallowedCatchTryIsConsumed(file *scanner.File, statements uint32) bool {
	catchNode, ok := file.FlatParent(statements)
	if !ok {
		return false
	}
	if t := file.FlatType(catchNode); t != "catch_block" && t != "catch_clause" {
		return false
	}
	tryNode, ok := file.FlatParent(catchNode)
	if !ok || file.FlatType(tryNode) != "try_expression" {
		return false
	}
	tryParent, ok := file.FlatParent(tryNode)
	if !ok {
		return false
	}
	switch file.FlatType(tryParent) {
	case "statements", "control_structure_body", "block":
		return false
	}
	return true
}

func swallowedHasNamedStatement(file *scanner.File, statements uint32) bool {
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return true
		}
	}
	return false
}

func swallowedWalkCatchBody(file *scanner.File, root uint32, fn func(uint32) bool) {
	var walk func(uint32) bool
	walk = func(node uint32) bool {
		if node != root {
			switch file.FlatType(node) {
			case "lambda_literal", "function_declaration", "class_declaration", "object_declaration",
				// A nested catch re-binds the exception name in its own scope;
				// its body's references belong to that catch, not the outer
				// one, so they must not count toward the outer catch's usage.
				"catch_block", "catch_clause":
				return true
			}
		}
		if !fn(node) {
			return false
		}
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !walk(child) {
				return false
			}
		}
		return true
	}
	walk(root)
}
