package rules

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's WebViewApiAvailabilityDetector, WorkManagerDetector (EnqueueWork) and
// ShortcutUsageDetector (ReportShortcutUsage).

// WebViewAPIAvailabilityRule mirrors Lint's WebViewApiAvailability: a version-checked call to a WebView method from
// API 22-28 that androidx.webkit's WebViewCompat also offers.
type WebViewAPIAvailabilityRule struct {
	AndroidRule
}

// EnqueueWorkRule mirrors Lint's EnqueueWork: a WorkContinuation that is never enqueued.
type EnqueueWorkRule struct {
	AndroidRule
}

// ReportShortcutUsageRule mirrors Lint's ReportShortcutUsage: dynamic shortcuts published in an app that never reports
// shortcut usage.
type ReportShortcutUsageRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the receiver and the guard must both be proven.
func (r *WebViewAPIAvailabilityRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium-high base confidence: the continuation is tracked through locals in the function.
func (r *EnqueueWorkRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// Confidence reports a medium-high base confidence: the receivers are proven from source.
func (r *ReportShortcutUsageRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// ---------------------------------------------------------------------------
// SDK_INT guards
// ---------------------------------------------------------------------------

// sdkGuardLevel returns the highest API level an enclosing condition proves at idx, as Lint's
// isWithinVersionCheckConditional does for these checks: the then branch of `SDK_INT >= N` (or `> N-1`), the else branch
// of `SDK_INT < N`, a `when` entry and the else after its entries, `when (SDK_INT) { in N..M -> }`, a Java ternary, the
// right side of `&&` (and of `||` for a failing left side), a Boolean local initialized with such a check, and a call of
// a same-file @ChecksSdkIntAtLeast or one-line check function. The walk goes through lambdas and the methods of
// anonymous objects; it stops at a named function or class. Early returns do not count. It returns 0 when nothing
// proves a level.
func sdkGuardLevel(file *scanner.File, idx uint32) int {
	best := 0
	raise := func(lvl int) {
		if lvl > best {
			best = lvl
		}
	}
	for child := idx; ; {
		p, ok := file.FlatParent(child)
		if !ok {
			return best
		}
		switch file.FlatType(p) {
		case "if_expression", "if_statement":
			cond, then, els := sdkIfParts(file, p)
			if cond != 0 && (child == then || child == els) {
				raise(sdkConditionLevel(file, cond, child == then))
			}
		case "when_entry":
			if file.FlatType(child) == "control_structure_body" {
				raise(sdkWhenEntryLevel(file, p))
			}
		case "ternary_expression":
			// Java `SDK_INT >= N ? webView.call() : fallback`.
			if parts := sdkNamedParts(file, p); len(parts) == 3 && (child == parts[1] || child == parts[2]) {
				raise(sdkConditionLevel(file, parts[0], child == parts[1]))
			}
		case "conjunction_expression", "disjunction_expression":
			// Kotlin `a && b` runs b when a holds; `a || b` when a fails.
			for c := file.FlatFirstChild(p); c != 0 && c != child; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					raise(sdkConditionLevel(file, c, file.FlatType(p) == "conjunction_expression"))
				}
			}
		case "binary_expression":
			if op, left, right, ok := versionCheckComparison(file, p); ok && trimSkipParens(file, right) == trimSkipParens(file, child) {
				switch op {
				case "&&":
					raise(sdkConditionLevel(file, left, true))
				case "||":
					raise(sdkConditionLevel(file, left, false))
				}
			}
		case "function_declaration", "method_declaration", "class_body":
			// The methods of an anonymous object run where it is created; any other function or class stops the walk.
			if !sdkAnonymousScope(file, p) {
				return best
			}
		case "source_file", "program":
			return best
		}
		child = p
	}
}

// sdkAnonymousScope reports a method or class body of an anonymous object: Kotlin `object : T { }`, Java
// `new T() { }`.
func sdkAnonymousScope(file *scanner.File, n uint32) bool {
	body := n
	if t := file.FlatType(n); t != "class_body" {
		var ok bool
		if body, ok = file.FlatParent(n); !ok || file.FlatType(body) != "class_body" {
			return false
		}
	}
	owner, ok := file.FlatParent(body)
	return ok && (file.FlatType(owner) == "object_literal" || file.FlatType(owner) == "object_creation_expression")
}

// sdkWhenEntryLevel returns the level a when entry proves for its body: its own conditions, or, for an else entry, the
// failure of every entry before it. A `when (SDK_INT)` entry proves `in N..M` and `N`.
func sdkWhenEntryLevel(file *scanner.File, entry uint32) int {
	when, ok := file.FlatParent(entry)
	if !ok {
		return 0
	}
	var subject uint32
	if subj, ok := file.FlatFindChild(when, "when_subject"); ok {
		subject = file.FlatNamedChild(subj, 0)
		if subject == 0 || !versionCheckIsSdkInt(file, trimSkipParens(file, subject)) {
			return 0
		}
	}
	conditionLevel := func(c uint32, positive bool) int {
		expr := file.FlatNamedChild(c, 0)
		if subject == 0 {
			return sdkConditionLevel(file, expr, positive)
		}
		if !positive {
			return 0
		}
		if file.FlatType(expr) == "range_test" {
			if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(expr)), "!") {
				return 0
			}
			if rng, ok := file.FlatFindChild(expr, "range_expression"); ok {
				if v, ok := sdkLevelValue(file, file.FlatNamedChild(rng, 0)); ok {
					return v
				}
			}
			return 0
		}
		if v, ok := sdkLevelValue(file, expr); ok {
			return v
		}
		return 0
	}
	isElse := false
	for c := file.FlatFirstChild(entry); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "else" {
			isElse = true
		}
	}
	best := 0
	if !isElse {
		for c := file.FlatFirstChild(entry); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "when_condition" {
				best = max(best, conditionLevel(c, true))
			}
		}
		return best
	}
	for e := file.FlatFirstChild(when); e != 0 && e != entry; e = file.FlatNextSib(e) {
		if file.FlatType(e) != "when_entry" {
			continue
		}
		for c := file.FlatFirstChild(e); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "when_condition" {
				best = max(best, conditionLevel(c, false))
			}
		}
	}
	return best
}

