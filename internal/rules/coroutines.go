package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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

func (r *CollectInOnCreateWithoutLifecycleRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *CollectInOnCreateWithoutLifecycleRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "collect" {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !lifecycleCollectCallbacks[extractIdentifierFlat(file, fn)] {
		return nil
	}

	if hasAncestorCallNamedFlat(file, idx, "repeatOnLifecycle") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Flow.collect inside onCreate/onStart/onViewCreated should be wrapped in repeatOnLifecycle to stop collecting when the lifecycle is stopped.",
	)}
}

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

func (r *GlobalCoroutineUsageRule) NodeTypes() []string {
	return []string{"call_expression", "navigation_expression"}
}

func (r *GlobalCoroutineUsageRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	switch file.FlatType(idx) {
	case "call_expression":
		// Check for GlobalScope.launch { } or GlobalScope.async { }
		if file.FlatChildCount(idx) == 0 {
			return nil
		}
		nav := file.FlatChild(idx, 0)
		if file.FlatType(nav) != "navigation_expression" {
			return nil
		}
		if file.FlatChildCount(nav) < 2 {
			return nil
		}
		receiver := file.FlatNodeText(file.FlatChild(nav, 0))
		if receiver != "GlobalScope" {
			return nil
		}
		// The last child is a navigation_suffix node (e.g., ".launch").
		// Extract the simple_identifier from within it.
		navSuffix := file.FlatChild(nav, file.FlatChildCount(nav)-1)
		callee := ""
		if file.FlatType(navSuffix) == "navigation_suffix" {
			for j := 0; j < file.FlatChildCount(navSuffix); j++ {
				if child := file.FlatChild(navSuffix, j); file.FlatType(child) == "simple_identifier" {
					callee = file.FlatNodeText(child)
					break
				}
			}
		} else {
			callee = file.FlatNodeText(navSuffix)
		}
		if callee != "launch" && callee != "async" {
			return nil
		}
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"GlobalScope usage detected. Prefer structured concurrency with a proper CoroutineScope.")
		// Fix: remove "GlobalScope." prefix (from receiver start to navigation_suffix's simple_identifier start)
		recvStart := int(file.FlatStartByte(file.FlatChild(nav, 0)))
		calleeStart := int(file.FlatStartByte(navSuffix)) + 1
		if file.FlatType(navSuffix) == "navigation_suffix" {
			for j := 0; j < file.FlatChildCount(navSuffix); j++ {
				if child := file.FlatChild(navSuffix, j); file.FlatType(child) == "simple_identifier" {
					calleeStart = int(file.FlatStartByte(child))
					break
				}
			}
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   recvStart,
			EndByte:     calleeStart,
			Replacement: "",
		}
		return []scanner.Finding{f}

	case "navigation_expression":
		// Catch standalone GlobalScope.someProperty or GlobalScope references in navigation
		// but skip if the parent is a call_expression (handled above)
		parent, ok := file.FlatParent(idx)
		if ok && file.FlatType(parent) == "call_expression" && file.FlatChild(parent, 0) == idx {
			return nil
		}
		if file.FlatChildCount(idx) < 2 {
			return nil
		}
		receiver := file.FlatNodeText(file.FlatChild(idx, 0))
		if receiver != "GlobalScope" {
			return nil
		}
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"GlobalScope usage detected. Prefer structured concurrency with a proper CoroutineScope.")}
	}
	return nil
}

// InjectDispatcherRule detects hardcoded Dispatchers.IO/Default/Unconfined in call expressions.
type InjectDispatcherRule struct {
	FlatDispatchBase
	BaseRule
	DispatcherNames []string
}

func (r *InjectDispatcherRule) NodeTypes() []string { return []string{"call_expression"} }

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

func (r *InjectDispatcherRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Outer call_expression wrappers for trailing lambdas are handled by the
	// nested call_expression that owns the value arguments.
	if first := file.FlatChild(idx, 0); first != 0 && file.FlatType(first) == "call_expression" {
		return nil
	}

	args := directCallArgumentsFlat(file, idx)
	if args == 0 {
		return nil
	}

	dispatcherNode, dispatcherName := findDirectDispatcherArgumentFlat(file, args)
	if dispatcherNode == 0 {
		return nil
	}

	receiver, method := callCalleePartsFlat(file, idx)
	if isIdiomaticDispatcherHost(receiver, method) {
		return nil
	}

	// Skip Dispatchers.Main — it has no test substitution value (Main is
	// required to be the Android main thread and can't be "injected").
	// Dispatchers.IO and Dispatchers.Default are the meaningful targets for
	// test injection.
	if dispatcherName == "Main" {
		return nil
	}
	// Skip when the enclosing function is a member of a Kotlin `object`
	// declaration or is annotated `@JvmStatic` — there is no constructor
	// to inject a dispatcher into.
	if isInsideObjectOrJvmStaticFlat(file, idx) {
		return nil
	}
	matchLine := file.FlatRow(dispatcherNode) + 1
	matchCol := file.FlatCol(dispatcherNode) + 1
	return []scanner.Finding{r.Finding(file, matchLine, matchCol,
		fmt.Sprintf("Hardcoded Dispatchers.%s. Inject dispatchers for better testability.", dispatcherName))}
}

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
	resolver     typeinfer.TypeResolver
	oracleLookup oracle.Lookup // optional, extracted from CompositeResolver
}

