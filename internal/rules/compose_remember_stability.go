package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// composeEnclosingLambdaArgumentName walks up from idx looking for the
// nearest lambda_literal ancestor that is itself a named value_argument, and
// returns the argument label (e.g. "onClick", "onValueChange"). Returns ""
// if no such enclosing lambda exists, or if the lambda is not a named
// argument. This powers callback/handler detection without needing type
// info about Compose itself.
func composeEnclosingLambdaArgumentName(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		parent, pok := file.FlatParent(cur)
		if !pok {
			return ""
		}
		// Lambdas passed as trailing lambdas are wrapped in annotated_lambda
		// / lambda_argument, not value_argument — skip those.
		if file.FlatType(parent) != "value_argument" {
			return ""
		}
		if file.FlatNamedChildCount(parent) < 2 {
			return ""
		}
		label := file.FlatNamedChild(parent, 0)
		if label == 0 || file.FlatType(label) != "simple_identifier" {
			return ""
		}
		return file.FlatNodeText(label)
	}
	return ""
}

// composeLambdaReferencedParams returns the distinct parameters of the nearest
// enclosing function_declaration that the lambda at lambdaIdx references, in
// source order, plus that function. Used to detect `remember { f(param) }` /
// `LaunchedEffect(Unit) { f(param) }` patterns where the lambda captures
// outer state without listing it as a key.
func composeLambdaReferencedParams(file *scanner.File, lambdaIdx uint32) ([]string, uint32) {
	fn, ok := flatEnclosingFunction(file, lambdaIdx)
	if !ok {
		return nil, 0
	}
	paramNames := flatFunctionParameterNames(file, fn)
	if len(paramNames) == 0 {
		return nil, fn
	}
	paramSet := make(map[string]struct{}, len(paramNames))
	for _, n := range paramNames {
		paramSet[n] = struct{}{}
	}
	return composeLambdaReferencedNames(file, lambdaIdx, paramSet), fn
}

// composeLambdaReferencedNames returns the distinct names in set that the lambda reads, in source order.
func composeLambdaReferencedNames(file *scanner.File, lambdaIdx uint32, set map[string]struct{}) []string {
	var hits []string
	seen := map[string]struct{}{}
	file.FlatWalkAllNodes(lambdaIdx, func(idx uint32) {
		if file.FlatType(idx) != "simple_identifier" {
			return
		}
		text := file.FlatNodeText(idx)
		if _, match := set[text]; !match {
			return
		}
		if _, dup := seen[text]; dup {
			return
		}
		if composeIdentShadowedWithin(file, lambdaIdx, idx, text) {
			return
		}
		// Filter out identifiers that appear as a label/name (declaration or
		// named-argument label), not as a reference — those don't count as a
		// capture.
		parent, pok := file.FlatParent(idx)
		if !pok {
			return
		}
		switch file.FlatType(parent) {
		case "variable_declaration", "parameter", "navigation_suffix":
			return
		case "value_argument":
			// The first named child of a value_argument whose count is >= 2
			// is the argument label (e.g. `onClick = { ... }`), not a reference.
			if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == idx {
				return
			}
		}
		seen[text] = struct{}{}
		hits = append(hits, text)
	})
	return hits
}

// composeLocalsDerivedFromParams returns the plain `val` locals declared before the lambda in fn whose initializer
// reads a parameter or another such local, so the local changes when the parameter does. Locals holding remembered
// or observed state are skipped: they are stable holders, not copies of the parameter.
func composeLocalsDerivedFromParams(file *scanner.File, fn, lambda uint32, params map[string]struct{}) map[string]struct{} {
	known := make(map[string]struct{}, len(params))
	for n := range params {
		known[n] = struct{}{}
	}
	derived := map[string]struct{}{}
	lambdaStart := file.FlatStartByte(lambda)
	file.FlatWalkNodes(fn, "property_declaration", func(prop uint32) {
		if file.FlatEndByte(prop) > lambdaStart || propertyDeclarationIsVar(file, prop) {
			return
		}
		if _, delegated := file.FlatFindChild(prop, "property_delegate"); delegated {
			return
		}
		// Only a declaration whose scope encloses the lambda is visible to it.
		if scope, ok := file.FlatParent(prop); !ok || file.FlatStartByte(scope) > lambdaStart || file.FlatEndByte(scope) < file.FlatEndByte(lambda) {
			return
		}
		name := propertyDeclarationName(file, prop)
		init := propertyInitializerExpression(file, prop)
		if name == "" || init == 0 || composeInitializerHoldsState(file, init) {
			return
		}
		if len(composeLambdaReferencedNames(file, init, known)) == 0 {
			return
		}
		known[name] = struct{}{}
		derived[name] = struct{}{}
	})
	return derived
}

