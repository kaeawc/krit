package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// hasAnnotation is defined in library.go

// ---------------------------------------------------------------------------
// Batch 1: rules with existing fixtures
// ---------------------------------------------------------------------------

// ChannelReceiveWithoutCloseRule detects Channel properties that are never closed.
type ChannelReceiveWithoutCloseRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ChannelReceiveWithoutCloseRule) Confidence() float64 { return api.ConfidenceMedium }

var channelReceiveCalls = map[string]bool{
	"receive": true, "receiveCatching": true, "receiveAsFlow": true, "consumeEach": true,
}

var coroutineLaunchCalls = map[string]bool{"launch": true, "async": true, "produce": true, "actor": true}

// channelNonConsumingCalls are calls on a channel that send, inspect or finish it. Any other call on the
// channel is an extension or helper that may consume it, so its caller decides how the receive ends.
var channelNonConsumingCalls = map[string]bool{
	"send": true, "trySend": true, "trySendBlocking": true, "offer": true, "close": true, "cancel": true,
	"invokeOnClose": true, "hashCode": true, "equals": true, "toString": true,
}

// channelReceivesAllScopeBound reports that the file receives from the channel (root 0 is the whole
// file, since state holders are often sibling classes) and every receiver coroutine ends with a
// cancellable scope. Receives outside a launch end with the caller.
func channelReceivesAllScopeBound(file *scanner.File, classDecl uint32, propName string) bool {
	var sites []uint32
	file.FlatWalkNodes(classDecl, "call_expression", func(call uint32) {
		callee := flatCallExpressionName(file, call)
		if flatReceiverNameFromCall(file, call) == propName && (channelReceiveCalls[callee] || !channelNonConsumingCalls[callee]) {
			sites = append(sites, call)
		}
	})
	file.FlatWalkNodes(classDecl, "navigation_expression", func(nav uint32) {
		if text := file.FlatNodeText(nav); text == propName+".onReceive" || text == propName+".onReceiveCatching" {
			sites = append(sites, nav)
		}
	})
	file.FlatWalkNodes(classDecl, "for_statement", func(loop uint32) {
		if forLoopIterableName(file, loop) == propName {
			sites = append(sites, loop)
		}
	})
	if len(sites) == 0 {
		return false
	}
	for _, site := range sites {
		if channelReceiveInUnboundLaunch(file, classDecl, site, 0) {
			return false
		}
	}
	return true
}

// isTestDoubleName reports Fake*/Stub*/Test* (and *Fake/*Test) type names, which live in main source
// sets of fixture modules.
func isTestDoubleName(name string) bool {
	for _, prefix := range []string{"Fake", "Stub", "Test"} {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) && name[len(prefix)] >= 'A' && name[len(prefix)] <= 'Z' {
			return true
		}
	}
	return strings.HasSuffix(name, "Fake") || strings.HasSuffix(name, "Test")
}

func isTestDoubleClassName(file *scanner.File, classDecl uint32) bool {
	return isTestDoubleName(extractIdentifierFlat(file, classDecl))
}

// enclosedInTestDouble reports idx inside, or being, a class or object with a test-double name.
func enclosedInTestDouble(file *scanner.File, idx uint32) bool {
	for cur, ok := idx, true; ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			if isTestDoubleClassName(file, cur) {
				return true
			}
		}
	}
	return false
}

// forLoopIterableName returns the identifier iterated by `for (x in name)`.
func forLoopIterableName(file *scanner.File, loop uint32) string {
	name := ""
	for child := file.FlatFirstChild(loop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body", "statements":
			return name
		case "simple_identifier":
			name = file.FlatNodeText(child)
		case "variable_declaration", "multi_variable_declaration":
		default:
			if file.FlatIsNamed(child) {
				name = ""
			}
		}
	}
	return name
}

