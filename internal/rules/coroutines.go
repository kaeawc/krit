package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// CollectInOnCreateWithoutLifecycleRule detects Flow.collect calls in lifecycle
// callbacks that are not wrapped by repeatOnLifecycle.
type CollectInOnCreateWithoutLifecycleRule struct {
	FlatDispatchBase
	BaseRule
}

var lifecycleCollectCallbacks = map[string]bool{
	"onCreate":      true,
	"onStart":       true,
	"onViewCreated": true,
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *CollectInOnCreateWithoutLifecycleRule) Confidence() float64 { return 0.75 }

func hasAncestorCallNamedFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			return false
		}
		if file.FlatType(p) != "call_expression" {
			continue
		}
		if flatCallExpressionName(file, p) == name {
			return true
		}
		text := strings.TrimSpace(file.FlatNodeText(p))
		if strings.HasPrefix(text, name+"(") || strings.Contains(text, "."+name+"(") {
			return true
		}
	}
	return false
}

// GlobalCoroutineUsageRule detects GlobalScope.launch/async and direct GlobalScope references.
type GlobalCoroutineUsageRule struct {
	FlatDispatchBase
	BaseRule
}

// Description implements DescriptionProvider.
func (*GlobalCoroutineUsageRule) Description() string {
	return "Flags use of GlobalScope for launching coroutines. Coroutines in GlobalScope are not tied to any lifecycle and can leak if not cancelled. Prefer a structured CoroutineScope."
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *GlobalCoroutineUsageRule) Confidence() float64 { return 0.75 }

// InjectDispatcherRule detects hardcoded Dispatchers.IO/Default/Unconfined in call expressions.
type InjectDispatcherRule struct {
	FlatDispatchBase
	BaseRule
	DispatcherNames []string
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// flags any call that passes Dispatchers.IO/Default/Unconfined as an
// argument, with exclusions for idiomatic dispatcher hosts (e.g.
// withContext, flowOn), `Main` (which can't be injected), and
// object/@JvmStatic enclosing functions. It does NOT understand DI
// annotations — a class that already accepts `@Inject` would still
// trip the rule if the test-injected default happens to be a
// Dispatchers constant. Medium confidence reflects the DI-awareness
// gap noted in roadmap/17.
func (r *InjectDispatcherRule) Confidence() float64 { return 0.75 }

func isInsideObjectOrJvmStaticFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "object_declaration", "companion_object":
			return true
		case "function_declaration":
			if hasAnnotationFlat(file, p, "JvmStatic") {
				return true
			}
		case "class_declaration":
			return false
		}
	}
	return false
}

func directCallArgumentsFlat(file *scanner.File, idx uint32) uint32 {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 || file.FlatType(child) != "call_suffix" {
			continue
		}
		for j := 0; j < file.FlatNamedChildCount(child); j++ {
			gc := file.FlatNamedChild(child, j)
			if gc != 0 && file.FlatType(gc) == "value_arguments" {
				return gc
			}
		}
	}
	return 0
}

func findDirectDispatcherArgumentFlat(file *scanner.File, args uint32) (uint32, string) {
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		arg := file.FlatNamedChild(args, i)
		if arg == 0 || file.FlatType(arg) != "value_argument" || file.FlatNamedChildCount(arg) == 0 {
			continue
		}
		value := file.FlatNamedChild(arg, 0)
		if value == 0 || file.FlatType(value) != "navigation_expression" {
			continue
		}
		receiver := ""
		if file.FlatNamedChildCount(value) > 0 {
			first := file.FlatNamedChild(value, 0)
			if file.FlatType(first) == "simple_identifier" {
				receiver = file.FlatNodeText(first)
			}
		}
		member := flatNavigationExpressionLastIdentifier(file, value)
		if receiver == "Dispatchers" && (member == "IO" || member == "Default" || member == "Unconfined" || member == "Main") {
			return value, member
		}
	}
	return 0, ""
}

func callCalleePartsFlat(file *scanner.File, idx uint32) (receiver string, method string) {
	if file == nil || file.FlatChildCount(idx) == 0 {
		return "", ""
	}
	first := file.FlatChild(idx, 0)
	switch file.FlatType(first) {
	case "simple_identifier":
		return "", file.FlatNodeText(first)
	case "navigation_expression":
		return flatReceiverNameFromCall(file, idx), flatNavigationExpressionLastIdentifier(file, first)
	default:
		return "", ""
	}
}

func isIdiomaticDispatcherHost(receiver string, method string) bool {
	switch method {
	case "flowOn", "shareIn":
		return true
	case "CoroutineScope":
		return receiver == ""
	case "async":
		return receiver == "viewModelScope"
	case "launch":
		return receiver == "viewModelScope" || receiver == "lifecycleScope"
	case "launchWhenCreated", "launchWhenStarted", "launchWhenResumed":
		return receiver == "lifecycleScope"
	default:
		return false
	}
}

