package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/coroutines"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// RedundantSuspendModifierRule detects suspend functions with no suspend calls inside.
// With type inference: uses ResolveNode on call expressions to check if the call
// target is a suspend function, beyond the hardcoded known list.
// With oracle: uses oracle call-target metadata to distinguish resolved suspend
// calls from resolved non-suspend calls, with known FQN/name fallbacks for older
// oracle payloads.
type RedundantSuspendModifierRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule uses a name-based allow-list (commonNonSuspendCallees) and a
// call-target suspend metadata and the known-suspend-FQN set to decide whether a call is suspending. It
// relies on the oracle for the accurate case; without it, any
// identifier not in the allow-list is treated as potentially-suspend
// and the rule suppresses the finding, so the remaining positives are
// reliable but the rule may miss cases where the suspend modifier is
// genuinely redundant.
func (r *RedundantSuspendModifierRule) Confidence() float64 { return api.ConfidenceMedium }

// Known suspend functions from the standard library / coroutines.
var knownSuspendFunctions = map[string]bool{
	"delay": true, "await": true, "withContext": true, "coroutineScope": true,
	"supervisorScope": true, "yield": true, "withTimeout": true,
	"withTimeoutOrNull": true, "awaitAll": true, "joinAll": true,
	"suspendCoroutine": true, "suspendCancellableCoroutine": true,
	"emit": true, "collect": true, "send": true, "receive": true,
	"receiveCatching": true, "join": true, "cancelAndJoin": true,
	"mutex": true, "acquire": true, "launch": true, "async": true,
	"runBlocking": true,
}

// knownSuspendFQNs maps fully-qualified function names to true.
// Used when the oracle resolves a call target to its FQN.
var knownSuspendFQNs = map[string]bool{
	"kotlinx.coroutines.delay":                                   true,
	"kotlinx.coroutines.yield":                                   true,
	"kotlinx.coroutines.withContext":                             true,
	"kotlinx.coroutines.coroutineScope":                          true,
	"kotlinx.coroutines.supervisorScope":                         true,
	"kotlinx.coroutines.withTimeout":                             true,
	"kotlinx.coroutines.withTimeoutOrNull":                       true,
	"kotlinx.coroutines.awaitAll":                                true,
	"kotlinx.coroutines.joinAll":                                 true,
	"kotlinx.coroutines.launch":                                  true,
	"kotlinx.coroutines.async":                                   true,
	"kotlinx.coroutines.runBlocking":                             true,
	"kotlinx.coroutines.suspendCancellableCoroutine":             true,
	"kotlin.coroutines.suspendCoroutine":                         true,
	"kotlinx.coroutines.flow.Flow.collect":                       true,
	"kotlinx.coroutines.flow.Flow.emit":                          true,
	"kotlinx.coroutines.flow.FlowCollector.emit":                 true,
	"kotlinx.coroutines.channels.SendChannel.send":               true,
	"kotlinx.coroutines.channels.ReceiveChannel.receive":         true,
	"kotlinx.coroutines.channels.ReceiveChannel.receiveCatching": true,
	"kotlinx.coroutines.Job.join":                                true,
	"kotlinx.coroutines.Job.cancelAndJoin":                       true,
	"kotlinx.coroutines.Deferred.await":                          true,
	"kotlinx.coroutines.sync.Mutex.lock":                         true,
}

// suspendFQNPrefixes are package prefixes that indicate a call target is likely
// a suspend function when no exact FQN match is found.
var suspendFQNPrefixes = []string{
	"kotlinx.coroutines.",
}

// commonNonSuspendCallees is a small allow-list of stdlib/common identifiers
// that are definitely not suspend functions. Any other identifier is treated
// as potentially-suspend and suppresses the RedundantSuspendModifier finding.
var commonNonSuspendCallees = map[string]bool{
	"println": true, "print": true,
	"require": true, "check": true, "error": true,
	"requireNotNull": true, "checkNotNull": true,
	"listOf": true, "setOf": true, "mapOf": true, "arrayOf": true,
	"mutableListOf": true, "mutableSetOf": true, "mutableMapOf": true,
	"emptyList": true, "emptySet": true, "emptyMap": true,
	"Pair": true, "Triple": true,
	"String": true, "Int": true, "Long": true, "Float": true, "Double": true,
}