// channelReceiveInUnboundLaunch reports a site under GlobalScope or a fresh CoroutineScope(...) launch,
// following calls to the enclosing function one level so `launch { drain() }` is seen.
func channelReceiveInUnboundLaunch(file *scanner.File, classDecl, site uint32, depth int) bool {
	for p, ok := file.FlatParent(site); ok && p != classDecl; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			if coroutineLaunchCalls[flatCallNameAny(file, p)] && launchScopeIsUnbound(file, p) {
				return true
			}
		case "function_declaration":
			if depth > 0 {
				return false
			}
			name := flatFunctionName(file, p)
			found := false
			file.FlatWalkNodes(classDecl, "call_expression", func(call uint32) {
				if !found && flatCallExpressionName(file, call) == name &&
					channelReceiveInUnboundLaunch(file, classDecl, call, depth+1) {
					found = true
				}
			})
			return found
		}
	}
	return false
}

// launchScopeIsUnbound reports a launch whose receiver is GlobalScope or a freshly built CoroutineScope(...).
func launchScopeIsUnbound(file *scanner.File, call uint32) bool {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		for child := file.FlatFirstChild(call); child != 0 && nav == 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				nav, _ = flatCallExpressionParts(file, child)
			}
		}
	}
	if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
		return false
	}
	recv := file.FlatNamedChild(nav, 0)
	if file.FlatType(recv) == "call_expression" {
		return flatCallExpressionName(file, recv) == "CoroutineScope"
	}
	return file.FlatNodeText(recv) == "GlobalScope"
}

// CollectionsSynchronizedListIterationRule detects iteration over synchronized wrappers without external sync.
type CollectionsSynchronizedListIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CollectionsSynchronizedListIterationRule) Confidence() float64 { return api.ConfidenceMedium }

var synchronizedCollectionFactories = map[string]bool{
	"synchronizedList": true, "synchronizedSet": true, "synchronizedMap": true,
}

// ConcurrentModificationIterationRule detects collection mutation inside for loops.
type ConcurrentModificationIterationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ConcurrentModificationIterationRule) Confidence() float64 { return api.ConfidenceMedium }

var mutatingMethods = map[string]bool{
	"remove": true, "add": true, "addAll": true, "removeAll": true, "clear": true,
}

// CoroutineScopeCreatedButNeverCancelledRule detects CoroutineScope properties without cancel().
type CoroutineScopeCreatedButNeverCancelledRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// DeferredAwaitInFinallyRule detects .await() calls inside finally blocks.
type DeferredAwaitInFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DeferredAwaitInFinallyRule) Confidence() float64 { return api.ConfidenceMedium }

// FlowWithoutFlowOnRule detects flow chains with collect but no flowOn.
type FlowWithoutFlowOnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FlowWithoutFlowOnRule) Confidence() float64 { return api.ConfidenceMedium }

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

func (r *SynchronizedOnStringRule) Confidence() float64 { return api.ConfidenceMedium }

// SynchronizedOnBoxedPrimitiveRule detects synchronized() on boxed primitives.
type SynchronizedOnBoxedPrimitiveRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SynchronizedOnBoxedPrimitiveRule) Confidence() float64 { return api.ConfidenceMedium }

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

func (r *SynchronizedOnNonFinalRule) Confidence() float64 { return api.ConfidenceMedium }

func isVarPropertyInScope(file *scanner.File, fromIdx uint32, varName string) bool {
	classDecl, ok := flatEnclosingAncestor(file, fromIdx, "class_declaration", "object_declaration")
	if !ok {
		return isVarMemberNamed(file, 0, varName)
	}
	// A parameter, local or loop variable of the same name hides the property.
	if dclIdentifierShadowed(file, fromIdx, varName, classDecl) {
		return false
	}
	return isVarMemberNamed(file, classDecl, varName)
}