func (r *RedundantSuspendModifierRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
	// Extract oracle if the resolver is a CompositeResolver
	if cr, ok := res.(*oracle.CompositeResolver); ok {
		r.oracleLookup = cr.Oracle()
	}
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

// suspendFQNPrefixes are package prefixes that indicate a call target is likely
// a suspend function when no exact FQN match is found.
var suspendFQNPrefixes = []string{
	"kotlinx.coroutines.",
}

func (r *RedundantSuspendModifierRule) NodeTypes() []string { return []string{"function_declaration"} }

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

func (r *RedundantSuspendModifierRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if function has suspend modifier
	if !hasSuspendModifierFlat(file, idx) {
		return nil
	}
	// Skip open/abstract/override functions — the suspend modifier is
	// required to match the contract; implementations may suspend.
	if file.FlatHasModifier(idx, "open") ||
		file.FlatHasModifier(idx, "abstract") ||
		file.FlatHasModifier(idx, "override") {
		return nil
	}
	// Skip functions inside interface declarations — default implementations
	// that don't suspend are meant to be overridden by ones that do.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "class_declaration" {
			for i := 0; i < file.FlatChildCount(p); i++ {
				c := file.FlatChild(p, i)
				if file.FlatType(c) == "interface" {
					return nil
				}
			}
			break
		}
	}
	// Skip abstract/interface methods (no body)
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	// Check if any call expression inside is a known suspend function.
	// We also track whether any call could not be resolved — if so, we
	// conservatively skip the report because the unresolved call might be
	// a project-defined suspend function.
	hasSuspendCall := false
	hasUnresolvedCall := false
	file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
		if hasSuspendCall {
			return
		}

		resolved := false // tracks whether this particular call was positively resolved

		// Oracle path: use LookupCallTarget for precise FQN resolution
		if r.oracleLookup != nil {
			line := file.FlatRow(callIdx) + 1
			col := file.FlatCol(callIdx) + 1
			if ct := r.oracleLookup.LookupCallTarget(file.Path, line, col); ct != "" {
				resolved = true
				if knownSuspendFQNs[ct] {
					hasSuspendCall = true
					return
				}
				// Check prefix match for kotlinx.coroutines package
				for _, prefix := range suspendFQNPrefixes {
					if strings.HasPrefix(ct, prefix) {
						hasSuspendCall = true
						return
					}
				}
			}
		}

		callText := file.FlatNodeText(callIdx)
		// Check the hardcoded known suspend functions
		for name := range knownSuspendFunctions {
			if strings.HasPrefix(callText, name+"(") || strings.HasPrefix(callText, name+" ") ||
				strings.HasPrefix(callText, name+"{") || strings.HasPrefix(callText, name+"<") ||
				strings.Contains(callText, "."+name+"(") {
				hasSuspendCall = true
				return
			}
		}
		// With resolver: check if the call target resolves to a suspend function
		if r.resolver != nil {
			resolvedType := r.resolver.ResolveFlatNode(callIdx, file)
			if resolvedType.Kind != typeinfer.TypeUnknown {
				resolved = true
				// If the resolver can identify the call target, check its name
				// against known suspend patterns via class hierarchy
				callName := resolvedType.Name
				if callName != "" && knownSuspendFunctions[callName] {
					hasSuspendCall = true
					return
				}
			}
			// Also try resolving the function name directly
			funcIdent := file.FlatFindChild(callIdx, "simple_identifier")
			if funcIdent != 0 {
				funcName := file.FlatNodeText(funcIdent)
				resolvedByName := r.resolver.ResolveByNameFlat(funcName, funcIdent, file)
				if resolvedByName != nil && resolvedByName.Kind != typeinfer.TypeUnknown {
					resolved = true
					// If the resolved type has FQN in a coroutine package, it's likely suspend
					if strings.Contains(resolvedByName.FQN, "kotlinx.coroutines") {
						hasSuspendCall = true
						return
					}
				}
			}
		}

		// Any call whose callee is a user identifier is potentially a
		// project-defined suspend function. The resolver can identify the
		// return type of such calls but cannot determine whether the callee
		// is declared `suspend` (that's call-graph analysis). Conservatively
		// mark these as unresolved unless the callee is a known-non-suspend
		// stdlib name.
		if file.FlatChildCount(callIdx) > 0 {
			first := file.FlatChild(callIdx, 0)
			if first != 0 {
				ft := file.FlatType(first)
				if ft == "navigation_expression" || ft == "simple_identifier" {
					calleeName := file.FlatNodeText(first)
					if dot := strings.LastIndex(calleeName, "."); dot >= 0 {
						calleeName = calleeName[dot+1:]
					}
					if !commonNonSuspendCallees[calleeName] {
						hasUnresolvedCall = true
					}
				}
			}
		}
		_ = resolved
	})
	if !hasSuspendCall && hasUnresolvedCall {
		// At least one call could not be resolved — it might be suspend.
		// Err on the side of caution and don't flag.
		return nil
	}
	if !hasSuspendCall {
		name := extractIdentifierFlat(file, idx)
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function '%s' has a redundant suspend modifier. No suspend calls found inside.", name))
		// Find the "suspend" modifier node in the AST and remove it plus trailing space
		suspendNode := file.FlatFindModifierNode(idx, "suspend")
		if suspendNode != 0 {
			endByte := int(file.FlatEndByte(suspendNode))
			// Consume a single trailing space after "suspend" if present
			if endByte < len(file.Content) && file.Content[endByte] == ' ' {
				endByte++
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(suspendNode)),
				EndByte:     endByte,
				Replacement: "",
			}
		}
		return []scanner.Finding{f}
	}
	return nil
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

func (r *SleepInsteadOfDelayRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SleepInsteadOfDelayRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if this call_expression is Thread.sleep(...)
	// tree-sitter structure: call_expression -> navigation_expression ("Thread.sleep") + call_suffix
	if file.FlatChildCount(idx) == 0 {
		return nil
	}
	callee := file.FlatChild(idx, 0)
	if file.FlatType(callee) != "navigation_expression" {
		return nil
	}
	navText := file.FlatNodeText(callee)
	// Match Thread.sleep (allow whitespace around the dot)
	if !strings.HasPrefix(navText, "Thread") || !strings.HasSuffix(navText, "sleep") {
		return nil
	}
	// More precise: split on '.' and verify parts
	parts := strings.SplitN(navText, ".", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "Thread" || strings.TrimSpace(parts[1]) != "sleep" {
		return nil
	}

	// Walk up ancestors to check if inside a suspend function or coroutine builder lambda
	if !isInsideSuspendContextFlat(file, idx) {
		return nil
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Thread.sleep() used in suspend context. Use delay() instead.")
	// Auto-fix: replace "Thread.sleep(" with "delay("
	startByte := int(file.FlatStartByte(callee))
	// EndByte of callee covers "Thread.sleep", we need to include the opening paren
	// from the call_suffix. The call_suffix child starts right after the navigation_expression.
	endByte := int(file.FlatEndByte(callee))
	if file.FlatChildCount(idx) > 1 {
		suffix := file.FlatChild(idx, 1)
		suffixText := file.FlatNodeText(suffix)
		if strings.HasPrefix(suffixText, "(") {
			endByte = int(file.FlatStartByte(suffix)) + 1 // include the "("
		}
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     endByte,
		Replacement: "delay(",
	}
	return []scanner.Finding{f}
}

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

func (r *SuspendFunWithFlowReturnTypeRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *SuspendFunWithFlowReturnTypeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasSuspendModifierFlat(file, idx) {
		return nil
	}
	// Check return type annotation for Flow types
	// The return type appears after ":" in the function declaration, before the body
	hasFlowReturn := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "user_type" || file.FlatType(child) == "nullable_type" {
			typeText := file.FlatNodeText(child)
			// Extract the base type name (strip generics)
			baseName := typeText
			if idx := strings.Index(baseName, "<"); idx >= 0 {
				baseName = baseName[:idx]
			}
			if idx := strings.Index(baseName, "?"); idx >= 0 {
				baseName = baseName[:idx]
			}
			baseName = strings.TrimSpace(baseName)
			if flowTypeNames[baseName] {
				hasFlowReturn = true
				break
			}
		}
	}
	if !hasFlowReturn {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Suspend function returns a Flow type. A function that returns Flow should not be suspend. The flow builder is cold and does not require a coroutine.")
	// Fix: remove "suspend " keyword
	line := file.Lines[file.FlatRow(idx)]
	if loc := suspendKeywordRe.FindStringIndex(line); loc != nil {
		lineStart := file.LineOffset(file.FlatRow(idx))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   lineStart + loc[0],
			EndByte:     lineStart + loc[1],
			Replacement: "",
		}
	}
	return []scanner.Finding{f}
}

func (r *SuspendFunWithFlowReturnTypeRule) CheckLines(_ *scanner.File) []scanner.Finding { return nil }

// CoroutineLaunchedInTestWithoutRunTestRule detects launch/async in @Test without runTest.
type CoroutineLaunchedInTestWithoutRunTestRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *CoroutineLaunchedInTestWithoutRunTestRule) Confidence() float64 { return 0.75 }