// composeInitializerHoldsState reports an initializer that creates or observes Compose state.
func composeInitializerHoldsState(file *scanner.File, init uint32) bool {
	holds := false
	check := func(call uint32) {
		name := flatCallNameAny(file, call)
		if strings.HasPrefix(name, "remember") || strings.HasPrefix(name, "mutableState") || strings.HasPrefix(name, "mutable") && strings.HasSuffix(name, "StateOf") ||
			name == "derivedStateOf" || name == "produceState" || strings.HasPrefix(name, "collectAs") {
			holds = true
		}
	}
	if file.FlatType(init) == "call_expression" {
		check(init)
	}
	file.FlatWalkNodes(init, "call_expression", check)
	return holds
}

// composeParamTypeTexts maps each parameter name of fn to its declared type text.
func composeParamTypeTexts(file *scanner.File, fn uint32) map[string]string {
	out := map[string]string{}
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return out
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		ident, ok := file.FlatFindChild(child, "simple_identifier")
		if !ok {
			continue
		}
		_, typ, _ := strings.Cut(file.FlatNodeText(child), ":")
		out[file.FlatNodeText(ident)] = strings.TrimSpace(typ)
	}
	return out
}

var composeTrackedStateTypeRe = regexp.MustCompile(`^(State<.*>|(Lazy(List|Grid|StaggeredGrid)|Scroll|Pager|TextField)State)\??$`)

// composeParamReadOnlyInTrackingBlock reports a function-typed or Compose state-holder parameter whose
// every read sits inside derivedStateOf or snapshotFlow, which track the read and so never go stale.
func composeParamReadOnlyInTrackingBlock(file *scanner.File, lambda uint32, name, typ string) bool {
	if !strings.Contains(typ, "->") && !composeTrackedStateTypeRe.MatchString(typ) {
		return false
	}
	reads, tracked := 0, 0
	file.FlatWalkAllNodes(lambda, func(idx uint32) {
		if file.FlatType(idx) != "simple_identifier" || !file.FlatNodeTextEquals(idx, name) {
			return
		}
		if parent, ok := file.FlatParent(idx); ok {
			switch file.FlatType(parent) {
			case "variable_declaration", "parameter", "navigation_suffix":
				return
			}
		}
		reads++
		for p, ok := file.FlatParent(idx); ok && p != lambda; p, ok = file.FlatParent(p) {
			if file.FlatType(p) == "lambda_literal" {
				if owner, found := flatEnclosingAncestor(file, p, "call_expression"); found {
					if n := flatCallNameAny(file, owner); n == "derivedStateOf" || n == "snapshotFlow" || n == "movableContentOf" {
						tracked++
						return
					}
				}
			}
		}
		// rememberUpdatedState(callback) wraps the lambda so later calls see the latest one.
		if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "value_argument" {
			if owner, found := flatEnclosingAncestor(file, p, "call_expression"); found && flatCallNameAny(file, owner) == "rememberUpdatedState" {
				tracked++
			}
		}
	})
	return reads > 0 && reads == tracked
}

// composeParamIsStableNavigator reports a Circuit Navigator parameter, which lives as long as the presenter.
func composeParamIsStableNavigator(typ string) bool {
	typ = strings.TrimSuffix(strings.TrimSpace(typ), "?")
	return typ == "Navigator" || strings.HasSuffix(typ, ".Navigator")
}