// sdkIfParts returns the condition, then branch and else branch nodes of a Kotlin or Java if.
func sdkIfParts(file *scanner.File, n uint32) (cond, then, els uint32) {
	if file.Language == scanner.LangJava {
		// An if_statement's named children are the condition, the consequence and the alternative, whatever kind of
		// statement each branch is (a throw, a loop, a block).
		parts := sdkNamedParts(file, n)
		if len(parts) < 2 {
			return 0, 0, 0
		}
		cond = parts[0]
		if file.FlatType(cond) == "parenthesized_expression" {
			cond = file.FlatNamedChild(cond, 0)
		}
		then = parts[1]
		if len(parts) > 2 {
			els = parts[2]
		}
		return trimSkipParens(file, cond), then, els
	}
	var bodies []uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		switch t := file.FlatType(c); {
		case t == "control_structure_body" || t == "if_expression":
			bodies = append(bodies, c)
		case file.FlatIsNamed(c) && cond == 0:
			cond = c
		}
	}
	if len(bodies) > 0 {
		then = bodies[0]
	}
	if len(bodies) > 1 {
		els = bodies[1]
	}
	return trimSkipParens(file, cond), then, els
}

// sdkNamedParts returns the named children of a node, comments left out.
func sdkNamedParts(file *scanner.File, n uint32) []uint32 {
	var parts []uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); file.FlatIsNamed(c) && t != "line_comment" && t != "block_comment" && t != "multiline_comment" {
			parts = append(parts, c)
		}
	}
	return parts
}

// sdkConditionLevel evaluates a condition for the branch it guards: positive for the then branch, negated for else.
func sdkConditionLevel(file *scanner.File, cond uint32, positive bool) int {
	return sdkConditionLevelDepth(file, cond, positive, 0)
}

func sdkConditionLevelDepth(file *scanner.File, cond uint32, positive bool, depth int) int {
	if cond == 0 || depth > 4 {
		return 0
	}
	cond = trimSkipParens(file, cond)
	if lvl, ok := sdkIndirectCheck(file, cond, positive, depth); ok {
		return lvl
	}
	switch file.FlatType(cond) {
	case "prefix_expression", "unary_expression":
		if op := file.FlatFirstChild(cond); op != 0 && file.FlatNodeText(op) == "!" {
			return sdkConditionLevel(file, file.FlatNamedChild(cond, 0), !positive)
		}
		return 0
	case "disjunction_expression":
		// The else branch of `a || b` runs when neither holds, so a failing comparison in either proves a level.
		if positive {
			return 0
		}
		best := 0
		for c := file.FlatFirstChild(cond); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				best = max(best, sdkConditionLevel(file, c, false))
			}
		}
		return best
	case "conjunction_expression":
		if !positive {
			return 0
		}
		best := 0
		for c := file.FlatFirstChild(cond); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				if lvl := sdkConditionLevel(file, c, true); lvl > best {
					best = lvl
				}
			}
		}
		return best
	}
	op, left, right, ok := versionCheckComparison(file, cond)
	if !ok {
		return 0
	}
	if op == "&&" && positive {
		return max(sdkConditionLevel(file, left, true), sdkConditionLevel(file, right, true))
	}
	if op == "||" && !positive {
		return max(sdkConditionLevel(file, left, false), sdkConditionLevel(file, right, false))
	}
	if !versionCheckIsSdkInt(file, left) {
		// `N <= SDK_INT` is `SDK_INT >= N`.
		flipped, known := map[string]string{"<=": ">=", "<": ">", ">=": "<=", ">": "<"}[op]
		if !known || !versionCheckIsSdkInt(file, right) {
			return 0
		}
		op, right = flipped, left
	}
	v, ok := sdkLevelValue(file, right)
	if !ok {
		return 0
	}
	switch {
	case positive && op == ">=", !positive && op == "<":
		return v
	case positive && op == ">", !positive && op == "<=":
		return v + 1
	}
	return 0
}

// sdkIndirectCheck evaluates a condition that holds a check elsewhere: a Boolean local initialized with one, or a call
// of a same-file function annotated @ChecksSdkIntAtLeast(api = N) or whose body is a single check.
func sdkIndirectCheck(file *scanner.File, cond uint32, positive bool, depth int) (int, bool) {
	switch file.FlatType(cond) {
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(cond)
		if init := lintInitializerOf(file, cond, name, false); init != 0 && init != cond {
			return sdkConditionLevelDepth(file, init, positive, depth+1), true
		}
		return 0, true
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, cond)
		if !ok || len(call.Args) != 0 || (call.ReceiverNode != 0 && strings.TrimSpace(call.ReceiverText) != "this") {
			return 0, false
		}
		fn := sdkSameFileCheckFunction(file, call.Name)
		if fn == 0 {
			return 0, false
		}
		if ann, ok := annotationNamed(file, fn, checksSdkIntAtLeastFQN); ok {
			if api, ok := versionCheckAnnotationAPI(file, ann); ok && positive {
				return api, true
			}
			return 0, true
		}
		if expr := sdkSingleExpressionBody(file, fn); expr != 0 {
			return sdkConditionLevelDepth(file, expr, positive, depth+1), true
		}
		return 0, true
	}
	return 0, false
}

