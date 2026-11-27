package rules

import (
	"bytes"
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

// sdkGuardLevel returns the highest API level an enclosing if proves at idx: the then-branch of `SDK_INT >= N` (or
// `> N-1`), the else-branch of `SDK_INT < N` (or `<= N-1`), and && conditions holding such a comparison. It returns 0
// when no enclosing condition proves a level. Early returns do not count, matching Lint's
// isWithinVersionCheckConditional for these checks.
func sdkGuardLevel(file *scanner.File, idx uint32) int {
	best := 0
	for child := idx; ; {
		p, ok := file.FlatParent(child)
		if !ok {
			return best
		}
		switch file.FlatType(p) {
		case "if_expression", "if_statement":
			cond, then, els := sdkIfParts(file, p)
			if cond != 0 && (child == then || child == els) {
				if lvl := sdkConditionLevel(file, cond, child == then); lvl > best {
					best = lvl
				}
			}
		case "when_entry":
			// A subject-less `when { SDK_INT >= N -> ... }` entry runs its body when its condition holds.
			if file.FlatType(child) == "control_structure_body" {
				for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) == "when_condition" {
						if lvl := sdkConditionLevel(file, file.FlatNamedChild(c, 0), true); lvl > best {
							best = lvl
						}
					}
				}
			}
		case "function_declaration", "method_declaration", "lambda_literal", "lambda_expression", "class_body", "source_file":
			return best
		}
		child = p
	}
}

// sdkIfParts returns the condition, then branch and else branch nodes of a Kotlin or Java if.
func sdkIfParts(file *scanner.File, n uint32) (cond, then, els uint32) {
	var bodies []uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		switch t := file.FlatType(c); {
		case t == "parenthesized_expression" && cond == 0 && file.Language == scanner.LangJava:
			cond = file.FlatNamedChild(c, 0)
		case t == "control_structure_body" || t == "block" || t == "expression_statement" || t == "if_statement" ||
			t == "return_statement" || t == "if_expression":
			bodies = append(bodies, c)
		case file.FlatIsNamed(c) && cond == 0 && file.Language != scanner.LangJava:
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

// sdkConditionLevel evaluates a condition for the branch it guards: positive for the then branch, negated for else.
func sdkConditionLevel(file *scanner.File, cond uint32, positive bool) int {
	cond = trimSkipParens(file, cond)
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
		return 0
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
	if !known || len(call.Args)+webViewTrailingLambdas(file, ctx.Idx) != m.arity || !webViewReceiver(ctx, call, m.static) {
		return
	}
	if sdkGuardLevel(file, ctx.Idx) < m.level {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Consider using `WebViewCompat."+call.Name+"` instead which will support more devices."))
}

func webViewReceiver(ctx *api.Context, call lintCall, static bool) bool {
	file := ctx.File
	if static {
		return call.ReceiverNode != 0 && (strings.TrimSpace(call.ReceiverText) == webViewFQN ||
			(strings.TrimSpace(call.ReceiverText) == "WebView" && lintTypeRefMatches(file, "WebView", webViewFQN)))
	}
	if call.ReceiverNode == 0 || strings.TrimSpace(call.ReceiverText) == "this" {
		cls := recyclerEnclosingClass(file, call.Idx)
		return cls != 0 && lintHasSupertype(file, cls, []string{webViewFQN}, false, 0)
	}
	if lintReceiverIsType(ctx, call, webViewFQN) {
		return true
	}
	decl := lintSameFileClass(file, lintReceiverTypeText(file, call))
	return decl != 0 && lintHasSupertype(file, decl, []string{webViewFQN}, false, 0)
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
		if !workIsContinuation(file, call.ReceiverNode) {
			return
		}
	case "combine":
		if text := strings.TrimSpace(call.ReceiverText); text != "WorkContinuation" && text != workContinuationFQN {
			return
		}
	default:
		return
	}
	fn, ok := flatEnclosingCallable(file, ctx.Idx)
	if !ok || fn == 0 {
		return
	}
	fnDecl := fn
	for file.FlatType(fnDecl) != "function_declaration" && file.FlatType(fnDecl) != "method_declaration" {
		p, ok := file.FlatParent(fnDecl)
		if !ok {
			return
		}
		fnDecl = p
	}
	if lintTypeRefMatches(file, workDeclaredReturnType(file, fnDecl), workContinuationFQN) {
		return
	}
	if workReachesEnqueue(file, fnDecl, ctx.Idx) {
		return
	}
	name := ""
	if v := workAssignedLocal(file, ctx.Idx); v != "" {
		name = "`" + v + "` "
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "WorkContinuation "+name+"not enqueued: did you forget to call `enqueue()`?"))
}

// workIsManager reports a WorkManager receiver: WorkManager.getInstance(...) or a variable declared or initialized as
// one.
func workIsManager(file *scanner.File, recv uint32) bool {
	if recv == 0 {
		return false
	}
	recv = trimSkipParens(file, recv)
	text := strings.TrimSpace(file.FlatNodeText(recv))
	if strings.HasPrefix(text, "WorkManager.getInstance(") || strings.HasPrefix(text, workManagerFQN+".getInstance(") {
		return true
	}
	if lintIdentOnly(text) {
		if lintTypeRefMatches(file, lintDeclaredTypeText(file, recv, text), workManagerFQN) {
			return true
		}
		return workLocalInitializedBy(file, recv, text, func(init string) bool {
			return strings.HasPrefix(init, "WorkManager.getInstance(") || strings.HasPrefix(init, workManagerFQN+".getInstance(")
		})
	}
	return false
}

// workIsContinuation reports a WorkContinuation receiver: a chained beginWith/beginUniqueWork/then/combine call, or a
// variable declared or initialized as one.
func workIsContinuation(file *scanner.File, recv uint32) bool {
	if recv == 0 {
		return false
	}
	recv = trimSkipParens(file, recv)
	if inner, ok := lintCallAt(file, recv); ok {
		switch inner.Name {
		case "beginWith", "beginUniqueWork":
			return workIsManager(file, inner.ReceiverNode)
		case "then":
			return workIsContinuation(file, inner.ReceiverNode)
		case "combine":
			return strings.TrimSpace(inner.ReceiverText) == "WorkContinuation"
		}
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(recv))
	if !lintIdentOnly(text) {
		return false
	}
	return lintTypeRefMatches(file, lintDeclaredTypeText(file, recv, text), workContinuationFQN) ||
		workLocalInitializedBy(file, recv, text, func(init string) bool {
			return strings.Contains(init, ".beginWith(") || strings.Contains(init, ".beginUniqueWork(") || strings.Contains(init, ".then(")
		})
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
	case "assignment", "assignment_expression":
		// `c = wm.beginWith(a)` for a local declared earlier.
		if target := file.FlatNamedChild(p, 0); target != 0 && target != call {
			if name := strings.TrimSpace(file.FlatNodeText(target)); lintIdentOnly(name) {
				return name
			}
		}
	}
	return ""
}