// isVarMemberNamed reports whether classDecl (or the file when it is 0) declares a member
// `var` named varName, either in its body or as a primary-constructor property.
func isVarMemberNamed(file *scanner.File, classDecl uint32, varName string) bool {
	hasVarToken := func(node uint32) bool {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "var" || file.FlatNodeTextEquals(child, "var") {
				return true
			}
		}
		return false
	}
	found := false
	walk := func(root uint32) {
		file.FlatWalkNodes(root, "property_declaration", func(propIdx uint32) {
			if found || extractIdentifierFlat(file, propIdx) != varName {
				return
			}
			parent, ok := file.FlatParent(propIdx)
			if !ok {
				return
			}
			switch file.FlatType(parent) {
			case "class_body":
				if gp, ok := file.FlatParent(parent); !ok || (classDecl != 0 && gp != classDecl) {
					return
				}
			case "source_file":
				if classDecl != 0 {
					return
				}
			default:
				return
			}
			found = hasVarToken(propIdx)
		})
	}
	if classDecl == 0 {
		walk(0)
		return found
	}
	walk(classDecl)
	if found {
		return true
	}
	file.FlatWalkNodes(classDecl, "class_parameter", func(p uint32) {
		if found || !hasVarToken(p) {
			return
		}
		if extractIdentifierFlat(file, p) == varName {
			if owner, ok := flatEnclosingAncestor(file, p, "class_declaration"); ok && owner == classDecl {
				found = true
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

func (r *VolatileMissingOnDclRule) Confidence() float64 { return api.ConfidenceMedium }

// countDclNullChecksSplit counts `prop == null` checks outside and inside synchronized scopes.
func countDclNullChecksSplit(file *scanner.File, classDecl uint32, propName string) (outside, inside int) {
	if file == nil || classDecl == 0 || propName == "" {
		return 0, 0
	}
	file.FlatWalkNodes(classDecl, "equality_expression", func(eqIdx uint32) {
		operand, op, ok := flatNullComparisonOperand(file, eqIdx)
		if !ok || op != "==" {
			return
		}
		if file.FlatType(operand) != "simple_identifier" {
			return
		}
		if !file.FlatNodeTextEquals(operand, propName) {
			return
		}
		if dclIdentifierShadowed(file, operand, propName, classDecl) {
			return
		}
		if dclInsideSynchronizedScope(file, operand) {
			inside++
		} else {
			outside++
		}
	})
	return outside, inside
}

// dclInsideSynchronizedScope reports a synchronized block, @Synchronized function, or *Locked function.
func dclInsideSynchronizedScope(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) != "call_expression" {
			continue
		}
		if name := flatCallNameAny(file, parent); name == "synchronized" || name == "withLock" {
			return true
		}
	}
	fn, ok := flatEnclosingFunction(file, idx)
	return ok && (flatHasAnnotationNamed(file, fn, "Synchronized") || strings.HasSuffix(flatFunctionName(file, fn), "Locked"))
}

// dclIdentifierShadowed returns true when, between the comparison site and the
// enclosing class, some local property_declaration or function parameter
// declares the same name — meaning the simple_identifier may refer to a local,
// not the class property.
func dclIdentifierShadowed(file *scanner.File, idx uint32, name string, classDecl uint32) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	for current, ok := file.FlatParent(idx); ok && current != classDecl; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration", "anonymous_function", "lambda_literal", "function_body", "control_structure_body", "statements":
			if scopeDeclaresName(file, current, name) {
				return true
			}
		}
	}
	return false
}

func scopeDeclaresName(file *scanner.File, scope uint32, name string) bool {
	if file == nil || scope == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(scope, "property_declaration", func(propIdx uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, propIdx) == name {
			found = true
		}
	})
	if found {
		return true
	}
	for _, paramType := range []string{"parameter", "function_value_parameter", "lambda_parameter", "class_parameter", "variable_declaration"} {
		file.FlatWalkNodes(scope, paramType, func(p uint32) {
			if found {
				return
			}
			for ch := file.FlatFirstChild(p); ch != 0; ch = file.FlatNextSib(ch) {
				if file.FlatType(ch) == "simple_identifier" && file.FlatNodeTextEquals(ch, name) {
					found = true
					return
				}
			}
		})
		if found {
			return true
		}
	}
	return found
}