// sdkSameFileCheckFunction returns the parameterless function of the file with that name, or 0.
func sdkSameFileCheckFunction(file *scanner.File, name string) uint32 {
	var found uint32
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(0, kind, func(fn uint32) {
			if found == 0 && recyclerName(file, fn) == name && len(recyclerParams(file, fn)) == 0 {
				found = fn
			}
		})
	}
	return found
}

// sdkSingleExpressionBody returns the expression a function's body consists of: a Kotlin expression body, or a body
// that is a single return.
func sdkSingleExpressionBody(file *scanner.File, fn uint32) uint32 {
	body := recyclerFunctionBody(file, fn)
	if body == 0 {
		return 0
	}
	if file.FlatType(body) == "function_body" && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(body)), "=") {
		return file.FlatNamedChild(body, 0)
	}
	block := body
	if b, ok := file.FlatFindChild(body, "block"); ok {
		block = b
	}
	block = versionCheckStatements(file, block)
	if versionCheckStatementCount(file, block) != 1 {
		return 0
	}
	stmt := versionCheckFirstStatement(file, block)
	switch file.FlatType(stmt) {
	case "return_statement", "jump_expression":
		if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(stmt)), "return") {
			return file.FlatNamedChild(stmt, 0)
		}
	}
	return 0
}