// walkCallsRespectingScope walks descendants of root, invoking fn on every
// call_expression whose enclosing function scope is `root`. The walk stops
// descending into nested function declarations, anonymous functions, classes,
// objects, and object literals because their bodies define their own scopes
// and any suspend calls inside them do not belong to the parent function.
// Lambda literals are intentionally NOT excluded: lambdas passed to inline
// functions (withContext, coroutineScope, runCatching, apply, let, also, ...)
// execute in the enclosing suspend context, and without resolver-level
// knowledge of inlineness we keep the conservative behavior of treating
// lambdas as part of the parent scope.
//
// The callback returns true to keep walking and false to stop the entire walk.
func walkCallsRespectingScope(file *scanner.File, root uint32, fn func(uint32) bool) {
	walkCallsRespectingScopeSkipping(file, root, nil, fn)
}

// walkCallsRespectingScopeSkipping is walkCallsRespectingScope with a hook that keeps the walk out of a call's
// arguments, for calls whose lambdas run in their own context.
func walkCallsRespectingScopeSkipping(file *scanner.File, root uint32, skipArguments func(call uint32) bool, fn func(uint32) bool) {
	if file == nil || file.FlatTree == nil || fn == nil {
		return
	}
	stopped := false
	var visit func(node uint32)
	var walkChildren func(idx uint32, skipSuffix bool)
	visit = func(node uint32) {
		switch file.FlatType(node) {
		case "function_declaration",
			"anonymous_function",
			"class_declaration",
			"object_declaration",
			"object_literal":
			// Skip: nested scope owns its own suspend context.
			return
		case "call_expression":
			if !fn(node) {
				stopped = true
				return
			}
			walkChildren(node, skipArguments != nil && skipArguments(node))
			return
		}
		walkChildren(node, false)
	}
	walkChildren = func(idx uint32, skipSuffix bool) {
		for child := file.FlatTree.FirstChildren[idx]; child != 0; child = file.FlatTree.NextSibs[child] {
			if stopped {
				return
			}
			if skipSuffix && file.FlatType(child) == "call_suffix" {
				continue
			}
			visit(child)
		}
	}
	walkChildren(root, false)
}

// nonInlineScopeBuilders are calls whose lambda runs later or elsewhere, so a suspend call inside it does not
// make the surrounding code suspend.
var nonInlineScopeBuilders = map[string]bool{
	"launch": true, "async": true, "produce": true, "actor": true, "runBlocking": true,
	"flow": true, "channelFlow": true, "callbackFlow": true,
}

// SleepInsteadOfDelayRule detects Thread.sleep() usage inside suspend functions
// and coroutine builder lambdas (launch, async, runBlocking, withContext, etc.).
type SleepInsteadOfDelayRule struct {
	FlatDispatchBase
	BaseRule
}

// coroutineBuilders are functions whose trailing lambda runs in a suspend context.
var coroutineBuilders = map[string]bool{
	"launch": true, "async": true, "runBlocking": true,
	"withContext": true, "coroutineScope": true, "supervisorScope": true,
	"withTimeout": true, "withTimeoutOrNull": true,
	// Builders whose block parameter has a suspend function type.
	"flow": true, "channelFlow": true, "callbackFlow": true,
	"produce": true, "actor": true, "runTest": true,
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SleepInsteadOfDelayRule) Confidence() float64 { return api.ConfidenceMedium }

func isInsideSuspendContextFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			return hasSuspendModifierFlat(file, p)
		case "lambda_literal":
			if isCoroutineBuilderLambdaFlat(file, p) {
				return true
			}
			return false
		}
	}
	return false
}

func isCoroutineBuilderLambdaFlat(file *scanner.File, lambdaNode uint32) bool {
	parent, ok := file.FlatParent(lambdaNode)
	if !ok {
		return false
	}
	if file.FlatType(parent) == "annotated_lambda" {
		parent, ok = file.FlatParent(parent)
		if !ok {
			return false
		}
	}
	if file.FlatType(parent) != "call_suffix" {
		return false
	}
	callExpr, ok := file.FlatParent(parent)
	if !ok || file.FlatType(callExpr) != "call_expression" {
		return false
	}
	calleeName := flatCallExpressionName(file, callExpr)
	return coroutineBuilders[calleeName]
}