func (r *CoroutineLaunchedInTestWithoutRunTestRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *CoroutineLaunchedInTestWithoutRunTestRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasAnnotationFlat(file, idx, "Test") {
		return nil
	}
	funText := file.FlatNodeText(idx)
	if strings.Contains(funText, "runTest") {
		return nil
	}
	// Check for launch or async calls
	var findings []scanner.Finding
	file.FlatWalkNodes(idx, "call_expression", func(callNode uint32) {
		callText := file.FlatNodeText(callNode)
		if strings.HasPrefix(callText, "launch") || strings.HasPrefix(callText, "async") {
			findings = append(findings, r.Finding(file, file.FlatRow(callNode)+1, file.FlatCol(callNode)+1,
				"Coroutine launched in @Test without runTest. Use runTest { } to properly handle coroutines in tests."))
		}
	})
	return findings
}

func (r *CoroutineLaunchedInTestWithoutRunTestRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// SuspendFunInFinallySectionRule detects suspend calls in finally blocks.
type SuspendFunInFinallySectionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunInFinallySectionRule) Confidence() float64 { return 0.75 }

func (r *SuspendFunInFinallySectionRule) NodeTypes() []string { return []string{"finally_block"} }

func (r *SuspendFunInFinallySectionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	file.FlatWalkNodes(idx, "call_expression", func(callNode uint32) {
		callText := file.FlatNodeText(callNode)
		for name := range knownSuspendFunctions {
			if strings.HasPrefix(callText, name+"(") || strings.HasPrefix(callText, name+" ") ||
				strings.HasPrefix(callText, name+"{") {
				findings = append(findings, r.Finding(file, file.FlatRow(callNode)+1, file.FlatCol(callNode)+1,
					fmt.Sprintf("Suspend function '%s' called in finally block. This may not execute if the coroutine is cancelled.", name)))
				return
			}
		}
	})
	return findings
}

// SuspendFunSwallowedCancellationRule detects catching CancellationException without rethrow.
type SuspendFunSwallowedCancellationRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *SuspendFunSwallowedCancellationRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// catch blocks swallow CancellationException without rethrow is structural
// but depends on the resolver to recognize aliases of
// CancellationException. Classified per roadmap/17.
func (r *SuspendFunSwallowedCancellationRule) Confidence() float64 { return 0.75 }

func (r *SuspendFunSwallowedCancellationRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *SuspendFunSwallowedCancellationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	caughtType := extractCaughtTypeNameFlat(file, idx)
	if caughtType == "" {
		return nil
	}

	// Check if the caught type catches CancellationException (directly or transitively)
	catchesCancellation := false
	if caughtType == "CancellationException" {
		catchesCancellation = true
	} else if r.resolver != nil {
		catchesCancellation = r.resolver.IsExceptionSubtype("CancellationException", caughtType)
	} else {
		catchesCancellation = typeinfer.IsSubtypeOfException("CancellationException", caughtType)
	}

	if !catchesCancellation {
		return nil
	}

	caughtVar := extractCaughtVarNameFlat(file, idx)
	catchText := file.FlatNodeText(idx)
	// Check if the exception is rethrown
	rethrowPattern := fmt.Sprintf(`\bthrow\s+%s\b`, regexp.QuoteMeta(caughtVar))
	matched, err := regexp.MatchString(rethrowPattern, catchText)
	if err != nil {
		// Fallback: simple string check if regex fails
		matched = strings.Contains(catchText, "throw "+caughtVar)
	}
	if !matched {
		msg := "CancellationException is caught but not rethrown. This can break structured concurrency."
		if caughtType != "CancellationException" {
			msg = fmt.Sprintf("Catching '%s' swallows CancellationException without rethrowing. This can break structured concurrency.", caughtType)
		}
		f := r.Finding(file, file.FlatRow(idx)+1, 1, msg)
		// Fix: insert "throw <var>" before the closing brace of the catch block
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
			varName := caughtVar
			if varName == "" {
				varName = "e"
			}
			insertion := indent + "    throw " + varName + "\n" + indent
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   endByte - 1,
				EndByte:     endByte,
				Replacement: insertion + "}",
			}
		}
		return []scanner.Finding{f}
	}
	return nil
}

// SuspendFunWithCoroutineScopeReceiverRule detects suspend fun CoroutineScope.x().
type SuspendFunWithCoroutineScopeReceiverRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *SuspendFunWithCoroutineScopeReceiverRule) Confidence() float64 { return 0.75 }