// sdkLevelValue evaluates an API level operand: a literal, a Build.VERSION_CODES constant or a same-file constant.
func sdkLevelValue(file *scanner.File, n uint32) (int, bool) {
	if v, ok := lintEvalInt(file, n, sdkVersionCodes); ok {
		return int(v), true
	}
	text := strings.TrimSpace(file.FlatNodeText(n))
	if i := strings.LastIndex(text, "VERSION_CODES."); i >= 0 {
		if lvl, ok := versionCodeLevels[text[i+len("VERSION_CODES."):]]; ok {
			return lvl, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// WebViewApiAvailability
// ---------------------------------------------------------------------------

const webViewFQN = "android.webkit.WebView"

// webViewCompatMethods are the android.webkit.WebView methods added in API 22-28 that Lint points at WebViewCompat
// for (from the SDK's api-versions.xml, without Lint's blocked list), with their arity, level and whether they are
// static.
var webViewCompatMethods = map[string]struct {
	arity, level int
	static       bool
}{
	"createWebMessageChannel":         {0, 23, false},
	"getCurrentWebViewPackage":        {0, 26, true},
	"getSafeBrowsingPrivacyPolicyUrl": {0, 27, true},
	"getWebChromeClient":              {0, 26, false},
	"getWebViewClient":                {0, 26, false},
	"postVisualStateCallback":         {2, 23, false},
	"postWebMessage":                  {2, 23, false},
	"setSafeBrowsingWhitelist":        {2, 27, true},
	"startSafeBrowsing":               {2, 27, true},
}

// lintCalleeName returns a call's method name without building the full call view.
func lintCalleeName(file *scanner.File, idx uint32) string {
	if file.FlatType(idx) == "method_invocation" {
		return javaMethodInvocationName(file, idx)
	}
	return flatCallExpressionName(file, idx)
}

// webViewTrailingLambdas counts the lambdas written after a Kotlin call's parentheses, which are arguments too:
// WebView.startSafeBrowsing(context) { ok -> }.
func webViewTrailingLambdas(file *scanner.File, call uint32) int {
	n := 0
	count := func(call uint32) {
		suffix, ok := file.FlatFindChild(call, "call_suffix")
		if !ok {
			return
		}
		for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "annotated_lambda" {
				n++
			}
		}
	}
	count(call)
	// `f(x) { }` parses as a call whose callee is the call `f(x)`, holding the lambda.
	if p, ok := file.FlatParent(call); ok && file.FlatType(p) == "call_expression" && file.FlatFirstChild(p) == call {
		count(p)
	}
	return n
}

func (r *WebViewAPIAvailabilityRule) fileMentions() []string { return []string{"WebView"} }

func (r *EnqueueWorkRule) fileMentions() []string { return []string{"Work"} }

func (r *WebViewAPIAvailabilityRule) check(ctx *api.Context) {
	file := ctx.File
	if _, known := webViewCompatMethods[lintCalleeName(file, ctx.Idx)]; !known {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	m, known := webViewCompatMethods[call.Name]
	if !known || len(call.Args)+webViewTrailingLambdas(file, ctx.Idx) != m.arity || !webViewReceiver(ctx, call, m.static, m.arity) {
		return
	}
	if sdkGuardLevel(file, ctx.Idx) < m.level {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Consider using `WebViewCompat."+call.Name+"` instead which will support more devices."))
}

func webViewReceiver(ctx *api.Context, call lintCall, static bool, arity int) bool {
	file := ctx.File
	if static {
		return call.ReceiverNode != 0 && (strings.TrimSpace(call.ReceiverText) == webViewFQN ||
			(strings.TrimSpace(call.ReceiverText) == "WebView" && lintTypeRefMatches(file, "WebView", webViewFQN)))
	}
	if call.ReceiverNode == 0 || strings.TrimSpace(call.ReceiverText) == "this" {
		// `with(webView) { getWebChromeClient() }` and `webView.run { ... }` call on the scope's receiver.
		if recv := webViewScopeReceiver(file, call.Idx); recv != 0 {
			name := strings.TrimSpace(file.FlatNodeText(recv))
			return lintIdentOnly(name) && !webViewNameShadowed(file, call.Idx) &&
				lintTypeRefMatches(file, lintDeclaredTypeText(file, recv, name), webViewFQN)
		}
		cls := recyclerEnclosingClass(file, call.Idx)
		return cls != 0 && lintHasSupertype(file, cls, []string{webViewFQN}, false, 0) && !webViewSubclassDeclares(file, cls, call.Name, arity)
	}
	if lintReceiverTypeText(file, call) == "WebView" && webViewNameShadowed(file, call.Idx) {
		return false
	}
	if lintReceiverIsType(ctx, call, webViewFQN) {
		return true
	}
	decl := lintSameFileClass(file, lintReceiverTypeText(file, call))
	return decl != 0 && lintHasSupertype(file, decl, []string{webViewFQN}, false, 0) && !webViewSubclassDeclares(file, decl, call.Name, arity)
}

// webViewScopeReceiver returns the receiver of the scope lambda an unqualified call runs in: the argument of
// `with(x) { }`, or x in `x.run { }` / `x.apply { }`; 0 when there is none.
func webViewScopeReceiver(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			annotated, ok := file.FlatParent(p)
			if !ok || file.FlatType(annotated) != "annotated_lambda" {
				return 0
			}
			suffix, _ := file.FlatParent(annotated)
			owner, _ := file.FlatParent(suffix)
			// `with(x) { }` parses as a call whose callee is the call `with(x)`, holding the lambda.
			if inner := file.FlatFirstChild(owner); file.FlatType(inner) == "call_expression" {
				owner = inner
			}
			call, ok := lintCallAt(file, owner)
			if !ok {
				return 0
			}
			switch {
			case call.Name == "with" && len(call.Args) >= 1:
				return call.Args[0]
			case (call.Name == "run" || call.Name == "apply") && call.ReceiverNode != 0:
				return call.ReceiverNode
			}
			return 0
		case "function_declaration", "class_body", "source_file":
			return 0
		}
	}
	return 0
}

// webViewNameShadowed reports a class of the file named WebView whose scope holds idx, such as one nested in the
// enclosing object: there `WebView` is that class, not android.webkit.WebView.
func webViewNameShadowed(file *scanner.File, idx uint32) bool {
	decl := lintSameFileClass(file, "WebView")
	if decl == 0 {
		return false
	}
	scope, ok := file.FlatParent(decl)
	return ok && file.FlatStartByte(idx) >= file.FlatStartByte(scope) && file.FlatEndByte(idx) <= file.FlatEndByte(scope)
}

// webViewSubclassDeclares reports a method of that name and arity declared by the WebView subclass or a same-file
// class between it and WebView: the call resolves to it, an override or an overload, not to android.webkit.WebView's.
func webViewSubclassDeclares(file *scanner.File, cls uint32, name string, arity int) bool {
	seen := map[uint32]bool{cls: true}
	for level, depth := []uint32{cls}, 0; len(level) > 0 && depth < 8; depth++ {
		var next []uint32
		for _, c := range level {
			if body, ok := file.FlatFindChild(c, "class_body"); ok {
				for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
					if t := file.FlatType(m); (t == "function_declaration" || t == "method_declaration") &&
						recyclerName(file, m) == name && len(recyclerParams(file, m)) == arity {
						return true
					}
				}
			}
			for _, ref := range lintDirectSupertypeRefs(file, c) {
				if d := lintSameFileClass(file, ref.simple); d != 0 && !seen[d] {
					seen[d] = true
					next = append(next, d)
				}
			}
		}
		level = next
	}
	return false
}

// ---------------------------------------------------------------------------
// EnqueueWork
// ---------------------------------------------------------------------------

const (
	workManagerFQN      = "androidx.work.WorkManager"
	workContinuationFQN = "androidx.work.WorkContinuation"
)

var workEnqueueingCalls = map[string]bool{"enqueue": true, "enqueueSync": true, "enqueueUniquePeriodicWork": true, "then": true, "combine": true}

func (r *EnqueueWorkRule) check(ctx *api.Context) {
	file := ctx.File
	switch lintCalleeName(file, ctx.Idx) {
	case "beginWith", "beginUniqueWork", "then", "combine":
	default:
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	switch call.Name {
	case "beginWith", "beginUniqueWork":
		if !workIsManager(file, call.ReceiverNode) {
			return
		}
	case "then":
		if !workIsContinuation(file, call.ReceiverNode, 0) {
			return
		}
	case "combine":
		if !workIsContinuationClass(file, call.ReceiverText) {
			return
		}
	default:
		return
	}
	body := workEnclosingBody(file, ctx.Idx)
	if body == 0 {
		return
	}
	if t := file.FlatType(body); (t == "function_declaration" || t == "method_declaration") &&
		lintTypeRefMatches(file, workDeclaredReturnType(file, body), workContinuationFQN) {
		return
	}
	if workReachesEnqueue(file, body, ctx.Idx) {
		return
	}
	name := ""
	if v := workAssignedLocal(file, ctx.Idx); v != "" {
		name = "`" + v + "` "
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "WorkContinuation "+name+"not enqueued: did you forget to call `enqueue()`?"))
}

// workEnclosingBody returns the function, constructor or initializer block whose code holds the call, as Lint checks
// each method body: Kotlin init blocks and Java constructors count.
func workEnclosingBody(file *scanner.File, idx uint32) uint32 {
	for n, ok := file.FlatParent(idx); ok; n, ok = file.FlatParent(n) {
		switch file.FlatType(n) {
		case "function_declaration", "method_declaration", "constructor_declaration", "secondary_constructor",
			"anonymous_initializer":
			// Lint checks code in a method: a Java static initializer is not one.
			return n
		case "class_body", "source_file":
			return 0
		}
	}
	return 0
}

// workIsManager reports a WorkManager receiver: WorkManager.getInstance(...) or a variable declared or initialized as
// one. The class name must resolve to androidx.work.WorkManager, not a same-named class of the project.
func workIsManager(file *scanner.File, recv uint32) bool {
	if recv == 0 {
		return false
	}
	recv = trimSkipParens(file, recv)
	if workIsGetInstance(file, recv) {
		return true
	}
	text := strings.TrimSpace(file.FlatNodeText(recv))
	if lintIdentOnly(text) {
		if lintTypeRefMatches(file, lintDeclaredTypeText(file, recv, text), workManagerFQN) {
			return true
		}
		init := lintInitializerOf(file, recv, text, true)
		return init != 0 && workIsGetInstance(file, trimSkipParens(file, init))
	}
	return false
}

// workIsGetInstance reports a WorkManager.getInstance(...) call on androidx.work.WorkManager.
func workIsGetInstance(file *scanner.File, expr uint32) bool {
	call, ok := lintCallAt(file, expr)
	if !ok || call.Name != "getInstance" || call.ReceiverNode == 0 {
		return false
	}
	recv := strings.Join(strings.Fields(call.ReceiverText), "")
	return recv == workManagerFQN || (recv == "WorkManager" && lintTypeRefMatches(file, "WorkManager", workManagerFQN))
}

// workIsContinuationClass reports the WorkContinuation class itself, as the receiver of its static combine.
func workIsContinuationClass(file *scanner.File, text string) bool {
	text = strings.Join(strings.Fields(text), "")
	return text == workContinuationFQN || (text == "WorkContinuation" && lintTypeRefMatches(file, "WorkContinuation", workContinuationFQN))
}

// workIsContinuation reports a WorkContinuation receiver: a beginWith/beginUniqueWork call on a WorkManager, a then
// call on a continuation, WorkContinuation.combine, or a variable declared as one or initialized with one. depth stops
// a variable whose initializer reads itself.
func workIsContinuation(file *scanner.File, recv uint32, depth int) bool {
	if recv == 0 || depth > 8 {
		return false
	}
	recv = trimSkipParens(file, recv)
	if inner, ok := lintCallAt(file, recv); ok {
		switch inner.Name {
		case "beginWith", "beginUniqueWork":
			return workIsManager(file, inner.ReceiverNode)
		case "then":
			return workIsContinuation(file, inner.ReceiverNode, depth+1)
		case "combine":
			return workIsContinuationClass(file, inner.ReceiverText)
		}
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(recv))
	if !lintIdentOnly(text) {
		return false
	}
	if lintTypeRefMatches(file, lintDeclaredTypeText(file, recv, text), workContinuationFQN) {
		return true
	}
	init := lintInitializerOf(file, recv, text, true)
	return init != 0 && init != recv && workIsContinuation(file, init, depth+1)
}

// workLocalInitializedBy reports a local of the enclosing function named name whose initializer text satisfies ok.
func workLocalInitializedBy(file *scanner.File, at uint32, name string, ok func(string) bool) bool {
	init := lintInitializerOf(file, at, name, true)
	return init != 0 && ok(strings.Join(strings.Fields(file.FlatNodeText(init)), ""))
}

func workDeclaredReturnType(file *scanner.File, fn uint32) string {
	if file.Language == scanner.LangJava {
		for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				return lintTrimGenerics(file.FlatNodeText(c))
			case "identifier", "formal_parameters":
				return ""
			}
		}
		return ""
	}
	seenParams := false
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "function_value_parameters":
			seenParams = true
		case "user_type", "nullable_type":
			if seenParams {
				return strings.TrimSuffix(lintTrimGenerics(file.FlatNodeText(c)), "?")
			}
		case "function_body":
			// An expression body without a declared type: Lint takes the inferred type.
			if expr := file.FlatNamedChild(c, 0); expr != 0 && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(c)), "=") {
				if inner, ok := lintCallAt(file, expr); ok && (inner.Name == "beginWith" || inner.Name == "beginUniqueWork" ||
					inner.Name == "then" || inner.Name == "combine") {
					return workContinuationFQN
				}
			}
		}
	}
	return ""
}