// RedundantSuspendModifierRule detects suspend functions with no suspend calls inside.
// With type inference: uses ResolveNode on call expressions to check if the call
// target is a suspend function, beyond the hardcoded known list.
// With oracle: uses LookupCallTarget to resolve the FQN of called functions and
// check if they are known suspend functions or in kotlinx.coroutines.
type RedundantSuspendModifierRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule uses a name-based allow-list (commonNonSuspendCallees) and a
// known-suspend-FQN set to decide whether a call is suspending. It
// relies on the oracle for the accurate case; without it, any
// identifier not in the allow-list is treated as potentially-suspend
// and the rule suppresses the finding, so the remaining positives are
// reliable but the rule may miss cases where the suspend modifier is
// genuinely redundant.
func (r *RedundantSuspendModifierRule) Confidence() float64 { return 0.75 }

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

func redundantSuspendCallTargetCallees() []string {
	return []string{
		"acquire",
		"async",
		"await",
		"awaitAll",
		"cancelAndJoin",
		"collect",
		"coroutineScope",
		"delay",
		"emit",
		"join",
		"joinAll",
		"launch",
		"lock",
		"mutex",
		"receive",
		"receiveCatching",
		"runBlocking",
		"send",
		"supervisorScope",
		"suspendCancellableCoroutine",
		"suspendCoroutine",
		"withContext",
		"withTimeout",
		"withTimeoutOrNull",
		"yield",
	}
}

func redundantSuspendCallTargetLexicalHints() map[string][]string {
	hints := make(map[string][]string)
	for _, callee := range redundantSuspendCallTargetCallees() {
		hints[callee] = []string{"kotlinx.coroutines", "kotlin.coroutines"}
	}
	hints["collect"] = append(hints["collect"], "kotlinx.coroutines.flow", "Flow")
	hints["emit"] = append(hints["emit"], "kotlinx.coroutines.flow", "FlowCollector")
	hints["send"] = append(hints["send"], "kotlinx.coroutines.channels", "SendChannel")
	hints["receive"] = append(hints["receive"], "kotlinx.coroutines.channels", "ReceiveChannel")
	hints["receiveCatching"] = append(hints["receiveCatching"], "kotlinx.coroutines.channels", "ReceiveChannel")
	hints["join"] = append(hints["join"], "Job")
	hints["cancelAndJoin"] = append(hints["cancelAndJoin"], "Job")
	hints["await"] = append(hints["await"], "Deferred")
	hints["lock"] = append(hints["lock"], "Mutex", "kotlinx.coroutines.sync")
	return hints
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
func (r *SleepInsteadOfDelayRule) Confidence() float64 { return 0.75 }

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
type SuspendFunWithFlowReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

var flowTypeNames = map[string]bool{
	"Flow": true, "StateFlow": true, "SharedFlow": true,
	"MutableStateFlow": true, "MutableSharedFlow": true,
}

var suspendKeywordRe = regexp.MustCompile(`\bsuspend\s+`)

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunWithFlowReturnTypeRule) Confidence() float64 { return 0.75 }

// CoroutineLaunchedInTestWithoutRunTestRule detects launch/async in @Test without runTest.
type CoroutineLaunchedInTestWithoutRunTestRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *CoroutineLaunchedInTestWithoutRunTestRule) Confidence() float64 { return 0.75 }

// SuspendFunInFinallySectionRule detects suspend calls in finally blocks.
type SuspendFunInFinallySectionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunInFinallySectionRule) Confidence() float64 { return 0.75 }

// SuspendFunSwallowedCancellationRule detects catching CancellationException without rethrow.
type SuspendFunSwallowedCancellationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// catch blocks swallow CancellationException without rethrow is structural
// but depends on the resolver to recognize aliases of
// CancellationException. Classified per roadmap/17.
func (r *SuspendFunSwallowedCancellationRule) Confidence() float64 { return 0.75 }

// SuspendFunWithCoroutineScopeReceiverRule detects suspend fun CoroutineScope.x().
type SuspendFunWithCoroutineScopeReceiverRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunWithCoroutineScopeReceiverRule) Confidence() float64 { return 0.75 }

func hasSuspendModifierFlat(file *scanner.File, idx uint32) bool {
	return file.FlatHasModifier(idx, "suspend")
}

// hasAnnotation is defined in library.go

// ---------------------------------------------------------------------------
// Batch 1: rules with existing fixtures
// ---------------------------------------------------------------------------

