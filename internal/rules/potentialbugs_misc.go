package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// HasPlatformTypeRule detects public fun/property without explicit return type.
// With type inference: uses ResolveNode on expression-body return expressions
// to determine if the inferred type comes from Java interop (platform type).
// When the resolver can determine a concrete Kotlin type, the finding is suppressed.
// ---------------------------------------------------------------------------
type HasPlatformTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because the rule
// falls back to a string-prefix heuristic on "java./javax./android./Java/Javax"
// when the resolver cannot determine the return type. Findings from that
// fallback path carry known false-positive risk on identifiers that
// happen to start with those prefixes but are not Java interop.
func (r *HasPlatformTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// IgnoredReturnValueRule detects call expressions whose non-Unit return value
// is discarded. Uses type inference to resolve return types and tree-sitter
// parent-node analysis to determine if the result is used (assigned, returned,
// passed as argument). Falls back to regex when resolver is unavailable.
// ---------------------------------------------------------------------------
type IgnoredReturnValueRule struct {
	FlatDispatchBase
	BaseRule
	RestrictToConfig             bool
	ReturnValueAnnotations       []string
	IgnoreReturnValueAnnotations []string
	ReturnValueTypes             []string
	IgnoreFunctionCall           []string
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule needs type inference to resolve the return type of the call
// target. When the resolver is unavailable it falls back to name-based
// heuristics against ReturnValueTypes/IgnoreFunctionCall lists, which
// can miss custom wrapper APIs or fire on look-alike names.
func (r *IgnoredReturnValueRule) Confidence() float64 { return api.ConfidenceMedium }

// returnValueTypes that should always be flagged when discarded.
var returnValueFQNs = map[string]bool{
	"kotlin.sequences.Sequence":                 true,
	"kotlinx.coroutines.flow.Flow":              true,
	"kotlinx.coroutines.flow.StateFlow":         true,
	"kotlinx.coroutines.flow.SharedFlow":        true,
	"kotlinx.coroutines.flow.MutableStateFlow":  true,
	"kotlinx.coroutines.flow.MutableSharedFlow": true,
}

var defaultReturnValueTypePatterns = []string{
	"kotlin.Function*",
	"kotlin.sequences.Sequence",
	"kotlinx.coroutines.flow.*Flow",
	"java.util.stream.*Stream",
}

// checkReturnValueAnnotations are annotation FQNs (or simple names) that mark
// a function's return value as must-use.
var checkReturnValueAnnotations = map[string]bool{
	"CheckReturnValue": true,
	"CheckResult":      true,
	"com.google.errorprone.annotations.CheckReturnValue": true,
	"androidx.annotation.CheckResult":                    true,
	"javax.annotation.CheckReturnValue":                  true,
}

// canIgnoreReturnValueAnnotations are annotation FQNs that override @CheckReturnValue.
var canIgnoreReturnValueAnnotations = map[string]bool{
	"CanIgnoreReturnValue": true,
	"com.google.errorprone.annotations.CanIgnoreReturnValue": true,
}

// functionalOps are method names where discarding the result is almost certainly a bug
var functionalOps = map[string]bool{
	"map": true, "flatMap": true, "filter": true, "filterNot": true,
	"filterIsInstance": true, "sorted": true, "sortedBy": true, "sortedWith": true,
	"reversed": true, "zip": true, "take": true, "drop": true, "distinct": true,
	"groupBy": true, "associate": true, "partition": true, "fold": true, "reduce": true,
	"plus": true, "minus": true, "toList": true, "toSet": true, "toMap": true,
	"mapKeys": true, "mapValues": true, "flatten": true,
	"asSequence": true, "asFlow": true,
}

var ignoredReturnValueInPlaceMutationCalls = map[string]bool{
	"clear":   true,
	"dispose": true,
	"fill":    true,
	"reverse": true,
	"shuffle": true,
	"sort":    true,
}

var ignoredReturnValueEmitterSignalCalls = map[string]bool{
	"onComplete": true,
	"onError":    true,
	"onNext":     true,
}

var ignoredReturnValueVerificationContextCalls = map[string]bool{
	"verify":         true,
	"coVerify":       true,
	"verifyBlocking": true,
}

var ignoredReturnValueBuilderCallbackCalls = map[string]bool{
	"listener":  true,
	"onCancel":  true,
	"onError":   true,
	"onFailure": true,
	"onStart":   true,
	"onSuccess": true,
}

var ignoredReturnValueTestSetupSideEffectCalls = map[string]bool{
	"attach":     true,
	"bind":       true,
	"create":     true,
	"delete":     true,
	"detach":     true,
	"hide":       true,
	"initialize": true,
	"load":       true,
	"refresh":    true,
	"render":     true,
	"search":     true,
	"show":       true,
	"start":      true,
	"stop":       true,
	"submit":     true,
	"track":      true,
	"update":     true,
}

func ignoredReturnValueInsideVerificationContext(file *scanner.File, idx uint32) bool {
	root := ignoredReturnValueStatementRoot(file, idx)
	if root == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(root, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		found = ignoredReturnValueVerificationContextCalls[flatCallNameAny(file, n)]
	})
	return found
}

func ignoredReturnValueIsEmitterSignalCall(file *scanner.File, idx uint32, funcName string) bool {
	if !ignoredReturnValueEmitterSignalCalls[funcName] {
		return false
	}
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver == "" {
		return false
	}
	lower := strings.ToLower(receiver)
	switch lower {
	case "emitter", "observer", "source", "subject":
		return true
	default:
		return strings.HasSuffix(lower, "emitter") ||
			strings.HasSuffix(lower, "observer") ||
			strings.HasSuffix(lower, "source") ||
			strings.HasSuffix(lower, "subject")
	}
}

func ignoredReturnValueIsSideEffectFoldDispatch(file *scanner.File, idx uint32, funcName string) bool {
	if file == nil || idx == 0 || funcName != "fold" {
		return false
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return false
	}
	lambdaCount := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		lambda := ignoredReturnValueArgumentLambda(file, arg)
		if lambda == 0 {
			continue
		}
		lambdaCount++
		if !ignoredReturnValueLambdaHasSideEffect(file, lambda) {
			return false
		}
	}
	return lambdaCount >= 2
}