// workAssignedLocal returns the local variable the call initializes, as Lint names it in the message.
func workAssignedLocal(file *scanner.File, call uint32) string {
	p, ok := file.FlatParent(call)
	for ok && file.FlatType(p) == "parenthesized_expression" {
		p, ok = file.FlatParent(p)
	}
	if !ok {
		return ""
	}
	switch file.FlatType(p) {
	case "property_declaration":
		if vd, found := file.FlatFindChild(p, "variable_declaration"); found {
			return lintDeclaredName(file, vd)
		}
	case "variable_declarator":
		if id, found := file.FlatFindChild(p, "identifier"); found {
			return file.FlatNodeText(id)
		}
	}
	// An assignment, to a field or to a local declared earlier, names no variable: Lint names only a local's
	// initializer.
	return ""
}

// workReachesEnqueue follows the continuation as Lint's DataFlowAnalyzer does (workFlow): it is enqueued when the
// value, or a local holding it, reaches enqueue/enqueueSync/enqueueUniquePeriodicWork/then/combine, an argument of
// combine, or a return. Like Lint, a value passed to any other call (listOf(a, b) included) is not followed.
func workReachesEnqueue(file *scanner.File, body, call uint32) bool {
	f := &workFlow{file: file, body: body, tracked: map[uint32]bool{}}
	return f.escapes(call)
}