// MutableStateInObjectRule detects var properties inside object declarations.
type MutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MutableStateInObjectRule) Confidence() float64 { return api.ConfidenceMedium }

var threadSafeTypes = map[string]bool{
	"AtomicInteger": true, "AtomicLong": true, "AtomicBoolean": true,
	"AtomicReference": true, "ConcurrentHashMap": true, "CopyOnWriteArrayList": true,
	// kotlin.concurrent.atomics (Kotlin 2.1+)
	"AtomicInt": true, "AtomicIntArray": true, "AtomicLongArray": true, "AtomicArray": true,
}

func mutableStateInObjectShouldSkip(file *scanner.File, objectIdx, propIdx uint32, propText, propName string) bool {
	if file == nil || objectIdx == 0 || propIdx == 0 {
		return true
	}
	if scanner.IsTestFile(file.Path) || pathIsSampleOrPlayground(file.Path) || enclosedInTestDouble(file, objectIdx) {
		return true
	}
	if propName == "" {
		return true
	}
	if mutableObjectPropertyHasNoBackingField(file, propIdx) {
		return true
	}
	if mutableObjectVarResetByLifecycle(file, objectIdx, propIdx, propName) {
		return true
	}
	if strings.Contains(propText, "private") && mutableObjectVarOnlyWrittenByTestHooks(file, objectIdx, propIdx, propName) {
		return true
	}
	if strings.Contains(propText, "@Volatile") || strings.Contains(propText, "Volatile") {
		return true
	}
	for typeName := range threadSafeTypes {
		if strings.Contains(propText, typeName) {
			return true
		}
	}
	if strings.Contains(propText, "private") && mutableObjectPropertyReferencesAreSynchronized(file, objectIdx, propIdx, propName) {
		return true
	}
	return false
}

// mutableObjectPropertyHasNoBackingField reports a property with a custom getter, no initializer or delegate, and
// no `field` use in its accessors. It stores nothing, so there is no shared state.
func mutableObjectPropertyHasNoBackingField(file *scanner.File, propIdx uint32) bool {
	if propertyInitializerExpression(file, propIdx) != 0 || file.FlatHasChildOfType(propIdx, "property_delegate") {
		return false
	}
	// The grammar places accessors as siblings that follow the property.
	hasGetter, usesField := false, false
	for acc := file.FlatNextSib(propIdx); acc != 0; acc = file.FlatNextSib(acc) {
		t := file.FlatType(acc)
		if t != "getter" && t != "setter" {
			if file.FlatIsNamed(acc) && t != "line_comment" && t != "multiline_comment" {
				break
			}
			continue
		}
		hasGetter = hasGetter || t == "getter"
		file.FlatWalkNodes(acc, "simple_identifier", func(id uint32) {
			if file.FlatNodeTextEquals(id, "field") {
				usesField = true
			}
		})
	}
	return hasGetter && !usesField
}

func mutableObjectPropertyReferencesAreSynchronized(file *scanner.File, objectIdx, propIdx uint32, propName string) bool {
	if mutableObjectPropertyReferencesAreSynchronizedByText(file, objectIdx, propName) {
		return true
	}
	seenReference := false
	allSynchronized := true
	file.FlatWalkNodes(objectIdx, "simple_identifier", func(candidate uint32) {
		if !allSynchronized {
			return
		}
		if candidate == 0 || !file.FlatNodeTextEquals(candidate, propName) {
			return
		}
		if flatNodeHasAncestor(file, candidate, propIdx) {
			return
		}
		seenReference = true
		if !mutableStateReferenceIsSynchronized(file, objectIdx, candidate, map[uint32]bool{}) {
			allSynchronized = false
		}
	})
	return seenReference && allSynchronized
}

