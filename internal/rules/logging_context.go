package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// LogWithoutCorrelationIDRule detects logger calls inside coroutine builders
// whose context does not include MDCContext().
type LogWithoutCorrelationIDRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LogWithoutCorrelationIDRule) Confidence() float64 { return api.ConfidenceMedium }

// NullableStructuredFieldRule detects addKeyValue fields that pass nullable
// safe-call expressions without an explicit fallback value.
type NullableStructuredFieldRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NullableStructuredFieldRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullableStructuredFieldRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "addKeyValue" {
		return false
	}
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return false
	}
	valueArg := flatPositionalValueArgument(file, args, 1)
	if valueArg == 0 {
		valueArg = flatNamedValueArgument(file, args, "value")
	}
	expr := flatValueArgumentExpression(file, valueArg)
	if expr == 0 {
		return false
	}
	return nullableSafeCallWithoutElvis(file, expr)
}

// nullableSafeCallWithoutElvis reports whether the expression rooted at idx
// contains an actual `?.` safe-navigation token outside any string literal
// and is not the receiver of an `?:` Elvis expression that supplies a
// fallback. Detection runs over the flat AST rather than raw text so that
// string-literal contents such as `"use ?. operator"` cannot trigger a
// false positive.
func nullableSafeCallWithoutElvis(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	// An Elvis expression at the top supplies the fallback; any safe call
	// inside is paired with `?:` so the rule should not fire.
	if file.FlatType(idx) == "elvis_expression" {
		return false
	}
	return flatExpressionHasSafeCallOutsideStrings(file, idx)
}

// flatExpressionHasSafeCallOutsideStrings walks the flat AST rooted at idx
// and returns true when it finds a `?.` token node that is not nested inside
// a string literal. It also returns false if any enclosing/descendant Elvis
// expression provides a fallback for the safe call.
func flatExpressionHasSafeCallOutsideStrings(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	hasSafe := false
	hasElvis := false
	var walk func(uint32)
	walk = func(node uint32) {
		if node == 0 || hasSafe && hasElvis {
			return
		}
		switch file.FlatType(node) {
		case "string_literal", "line_string_literal", "multi_line_string_literal",
			"raw_string_literal", "character_literal",
			"line_comment", "multiline_comment":
			return
		case "?.":
			hasSafe = true
		case "elvis_expression":
			hasElvis = true
		}
		for i := 0; i < file.FlatChildCount(node); i++ {
			walk(file.FlatChild(node, i))
		}
	}
	walk(idx)
	return hasSafe && !hasElvis
}

func firstCorrelationSensitiveLogCallFlat(file *scanner.File, idx uint32) uint32 {
	var match uint32
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if match != 0 || file.FlatType(candidate) != "call_expression" {
			return
		}

		receiver := flatReceiverNameFromCall(file, candidate)
		if receiver != "logger" && receiver != "log" && receiver != "Timber" {
			return
		}

		switch flatCallExpressionName(file, candidate) {
		case "trace", "debug", "info", "warn", "warning", "error":
			match = candidate
		}
	})
	return match
}

// LoggerWithoutLoggerFieldRule detects LoggerFactory.getLogger(...) calls
// inside function bodies where a per-call logger instance would be created.
type LoggerWithoutLoggerFieldRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LoggerWithoutLoggerFieldRule) Confidence() float64 { return api.ConfidenceMedium }

// MdcPutNoRemoveRule detects MDC.put("key", value) calls inside a function
// body where the same function does not contain a matching MDC.remove("key")
// or MDC.clear(). MDC values otherwise leak across requests when the thread
// is reused. The MDCCloseable form (MDC.putCloseable("key", value).use { ... })
// is naturally excluded because its call name is putCloseable, not put.
type MdcPutNoRemoveRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *MdcPutNoRemoveRule) Confidence() float64 { return api.ConfidenceMedium }

// mdcStaticKeyFlat returns the literal value of `call`'s first positional
// argument when that argument is a non-interpolated string literal. Used by
// MDC.put / MDC.remove key tracking and by StructuredLogKeyMixedCaseRule's
// key-convention scan. The boolean is false when the argument is dynamic.
func mdcStaticKeyFlat(file *scanner.File, call uint32) (string, bool) {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return "", false
	}
	keyArg := flatPositionalValueArgument(file, args, 0)
	if keyArg == 0 {
		return "", false
	}
	expr := flatValueArgumentExpression(file, keyArg)
	if expr == 0 || file.FlatType(expr) != "string_literal" {
		return "", false
	}
	if flatContainsStringInterpolation(file, expr) {
		return "", false
	}
	return stringLiteralContent(file, expr), true
}

// mdcRemoveOrClearMatchesFlat reports whether `fn`'s body contains an
// MDC.remove(key) for the given key, or an MDC.clear() call, within
// the same synchronous scope as the matching MDC.put. Calls that live
// inside a nested lambda, anonymous function, or local class/object —
// e.g. `launch { MDC.clear() }` — execute on a different thread or
// at a different time and do not actually clean up the put we paired
// against, so flatWalkLocalScope stops at those boundaries. When the
// key is not a static literal, any in-scope MDC.remove(...) call
// counts as a match.
func mdcRemoveOrClearMatchesFlat(file *scanner.File, fn uint32, key string, keyKnown bool) bool {
	if file == nil || fn == 0 {
		return false
	}
	matched := false
	flatWalkLocalScope(file, fn, func(candidate uint32) {
		if matched {
			return
		}
		if file.FlatType(candidate) != "call_expression" {
			return
		}
		if flatReceiverNameFromCall(file, candidate) != "MDC" {
			return
		}
		switch flatCallExpressionName(file, candidate) {
		case "clear":
			matched = true
			return
		case "remove":
			if !keyKnown {
				matched = true
				return
			}
			rmKey, ok := mdcStaticKeyFlat(file, candidate)
			if !ok || rmKey == key {
				matched = true
			}
		}
	})
	return matched
}

// MdcAcrossCoroutineBoundaryRule detects an `MDC.put(...)` followed by a
// coroutine builder (`launch`/`async`/`withContext`) in the same statement
// scope whose context argument does not include `MDCContext()`. MDC values do
// not propagate to a coroutine running on a different dispatcher unless the
// caller installs `MDCContext` in the coroutine context.
type MdcAcrossCoroutineBoundaryRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *MdcAcrossCoroutineBoundaryRule) Confidence() float64 { return api.ConfidenceMedium }

// firstUnpropagatedCoroutineBuilderAfterFlat walks subsequent siblings of a
// statement-level call_expression and returns the first sibling call that
// looks like a coroutine builder without `MDCContext()` in its context arg.
// Walks only direct siblings — it does not descend into nested scopes.
func firstUnpropagatedCoroutineBuilderAfterFlat(file *scanner.File, idx uint32) (uint32, string) {
	if file == nil || idx == 0 {
		return 0, ""
	}
	for sib := file.FlatNextSib(idx); sib != 0; sib = file.FlatNextSib(sib) {
		if file.FlatType(sib) != "call_expression" {
			continue
		}
		name, args, lambda := coroutineBuilderPartsFlat(file, sib)
		if name != "launch" && name != "async" && name != "withContext" {
			continue
		}
		if lambda == 0 {
			continue
		}
		if coroutineContextHasMDCFlat(file, args) {
			continue
		}
		return sib, name
	}
	return 0, ""
}