// composeEnclosingKeyBlockParams returns the names mentioned in the key arguments of every key(...) block around call.
func composeEnclosingKeyBlockParams(file *scanner.File, call uint32) map[string]bool {
	out := map[string]bool{}
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "call_expression" || flatCallNameAny(file, p) != "key" {
			continue
		}
		if args := flatCallKeyArguments(file, p); args != 0 {
			file.FlatWalkNodes(args, "simple_identifier", func(id uint32) { out[file.FlatNodeText(id)] = true })
		}
	}
	return out
}

// composeRememberIsUnkeyedFallbackArm reports a keyless remember in one branch of an if whose other branch calls
// remember with keys, as in `if (k != null) remember(k) { ... } else remember { ... }`.
func composeRememberIsUnkeyedFallbackArm(file *scanner.File, call uint32) bool {
	child := call
	for {
		p, ok := file.FlatParent(child)
		if !ok {
			return false
		}
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal":
			return false
		case "if_expression":
			if file.FlatType(child) != "control_structure_body" {
				return false
			}
			found := false
			for sib := file.FlatFirstChild(p); sib != 0; sib = file.FlatNextSib(sib) {
				if sib == child || file.FlatType(sib) != "control_structure_body" {
					continue
				}
				file.FlatWalkNodes(sib, "call_expression", func(other uint32) {
					if flatCallNameAny(file, other) == "remember" && flatCallKeyArguments(file, other) != 0 {
						found = true
					}
				})
			}
			return found
		}
		child = p
	}
}