// workReachesEnqueue follows the continuation as Lint's DataFlowAnalyzer does: it is enqueued when it (or a local it
// initializes, or a list a local is added to or built from) is the receiver of enqueue/enqueueSync/
// enqueueUniquePeriodicWork/then/combine, or an argument of combine.
func workReachesEnqueue(file *scanner.File, fn, call uint32) bool {
	if workValueEnqueued(file, call) {
		return true
	}
	tracked := map[string]bool{}
	if v := workAssignedLocal(file, call); v != "" {
		tracked[v] = true
	}
	if len(tracked) == 0 {
		return false
	}
	// Two passes so a list built after the continuation is tracked before its use.
	for pass := 0; pass < 2; pass++ {
		enqueued := false
		file.FlatWalkAllNodes(fn, func(n uint32) {
			if enqueued {
				return
			}
			t := file.FlatType(n)
			if t != "simple_identifier" && t != "identifier" {
				return
			}
			name := file.FlatNodeText(n)
			if !tracked[name] {
				return
			}
			use := n
			if workValueEnqueued(file, use) {
				enqueued = true
				return
			}
			// Passed to listOf(...) or list.add(...): track the list.
			if args, ok := file.FlatParent(use); ok {
				if t := file.FlatType(args); t == "value_argument" {
					args, _ = file.FlatParent(args)
				}
				if owner, ok := file.FlatParent(args); ok {
					if file.FlatType(owner) == "call_suffix" {
						owner, _ = file.FlatParent(owner)
					}
					if inner, ok := lintCallAt(file, owner); ok {
						if inner.ReceiverNode != 0 && lintIdentOnly(strings.TrimSpace(inner.ReceiverText)) {
							tracked[strings.TrimSpace(inner.ReceiverText)] = true
						} else if v := workAssignedLocal(file, owner); v != "" {
							tracked[v] = true
						}
					}
				}
			}
		})
		if enqueued {
			return true
		}
	}
	return false
}