func mutableStateReferenceIsSynchronized(file *scanner.File, objectIdx, refIdx uint32, visiting map[uint32]bool) bool {
	if flatNodeInsideCallNamed(file, refIdx, "synchronized") {
		return true
	}
	fn, ok := flatEnclosingFunction(file, refIdx)
	if !ok {
		return false
	}
	return mutableStateFunctionIsSynchronized(file, objectIdx, fn, visiting)
}

func mutableStateFunctionIsSynchronized(file *scanner.File, objectIdx, fn uint32, visiting map[uint32]bool) bool {
	if file == nil || fn == 0 {
		return false
	}
	fnText := file.FlatNodeText(fn)
	if strings.Contains(fnText, "@Synchronized") {
		return true
	}
	if !strings.Contains(fnText, "private") {
		return false
	}
	name := flatFunctionName(file, fn)
	if name == "" {
		return false
	}
	if visiting[fn] {
		return false
	}
	visiting[fn] = true
	defer delete(visiting, fn)

	seenCall := false
	allCallsSynchronized := true
	file.FlatWalkNodes(objectIdx, "call_expression", func(call uint32) {
		if !allCallsSynchronized || flatCallExpressionName(file, call) != name {
			return
		}
		if flatNodeHasAncestor(file, call, fn) {
			allCallsSynchronized = false
			return
		}
		seenCall = true
		if !mutableStateReferenceIsSynchronized(file, objectIdx, call, visiting) {
			allCallsSynchronized = false
		}
	})
	return seenCall && allCallsSynchronized
}

func mutableObjectPropertyReferencesAreSynchronizedByText(file *scanner.File, objectIdx uint32, propName string) bool {
	objectText := file.FlatNodeText(objectIdx)
	if objectText == "" || propName == "" || !strings.Contains(objectText, "synchronized") {
		return false
	}
	lines := strings.Split(objectText, "\n")
	syncDepth := 0
	pendingSync := false
	seenReference := false
	for _, line := range lines {
		lineHasSync := strings.Contains(line, "synchronized(")
		lineSyncDepth := syncDepth
		if pendingSync || lineHasSync {
			lineSyncDepth = 1
		}
		if mutableStateLineReferencesProperty(line, propName) && !mutableStateLineDeclaresProperty(line, propName) {
			seenReference = true
			if lineSyncDepth == 0 {
				return false
			}
		}
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		if lineHasSync {
			pendingSync = true
		}
		if pendingSync && opens > 0 {
			syncDepth += opens
			pendingSync = false
		} else if syncDepth > 0 {
			syncDepth += opens
		}
		if syncDepth > 0 {
			syncDepth -= closes
			if syncDepth < 0 {
				syncDepth = 0
			}
		}
	}
	return seenReference
}

func mutableStateLineReferencesProperty(line, propName string) bool {
	return strings.Contains(line, propName)
}

func mutableStateLineDeclaresProperty(line, propName string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "var "+propName) || strings.Contains(line, "var\t"+propName)
}

func flatNodeHasAncestor(file *scanner.File, idx, ancestor uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if parent == ancestor {
			return true
		}
	}
	return false
}

func flatNodeInsideCallNamed(file *scanner.File, idx uint32, name string) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "call_expression" && flatCallExpressionName(file, parent) == name {
			return true
		}
	}
	return false
}

// pathIsSampleOrPlayground reports whether path is under a sample or playground source tree.
func pathIsSampleOrPlayground(path string) bool {
	lower := scanner.LowerSlashPath(path)
	for _, seg := range []string{"/sample/", "/samples/", "/playground/"} {
		if strings.Contains(lower, seg) {
			return true
		}
	}
	return false
}