// workFlow follows one continuation value through the code of a function body.
type workFlow struct {
	file    *scanner.File
	body    uint32
	tracked map[uint32]bool // local declarations already followed
	depth   int
}

// workScopeFunctions hand their receiver to a lambda, as `it` or as `this`; those in workReturnsReceiver also return it.
var (
	workScopeFunctions  = map[string]bool{"also": true, "let": true, "apply": true, "run": true, "takeIf": true, "takeUnless": true}
	workReturnsReceiver = map[string]bool{"also": true, "apply": true, "takeIf": true, "takeUnless": true}
)

// escapes reports whether the value of expr reaches an enqueueing use.
func (f *workFlow) escapes(expr uint32) bool {
	if f.depth > 64 {
		return false
	}
	f.depth++
	defer func() { f.depth-- }()
	file := f.file
	parent, ok := file.FlatParent(expr)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "parenthesized_expression":
		return f.escapes(parent)
	case "navigation_expression":
		// The receiver of a Kotlin call, `c.enqueue()` or `c?.enqueue()`.
		if file.FlatFirstChild(parent) != expr {
			return false
		}
		return f.receiverOf(parentOrZero(file, parent))
	case "method_invocation":
		if file.FlatNamedChild(parent, 0) != expr {
			return false
		}
		return f.receiverOf(parent)
	case "value_argument", "argument_list":
		return f.argumentOf(parent)
	case "elvis_expression", "ternary_expression":
		// `other ?: c`, `x ? c : d`: the value flows out of the expression (the condition does not).
		if file.FlatType(parent) == "ternary_expression" && file.FlatNamedChild(parent, 0) == expr {
			return false
		}
		return f.escapes(parent)
	case "control_structure_body", "statements":
		// The last expression of an if or when branch is the branch's value.
		if workLastNamed(file, parent) != expr {
			return false
		}
		return f.branchValue(parent)
	case "property_declaration":
		if vd, ok := file.FlatFindChild(parent, "variable_declaration"); ok {
			return f.local(parent, lintDeclaredName(file, vd))
		}
	case "variable_declarator":
		if id, ok := file.FlatFindChild(parent, "identifier"); ok {
			return f.local(parent, file.FlatNodeText(id))
		}
	case "assignment", "assignment_expression":
		// `c = wm.beginWith(a)` for a local declared earlier: follow the local. A field is not followed.
		target := file.FlatNamedChild(parent, 0)
		if target == expr || target == 0 {
			return false
		}
		name := strings.TrimSpace(file.FlatNodeText(target))
		if decl := f.declarationOf(target, name); decl != 0 {
			return f.local(decl, name)
		}
	case "jump_expression", "return_statement", "function_body":
		// Returned: the caller gets the continuation.
		return true
	}
	return false
}

// branchValue follows the value of an if or when branch to the if or when expression.
func (f *workFlow) branchValue(block uint32) bool {
	file := f.file
	for n, ok := file.FlatParent(block); ok; n, ok = file.FlatParent(n) {
		switch file.FlatType(n) {
		case "control_structure_body", "statements":
			continue
		case "if_expression":
			return f.escapes(n)
		case "when_entry":
			if w, ok := file.FlatParent(n); ok && file.FlatType(w) == "when_expression" {
				return f.escapes(w)
			}
		}
		return false
	}
	return false
}

// receiverOf reports whether the call the continuation is the receiver of enqueues it.
func (f *workFlow) receiverOf(call uint32) bool {
	inner, ok := lintCallAt(f.file, call)
	if !ok {
		return false
	}
	switch {
	case workEnqueueingCalls[inner.Name]:
		return true
	case inner.Name == "synchronous":
		return f.escapes(call)
	case workScopeFunctions[inner.Name]:
		if workScopeEnqueues(f.file, call, inner.Name) {
			return true
		}
		return workReturnsReceiver[inner.Name] && f.escapes(call)
	}
	return false
}

// argumentOf reports whether the call the continuation is an argument of enqueues it: combine does, and so does
// with(c) { enqueue() }. Other calls are not followed.
func (f *workFlow) argumentOf(arg uint32) bool {
	owner := arg
	for owner != 0 && f.file.FlatType(owner) != "call_expression" && f.file.FlatType(owner) != "method_invocation" {
		owner = parentOrZero(f.file, owner)
	}
	inner, ok := lintCallAt(f.file, owner)
	if !ok {
		return false
	}
	switch inner.Name {
	case "combine":
		return true
	case "with":
		return workScopeEnqueues(f.file, owner, "with")
	}
	return false
}

// local follows the uses of a local variable declared by decl, those that resolve to it.
func (f *workFlow) local(decl uint32, name string) bool {
	if name == "" || f.tracked[decl] {
		return false
	}
	f.tracked[decl] = true
	file := f.file
	found := false
	file.FlatWalkAllNodes(f.body, func(n uint32) {
		if found {
			return
		}
		if t := file.FlatType(n); (t != "simple_identifier" && t != "identifier") || file.FlatNodeText(n) != name {
			return
		}
		if file.FlatStartByte(n) < file.FlatEndByte(decl) || f.declarationOf(n, name) != decl {
			return
		}
		found = f.escapes(n)
	})
	return found
}