// ChannelReceiveWithoutCloseRule detects Channel properties that are never closed.
type ChannelReceiveWithoutCloseRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ChannelReceiveWithoutCloseRule) Confidence() float64 { return 0.75 }

// CollectionsSynchronizedListIterationRule detects iteration over synchronized wrappers without external sync.
type CollectionsSynchronizedListIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CollectionsSynchronizedListIterationRule) Confidence() float64 { return 0.75 }

var synchronizedCollectionFactories = map[string]bool{
	"synchronizedList": true, "synchronizedSet": true, "synchronizedMap": true,
}

// ConcurrentModificationIterationRule detects collection mutation inside for loops.
type ConcurrentModificationIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ConcurrentModificationIterationRule) Confidence() float64 { return 0.75 }

var mutatingMethods = map[string]bool{
	"remove": true, "add": true, "addAll": true, "removeAll": true, "clear": true,
}

// CoroutineScopeCreatedButNeverCancelledRule detects CoroutineScope properties without cancel().
type CoroutineScopeCreatedButNeverCancelledRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) Confidence() float64 { return 0.75 }

// DeferredAwaitInFinallyRule detects .await() calls inside finally blocks.
type DeferredAwaitInFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DeferredAwaitInFinallyRule) Confidence() float64 { return 0.75 }

// FlowWithoutFlowOnRule detects flow chains with collect but no flowOn.
type FlowWithoutFlowOnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FlowWithoutFlowOnRule) Confidence() float64 { return 0.75 }

var flowTerminalOps = map[string]bool{
	"collect": true, "first": true, "toList": true, "toSet": true, "single": true,
	"reduce": true, "fold": true, "count": true,
}

// ---------------------------------------------------------------------------
// Batch 2: synchronized / JVM primitive rules
// ---------------------------------------------------------------------------

// SynchronizedOnStringRule detects synchronized() with a string literal lock.
type SynchronizedOnStringRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SynchronizedOnStringRule) Confidence() float64 { return 0.75 }

// SynchronizedOnBoxedPrimitiveRule detects synchronized() on boxed primitives.
type SynchronizedOnBoxedPrimitiveRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SynchronizedOnBoxedPrimitiveRule) Confidence() float64 { return 0.75 }

var boxedPrimitiveTypes = map[string]bool{
	"Int": true, "Long": true, "Short": true, "Byte": true,
	"Float": true, "Double": true, "Boolean": true, "Char": true,
}

func resolvePropertyTypeInScope(file *scanner.File, fromIdx uint32, varName string) string {
	classDecl, ok := flatEnclosingAncestor(file, fromIdx, "class_declaration", "object_declaration")
	if !ok {
		return ""
	}
	var result string
	file.FlatWalkNodes(classDecl, "property_declaration", func(propIdx uint32) {
		if result != "" {
			return
		}
		if extractIdentifierFlat(file, propIdx) != varName {
			return
		}
		propText := file.FlatNodeText(propIdx)
		if i := strings.Index(propText, ":"); i >= 0 {
			afterColon := strings.TrimSpace(propText[i+1:])
			if eq := strings.Index(afterColon, "="); eq >= 0 {
				afterColon = strings.TrimSpace(afterColon[:eq])
			}
			afterColon = strings.TrimSuffix(afterColon, "?")
			if gt := strings.Index(afterColon, "<"); gt >= 0 {
				afterColon = afterColon[:gt]
			}
			result = strings.TrimSpace(afterColon)
		}
	})
	return result
}

// SynchronizedOnNonFinalRule detects synchronized() on a var property.
type SynchronizedOnNonFinalRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SynchronizedOnNonFinalRule) Confidence() float64 { return 0.75 }

func isVarPropertyInScope(file *scanner.File, fromIdx uint32, varName string) bool {
	classDecl, ok := flatEnclosingAncestor(file, fromIdx, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(classDecl, "property_declaration", func(propIdx uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, propIdx) != varName {
			return
		}
		for child := file.FlatFirstChild(propIdx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "var" || file.FlatNodeTextEquals(child, "var") {
				found = true
				return
			}
		}
	})
	return found
}

// VolatileMissingOnDclRule detects double-checked locking without @Volatile.
type VolatileMissingOnDclRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *VolatileMissingOnDclRule) Confidence() float64 { return 0.75 }

var dclPatternRe = regexp.MustCompile(`if\s*\(\s*(\w+)\s*==\s*null\s*\)`)

// MutableStateInObjectRule detects var properties inside object declarations.
type MutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MutableStateInObjectRule) Confidence() float64 { return 0.75 }

var threadSafeTypes = map[string]bool{
	"AtomicInteger": true, "AtomicLong": true, "AtomicBoolean": true,
	"AtomicReference": true, "ConcurrentHashMap": true, "CopyOnWriteArrayList": true,
}