// composeParamIsCallback reports whether a parameter looks like a callback,
// event sink or lambda, which should be read through rememberUpdatedState.
func composeParamIsCallback(name, typ string) bool {
	if strings.Contains(typ, "->") {
		return true
	}
	if len(name) > 2 && strings.HasPrefix(name, "on") && name[2] >= 'A' && name[2] <= 'Z' {
		return true
	}
	lower := strings.ToLower(name)
	for _, suffix := range []string{"sink", "callback", "listener", "handler", "action", "actions"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// composeIdentifierIsMethodReceiver reports whether ident is only the receiver of a method call, as in `x.start()`.
func composeIdentifierIsMethodReceiver(file *scanner.File, ident uint32) bool {
	nav, ok := file.FlatParent(ident)
	if !ok || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChild(nav, 0) != ident {
		return false
	}
	call, ok := file.FlatParent(nav)
	return ok && file.FlatType(call) == "call_expression"
}

// composeLambdaReadsDataParams reports every non-callback parameter whose value the lambda reads
// as data. Parameters used only as method-call receivers are left out unless withReceivers is set.
func composeLambdaReadsDataParams(file *scanner.File, lambdaIdx uint32, withReceivers bool) []string {
	names, fn := composeLambdaReferencedParams(file, lambdaIdx)
	if len(names) == 0 {
		return nil
	}
	types := composeParamTypeTexts(file, fn)
	var out []string
	for _, name := range names {
		if composeParamIsCallback(name, types[name]) {
			continue
		}
		hit := false
		file.FlatWalkAllNodes(lambdaIdx, func(idx uint32) {
			if hit || file.FlatType(idx) != "simple_identifier" || !file.FlatNodeTextEquals(idx, name) ||
				composeIdentShadowedWithin(file, lambdaIdx, idx, name) {
				return
			}
			parent, ok := file.FlatParent(idx)
			if !ok || (!withReceivers && composeIdentifierIsMethodReceiver(file, idx)) {
				return
			}
			switch file.FlatType(parent) {
			case "variable_declaration", "parameter", "navigation_suffix":
				return
			case "value_argument":
				if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == idx {
					return
				}
			}
			hit = true
		})
		if hit {
			out = append(out, name)
		}
	}
	return out
}

// composeNameList renders "a" as is and several names as "a, b".
func composeNameList(names []string) string { return strings.Join(names, ", ") }

// composeChangeVerb renders "x changes" for one name and "they change" for several.
func composeChangeVerb(names []string) string {
	if len(names) == 1 {
		return names[0] + " changes"
	}
	return "they change"
}

// composeNounPlural returns noun, or noun with an "s" for several.
func composeNounPlural(noun string, n int) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

// composeParamNoun returns "parameter" or "parameters" for the count.
func composeParamNoun(n int) string {
	if n == 1 {
		return "parameter"
	}
	return "parameters"
}

// composeLambdaHasOnceOnlyShape reports whether an effect body is a one-shot side effect:
// guarded by a top-level if, or calling a tracking, registration or UI-notification function.
func composeLambdaHasOnceOnlyShape(file *scanner.File, lambdaIdx uint32) bool {
	if stmts, ok := file.FlatFindChild(lambdaIdx, "statements"); ok {
		for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "if_expression" {
				return true
			}
		}
	}
	once := false
	file.FlatWalkNodes(lambdaIdx, "call_expression", func(call uint32) {
		lower := strings.ToLower(flatCallNameAny(file, call))
		// show* calls are host-state notifications such as a snackbar or dialog.
		if strings.HasPrefix(lower, "show") {
			once = true
		}
		for _, marker := range []string{"impression", "track", "analytics", "register", "toast", "snackbar", "log"} {
			if strings.Contains(lower, marker) {
				once = true
			}
		}
	})
	return once
}

// composeCommentSaysOneShot reports comment text that calls the code one-shot or run-once.
func composeCommentSaysOneShot(comment string) bool {
	text := strings.ToLower(comment)
	for _, phrase := range []string{"one-shot", "one shot", "oneshot", "only once", "runs once", "run once", "once on", "fires once"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// composeRememberDocumentedFirstCapture reports a comment before the remember statement, or inside its
// lambda, that says the first-composition value is captured on purpose.
func composeRememberDocumentedFirstCapture(file *scanner.File, call, lambda uint32) bool {
	says := func(comment string) bool {
		text := strings.ToLower(comment)
		for _, phrase := range []string{"first composition", "initial composition", "captured once", "capture once", "only once", "on purpose", "intentional", "initial value", "initial position", "initial state"} {
			if strings.Contains(text, phrase) {
				return true
			}
		}
		return false
	}
	found := false
	for _, kind := range []string{"line_comment", "multiline_comment"} {
		file.FlatWalkNodes(lambda, kind, func(c uint32) {
			if says(file.FlatNodeText(c)) {
				found = true
			}
		})
	}
	if found {
		return true
	}
	node := call
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(node) {
		if t := file.FlatType(p); t != "property_declaration" && t != "call_expression" && t != "statements" {
			break
		} else if t == "statements" {
			if _, hasPrev := file.FlatPrevSibling(node); !hasPrev {
				node = p
			}
			break
		}
		node = p
	}
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		t := file.FlatType(prev)
		if t != "line_comment" && t != "multiline_comment" {
			return false
		}
		if says(file.FlatNodeText(prev)) {
			return true
		}
	}
	return false
}

// composeParamIsInjectedFactory reports a factory-like parameter that the lambda only calls methods on.
func composeParamIsInjectedFactory(file *scanner.File, lambda uint32, name, typ string) bool {
	lower := strings.ToLower(name + " " + typ)
	if !strings.Contains(lower, "factory") && !strings.Contains(lower, "provider") && !strings.Contains(lower, "creator") {
		return false
	}
	reads, receivers := 0, 0
	file.FlatWalkNodes(lambda, "simple_identifier", func(idx uint32) {
		if !file.FlatNodeTextEquals(idx, name) {
			return
		}
		reads++
		if composeIdentifierIsMethodReceiver(file, idx) {
			receivers++
		}
	})
	return reads > 0 && reads == receivers
}

// composeParamIsInvokeOnlyFactory reports a function-typed parameter that the lambda only invokes, as in
// `remember { Holder(create()) }`. The call runs once to build the remembered object, and callers pass a fresh
// lambda on every recomposition, so a key would rebuild the object each time. A lambda that escapes (is passed
// along or stored) or is read without a call is not covered.
func composeParamIsInvokeOnlyFactory(file *scanner.File, lambda uint32, name, typ string) bool {
	if !strings.Contains(typ, "->") {
		return false
	}
	reads, invokes := 0, 0
	file.FlatWalkNodes(lambda, "simple_identifier", func(idx uint32) {
		if !file.FlatNodeTextEquals(idx, name) {
			return
		}
		parent, ok := file.FlatParent(idx)
		if !ok {
			return
		}
		switch file.FlatType(parent) {
		case "variable_declaration", "parameter", "navigation_suffix":
			return
		}
		reads++
		switch file.FlatType(parent) {
		case "call_expression":
			if file.FlatNamedChild(parent, 0) == idx {
				invokes++
			}
		case "navigation_expression":
			call, ok := file.FlatParent(parent)
			if ok && file.FlatType(call) == "call_expression" && file.FlatNamedChild(parent, 0) == idx {
				if suffix, found := file.FlatFindChild(parent, "navigation_suffix"); found &&
					strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(suffix)), ".") == "invoke" {
					invokes++
				}
			}
		}
	})
	return reads > 0 && reads == invokes
}