// mutableVarWrites returns the assignment and increment nodes that write propName under root.
func mutableVarWrites(file *scanner.File, root, propIdx uint32, propName string) []uint32 {
	var writes []uint32
	targetMatches := func(text string) bool {
		text = strings.TrimSpace(text)
		return text == propName || strings.HasSuffix(text, "."+propName)
	}
	file.FlatWalkAllNodes(root, func(n uint32) {
		if flatNodeHasAncestor(file, n, propIdx) {
			return
		}
		switch file.FlatType(n) {
		case "assignment":
			if target, ok := file.FlatFindChild(n, "directly_assignable_expression"); ok && targetMatches(file.FlatNodeText(target)) {
				writes = append(writes, n)
			}
		case "postfix_expression", "prefix_expression":
			text := file.FlatNodeText(n)
			if strings.Contains(text, "++") || strings.Contains(text, "--") {
				trimmed := strings.Trim(strings.TrimSpace(text), "+-")
				if targetMatches(trimmed) {
					writes = append(writes, n)
				}
			}
		}
	})
	return writes
}

var mutableVarLifecycleAnnotations = []string{"After", "Before", "AfterTest", "BeforeTest", "AfterEach", "BeforeEach", "AfterClass", "BeforeClass"}

// mutableObjectVarResetByLifecycle reports whether a test lifecycle function or a finally block writes the var.
func mutableObjectVarResetByLifecycle(file *scanner.File, _, propIdx uint32, propName string) bool {
	// Reset functions often live outside the object, so scan the whole file.
	for _, w := range mutableVarWrites(file, 0, propIdx, propName) {
		if _, inFinally := flatEnclosingAncestor(file, w, "finally_block"); inFinally {
			return true
		}
		fn, ok := flatEnclosingFunction(file, w)
		if !ok {
			continue
		}
		for _, ann := range mutableVarLifecycleAnnotations {
			if flatHasAnnotationNamed(file, fn, ann) {
				return true
			}
		}
	}
	return false
}

// mutableObjectVarOnlyWrittenByTestHooks reports whether every write to the var is in a test-only hook.
func mutableObjectVarOnlyWrittenByTestHooks(file *scanner.File, objectIdx, propIdx uint32, propName string) bool {
	writes := mutableVarWrites(file, objectIdx, propIdx, propName)
	if len(writes) == 0 {
		return false
	}
	for _, w := range writes {
		fn, ok := flatEnclosingFunction(file, w)
		if !ok || !functionIsTestHook(file, fn) {
			return false
		}
	}
	return true
}

func functionIsTestHook(file *scanner.File, fn uint32) bool {
	if flatHasAnnotationNamed(file, fn, "VisibleForTesting") {
		return true
	}
	name := flatFunctionName(file, fn)
	return strings.Contains(name, "Testing") || strings.HasSuffix(name, "ForTest") || strings.HasSuffix(name, "ForTests")
}

var concurrentIterationSafeMarkers = []string{
	"Iterator", ".iterator()", "listIterator", "Concurrent", "CopyOnWrite", ".toList()", ".toSet()", ".copyOf(", ".toTypedArray()",
}

// concurrentIterationIterableIsSafe reports whether the declared iterable is an iterator, a concurrent collection, or a copy.
func concurrentIterationIterableIsSafe(file *scanner.File, loop uint32, name string) bool {
	// Search the enclosing callable first, then the whole file so class-level properties count.
	scopes := []uint32{0}
	if scope, ok := flatEnclosingCallable(file, loop); ok {
		scopes = []uint32{scope, 0}
	}
	safe := false
	for _, scope := range scopes {
		for _, nodeType := range []string{"property_declaration", "parameter", "class_parameter"} {
			file.FlatWalkNodes(scope, nodeType, func(decl uint32) {
				if safe || extractIdentifierFlat(file, decl) != name {
					return
				}
				text := file.FlatNodeText(decl)
				for _, marker := range concurrentIterationSafeMarkers {
					if strings.Contains(text, marker) {
						safe = true
						return
					}
				}
			})
		}
	}
	return safe
}