// workValueEnqueued reports whether the expression is the receiver of an enqueueing call, through calls that return
// the same continuation (synchronous()), or an argument of combine.
func workValueEnqueued(file *scanner.File, expr uint32) bool {
	for depth := 0; depth < 8; depth++ {
		parent, ok := file.FlatParent(expr)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "parenthesized_expression":
			expr = parent
			continue
		case "value_argument", "argument_list", "value_arguments":
			owner := parent
			for owner != 0 && file.FlatType(owner) != "call_expression" && file.FlatType(owner) != "method_invocation" {
				owner = parentOrZero(file, owner)
			}
			if inner, ok := lintCallAt(file, owner); ok && inner.Name == "combine" {
				return true
			}
			return false
		}
		// The receiver of a call: Kotlin navigation, Java method_invocation object.
		var outer uint32
		switch file.FlatType(parent) {
		case "navigation_expression":
			if file.FlatFirstChild(parent) != expr {
				return false
			}
			outer = parentOrZero(file, parent)
		case "method_invocation":
			if file.FlatNamedChild(parent, 0) != expr {
				return false
			}
			outer = parent
		default:
			return false
		}
		inner, ok := lintCallAt(file, outer)
		if !ok {
			return false
		}
		if workEnqueueingCalls[inner.Name] {
			return true
		}
		if workScopeFunctions[inner.Name] {
			return workScopeEnqueues(file, outer, inner.Name)
		}
		if inner.Name != "synchronous" {
			return false
		}
		expr = outer
	}
	return false
}

// workScopeFunctions are the standard functions that hand their receiver to a lambda, as `it` or as `this`.
var workScopeFunctions = map[string]bool{"also": true, "let": true, "apply": true, "run": true}

// workScopeEnqueues reports a scope-function lambda that enqueues the continuation it receives: `.also { it.enqueue() }`
// or `.apply { enqueue() }`.
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
		case "also", "let":
			found = recv == param || strings.HasPrefix(recv, param+".")
		default:
			found = recv == "" || recv == "this" || strings.HasPrefix(recv, "this.")
		}
	})
	return found
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
	var sets []site
	reported := false
	libraryOnly := true
	roots := map[string]string{}
	seenRoots := map[string]bool{}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || isProjectTestFile(ctx, file.Path) || !bytes.Contains(file.Content, shortcutBytes) {
			continue
		}
		root := workModuleRoot(file.Path, roots)
		if root == "" {
			libraryOnly = false
		} else if !seenRoots[root] {
			seenRoots[root] = true
			libraryOnly = libraryOnly && readModuleIsLibrary(root)
		}
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
					reported = true
				case "setDynamicShortcuts", "addDynamicShortcuts":
					if shortcutReceiver(file, call) {
						sets = append(sets, site{file, n})
					}
				}
			})
		}
	}
	// Lint skips a library main project: skip only when every module with shortcut calls is a library.
	if reported || (libraryOnly && len(seenRoots) > 0) {
		return
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

// shortcutReceiver reports ShortcutManagerCompat's static calls and calls on a ShortcutManager: a variable declared as
// one or initialized from getSystemService(ShortcutManager...).
func shortcutReceiver(file *scanner.File, call lintCall) bool {
	recv := strings.TrimSpace(call.ReceiverText)
	if recv == "ShortcutManagerCompat" || recv == shortcutManagerCompatFQN {
		return recv == shortcutManagerCompatFQN || lintTypeRefMatches(file, recv, shortcutManagerCompatFQN)
	}
	if !lintIdentOnly(recv) {
		return false
	}
	if lintTypeRefMatches(file, lintDeclaredTypeText(file, call.ReceiverNode, recv), shortcutManagerFQN) {
		return true
	}
	return workLocalInitializedBy(file, call.ReceiverNode, recv, func(init string) bool {
		return strings.Contains(init, "getSystemService") && strings.Contains(init, "ShortcutManager")
	})
}

// parentOrZero returns the parent of n, or 0 at the root.
func parentOrZero(file *scanner.File, n uint32) uint32 {
	if p, ok := file.FlatParent(n); ok {
		return p
	}
	return 0
}