// composePrecedingComments returns the comments directly before node, nearest first.
func composePrecedingComments(file *scanner.File, node uint32) []string {
	// tree-sitter hangs a comment before the first statement on the block, not the statement list.
	if _, hasPrev := file.FlatPrevSibling(node); !hasPrev {
		if parent, ok := file.FlatParent(node); ok && file.FlatType(parent) == "statements" {
			node = parent
		}
	}
	var out []string
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		if t := file.FlatType(prev); t != "line_comment" && t != "multiline_comment" {
			break
		}
		out = append(out, file.FlatNodeText(prev))
	}
	return out
}

// composeEffectDocumentedOneShot reports a comment or KDoc directly before the effect call (or its
// statement) that calls it one-shot.
func composeEffectDocumentedOneShot(file *scanner.File, call uint32) bool {
	node := call
	if parent, ok := file.FlatParent(call); ok && file.FlatType(parent) == "call_expression" {
		node = parent
	}
	for _, c := range composePrecedingComments(file, node) {
		if composeCommentSaysOneShot(c) {
			return true
		}
	}
	return false
}

// composeAssignmentDocumentedIdempotent reports a comment directly before the assignment that calls it idempotent or eager on purpose.
func composeAssignmentDocumentedIdempotent(file *scanner.File, assignment uint32) bool {
	for _, c := range composePrecedingComments(file, assignment) {
		text := strings.ToLower(c)
		for _, phrase := range []string{"idempotent", "eager", "safe to repeat", "safe to run every"} {
			if strings.Contains(text, phrase) {
				return true
			}
		}
	}
	return false
}

// composeSeedsMutableState reports whether the lambda calls a mutable-state factory,
// which seeds a local copy from the parameter on purpose.
func composeSeedsMutableState(file *scanner.File, lambdaIdx uint32) bool {
	seeds := false
	file.FlatWalkNodes(lambdaIdx, "call_expression", func(call uint32) {
		switch flatCallNameAny(file, call) {
		case "mutableStateOf", "mutableIntStateOf", "mutableFloatStateOf", "mutableLongStateOf",
			"mutableDoubleStateOf", "mutableStateListOf", "mutableStateMapOf", "toMutableStateList",
			"toMutableStateMap":
			seeds = true
		}
	})
	return seeds
}

// composeRememberedVariableName returns the name a remember call is assigned to, or "".
func composeRememberedVariableName(file *scanner.File, call uint32) string {
	prop, ok := flatEnclosingAncestor(file, call, "property_declaration")
	if !ok {
		return ""
	}
	decl, ok := file.FlatFindChild(prop, "variable_declaration")
	if !ok {
		return ""
	}
	if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
		return file.FlatNodeText(ident)
	}
	return ""
}