var coroutineScopeParentContextTokens = []string{"backgroundScope", "viewModelScope", "lifecycleScope", "coroutineContext", "parentJob"}

// coroutineScopeHasParentJob reports a scope whose context already carries a parent Job, so cancelling the parent cancels it.
func coroutineScopeHasParentJob(file *scanner.File, prop uint32) bool {
	text := file.FlatNodeText(prop)
	for _, token := range coroutineScopeParentContextTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return regexp.MustCompile(`Job\(\s*[^)\s]`).MatchString(text)
}

// coroutineScopeCancelledInFile reports a cancel or cancelChildren on the scope anywhere in the file,
// including scopes that are collected and cancelled in a loop.
func coroutineScopeCancelledInFile(file *scanner.File, name string) bool {
	cancelled := false
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if cancelled {
			return
		}
		callee := flatCallExpressionName(file, call)
		if callee != "cancel" && callee != "cancelChildren" {
			return
		}
		if flatReceiverNameFromCall(file, call) == name || coroutineScopeCancelPath(file, call, name) {
			cancelled = true
		}
	})
	if cancelled {
		return true
	}
	content := string(file.Content)
	quoted := regexp.QuoteMeta(name)
	collected := regexp.MustCompile(`\.add\(\s*` + quoted + `\s*\)|\+=\s*` + quoted + `\b|listOf\([^)]*\b` + quoted + `\b`).MatchString(content)
	return collected && regexp.MustCompile(`\.cancel(Children)?\(`).MatchString(content)
}

// optsInTo reports an @OptIn(marker::class) on idx, an enclosing declaration, or the file.
func optsInTo(file *scanner.File, idx uint32, marker string) bool {
	// The source_file root has index 0, so walk until the parent lookup fails.
	for cur, ok := idx, true; ok; cur, ok = file.FlatParent(cur) {
		for child := file.FlatFirstChild(cur); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "modifiers", "annotation", "file_annotation":
				text := file.FlatNodeText(child)
				if strings.Contains(text, "OptIn") && strings.Contains(text, marker) {
					return true
				}
			}
		}
	}
	return false
}

// coroutineScopeCancelPath reports `scope.coroutineContext.cancel()`, `scope.coroutineContext[Job]?.cancel()`,
// `scope.coroutineContext.job.cancelChildren()` and the bare `coroutineContext...` forms used inside the scope.
func coroutineScopeCancelPath(file *scanner.File, call uint32, name string) bool {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return false
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(nav)), "")
	re := regexp.MustCompile(`^(?:` + regexp.QuoteMeta(name) + `\??\.)?coroutineContext(?:\[Job\]|\.job)?\??\.cancel(?:Children)?$`)
	return re.MatchString(text)
}

// coroutineScopeHandedToConsumer reports the scope passed as a plain argument to a call that keeps it
// for its own lifetime, such as a data store factory. Flow sharing operators only borrow the scope.
func coroutineScopeHandedToConsumer(file *scanner.File, classDecl uint32, name string) bool {
	found := false
	file.FlatWalkNodes(classDecl, "call_expression", func(call uint32) {
		if found || scopeBorrowingCalls[flatCallExpressionName(file, call)] {
			return
		}
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return
		}
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			if expr := flatValueArgumentExpression(file, arg); expr != 0 &&
				file.FlatType(expr) == "simple_identifier" && file.FlatNodeText(expr) == name {
				found = true
				return
			}
		}
	})
	return found
}

var scopeBorrowingCalls = map[string]bool{
	"stateIn": true, "shareIn": true, "launchIn": true, "produceIn": true,
	"add": true, "plus": true, "listOf": true, "mutableListOf": true, "println": true,
}