// SuspendFunWithFlowReturnTypeRule detects suspend fun returning Flow.
// SuspendFunWithFlowReturnTypeRule has been moved to
// internal/rules/coroutines as the first rule of the per-domain
// subpackage migration. The alias keeps Meta() / IsFixable() and the
// AllMetaProviders sample list working unchanged.
type SuspendFunWithFlowReturnTypeRule = coroutines.SuspendFunWithFlowReturnTypeRule

// CoroutineLaunchedInTestWithoutRunTestRule detects launch/async in @Test without runTest.
type CoroutineLaunchedInTestWithoutRunTestRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *CoroutineLaunchedInTestWithoutRunTestRule) Confidence() float64 { return api.ConfidenceMedium }

// SuspendFunInFinallySectionRule detects suspend calls in finally blocks.
type SuspendFunInFinallySectionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunInFinallySectionRule) Confidence() float64 { return api.ConfidenceMedium }

// SuspendFunSwallowedCancellationRule detects catching CancellationException without rethrow.
type SuspendFunSwallowedCancellationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// catch blocks swallow CancellationException without rethrow is structural
// but depends on the resolver to recognize aliases of
// CancellationException. Classified per roadmap/17.
func (r *SuspendFunSwallowedCancellationRule) Confidence() float64 { return api.ConfidenceMedium }

func enclosingTryExpressionFlat(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "try_expression" {
			return parent
		}
		if file.FlatType(parent) == "function_declaration" {
			return 0
		}
	}
	return 0
}

func isInsideSuspendFunctionFlat(file *scanner.File, idx uint32) bool {
	fn, ok := flatEnclosingAncestor(file, idx, "function_declaration")
	return ok && hasSuspendModifierFlat(file, fn)
}

func tryBlockHasSuspendCallFlat(file *scanner.File, tryExpr uint32, resolver typeinfer.TypeResolver) bool {
	body := tryBodyFlat(file, tryExpr)
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if found {
			return
		}
		if callInsideNestedFunctionFlat(file, body, call) {
			return
		}
		if resolver != nil {
			if provider, ok := resolver.(oracleLookupProvider); ok && provider.Oracle() != nil {
				if isSuspend, ok := oracleLookupCallTargetSuspendFlat(provider.Oracle(), file, call); ok {
					found = isSuspend
					return
				}
			}
		}
		found = isSuspendingCallName(flatCallExpressionName(file, call))
	})
	return found
}

func tryBodyFlat(file *scanner.File, tryExpr uint32) uint32 {
	if file == nil || tryExpr == 0 {
		return 0
	}
	for child := file.FlatFirstChild(tryExpr); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "catch_block", "finally_block":
			return 0
		case "control_structure_body", "statements":
			return child
		}
	}
	return 0
}

func callInsideNestedFunctionFlat(file *scanner.File, root, call uint32) bool {
	for parent, ok := file.FlatParent(call); ok && parent != root; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "function_declaration" || file.FlatType(parent) == "lambda_literal" {
			return true
		}
	}
	return false
}

// SuspendFunWithCoroutineScopeReceiverRule detects suspend fun CoroutineScope.x().
type SuspendFunWithCoroutineScopeReceiverRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunWithCoroutineScopeReceiverRule) Confidence() float64 { return api.ConfidenceMedium }

func hasSuspendModifierFlat(file *scanner.File, idx uint32) bool {
	return file.FlatHasModifier(idx, "suspend")
}

// suspendFunctionLocalNames returns the parameter and local variable names declared in the function.
func suspendFunctionLocalNames(file *scanner.File, fn uint32) map[string]bool {
	locals := make(map[string]bool)
	file.FlatWalkAllNodes(fn, func(n uint32) {
		switch file.FlatType(n) {
		case "parameter", "variable_declaration":
			if name := extractIdentifierFlat(file, n); name != "" {
				locals[name] = true
			}
		}
	})
	return locals
}