// ---------------------------------------------------------------------------
// Batch 3: Flow / StateFlow rules
// ---------------------------------------------------------------------------

// StateFlowMutableLeakRule detects publicly exposed MutableStateFlow.
type StateFlowMutableLeakRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *StateFlowMutableLeakRule) Confidence() float64 { return 0.75 }

// SharedFlowWithoutReplayRule detects MutableSharedFlow() with no buffer config.
type SharedFlowWithoutReplayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedFlowWithoutReplayRule) Confidence() float64 { return 0.75 }

// StateFlowCompareByReferenceRule detects .map{}.distinctUntilChanged() on StateFlow.
type StateFlowCompareByReferenceRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *StateFlowCompareByReferenceRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 4: coroutine scope / context rules
// ---------------------------------------------------------------------------

// GlobalScopeLaunchInViewModelRule detects GlobalScope.launch in ViewModel/Presenter classes.
type GlobalScopeLaunchInViewModelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *GlobalScopeLaunchInViewModelRule) Confidence() float64 { return 0.75 }

// SupervisorScopeInEventHandlerRule detects supervisorScope with a single child operation.
type SupervisorScopeInEventHandlerRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SupervisorScopeInEventHandlerRule) Confidence() float64 { return 0.75 }

// WithContextInSuspendFunctionNoopRule detects nested withContext with the same dispatcher.
type WithContextInSuspendFunctionNoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WithContextInSuspendFunctionNoopRule) Confidence() float64 { return 0.75 }

func extractWithContextDispatcher(file *scanner.File, callIdx uint32) string {
	args := flatCallKeyArguments(file, callIdx)
	if args == 0 {
		return ""
	}
	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return ""
	}
	argText := strings.TrimSpace(file.FlatNodeText(firstArg))
	if strings.HasPrefix(argText, "Dispatchers.") {
		return argText
	}
	return ""
}

// LaunchWithoutCoroutineExceptionHandlerRule detects launch{} with throw but no handler.
type LaunchWithoutCoroutineExceptionHandlerRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LaunchWithoutCoroutineExceptionHandlerRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 5: cross-module rule
// ---------------------------------------------------------------------------

// MainDispatcherInLibraryCodeRule detects Dispatchers.Main in library modules
// without kotlinx-coroutines-android dependency.
type MainDispatcherInLibraryCodeRule struct {
	BaseRule
}

func (r *MainDispatcherInLibraryCodeRule) IsFixable() bool     { return false }
func (r *MainDispatcherInLibraryCodeRule) Confidence() float64 { return 0.75 }

func (r *MainDispatcherInLibraryCodeRule) check(ctx *v2.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}

	for modPath, mod := range pmi.Graph.Modules {
		if !isAndroidLibraryModule(mod) {
			continue
		}
		if hasCoroutinesAndroidDep(mod) {
			continue
		}
		files := pmi.ModuleFiles[modPath]
		for _, file := range files {
			for i, line := range file.Lines {
				if strings.Contains(line, "Dispatchers.Main") {
					col := strings.Index(line, "Dispatchers.Main") + 1
					ctx.Emit(scanner.Finding{
						File:     file.Path,
						Line:     i + 1,
						Col:      col,
						RuleSet:  r.RuleSetName,
						Rule:     r.RuleName,
						Severity: r.Sev,
						Message:  "Dispatchers.Main used in a library module without kotlinx-coroutines-android dependency.",
					})
				}
			}
		}
	}
}

func (r *MainDispatcherInLibraryCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{
		NeedsFiles:        true,
		NeedsDependencies: true,
		NeedsIndex:        false,
	}
}

func isAndroidLibraryModule(mod *module.Module) bool {
	buildFile := filepath.Join(mod.Dir, "build.gradle.kts")
	cfg, err := android.ParseBuildGradle(buildFile)
	if err != nil {
		buildFile = filepath.Join(mod.Dir, "build.gradle")
		cfg, err = android.ParseBuildGradle(buildFile)
		if err != nil {
			return false
		}
	}
	for _, plugin := range cfg.Plugins {
		if plugin == "com.android.library" {
			return true
		}
	}
	return false
}

func hasCoroutinesAndroidDep(mod *module.Module) bool {
	buildFile := filepath.Join(mod.Dir, "build.gradle.kts")
	cfg, err := android.ParseBuildGradle(buildFile)
	if err != nil {
		buildFile = filepath.Join(mod.Dir, "build.gradle")
		cfg, err = android.ParseBuildGradle(buildFile)
		if err != nil {
			return false
		}
	}
	for _, dep := range cfg.Dependencies {
		if dep.Name == "kotlinx-coroutines-android" {
			return true
		}
	}
	return false
}