var scopeTeardownHooks = map[string]bool{
	"close": true, "dispose": true, "onCleared": true, "onDestroy": true, "release": true,
	"onForgotten": true, "onAbandoned": true, "shutdown": true, "destroy": true, "onDispose": true,
	"detach": true, "onDetach": true, "cleanup": true, "tearDown": true,
}

// scopeResetNames are extension calls on a scope that cancel its children when run from a teardown hook.
var scopeResetNames = map[string]bool{
	"reset": true, "clear": true, "cancelAll": true, "cancelChildren": true,
	"shutdown": true, "dispose": true, "close": true,
}

// coroutineScopeResetInTeardown reports `scope.reset()` or `scope.coroutineContext.clear()` style calls
// inside a teardown-named method. The extension is assumed to cancel the scope.
func coroutineScopeResetInTeardown(file *scanner.File, classDecl uint32, name string) bool {
	found := false
	file.FlatWalkNodes(classDecl, "call_expression", func(call uint32) {
		if found || !scopeResetNames[flatCallExpressionName(file, call)] {
			return
		}
		fn, ok := flatEnclosingFunction(file, call)
		if !ok || !scopeTeardownHooks[flatFunctionName(file, fn)] {
			return
		}
		if flatReceiverNameFromCall(file, call) == name {
			found = true
			return
		}
		nav, _ := flatCallExpressionParts(file, call)
		if nav == 0 {
			return
		}
		text := strings.Join(strings.Fields(file.FlatNodeText(nav)), "")
		prefix := name + "."
		if strings.HasPrefix(text, prefix+"coroutineContext.") || strings.HasPrefix(text, prefix+"coroutineContext?.") {
			found = true
		}
	})
	return found
}

// diContributionAnnotations mark a class the DI graph owns, so its lifetime is the graph's.
var diContributionAnnotations = []string{
	"@SingleIn(", "@ContributesBinding(", "@ContributesTo(", "@ContributesIntoSet(",
	"@ContributesIntoMap(", "@ContributesMultibinding(",
}

// coroutineScopeOwnerIsDISingleton reports a class that lives as long as a DI scope. `@Singleton` and
// `@SingleIn(AppScope::class)` last for the process. Another `@SingleIn` or an anvil/metro `@Contributes*` counts only when the class has
// no teardown hook that could cancel the scope.
func coroutineScopeOwnerIsDISingleton(file *scanner.File, classDecl uint32) bool {
	mods, ok := file.FlatFindChild(classDecl, "modifiers")
	if !ok {
		return false
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(mods)), "")
	if strings.Contains(text, "@Singleton") || strings.Contains(text, "@SingleIn(AppScope::class)") {
		return true
	}
	annotated := false
	for _, a := range diContributionAnnotations {
		if strings.Contains(text, a) {
			annotated = true
			break
		}
	}
	if !annotated {
		return false
	}
	hasHook := false
	file.FlatWalkNodes(classDecl, "function_declaration", func(fn uint32) {
		if scopeTeardownHooks[flatFunctionName(file, fn)] {
			hasHook = true
		}
	})
	return !hasHook
}

// mutationFollowedByLoopExit reports a statement that is followed in the same block by `break`, `return` or
// `throw`, so the iterator is never advanced after the mutation.
func mutationFollowedByLoopExit(file *scanner.File, call uint32) bool {
	stmt := call
	for {
		parent, ok := file.FlatParent(stmt)
		if !ok {
			return false
		}
		if file.FlatType(parent) == "statements" {
			break
		}
		switch file.FlatType(parent) {
		case "function_declaration", "lambda_literal", "for_statement":
			return false
		}
		stmt = parent
	}
	for next, ok := file.FlatNextSibling(stmt); ok; next, ok = file.FlatNextSibling(next) {
		if file.FlatType(next) != "jump_expression" {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(next))
		if strings.HasPrefix(text, "break") || strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw") {
			return true
		}
	}
	return false
}