// suspendCallIsPossibleChain reports a member call that may be suspend: a known dual-use name, an `await*` call, or
// any member call on a receiver that is not a local of the function.
func suspendCallIsPossibleChain(file *scanner.File, call uint32, name string, locals map[string]bool) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatType(navExpr) != "navigation_expression" {
		return false
	}
	if coroutines.DualUseChainNames[name] || strings.HasPrefix(name, "await") {
		return true
	}
	root := navExpr
	for file.FlatType(root) == "navigation_expression" || file.FlatType(root) == "call_expression" {
		first := file.FlatFirstChild(root)
		if first == 0 {
			return true
		}
		root = first
	}
	if file.FlatType(root) != "simple_identifier" {
		return true
	}
	return !locals[file.FlatNodeText(root)]
}

type forLoopVerdict int

const (
	forLoopPlain forLoopVerdict = iota
	forLoopUnknown
	forLoopSuspends
)

// forLoopSuspendVerdict reports whether a `for` loop in body iterates a channel, whose iterator suspends in `hasNext`.
// Loops over values of unresolved type are reported as unknown so the caller stays conservative.
func forLoopSuspendVerdict(ctx *api.Context, body uint32) forLoopVerdict {
	file := ctx.File
	verdict := forLoopPlain
	file.FlatWalkNodes(body, "for_statement", func(loop uint32) {
		if verdict == forLoopSuspends || callInsideNestedFunctionFlat(file, body, loop) {
			return
		}
		iterable := forLoopIterable(file, loop)
		if iterable == 0 {
			return
		}
		switch file.FlatType(iterable) {
		case "range_expression", "string_literal":
			return
		}
		if ctx.Resolver == nil {
			verdict = forLoopUnknown
			return
		}
		t := ctx.Resolver.ResolveFlatNode(iterable, file)
		if t == nil || t.Kind == typeinfer.TypeUnknown {
			verdict = forLoopUnknown
			return
		}
		if strings.HasSuffix(t.Name, "Channel") || strings.HasSuffix(t.FQN, "Channel") {
			verdict = forLoopSuspends
		}
	})
	return verdict
}

// forLoopIterable returns the expression after `in` in a for_statement, or 0.
func forLoopIterable(file *scanner.File, loop uint32) uint32 {
	seenIn := false
	for child := file.FlatFirstChild(loop); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "in" {
			seenIn = true
			continue
		}
		if seenIn && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// nonSuspendingKnownNames are in knownSuspendFunctions to keep RedundantSuspendModifier conservative
// but never suspend or throw CancellationException themselves.
var nonSuspendingKnownNames = map[string]bool{"launch": true, "async": true, "runBlocking": true, "mutex": true}

// isSuspendingCallName reports a name that is a suspending call in kotlinx.coroutines.
func isSuspendingCallName(name string) bool {
	return knownSuspendFunctions[name] && !nonSuspendingKnownNames[name]
}

// suspendCallInNonCancellableBlock reports a call nested in `withContext(NonCancellable) { ... }` below limit,
// the recommended way to suspend during cleanup. The withContext call itself counts as inside.
func suspendCallInNonCancellableBlock(file *scanner.File, call, limit uint32) bool {
	for n := call; n != 0 && n != limit; {
		if file.FlatType(n) == "call_expression" && flatCallNameAny(file, n) == "withContext" {
			text := file.FlatNodeText(n)
			if brace := strings.Index(text, "{"); brace >= 0 {
				text = text[:brace]
			}
			if strings.Contains(text, "NonCancellable") {
				return true
			}
		}
		parent, ok := file.FlatParent(n)
		if !ok {
			return false
		}
		n = parent
	}
	return false
}

// earlierCatchHandlesCancellation reports a preceding catch clause of the same try that names CancellationException.
func earlierCatchHandlesCancellation(file *scanner.File, catchNode uint32) bool {
	for prev, ok := file.FlatPrevSibling(catchNode); ok; prev, ok = file.FlatPrevSibling(prev) {
		if file.FlatType(prev) == "catch_block" && extractCaughtTypeNameFlat(file, prev) == "CancellationException" {
			return true
		}
	}
	return false
}

// catchBodyCallsEnsureActive reports an `ensureActive()` call, which rethrows the cancellation when the job is cancelled.
func catchBodyCallsEnsureActive(file *scanner.File, catchNode uint32) bool {
	found := false
	file.FlatWalkNodes(catchNode, "call_expression", func(call uint32) {
		if !found && flatCallExpressionName(file, call) == "ensureActive" {
			found = true
		}
	})
	return found
}

// functionReceiverIsCoroutineScope reports whether a function declaration's text declares a
// `CoroutineScope` (optionally nullable or package-qualified) receiver.
func functionReceiverIsCoroutineScope(decl string) bool {
	funAt := strings.Index(decl, "fun ")
	if funAt < 0 {
		funAt = strings.Index(decl, "fun<")
		if funAt < 0 {
			return false
		}
	}
	rest := strings.TrimSpace(decl[funAt+3:])
	if strings.HasPrefix(rest, "<") {
		depth := 0
		for i, ch := range rest {
			if ch == '<' {
				depth++
			} else if ch == '>' {
				depth--
				if depth == 0 {
					rest = strings.TrimSpace(rest[i+1:])
					break
				}
			}
		}
	}
	head, _, ok := strings.Cut(rest, "(")
	if !ok {
		return false
	}
	dot := strings.LastIndex(head, ".")
	if dot < 0 {
		return false
	}
	receiver := strings.TrimSuffix(strings.TrimSpace(head[:dot]), "?")
	return receiver == "CoroutineScope" || receiver == "kotlinx.coroutines.CoroutineScope"
}

// receiverTypeIsCoroutineScope follows detekt: the receiver is CoroutineScope or has it as a supertype, through
// a typealias, a type parameter bound, or a class of the project.
func receiverTypeIsCoroutineScope(ctx *api.Context, fn uint32) bool {
	file := ctx.File
	var recv uint32
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "." {
			break
		}
		switch file.FlatType(c) {
		case "user_type", "nullable_type", "parenthesized_type":
			recv = c
		}
	}
	if recv == 0 {
		return false
	}
	return typeNodeIsCoroutineScope(ctx, fn, recv, map[string]bool{})
}