func (r *SuspendFunWithCoroutineScopeReceiverRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *SuspendFunWithCoroutineScopeReceiverRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasSuspendModifierFlat(file, idx) {
		return nil
	}
	// Check for receiver type "CoroutineScope"
	// In the Kotlin grammar, the receiver type appears before the function name
	// as a user_type child followed by a "." token
	hasCoroutineScopeReceiver := false
	nodeText := file.FlatNodeText(idx)
	// The receiver type in tree-sitter Kotlin appears in the function text as "CoroutineScope."
	// before the function name. Check the node text for the pattern.
	funIdx := strings.Index(nodeText, "fun ")
	if funIdx >= 0 {
		afterFun := nodeText[funIdx+4:]
		trimmed := strings.TrimSpace(afterFun)
		if strings.HasPrefix(trimmed, "CoroutineScope.") {
			hasCoroutineScopeReceiver = true
		}
	}
	if !hasCoroutineScopeReceiver {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Suspend function with CoroutineScope receiver. A function should either be suspend or be an extension on CoroutineScope, not both.")
	// Fix: remove "suspend " keyword
	line := file.Lines[file.FlatRow(idx)]
	if loc := suspendKeywordRe.FindStringIndex(line); loc != nil {
		lineStart := file.LineOffset(file.FlatRow(idx))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   lineStart + loc[0],
			EndByte:     lineStart + loc[1],
			Replacement: "",
		}
	}
	return []scanner.Finding{f}
}

func (r *SuspendFunWithCoroutineScopeReceiverRule) CheckLines(_ *scanner.File) []scanner.Finding {
	return nil
}

func (r *SuspendFunWithCoroutineScopeReceiverRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

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
func (r *ChannelReceiveWithoutCloseRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *ChannelReceiveWithoutCloseRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "Channel<") && !strings.Contains(text, "Channel(") {
		return nil
	}
	propName := extractIdentifierFlat(file, idx)
	if propName == "" {
		return nil
	}
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok {
		return nil
	}
	classText := file.FlatNodeText(classDecl)
	if strings.Contains(classText, propName+".close()") || strings.Contains(classText, propName+".close(") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Channel '%s' is never closed. This leaks the receiver coroutine.", propName))}
}

// CollectionsSynchronizedListIterationRule detects iteration over synchronized wrappers without external sync.
type CollectionsSynchronizedListIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CollectionsSynchronizedListIterationRule) Confidence() float64 { return 0.75 }
func (r *CollectionsSynchronizedListIterationRule) NodeTypes() []string {
	return []string{"for_statement"}
}

var synchronizedCollectionFactories = map[string]bool{
	"synchronizedList": true, "synchronizedSet": true, "synchronizedMap": true,
}

func (r *CollectionsSynchronizedListIterationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	forText := file.FlatNodeText(idx)
	hasSyncFactory := false
	for name := range synchronizedCollectionFactories {
		if strings.Contains(forText, "Collections."+name) {
			hasSyncFactory = true
			break
		}
	}
	if !hasSyncFactory {
		return nil
	}
	if hasAncestorCallNamedFlat(file, idx, "synchronized") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Iterating over a Collections.synchronized* wrapper without external synchronization. The iterator is not thread-safe.")}
}

// ConcurrentModificationIterationRule detects collection mutation inside for loops.
type ConcurrentModificationIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ConcurrentModificationIterationRule) Confidence() float64 { return 0.75 }
func (r *ConcurrentModificationIterationRule) NodeTypes() []string { return []string{"for_statement"} }

var mutatingMethods = map[string]bool{
	"remove": true, "add": true, "addAll": true, "removeAll": true, "clear": true,
}

func (r *ConcurrentModificationIterationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	iterableNode := uint32(0)
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			iterableNode = child
		}
		if file.FlatType(child) == "control_structure_body" || file.FlatType(child) == "statements" {
			break
		}
	}
	if iterableNode == 0 {
		return nil
	}
	iterableName := file.FlatNodeText(iterableNode)
	if iterableName == "" {
		return nil
	}

	var findings []scanner.Finding
	body := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return nil
	}
	file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
		receiver := flatReceiverNameFromCall(file, callIdx)
		method := flatCallExpressionName(file, callIdx)
		if receiver == iterableName && mutatingMethods[method] {
			findings = append(findings, r.Finding(file, file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
				fmt.Sprintf("Collection '%s' is modified while being iterated. This causes ConcurrentModificationException.", iterableName)))
		}
	})
	return findings
}

// CoroutineScopeCreatedButNeverCancelledRule detects CoroutineScope properties without cancel().
type CoroutineScopeCreatedButNeverCancelledRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) Confidence() float64 { return 0.75 }
func (r *CoroutineScopeCreatedButNeverCancelledRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "CoroutineScope(") {
		return nil
	}
	propName := extractIdentifierFlat(file, idx)
	if propName == "" {
		return nil
	}
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok {
		return nil
	}
	classText := file.FlatNodeText(classDecl)
	if strings.Contains(classText, propName+".cancel()") || strings.Contains(classText, propName+".cancel(") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("CoroutineScope '%s' is created but never cancelled. This leaks coroutines.", propName))}
}

