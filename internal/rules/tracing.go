package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// WithContextWithoutTracingContextRule detects coroutine dispatcher boundary
// calls made while an OpenTelemetry span is active, but without propagating the
// tracing context through asContextElement().
type WithContextWithoutTracingContextRule struct {
	FlatDispatchBase
	BaseRule

	AllowedDispatchers []string
}

func (r *WithContextWithoutTracingContextRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WithContextWithoutTracingContextRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	name, args, lambda := coroutineBuilderPartsFlat(file, idx)
	if name != "withContext" && name != "async" && name != "launch" && name != "runInterruptible" {
		return "", false
	}
	if lambda == 0 {
		return "", false
	}
	dispatcherArg := flatPositionalValueArgument(file, args, 0)
	if dispatcherArg == 0 {
		return "", false
	}
	dispatcherExpr := flatValueArgumentExpression(file, dispatcherArg)
	dispatcher, ok := tracingDispatcherName(file, dispatcherExpr)
	if !ok || r.dispatcherAllowed(dispatcher) {
		return "", false
	}
	if tracingContextElementPresent(file, dispatcherExpr) {
		return "", false
	}
	if !hasEnclosingActiveSpan(file, idx) {
		return "", false
	}
	return name, true
}

func (r *WithContextWithoutTracingContextRule) dispatcherAllowed(dispatcher string) bool {
	for _, allowed := range r.AllowedDispatchers {
		normalized := strings.TrimPrefix(strings.TrimSpace(allowed), "Dispatchers.")
		if normalized == dispatcher {
			return true
		}
	}
	return false
}

func tracingDispatcherName(file *scanner.File, expr uint32) (string, bool) {
	if file == nil || expr == 0 {
		return "", false
	}
	text := file.FlatNodeText(expr)
	for _, dispatcher := range []string{"IO", "Default", "Unconfined"} {
		if strings.Contains(text, "Dispatchers."+dispatcher) {
			return dispatcher, true
		}
	}
	return "", false
}

func tracingContextElementPresent(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(expr, func(node uint32) {
		if found || file.FlatType(node) != "call_expression" {
			return
		}
		if flatCallExpressionName(file, node) == "asContextElement" {
			found = true
		}
	})
	return found
}

func hasEnclosingActiveSpan(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if fn, ok := flatEnclosingFunction(file, idx); ok && flatHasAnnotationNamed(file, fn, "WithSpan") {
		return true
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			if flatCallExpressionName(file, parent) == "use" && tracingSpanStartText(file.FlatNodeText(parent)) {
				return true
			}
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return enclosingFunctionHasPriorSpanStart(file, parent, idx)
		}
	}
	return false
}

func enclosingFunctionHasPriorSpanStart(file *scanner.File, fn, idx uint32) bool {
	if file == nil || fn == 0 || idx == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	targetRow := file.FlatRow(idx)
	found := false
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		if found || file.FlatRow(call) >= targetRow {
			return
		}
		if flatCallExpressionName(file, call) == "startSpan" && tracingSpanStartText(file.FlatNodeText(call)) {
			found = true
		}
	})
	return found
}

// SpanStartWithoutFinishRule detects spans started into local variables that
// are not closed by end(), use, or makeCurrent().use in the same scope.
type SpanStartWithoutFinishRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SpanStartWithoutFinishRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SpanStartWithoutFinishRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return "", false
	}
	owner, ok := flatEnclosingAncestor(file, idx, "function_declaration", "lambda_literal")
	if !ok {
		return "", false
	}
	name := propertyDeclarationName(file, idx)
	if name == "" {
		return "", false
	}
	init := propertyInitializerExpression(file, idx)
	if !spanStartInitializer(file, init) {
		return "", false
	}
	if spanFinishedAfterDeclaration(file, owner, idx, name) {
		return "", false
	}
	return name, true
}

func spanStartInitializer(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	compact := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
	return strings.Contains(compact, "spanBuilder(") &&
		strings.Contains(compact, ".startSpan(") &&
		strings.HasSuffix(compact, ".startSpan()")
}

func spanFinishedAfterDeclaration(file *scanner.File, owner, decl uint32, spanName string) bool {
	if file == nil || owner == 0 || decl == 0 || spanName == "" {
		return false
	}
	declRow := file.FlatRow(decl)
	found := false
	file.FlatWalkNodes(owner, "call_expression", func(call uint32) {
		if found || file.FlatRow(call) <= declRow || !callBelongsDirectlyToSpanOwner(file, call, owner) {
			return
		}
		if spanLifecycleCall(file, call, spanName) {
			found = true
		}
	})
	return found
}

func callBelongsDirectlyToSpanOwner(file *scanner.File, call, owner uint32) bool {
	for parent, ok := file.FlatParent(call); ok; parent, ok = file.FlatParent(parent) {
		if parent == owner {
			return true
		}
		switch file.FlatType(parent) {
		case "function_declaration", "lambda_literal":
			return false
		}
	}
	return false
}

func spanLifecycleCall(file *scanner.File, call uint32, spanName string) bool {
	name := flatCallExpressionName(file, call)
	switch name {
	case "end":
		return flatReceiverNameFromCall(file, call) == spanName
	case "use":
		text := strings.Join(strings.Fields(file.FlatNodeText(call)), "")
		return strings.HasPrefix(text, spanName+".use{") ||
			strings.HasPrefix(text, spanName+".use(") ||
			strings.HasPrefix(text, spanName+".makeCurrent().use{") ||
			strings.HasPrefix(text, spanName+".makeCurrent().use(")
	default:
		return false
	}
}

// SpanAttributeWithHighCardinalityRule detects span attributes whose keys are
// commonly unique per user/session/request and therefore noisy in trace indexes.
type SpanAttributeWithHighCardinalityRule struct {
	FlatDispatchBase
	BaseRule

	Keys []string
}

func (r *SpanAttributeWithHighCardinalityRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SpanAttributeWithHighCardinalityRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return "", false
	}
	method := flatCallExpressionName(file, idx)
	if method != "setAttribute" && method != "setAttributes" {
		return "", false
	}
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return "", false
	}
	keys := r.keySet()
	if method == "setAttribute" {
		first := flatPositionalValueArgument(file, args, 0)
		if first == 0 {
			first = flatNamedValueArgument(file, args, "key")
		}
		if key := firstHighCardinalityString(file, flatValueArgumentExpression(file, first), keys); key != "" {
			return key, true
		}
		return "", false
	}
	if key := firstHighCardinalityAttributeKeyCall(file, args, keys); key != "" {
		return key, true
	}
	return "", false
}

func (r *SpanAttributeWithHighCardinalityRule) keySet() map[string]bool {
	configured := r.Keys
	if len(configured) == 0 {
		configured = []string{"user_id", "session_id", "trace_id"}
	}
	keys := make(map[string]bool, len(configured))
	for _, key := range configured {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys[trimmed] = true
		}
	}
	return keys
}

func firstHighCardinalityAttributeKeyCall(file *scanner.File, root uint32, keys map[string]bool) string {
	if file == nil || root == 0 || len(keys) == 0 {
		return ""
	}
	found := ""
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found != "" {
			return
		}
		switch flatCallExpressionName(file, call) {
		case "stringKey", "longKey", "doubleKey", "booleanKey":
		default:
			return
		}
		_, args := flatCallExpressionParts(file, call)
		first := flatPositionalValueArgument(file, args, 0)
		found = firstHighCardinalityString(file, flatValueArgumentExpression(file, first), keys)
	})
	return found
}