// declarationOf returns the local declaration a use of name resolves to: the nearest one declared before it in a block
// that encloses it, unless a lambda or function parameter of that name is nearer. 0 for a parameter, field or none.
func (f *workFlow) declarationOf(use uint32, name string) uint32 {
	file := f.file
	binder := lintNameBinderAt(file, use, name)
	var best uint32
	consider := func(decl uint32) {
		if file.FlatStartByte(decl) >= file.FlatStartByte(use) {
			return
		}
		scope, ok := file.FlatParent(decl)
		if ok && file.FlatType(scope) == "local_variable_declaration" {
			decl = scope
			scope, ok = file.FlatParent(scope)
		}
		if !ok || file.FlatStartByte(use) < file.FlatStartByte(scope) || file.FlatEndByte(use) > file.FlatEndByte(scope) {
			return
		}
		if binder != 0 && file.FlatStartByte(decl) < file.FlatStartByte(binder) {
			// A parameter declared between this local and the use hides it.
			return
		}
		if best == 0 || file.FlatStartByte(decl) > file.FlatStartByte(best) {
			best = decl
		}
	}
	file.FlatWalkNodes(f.body, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
			consider(decl)
		}
	})
	file.FlatWalkNodes(f.body, "variable_declarator", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
			if p, ok := file.FlatParent(decl); ok && file.FlatType(p) == "local_variable_declaration" {
				consider(decl)
			}
		}
	})
	if best != 0 && file.FlatType(best) == "local_variable_declaration" {
		// The declarator, as escapes reaches it from its initializer.
		for c := file.FlatFirstChild(best); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "variable_declarator" {
				if id, ok := file.FlatFindChild(c, "identifier"); ok && file.FlatNodeText(id) == name {
					return c
				}
			}
		}
	}
	return best
}

// workLastNamed returns the last named child of n that is not a comment.
func workLastNamed(file *scanner.File, n uint32) uint32 {
	var last uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); file.FlatIsNamed(c) && t != "line_comment" && t != "multiline_comment" && t != "block_comment" {
			last = c
		}
	}
	return last
}

// workScopeEnqueues reports a scope-function lambda that enqueues the continuation it receives: `.also { it.enqueue() }`,
// `.apply { enqueue() }`, `with(c) { enqueue() }`.
func workScopeEnqueues(file *scanner.File, call uint32, scope string) bool {
	lambda := workTrailingLambda(file, call)
	if lambda == 0 {
		return false
	}
	param := "it"
	if params, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
		if vd, ok := file.FlatFindChild(params, "variable_declaration"); ok {
			param = lintDeclaredName(file, vd)
		}
	}
	found := false
	file.FlatWalkNodes(lambda, "call_expression", func(n uint32) {
		if found {
			return
		}
		c, ok := lintCallAt(file, n)
		if !ok || !workEnqueueingCalls[c.Name] {
			return
		}
		recv := strings.TrimSpace(c.ReceiverText)
		switch scope {
		case "also", "let", "takeIf", "takeUnless":
			found = (recv == param || strings.HasPrefix(recv, param+".")) && !workRebound(file, n, lambda, param)
		default:
			found = recv == "" || recv == "this" || strings.HasPrefix(recv, "this.")
		}
	})
	return found
}

// workRebound reports a lambda between the call and the scope lambda that declares param again, or, for `it`, any
// lambda without parameters of its own, whose implicit `it` is another value: `.also { others.forEach { it.enqueue() } }`.
func workRebound(file *scanner.File, call, scopeLambda uint32, param string) bool {
	for n, ok := file.FlatParent(call); ok && n != scopeLambda; n, ok = file.FlatParent(n) {
		switch file.FlatType(n) {
		case "lambda_literal":
			params, has := file.FlatFindChild(n, "lambda_parameters")
			if !has {
				if param == "it" {
					return true
				}
				continue
			}
			if lintParamsDeclare(file, params, param) {
				return true
			}
		case "function_declaration", "anonymous_function", "class_declaration", "object_literal":
			return true
		}
	}
	return false
}

// workTrailingLambda returns the lambda passed to a call after its parentheses, or 0.
func workTrailingLambda(file *scanner.File, call uint32) uint32 {
	for _, n := range []uint32{call, parentOrZero(file, call)} {
		if n == 0 || file.FlatType(n) != "call_expression" {
			continue
		}
		if suffix, ok := file.FlatFindChild(n, "call_suffix"); ok {
			if annotated, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
				if l, ok := file.FlatFindChild(annotated, "lambda_literal"); ok {
					return l
				}
			}
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// ReportShortcutUsage
// ---------------------------------------------------------------------------

const (
	shortcutManagerFQN       = "android.content.pm.ShortcutManager"
	shortcutManagerCompatFQN = "androidx.core.content.pm.ShortcutManagerCompat"
)

var shortcutBytes = []byte("Shortcut")

func (r *ReportShortcutUsageRule) check(ctx *api.Context) {
	type site struct {
		file *scanner.File
		call uint32
	}
	// Lint checks each module on its own (checkDependencies is off by default) and skips a library module: an
	// application module's set calls are reported when that module reports no usage. A file outside any module is
	// one module of its own.
	type module struct {
		sets     []site
		reported bool
	}
	modules := map[string]*module{}
	roots := map[string]string{}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || isProjectTestFile(ctx, file.Path) || !bytes.Contains(file.Content, shortcutBytes) {
			continue
		}
		var fileSets []site
		fileReported := false
		for _, t := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, t, func(n uint32) {
				call, ok := lintCallAt(file, n)
				if !ok {
					return
				}
				switch call.Name {
				case "pushDynamicShortcut", "reportShortcutUsed":
					// These names are distinctive: count a usage report whatever its receiver, so an unproven one
					// (getSystemService(...).reportShortcutUsed(id)) never turns into a false finding.
					fileReported = true
				case "setDynamicShortcuts", "addDynamicShortcuts":
					if shortcutReceiver(file, call) {
						fileSets = append(fileSets, site{file, n})
					}
				}
			})
		}
		if len(fileSets) == 0 && !fileReported {
			continue
		}
		root := workModuleRoot(file.Path, roots)
		m := modules[root]
		if m == nil {
			m = &module{}
			modules[root] = m
		}
		m.sets = append(m.sets, fileSets...)
		m.reported = m.reported || fileReported
	}
	keys := make([]string, 0, len(modules))
	for root := range modules {
		keys = append(keys, root)
	}
	sort.Strings(keys)
	var sets []site
	for _, root := range keys {
		m := modules[root]
		if m.reported || len(m.sets) == 0 || (root != "" && readModuleIsLibrary(root)) {
			continue
		}
		sets = append(sets, m.sets...)
	}
	for _, s := range sets {
		f := s.file
		ctx.Emit(scanner.Finding{File: f.Path, Line: f.FlatRow(s.call) + 1, Col: f.FlatCol(s.call) + 1,
			StartByte: int(f.FlatStartByte(s.call)), EndByte: int(f.FlatEndByte(s.call)),
			Message: "Calling this method indicates use of dynamic shortcuts, but there are no calls to methods that track " +
				"shortcut usage, such as `pushDynamicShortcut` or `reportShortcutUsed`. Calling these methods is recommended, " +
				"as they track shortcut usage and allow launchers to adjust which shortcuts appear based on activation history. " +
				"Please see https://developer.android.com/develop/ui/views/launch/shortcuts/managing-shortcuts#track-usage"})
	}
}