// composeResyncsStateFromParam reports whether the function re-assigns state from param outside
// the remember lambda, for example in a LaunchedEffect(param) or an assignment guarded by a check.
func composeResyncsStateFromParam(file *scanner.File, fn, lambda uint32, variable, param string) bool {
	lStart, lEnd := file.FlatStartByte(lambda), file.FlatEndByte(lambda)
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(param) + `\b`)
	found := false
	check := func(idx uint32) {
		if found || file.FlatStartByte(idx) >= lStart && file.FlatEndByte(idx) <= lEnd {
			return
		}
		text := file.FlatNodeText(idx)
		if wordRe.MatchString(text) && (variable == "" || strings.Contains(text, variable)) {
			found = true
		}
	}
	file.FlatWalkNodes(fn, "assignment", check)
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		switch flatCallNameAny(file, call) {
		case "LaunchedEffect", "SideEffect", "DisposableEffect":
			check(call)
		}
	})
	return found
}

// composeKeyRuleExemptFunction reports a composable whose keyless remember or constant-keyed effect is deliberate scaffolding:
// a preview or sample helper, a @VisibleForTesting function or one in a Fake/Test class, or a private function that
// only @Preview functions in the same file call.
func composeKeyRuleExemptFunction(file *scanner.File, fn uint32) bool {
	if composeUnstableParameterShouldSkipFunctionFlat(file, fn) || flatHasAnnotationNamed(file, fn, "VisibleForTesting") {
		return true
	}
	if cls := flatEnclosingClassName(file, fn); strings.HasPrefix(cls, "Fake") || strings.Contains(cls, "Test") {
		return true
	}
	return composePrivateFunctionOnlyCalledFromPreviews(file, fn)
}

// composePrivateFunctionOnlyCalledFromPreviews reports a private function whose every same-file call sits inside a
// preview or sample function.
func composePrivateFunctionOnlyCalledFromPreviews(file *scanner.File, fn uint32) bool {
	// Internal helpers are included because previews usually sit next to them in the same file.
	if !file.FlatHasModifier(fn, "private") && !file.FlatHasModifier(fn, "internal") {
		return false
	}
	name := extractIdentifierFlat(file, fn)
	calls, fromPreview := 0, 0
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if flatCallNameAny(file, call) != name {
			return
		}
		calls++
		for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
			if file.FlatType(p) == "function_declaration" && composeFunctionIsPreviewOrSampleFlat(file, p) {
				fromPreview++
				return
			}
		}
	})
	return calls > 0 && calls == fromPreview
}

// ComposeRememberWithoutKeyRule flags `remember { f(param) }` where the
// lambda body references an enclosing function parameter but the remember
// call has no key arguments. The cached value won't update when the
// parameter changes across recomposition.
type ComposeRememberWithoutKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberWithoutKeyRule) Confidence() float64 { return api.ConfidenceMedium }

// ComposeLaunchedEffectWithoutKeysRule flags `LaunchedEffect(Unit) { f(param) }`
// where the effect body references an enclosing parameter but the keys are
// constant (Unit / true / false / null). The effect won't re-run when the
// parameter changes.
type ComposeLaunchedEffectWithoutKeysRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeLaunchedEffectWithoutKeysRule) Confidence() float64 { return api.ConfidenceMedium }

var composeConstantKeyTexts = map[string]struct{}{
	"Unit":  {},
	"true":  {},
	"false": {},
	"null":  {},
}

// ComposeUnstableParameterRule flags `@Composable fun X(users: List<User>)`
// whose parameters use the mutable Kotlin collection types (`List`, `Map`,
// `Set`, `MutableList`, etc.) directly. Compose considers these unstable
// for recomposition-skipping unless the author wraps them in a stable type
// (`@Immutable data class`, `ImmutableList` / `PersistentList` from
// kotlinx.collections.immutable).
type ComposeUnstableParameterRule struct {
	FlatDispatchBase
	BaseRule
}

