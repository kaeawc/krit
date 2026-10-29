package rules

import (
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
	if file == nil || file.FlatTree == nil || fn == nil {
		return
	}
	stopped := false
	var walk func(idx uint32)
	walk = func(idx uint32) {
		if stopped {
			return
		}
		for child := file.FlatTree.FirstChildren[idx]; child != 0; child = file.FlatTree.NextSibs[child] {
			if stopped {
				return
			}
			ct := file.FlatType(child)
			switch ct {
			case "function_declaration",
				"anonymous_function",
				"class_declaration",
				"object_declaration",
				"object_literal":
				// Skip: nested scope owns its own suspend context.
				continue
			case "call_expression":
				if !fn(child) {
					stopped = true
					return
				}
			}
			walk(child)
		}
	}
	walk(root)
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
				if isSuspend, ok := provider.Oracle().LookupCallTargetSuspend(file.Path, file.FlatRow(call)+1, file.FlatCol(call)+1); ok {
					found = isSuspend
					return
				}
			}
		}
		found = knownSuspendFunctions[flatCallExpressionName(file, call)]
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