// DeferredAwaitInFinallyRule detects .await() calls inside finally blocks.
type DeferredAwaitInFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DeferredAwaitInFinallyRule) Confidence() float64 { return 0.75 }
func (r *DeferredAwaitInFinallyRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *DeferredAwaitInFinallyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "await" {
		return nil
	}
	_, inFinally := flatEnclosingAncestor(file, idx, "finally_block")
	if !inFinally {
		return nil
	}
	if hasAncestorCallNamedFlat(file, idx, "runCatching") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Deferred.await() in finally block can throw and mask the original exception. Wrap in runCatching.")}
}

// FlowWithoutFlowOnRule detects flow chains with collect but no flowOn.
type FlowWithoutFlowOnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FlowWithoutFlowOnRule) Confidence() float64 { return 0.75 }
func (r *FlowWithoutFlowOnRule) NodeTypes() []string { return []string{"call_expression"} }

var flowTerminalOps = map[string]bool{
	"collect": true, "first": true, "toList": true, "toSet": true, "single": true,
	"reduce": true, "fold": true, "count": true,
}

func (r *FlowWithoutFlowOnRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	terminalOp := flatNavigationExpressionLastIdentifier(file, navExpr)
	if !flowTerminalOps[terminalOp] {
		return nil
	}
	chainText := file.FlatNodeText(idx)
	if !strings.Contains(chainText, "flow {") && !strings.Contains(chainText, "flow{") {
		return nil
	}
	if strings.Contains(chainText, ".flowOn(") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Flow chain has a terminal operator without .flowOn(). Blocking operations in the flow builder may run on the wrong dispatcher.")}
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
func (r *SynchronizedOnStringRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SynchronizedOnStringRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "synchronized" {
		return nil
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return nil
	}
	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return nil
	}
	argExpr := flatValueArgumentExpression(file, firstArg)
	if argExpr == 0 {
		return nil
	}
	if file.FlatType(argExpr) == "string_literal" || file.FlatType(argExpr) == "line_string_literal" {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"synchronized() on a string literal. Interned strings share a monitor across classloaders. Use a dedicated Any() object.")}
	}
	return nil
}

// SynchronizedOnBoxedPrimitiveRule detects synchronized() on boxed primitives.
type SynchronizedOnBoxedPrimitiveRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SynchronizedOnBoxedPrimitiveRule) Confidence() float64 { return 0.75 }
func (r *SynchronizedOnBoxedPrimitiveRule) NodeTypes() []string { return []string{"call_expression"} }

var boxedPrimitiveTypes = map[string]bool{
	"Int": true, "Long": true, "Short": true, "Byte": true,
	"Float": true, "Double": true, "Boolean": true, "Char": true,
}

func (r *SynchronizedOnBoxedPrimitiveRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "synchronized" {
		return nil
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return nil
	}
	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return nil
	}
	argExpr := flatValueArgumentExpression(file, firstArg)
	if argExpr == 0 {
		return nil
	}
	argType := file.FlatType(argExpr)
	if argType == "integer_literal" || argType == "long_literal" || argType == "boolean_literal" ||
		argType == "real_literal" || argType == "character_literal" {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"synchronized() on a boxed primitive literal. Boxed primitives have identity-equality surprises. Use a dedicated Any() object.")}
	}
	if argType == "simple_identifier" {
		varName := file.FlatNodeText(argExpr)
		propType := resolvePropertyTypeInScope(file, idx, varName)
		if boxedPrimitiveTypes[propType] {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("synchronized() on a boxed primitive (%s). Boxed primitives have identity-equality surprises. Use a dedicated Any() object.", propType))}
		}
	}
	return nil
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
func (r *SynchronizedOnNonFinalRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SynchronizedOnNonFinalRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "synchronized" {
		return nil
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return nil
	}
	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return nil
	}
	argExpr := flatValueArgumentExpression(file, firstArg)
	if argExpr == 0 || file.FlatType(argExpr) != "simple_identifier" {
		return nil
	}
	varName := file.FlatNodeText(argExpr)
	if isVarPropertyInScope(file, idx, varName) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("synchronized() on non-final property '%s'. Reassignment changes the monitor object. Use val instead of var.", varName))}
	}
	return nil
}

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
func (r *VolatileMissingOnDclRule) NodeTypes() []string { return []string{"property_declaration"} }

var dclPatternRe = regexp.MustCompile(`if\s*\(\s*(\w+)\s*==\s*null\s*\)`)