// shortcutReceiver reports ShortcutManagerCompat's static calls and calls on a proven ShortcutManager
// (shortcutManagerExpr).
func shortcutReceiver(file *scanner.File, call lintCall) bool {
	recv := strings.TrimSpace(call.ReceiverText)
	if recv == "ShortcutManagerCompat" || recv == shortcutManagerCompatFQN {
		return recv == shortcutManagerCompatFQN || lintTypeRefMatches(file, recv, shortcutManagerCompatFQN)
	}
	return call.ReceiverNode != 0 && shortcutManagerExpr(file, call.ReceiverNode, 0)
}

// shortcutManagerExpr reports an expression that is a ShortcutManager: getSystemService(ShortcutManager::class.java)
// or (ShortcutManager.class), getSystemService<ShortcutManager>(), a cast to ShortcutManager, `!!` of one, or a variable
// declared as one, initialized with one or delegated to `lazy { one }`. A call that merely mentions the words is not.
func shortcutManagerExpr(file *scanner.File, expr uint32, depth int) bool {
	if expr == 0 || depth > 6 {
		return false
	}
	expr = trimSkipParens(file, expr)
	isManager := func(typ string) bool {
		typ = strings.TrimSuffix(strings.Join(strings.Fields(typ), ""), "?")
		return typ == shortcutManagerFQN || (typ == "ShortcutManager" && lintTypeRefMatches(file, "ShortcutManager", shortcutManagerFQN))
	}
	switch file.FlatType(expr) {
	case "postfix_expression":
		if strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(expr)), "!!") {
			return shortcutManagerExpr(file, file.FlatNamedChild(expr, 0), depth+1)
		}
		return false
	case "as_expression":
		return isManager(file.FlatNodeText(file.FlatNamedChild(expr, file.FlatNamedChildCount(expr)-1)))
	case "cast_expression":
		return isManager(file.FlatNodeText(file.FlatNamedChild(expr, 0)))
	}
	if call, ok := lintCallAt(file, expr); ok {
		if call.Name != "getSystemService" {
			return false
		}
		// getSystemService<ShortcutManager>()
		text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
		if k := strings.Index(text, "getSystemService<"); k >= 0 {
			rest := text[k+len("getSystemService<"):]
			if end := strings.Index(rest, ">"); end >= 0 && isManager(rest[:end]) {
				return true
			}
		}
		if len(call.Args) != 1 {
			return false
		}
		arg := strings.Join(strings.Fields(file.FlatNodeText(call.Args[0])), "")
		for _, suffix := range []string{"::class.java", ".class"} {
			if strings.HasSuffix(arg, suffix) && isManager(strings.TrimSuffix(arg, suffix)) {
				return true
			}
		}
		return false
	}
	name := strings.TrimSpace(file.FlatNodeText(expr))
	if !lintIdentOnly(name) {
		return false
	}
	if isManager(lintDeclaredTypeText(file, expr, name)) {
		return true
	}
	if init := lintInitializerOf(file, expr, name, true); init != 0 && init != expr {
		return shortcutManagerExpr(file, init, depth+1)
	}
	return shortcutLazyManager(file, expr, name, depth)
}

// shortcutLazyManager reports a property named name, visible at use, delegated to `lazy { ... }` whose last expression is
// a ShortcutManager.
func shortcutLazyManager(file *scanner.File, use uint32, name string, depth int) bool {
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		if found {
			return
		}
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != name || !lintDeclVisibleAt(file, decl, use) {
			return
		}
		delegate, ok := file.FlatFindChild(decl, "property_delegate")
		if !ok {
			return
		}
		file.FlatWalkNodes(delegate, "lambda_literal", func(l uint32) {
			if found {
				return
			}
			if stmts, ok := file.FlatFindChild(l, "statements"); ok {
				found = shortcutManagerExpr(file, workLastNamed(file, stmts), depth+1)
			}
		})
	})
	return found
}

// parentOrZero returns the parent of n, or 0 at the root.
func parentOrZero(file *scanner.File, n uint32) uint32 {
	if p, ok := file.FlatParent(n); ok {
		return p
	}
	return 0
}