func ignoredReturnValueIsSideEffectResultHandler(file *scanner.File, idx uint32, funcName string, returnType *typeinfer.ResolvedType) bool {
	if file == nil || idx == 0 || !ignoredReturnValueIsResultHandlerName(funcName) ||
		!ignoredReturnValueTypeLooksLikeKotlinResult(returnType) {
		return false
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda != 0 {
		return ignoredReturnValueLambdaHasSideEffect(file, lambda)
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		lambda = ignoredReturnValueArgumentLambda(file, arg)
		if lambda == 0 {
			continue
		}
		return ignoredReturnValueLambdaHasSideEffect(file, lambda)
	}
	return false
}

func ignoredReturnValueIsFlowBuilderResultHandler(file *scanner.File, idx uint32, funcName string, returnType *typeinfer.ResolvedType) bool {
	if file == nil || idx == 0 || !ignoredReturnValueIsFlowBuilderResultHandlerName(funcName) ||
		!ignoredReturnValueTypeLooksLikeFlow(returnType) ||
		!ignoredReturnValueInsideFlowBuilder(file, idx) {
		return false
	}
	return ignoredReturnValueCallHasSideEffectLambda(file, idx)
}

func ignoredReturnValueCallHasSideEffectLambda(file *scanner.File, idx uint32) bool {
	lambda := flatCallTrailingLambda(file, idx)
	if lambda != 0 {
		return ignoredReturnValueLambdaHasSideEffect(file, lambda)
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		lambda = ignoredReturnValueArgumentLambda(file, arg)
		if lambda != 0 {
			return ignoredReturnValueLambdaHasSideEffect(file, lambda)
		}
	}
	return false
}

func ignoredReturnValueIsResultHandlerName(name string) bool {
	switch name {
	case "onSuccess", "onFailure", "onError", "getOrElse":
		return true
	default:
		return false
	}
}

func ignoredReturnValueIsFlowBuilderResultHandlerName(name string) bool {
	switch name {
	case "onSuccess", "onFailure", "onError":
		return true
	default:
		return false
	}
}

func ignoredReturnValueTypeLooksLikeFlow(returnType *typeinfer.ResolvedType) bool {
	if !ignoredReturnValueTypeKnown(returnType) {
		return false
	}
	name := ignoredReturnValueTypeName(returnType)
	if strings.Contains(name, "<") {
		name = name[:strings.Index(name, "<")]
	}
	return name == "Flow" ||
		name == "kotlinx.coroutines.flow.Flow" ||
		strings.HasSuffix(name, ".Flow") ||
		strings.HasSuffix(name, "StateFlow") ||
		strings.HasSuffix(name, "SharedFlow")
}

func ignoredReturnValueInsideFlowBuilder(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			if ignoredReturnValueIsFlowBuilderName(lambdaCalleeNameForNestingFlat(file, p)) {
				return true
			}
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

func ignoredReturnValueIsFlowBuilderName(name string) bool {
	switch name {
	case "flow", "channelFlow", "callbackFlow":
		return true
	default:
		return false
	}
}

func ignoredReturnValueIsTestSetupSideEffectCall(file *scanner.File, idx uint32, funcName string) bool {
	if file == nil || idx == 0 || !ignoredReturnValueTestSetupSideEffectCalls[funcName] {
		return false
	}
	receiver := flatReceiverNameFromCall(file, idx)
	if !ignoredReturnValueLooksLikeTestSubjectReceiver(receiver) {
		return false
	}
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !testingQualityIsTestFunction(file, fn) {
		return false
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	return body != 0 && testingQualityBodyHasAssertionOrVerification(file, body)
}

func ignoredReturnValueLooksLikeTestSubjectReceiver(receiver string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return false
	}
	lower := strings.ToLower(receiver)
	switch lower {
	case "undertest", "under_test", "sut", "subject":
		return true
	}
	for _, suffix := range []string{
		"delegate", "factory", "helper", "manager", "presenter", "repository", "store", "usecase", "viewmodel",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func ignoredReturnValueTypeLooksLikeKotlinResult(returnType *typeinfer.ResolvedType) bool {
	if !ignoredReturnValueTypeKnown(returnType) {
		return false
	}
	name := ignoredReturnValueTypeName(returnType)
	return name == "Result" ||
		name == "kotlin.Result" ||
		strings.HasSuffix(name, ".Result") ||
		strings.HasPrefix(name, "Result<") ||
		strings.Contains(name, ".Result<")
}

func ignoredReturnValueIsRxJavaSubscribeDisposable(file *scanner.File, idx uint32, funcName string, returnType *typeinfer.ResolvedType) bool {
	if file == nil || idx == 0 || funcName != "subscribe" ||
		!ignoredReturnValueTypeLooksLikeRxJavaDisposable(returnType) {
		return false
	}
	return flatCallTrailingLambda(file, idx) != 0 || ignoredReturnValueCallHasLambdaArgument(file, idx)
}

func ignoredReturnValueTypeLooksLikeRxJavaDisposable(returnType *typeinfer.ResolvedType) bool {
	if !ignoredReturnValueTypeKnown(returnType) {
		return false
	}
	name := ignoredReturnValueTypeName(returnType)
	return name == "Disposable" ||
		name == "io.reactivex.disposables.Disposable" ||
		name == "io.reactivex.rxjava3.disposables.Disposable" ||
		strings.HasSuffix(name, ".Disposable")
}

func ignoredReturnValueIsBuilderCallbackCall(file *scanner.File, idx uint32, funcName string) bool {
	if file == nil || idx == 0 || !ignoredReturnValueBuilderCallbackCalls[funcName] {
		return false
	}
	if flatCallTrailingLambda(file, idx) == 0 && !ignoredReturnValueCallHasLambdaArgument(file, idx) {
		return false
	}
	if ignoredReturnValueCallHasBuilderReceiver(file, idx) {
		return true
	}
	if strings.Contains(file.FlatNodeText(idx), "Builder(") {
		return true
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			if ignoredReturnValueLambdaBelongsToBuilderCall(file, p) {
				return true
			}
			return false
		case "class_body", "function_declaration", "source_file":
			return false
		case "call_expression":
			if ignoredReturnValueCallHasBuilderReceiver(file, p) || strings.Contains(file.FlatNodeText(p), "Builder(") {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueLambdaBelongsToBuilderCall(file *scanner.File, lambda uint32) bool {
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			return ignoredReturnValueCallHasBuilderReceiver(file, p) || strings.Contains(file.FlatNodeText(p), "Builder(")
		case "function_declaration", "source_file":
			return false
		}
	}
	return false
}

func ignoredReturnValueCallHasLambdaArgument(file *scanner.File, idx uint32) bool {
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(args, func(n uint32) {
		if found {
			return
		}
		found = file.FlatType(n) == "lambda_literal"
	})
	return found
}

func ignoredReturnValueCallHasBuilderReceiver(file *scanner.File, idx uint32) bool {
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver == "" {
		return false
	}
	return strings.Contains(strings.ToLower(receiver), "builder")
}

func ignoredReturnValueArgumentLambda(file *scanner.File, arg uint32) uint32 {
	var lambda uint32
	file.FlatWalkAllNodes(arg, func(n uint32) {
		if lambda == 0 && file.FlatType(n) == "lambda_literal" {
			lambda = n
		}
	})
	return lambda
}

func ignoredReturnValueLambdaHasSideEffect(file *scanner.File, lambda uint32) bool {
	found := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if found || n == lambda {
			return
		}
		switch file.FlatType(n) {
		case "assignment", "augmented_assignment", "call_expression":
			found = true
		case "jump_expression":
			text := strings.TrimSpace(file.FlatNodeText(n))
			found = strings.HasPrefix(text, "throw ")
		}
	})
	return found
}

func ignoredReturnValueStatementRoot(file *scanner.File, idx uint32) uint32 {
	root := idx
	for {
		parent, ok := file.FlatParent(root)
		if !ok {
			return root
		}
		switch file.FlatType(parent) {
		case "statements", "function_body", "source_file":
			return root
		default:
			root = parent
		}
	}
}

func ignoredReturnValueCallIsValueAssignmentReceiver(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "navigation_expression":
			if !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(parent)), ".value") {
				continue
			}
			if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "statements" {
				stmt := strings.TrimSpace(file.FlatNodeText(gp))
				nav := strings.TrimSpace(file.FlatNodeText(parent))
				if strings.HasPrefix(stmt, nav) && strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(stmt, nav)), "=") {
					return true
				}
			}
		case "assignment":
			text := file.FlatNodeText(parent)
			eq := strings.Index(text, "=")
			if eq < 0 {
				return false
			}
			callStart := int(file.FlatStartByte(idx))
			assignStart := int(file.FlatStartByte(parent))
			if callStart >= assignStart+eq {
				return false
			}
			lhs := strings.TrimSpace(text[:eq])
			return strings.HasSuffix(lhs, ".value")
		case "statements", "function_body", "source_file":
			return false
		}
	}
	return false
}