func (r *VolatileMissingOnDclRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	isVar := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "var") {
			isVar = true
			break
		}
	}
	if !isVar {
		return nil
	}
	propName := extractIdentifierFlat(file, idx)
	if propName == "" {
		return nil
	}
	propText := file.FlatNodeText(idx)
	if !strings.Contains(propText, "null") {
		return nil
	}
	if hasAnnotationFlat(file, idx, "Volatile") {
		return nil
	}
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok {
		return nil
	}
	classText := file.FlatNodeText(classDecl)
	if !strings.Contains(classText, "synchronized") {
		return nil
	}
	nullCheckPattern := propName + " == null"
	nullChecks := strings.Count(classText, nullCheckPattern)
	if nullChecks < 2 {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Double-checked locking on '%s' without @Volatile. Add @Volatile or use 'by lazy'.", propName))}
}

// MutableStateInObjectRule detects var properties inside object declarations.
type MutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MutableStateInObjectRule) Confidence() float64 { return 0.75 }
func (r *MutableStateInObjectRule) NodeTypes() []string { return []string{"object_declaration"} }

var threadSafeTypes = map[string]bool{
	"AtomicInteger": true, "AtomicLong": true, "AtomicBoolean": true,
	"AtomicReference": true, "ConcurrentHashMap": true, "CopyOnWriteArrayList": true,
}

func (r *MutableStateInObjectRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if file.FlatType(idx) == "companion_object" {
		return nil
	}
	var findings []scanner.Finding
	file.FlatWalkNodes(idx, "property_declaration", func(propIdx uint32) {
		parent, ok := file.FlatParent(propIdx)
		if !ok {
			return
		}
		parentType := file.FlatType(parent)
		if parentType != "class_body" && parentType != "object_declaration" {
			return
		}
		if parentType == "class_body" {
			gp, ok := file.FlatParent(parent)
			if !ok || gp != idx {
				return
			}
		}
		propText := file.FlatNodeText(propIdx)
		isVar := false
		for child := file.FlatFirstChild(propIdx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatNodeTextEquals(child, "var") {
				isVar = true
				break
			}
		}
		if !isVar {
			return
		}
		for typeName := range threadSafeTypes {
			if strings.Contains(propText, typeName) {
				return
			}
		}
		propName := extractIdentifierFlat(file, propIdx)
		findings = append(findings, r.Finding(file, file.FlatRow(propIdx)+1, file.FlatCol(propIdx)+1,
			fmt.Sprintf("Mutable 'var %s' in object declaration. Shared mutable state without synchronization is a race condition.", propName)))
	})
	return findings
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
func (r *StateFlowMutableLeakRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *StateFlowMutableLeakRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	propText := file.FlatNodeText(idx)
	if !strings.Contains(propText, "MutableStateFlow") {
		return nil
	}
	if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "protected") {
		return nil
	}
	propName := extractIdentifierFlat(file, idx)
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("MutableStateFlow '%s' is publicly exposed. Keep it private and expose as StateFlow<T>.", propName))}
}

// SharedFlowWithoutReplayRule detects MutableSharedFlow() with no buffer config.
type SharedFlowWithoutReplayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedFlowWithoutReplayRule) Confidence() float64 { return 0.75 }
func (r *SharedFlowWithoutReplayRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *SharedFlowWithoutReplayRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	propText := file.FlatNodeText(idx)
	if !strings.Contains(propText, "MutableSharedFlow") {
		return nil
	}
	if strings.Contains(propText, "MutableSharedFlow(replay") ||
		strings.Contains(propText, "MutableSharedFlow(extraBufferCapacity") ||
		strings.Contains(propText, "MutableSharedFlow(\n") {
		return nil
	}
	if strings.Contains(propText, "MutableSharedFlow()") ||
		(strings.Contains(propText, "MutableSharedFlow<") && strings.Contains(propText, ">()")) {
		propName := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("MutableSharedFlow '%s' created without replay or extraBufferCapacity. Default config is lossy.", propName))}
	}
	return nil
}

// StateFlowCompareByReferenceRule detects .map{}.distinctUntilChanged() on StateFlow.
type StateFlowCompareByReferenceRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *StateFlowCompareByReferenceRule) Confidence() float64 { return 0.75 }
func (r *StateFlowCompareByReferenceRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *StateFlowCompareByReferenceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "distinctUntilChanged" {
		return nil
	}
	chainText := file.FlatNodeText(idx)
	if !strings.Contains(chainText, ".map") {
		return nil
	}
	if !strings.Contains(chainText, "state") && !strings.Contains(chainText, "State") &&
		!strings.Contains(chainText, "uiState") && !strings.Contains(chainText, "flow") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Redundant .distinctUntilChanged() after .map{}. StateFlow already deduplicates by structural equality.")}
}

// ---------------------------------------------------------------------------
// Batch 4: coroutine scope / context rules
// ---------------------------------------------------------------------------