func typeNodeIsCoroutineScope(ctx *api.Context, at, typeNode uint32, seen map[string]bool) bool {
	file := ctx.File
	switch file.FlatType(typeNode) {
	case "nullable_type", "parenthesized_type":
		for c := file.FlatFirstChild(typeNode); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				return typeNodeIsCoroutineScope(ctx, at, c, seen)
			}
		}
		return false
	case "user_type":
		last := ""
		for c := file.FlatFirstChild(typeNode); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "type_identifier" {
				last = file.FlatNodeText(c)
			}
		}
		return nameIsCoroutineScope(ctx, at, last, seen)
	}
	return false
}

func nameIsCoroutineScope(ctx *api.Context, at uint32, name string, seen map[string]bool) bool {
	if name == "" || seen[name] {
		return false
	}
	seen[name] = true
	file := ctx.File
	if name == "CoroutineScope" {
		return true
	}
	// A type parameter of the function or an enclosing class: look at its bound.
	for p := at; p != 0; {
		if tp, ok := file.FlatFindChild(p, "type_parameters"); ok {
			for param := file.FlatFirstChild(tp); param != 0; param = file.FlatNextSib(param) {
				if file.FlatType(param) != "type_parameter" || file.FlatNamedChildCount(param) < 2 ||
					file.FlatNodeText(file.FlatNamedChild(param, 0)) != name {
					continue
				}
				return typeNodeIsCoroutineScope(ctx, at, file.FlatNamedChild(param, 1), seen)
			}
		}
		parent, ok := file.FlatParent(p)
		if !ok {
			break
		}
		p = parent
	}
	// A typealias of this file.
	aliased := false
	file.FlatWalkNodes(0, "type_alias", func(alias uint32) {
		if aliased {
			return
		}
		if id, ok := file.FlatFindChild(alias, "type_identifier"); ok && file.FlatNodeText(id) == name {
			if target, ok := file.FlatFindChild(alias, "user_type"); ok {
				aliased = typeNodeIsCoroutineScope(ctx, alias, target, seen)
			}
		}
	})
	if aliased {
		return true
	}
	if ctx.Resolver == nil {
		return false
	}
	if info := ctx.Resolver.ClassHierarchy(name); info != nil {
		for _, super := range info.Supertypes {
			short := super[strings.LastIndexByte(super, '.')+1:]
			if short == "CoroutineScope" || nameIsCoroutineScope(ctx, at, short, seen) {
				return true
			}
		}
	}
	return false
}