var composeUnstableCollectionTypes = map[string]struct{}{
	"List":        {},
	"Map":         {},
	"Set":         {},
	"MutableList": {},
	"MutableMap":  {},
	"MutableSet":  {},
	"Collection":  {},
	"Iterable":    {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeUnstableParameterRule) Confidence() float64 { return api.ConfidenceMedium }

func composeUnstableParameterShouldSkipFunctionFlat(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 {
		return false
	}
	if isAndroidTestSupportArtifactSource(file.Path) {
		return true
	}
	for cur := fn; cur != 0; {
		if file.FlatType(cur) == "function_declaration" && composeFunctionIsPreviewOrSampleFlat(file, cur) {
			return true
		}
		parent, ok := file.FlatParent(cur)
		if !ok {
			break
		}
		cur = parent
	}
	return false
}

func composeFunctionIsPreviewOrSampleFlat(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	name := extractIdentifierFlat(file, fn)
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "preview") || strings.HasPrefix(lower, "sample") ||
		strings.HasPrefix(lower, "fixture") ||
		strings.HasSuffix(lower, "preview") || strings.HasSuffix(lower, "sample") ||
		strings.HasSuffix(lower, "fixture") {
		return true
	}
	mods, _ := file.FlatFindChild(fn, "modifiers")
	if mods == 0 {
		return false
	}
	modText := file.FlatNodeText(mods)
	return composeModifierTextHasPreviewAnnotation(modText) || composeModifierTextHasMultipreviewAnnotation(file, modText)
}

func composeModifierTextHasPreviewAnnotation(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '@' {
			continue
		}
		start := i + 1
		end := start
		for end < len(text) {
			c := text[end]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '_' || c == '.' {
				end++
				continue
			}
			break
		}
		// Covers @Preview, @PreviewLightDark, @PreviewFontScale, @PreviewScreenSizes and custom `*Preview` annotations.
		if end > start {
			simple := flatAnnotationSimpleName(text[start:end])
			if androidxPreviewAnnotations[simple] || strings.HasSuffix(simple, "Preview") {
				return true
			}
		}
	}
	return false
}

// ComposeRememberSaveableNonParcelableRule flags `rememberSaveable {
// SomeType() }` where SomeType() is a constructor call to a plain class
// with no `saver` / `stateSaver` argument passed. Without a saver,
// rememberSaveable falls back to AutoSaver which only handles primitives,
// Parcelables, and a few stdlib types.
//
// Since this rule is syntactic it can't verify Parcelable status — it
// warns on any constructor-looking call in the trailing lambda whose name
// doesn't match a well-known stdlib type. Users can suppress the finding
// locally when they know the type is Parcelable/Serializable.
type ComposeRememberSaveableNonParcelableRule struct {
	FlatDispatchBase
	BaseRule
}

var composeSaverSafeBuiltins = map[string]struct{}{
	"Int":     {},
	"Long":    {},
	"Float":   {},
	"Double":  {},
	"Boolean": {},
	"Char":    {},
	"String":  {},
	"Unit":    {},
	"Short":   {},
	"Byte":    {},
	"BigInt":  {},
	"Bundle":  {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberSaveableNonParcelableRule) Confidence() float64 { return api.ConfidenceMedium }

// composeIdentShadowedWithin reports an identifier that refers to a local declared inside root (a lambda
// parameter, a local val/var or a for variable) rather than to the enclosing function's parameter of the same name.
func composeIdentShadowedWithin(file *scanner.File, root, ident uint32, name string) bool {
	identStart := file.FlatStartByte(ident)
	declares := func(decl uint32) bool {
		found := false
		file.FlatWalkAllNodes(decl, func(n uint32) {
			if found || file.FlatType(n) != "simple_identifier" || !file.FlatNodeTextEquals(n, name) {
				return
			}
			if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "variable_declaration" {
				found = true
			}
		})
		return found
	}
	for cur, ok := file.FlatParent(ident); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if params, found := file.FlatFindChild(cur, "lambda_parameters"); found && declares(params) {
				return true
			}
		case "for_statement":
			if v, found := file.FlatFindChild(cur, "variable_declaration"); found && declares(v) {
				return true
			}
		case "statements":
			for child := file.FlatFirstChild(cur); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "property_declaration" || file.FlatStartByte(child) >= identStart {
					continue
				}
				if v, found := file.FlatFindChild(child, "variable_declaration"); found && declares(v) {
					return true
				}
				if v, found := file.FlatFindChild(child, "multi_variable_declaration"); found && declares(v) {
					return true
				}
			}
		}
		if cur == root {
			break
		}
	}
	return false
}