// GlobalScopeLaunchInViewModelRule detects GlobalScope.launch in ViewModel/Presenter classes.
type GlobalScopeLaunchInViewModelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *GlobalScopeLaunchInViewModelRule) Confidence() float64 { return 0.75 }
func (r *GlobalScopeLaunchInViewModelRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *GlobalScopeLaunchInViewModelRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver != "GlobalScope" {
		return nil
	}
	method := flatCallExpressionName(file, idx)
	if method != "launch" && method != "async" {
		return nil
	}
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok {
		return nil
	}
	className := extractIdentifierFlat(file, classDecl)
	if !strings.HasSuffix(className, "ViewModel") && !strings.HasSuffix(className, "Presenter") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("GlobalScope.%s in %s. Use viewModelScope instead for lifecycle-aware cancellation.", method, className))}
}

// SupervisorScopeInEventHandlerRule detects supervisorScope with a single child operation.
type SupervisorScopeInEventHandlerRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SupervisorScopeInEventHandlerRule) Confidence() float64 { return 0.75 }
func (r *SupervisorScopeInEventHandlerRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SupervisorScopeInEventHandlerRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "supervisorScope" {
		return nil
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	stmts := file.FlatFindChild(lambda, "statements")
	if stmts == 0 {
		return nil
	}
	stmtCount := 0
	for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			stmtCount++
		}
	}
	if stmtCount > 1 {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"supervisorScope with a single child operation. Supervisor semantics are only useful with multiple concurrent children.")}
}

// WithContextInSuspendFunctionNoopRule detects nested withContext with the same dispatcher.
type WithContextInSuspendFunctionNoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WithContextInSuspendFunctionNoopRule) Confidence() float64 { return 0.75 }
func (r *WithContextInSuspendFunctionNoopRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *WithContextInSuspendFunctionNoopRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "withContext" {
		return nil
	}
	dispatcher := extractWithContextDispatcher(file, idx)
	if dispatcher == "" {
		return nil
	}
	skippedSelf := false
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			break
		}
		if file.FlatType(p) != "call_expression" {
			continue
		}
		if !skippedSelf && flatCallNameAny(file, p) == "withContext" {
			pd := extractWithContextDispatcher(file, p)
			if pd == dispatcher {
				skippedSelf = true
				continue
			}
		}
		if flatCallNameAny(file, p) == "withContext" {
			parentDispatcher := extractWithContextDispatcher(file, p)
			if parentDispatcher == dispatcher {
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Redundant nested withContext(%s). The parent already switches to this dispatcher.", dispatcher))}
			}
		}
	}
	return nil
}

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
func (r *LaunchWithoutCoroutineExceptionHandlerRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *LaunchWithoutCoroutineExceptionHandlerRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	callee := flatCallNameAny(file, idx)
	if callee != "launch" {
		return nil
	}
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver != "GlobalScope" && receiver != "" {
		return nil
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	lambdaText := file.FlatNodeText(lambda)
	if !strings.Contains(lambdaText, "throw ") {
		return nil
	}
	callText := file.FlatNodeText(idx)
	if strings.Contains(callText, "CoroutineExceptionHandler") {
		return nil
	}
	fnDecl, ok := flatEnclosingFunction(file, idx)
	if ok {
		fnText := file.FlatNodeText(fnDecl)
		if strings.Contains(fnText, "CoroutineExceptionHandler") {
			return nil
		}
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"launch{} contains throw but no CoroutineExceptionHandler. Uncaught exceptions will crash the app.")}
}

// ---------------------------------------------------------------------------
// Batch 5: cross-module rule
// ---------------------------------------------------------------------------

// MainDispatcherInLibraryCodeRule detects Dispatchers.Main in library modules
// without kotlinx-coroutines-android dependency.
type MainDispatcherInLibraryCodeRule struct {
	BaseRule
	pmi *module.PerModuleIndex
}

func (r *MainDispatcherInLibraryCodeRule) IsFixable() bool     { return false }
func (r *MainDispatcherInLibraryCodeRule) Confidence() float64 { return 0.75 }

func (r *MainDispatcherInLibraryCodeRule) Check(_ *scanner.File) []scanner.Finding { return nil }

func (r *MainDispatcherInLibraryCodeRule) SetModuleIndex(pmi *module.PerModuleIndex) {
	r.pmi = pmi
}

func (r *MainDispatcherInLibraryCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{
		NeedsFiles:        true,
		NeedsDependencies: true,
		NeedsIndex:        false,
	}
}

func (r *MainDispatcherInLibraryCodeRule) CheckModuleAware() []scanner.Finding {
	if r.pmi == nil || r.pmi.Graph == nil {
		return nil
	}

	var findings []scanner.Finding
	for modPath, mod := range r.pmi.Graph.Modules {
		if !isAndroidLibraryModule(mod) {
			continue
		}
		if hasCoroutinesAndroidDep(mod) {
			continue
		}
		files := r.pmi.ModuleFiles[modPath]
		for _, file := range files {
			for i, line := range file.Lines {
				if strings.Contains(line, "Dispatchers.Main") {
					col := strings.Index(line, "Dispatchers.Main") + 1
					findings = append(findings, scanner.Finding{
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
	return findings
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
