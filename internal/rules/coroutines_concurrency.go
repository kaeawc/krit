package rules

import (
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

func (r *VolatileMissingOnDclRule) Confidence() float64 { return api.ConfidenceMedium }

// countDclNullChecks counts `propName == null` (or `null == propName`)
// equality expressions inside the class where the identifier resolves to the
// class property — i.e. is not shadowed by a local declaration or parameter
// in the enclosing function. This replaces a substring count that produced
// false positives whenever a local variable happened to share the name.
func countDclNullChecks(file *scanner.File, classDecl uint32, propName string) int {
	if file == nil || classDecl == 0 || propName == "" {
		return 0
	}
	count := 0
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
		count++
	})
	return count
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
	for _, paramType := range []string{"parameter", "function_value_parameter", "lambda_parameter", "class_parameter"} {
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
}

func mutableStateInObjectShouldSkip(file *scanner.File, objectIdx, propIdx uint32, propText, propName string) bool {
	if file == nil || objectIdx == 0 || propIdx == 0 {
		return true
	}
	if scanner.IsTestFile(file.Path) {
		return true
	}
	if propName == "" {
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
