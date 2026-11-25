package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RememberReturnTypeRule ports runtime-lint's RememberReturnType: a `remember` call whose calculation returns Unit.
type RememberReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// OpaqueUnitKeyRule ports runtime-lint's OpaqueUnitKey: an expression that always evaluates to Unit passed as a key
// of remember, LaunchedEffect, DisposableEffect, produceState, ReusableContent or key.
type OpaqueUnitKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// ProduceStateDoesNotAssignValueRule ports runtime-lint's ProduceStateDoesNotAssignValue: a produceState producer
// that never assigns `value`.
type ProduceStateDoesNotAssignValueRule struct {
	FlatDispatchBase
	BaseRule
}

// RememberSaveableSaverParameterRule ports runtime-saveable-lint's RememberSaveableSaverParameter: a Saver passed
// as the only vararg `inputs` argument of rememberSaveable instead of as `saver` / `stateSaver`.
type RememberSaveableSaverParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports medium confidence: Unit and Saver types are proven from same-file declarations.
func (r *RememberReturnTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: Unit types are proven from same-file declarations.
func (r *OpaqueUnitKeyRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: calls that might hand the producer scope to a helper are treated as
// assigning it.
func (r *ProduceStateDoesNotAssignValueRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: Saver types are proven from same-file declarations and Saver factories.
func (r *RememberSaveableSaverParameterRule) Confidence() float64 { return api.ConfidenceMedium }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *RememberReturnTypeRule) fileMentions() []string { return []string{acrRuntimePackage} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *OpaqueUnitKeyRule) fileMentions() []string { return []string{acrRuntimePackage} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *ProduceStateDoesNotAssignValueRule) fileMentions() []string {
	return []string{acrRuntimePackage}
}

// fileMentions gates the rule to files that can reference runtime-saveable.
func (r *RememberSaveableSaverParameterRule) fileMentions() []string {
	return []string{acrSaveablePackage}
}

// fixLevel declares the `saver = ` rewrite, which changes which parameter receives the Saver (and stops it from
// resetting the state on every recomposition): a semantic fix.
func (r *RememberSaveableSaverParameterRule) fixLevel() api.FixLevel { return api.FixSemantic }

func (r *RememberReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RememberReturnType", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The calculation's Unit type is proven only from `remember<Unit>`, assignments and loops, `Unit`, println, same-file Unit functions and `() -> Unit` values; library calls returning Unit are not recognized.",
		}}
}

func (r *OpaqueUnitKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "OpaqueUnitKey", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A key counts as Unit only when it is a call of a same-file Unit function, println, a same-file Unit property or an if/when whose branches all are.",
		}}
}

func (r *ProduceStateDoesNotAssignValueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ProduceStateDoesNotAssignValue", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A producer that calls a function which might receive the ProduceStateScope or a MutableState (an implicit-receiver call krit cannot resolve, or one passed `this` or a value of unknown type) is treated as assigning value.",
		}}
}

func (r *RememberSaveableSaverParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RememberSaveableSaverParameter", RuleSet: "compose", DefaultActive: true,
		FixLevel: "semantic",
		KnownLimitations: []string{
			"The argument must be proven to be a Saver: Saver(...), listSaver, mapSaver, autoSaver, an object : Saver, or a same-file declaration of a Saver type.",
		}}
}

const acrSaveablePackage = acrRuntimePackage + ".saveable"

func registerAndroidxComposeRuntimeRememberArgRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	unit := &RememberReturnTypeRule{BaseRule: BaseRule{RuleName: "RememberReturnType", RuleSetName: "compose", Sev: "error",
		Desc: "Detects remember calls whose calculation returns Unit, which remembers nothing; use SideEffect or a MutableState instead."}}
	registerAcrRule(unit.BaseRule, unit, []string{"call_expression"}, kotlin, unit.check)

	opaque := &OpaqueUnitKeyRule{BaseRule: BaseRule{RuleName: "OpaqueUnitKey", RuleSetName: "compose", Sev: "warning",
		Desc: "Detects remember/LaunchedEffect/DisposableEffect/produceState/ReusableContent/key keys that always evaluate to Unit."}}
	registerAcrRule(opaque.BaseRule, opaque, []string{"call_expression"}, kotlin, opaque.check)

	produce := &ProduceStateDoesNotAssignValueRule{BaseRule: BaseRule{RuleName: "ProduceStateDoesNotAssignValue",
		RuleSetName: "compose", Sev: "error", Desc: "Detects produceState producers that never assign value, so the state never changes."}}
	registerAcrRule(produce.BaseRule, produce, []string{"call_expression"}, kotlin, produce.check)

	saver := &RememberSaveableSaverParameterRule{BaseRule: BaseRule{RuleName: "RememberSaveableSaverParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects a Saver passed to rememberSaveable's vararg inputs instead of the saver parameter."}}
	registerAcrRule(saver.BaseRule, saver, []string{"call_expression"}, kotlin, saver.check)
}

// acrRuntimeCall reports whether call targets androidx.compose.runtime.<name> (or pkg.<name>) without a receiver,
// and is the outer node of `name(args) { }`.
func acrRuntimeCall(file *scanner.File, call uint32, pkg string, names ...string) (string, bool) {
	if acrIsInnerCallOfTrailingLambda(file, call) {
		return "", false
	}
	name, _, hasReceiver := acrCallCallee(file, call)
	for _, n := range names {
		if name == n {
			if hasReceiver {
				return name, acrCallTargets(file, call, pkg, name)
			}
			return name, acrCalleeResolvesTo(file, call, pkg, name)
		}
	}
	return "", false
}

// acrCallArgument is one value argument of a call.
type acrCallArgument struct {
	node, expr uint32
	label      string
	spread     bool
}

// acrCallArguments returns a call's value arguments (not its trailing lambda), looking through `f(args) { }`.
func acrCallArguments(file *scanner.File, call uint32) []acrCallArgument {
	inner := call
	if first := file.FlatFirstChild(call); first != 0 && file.FlatType(first) == "call_expression" &&
		acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(first)) {
		inner = first
	}
	suffix := file.FlatNextSib(file.FlatFirstChild(inner))
	if suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return nil
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return nil
	}
	var out []acrCallArgument
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		a := acrCallArgument{node: arg, expr: flatValueArgumentExpression(file, arg), label: flatValueArgumentLabel(file, arg)}
		for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "*" {
				a.spread = true
			}
		}
		out = append(out, a)
	}
	return out
}

// acrHasTrailingLambda reports whether a call ends with a trailing lambda.
func acrHasTrailingLambda(file *scanner.File, call uint32) bool {
	suffix := file.FlatNextSib(file.FlatFirstChild(call))
	return suffix != 0 && file.FlatType(suffix) == "call_suffix" && file.FlatHasChildOfType(suffix, "annotated_lambda")
}

// acrExprIsUnit proves an expression always evaluates to Unit: a call of a same-file function (or same-file class
// member) returning Unit, println/print, a same-file property declared `: Unit`, `kotlin.Unit`, or an if/when
// whose branches all are. The bare literal `Unit` is reported by the callers that need it.
func acrExprIsUnit(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		if name == "" {
			return false
		}
		if hasReceiver {
			class := acrReceiverClass(file, file.FlatFirstChild(acrCalleeExpression(file, expr)))
			member := acrClassMemberDeclaration(file, class, name, false)
			return member != 0 && acrFunctionReturnKind(file, member) == acrReturnUnit
		}
		if fn := acrSameFileFunction(file, name); fn != 0 {
			return !flatFunctionDeclarationIsExtension(file, fn) && acrFunctionReturnKind(file, fn) == acrReturnUnit
		}
		return (name == "println" || name == "print") && acrDefaultImported(file, name) && !acrFileDeclaresAnyFunction(file, name)
	case "simple_identifier":
		decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr))
		return decl != 0 && acrTypeIsUnit(file, acrDeclaredType(file, decl))
	case "navigation_expression":
		if strings.Join(strings.Fields(file.FlatNodeText(expr)), "") == "kotlin.Unit" {
			return true
		}
		decl := acrExprDeclaration(file, expr)
		return decl != 0 && acrTypeIsUnit(file, acrDeclaredType(file, decl))
	case "if_expression":
		branches := 0
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "control_structure_body" {
				if !acrBodyIsUnit(file, child, depth+1) {
					return false
				}
				branches++
			}
		}
		return branches == 2
	}
	return false
}

// acrBodyIsUnit reports whether a branch body's value is proven Unit.
func acrBodyIsUnit(file *scanner.File, body uint32, depth int) bool {
	inner := flatFirstNamedChild(file, body)
	if inner != 0 && file.FlatType(inner) == "statements" {
		inner = flatLastNamedChild(file, inner)
	}
	if inner == 0 {
		return false
	}
	return acrStatementIsUnit(file, inner, depth)
}

// acrStatementIsUnit reports whether the last statement of a block (or lambda) makes the block's value Unit.
func acrStatementIsUnit(file *scanner.File, stmt uint32, depth int) bool {
	switch file.FlatType(stmt) {
	case "assignment", "for_statement", "while_statement", "do_while_statement":
		return true
	case "simple_identifier":
		if file.FlatNodeTextEquals(stmt, "Unit") && acrDefaultImported(file, "Unit") {
			return true
		}
	}
	return acrExprIsUnit(file, stmt, depth)
}

func acrTypeIsUnit(file *scanner.File, typeNode uint32) bool {
	if typeNode == 0 || file.FlatType(typeNode) != "user_type" {
		return false
	}
	name := acrTypeNodeName(file, typeNode)
	return name == "kotlin.Unit" || name == "Unit" && acrDefaultImported(file, "Unit")
}

// acrCalculationReturnsUnit mirrors isReallyRememberingUnit for remember/retain: an explicit `<Unit>` type argument,
// a calculation lambda whose result is proven Unit, or a calculation value typed `() -> Unit`.
func acrCalculationReturnsUnit(file *scanner.File, call uint32, calculationLabel string) bool {
	if args := acrCallTypeArguments(file, call); args != 0 {
		proj := flatFirstNamedChild(file, args)
		return proj != 0 && acrTypeIsUnit(file, flatFirstNamedChild(file, proj))
	}
	if acrHasTrailingLambda(file, call) {
		last := acrTrailingLambdaResult(file, call)
		// An empty calculation returns Unit.
		return last == 0 || acrStatementIsUnit(file, last, 0)
	}
	args := acrCallArguments(file, call)
	var calc uint32
	for _, a := range args {
		if a.label == calculationLabel {
			calc = a.expr
		}
	}
	if calc == 0 && len(args) > 0 && args[len(args)-1].label == "" {
		calc = args[len(args)-1].expr
	}
	calc = acrUnwrapParens(file, calc)
	if calc == 0 {
		return false
	}
	if file.FlatType(calc) == "lambda_literal" {
		stmts, ok := file.FlatFindChild(calc, "statements")
		return !ok || acrStatementIsUnit(file, flatLastNamedChild(file, stmts), 0)
	}
	decl := acrExprDeclaration(file, calc)
	if decl == 0 {
		return false
	}
	t := acrDeclaredType(file, decl)
	if t == 0 || file.FlatType(t) != "function_type" {
		return false
	}
	params, _ := file.FlatFindChild(t, "function_type_parameters")
	return (params == 0 || file.FlatNamedChildCount(params) == 0) && acrTypeIsUnit(file, flatLastNamedChild(file, t))
}

func (r *RememberReturnTypeRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if _, ok := acrRuntimeCall(file, call, acrRuntimePackage, "remember"); !ok ||
		!acrCalculationReturnsUnit(file, call, "calculation") {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "`remember` calls must not return `Unit`")
}

// acrKeyedAPIs lists, per runtime API, the labels of its non-key parameters and how many leading positional
// arguments are not keys (produceState's initialValue).
var acrKeyedAPIs = map[string]struct {
	others  map[string]bool
	leading int
}{
	"remember":         {acrNames("calculation"), 0},
	"LaunchedEffect":   {acrNames("block"), 0},
	"DisposableEffect": {acrNames("effect"), 0},
	"produceState":     {acrNames("initialValue", "producer"), 1},
	"ReusableContent":  {acrNames("content"), 0},
	"key":              {acrNames("block"), 0},
}

func (r *OpaqueUnitKeyRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, ok := acrRuntimeCall(file, call, acrRuntimePackage,
		"remember", "LaunchedEffect", "DisposableEffect", "produceState", "ReusableContent", "key")
	if !ok {
		return
	}
	api := acrKeyedAPIs[name]
	var keys []acrCallArgument
	positional := 0
	for _, a := range acrCallArguments(file, call) {
		switch {
		case a.label != "":
			if !api.others[a.label] {
				keys = append(keys, a)
			}
		default:
			positional++
			if positional > api.leading {
				keys = append(keys, a)
			}
		}
	}
	// Without a trailing lambda the last positional argument is the calculation / effect / content.
	if !acrHasTrailingLambda(file, call) && len(keys) > 0 && keys[len(keys)-1].label == "" {
		keys = keys[:len(keys)-1]
	}
	positionalKeys := 0
	for _, k := range keys {
		if k.label == "" {
			positionalKeys++
		}
	}
	index := 0
	for _, k := range keys {
		param := k.label
		if param == "" {
			index++
			switch {
			case name == "ReusableContent":
				param = "key"
			case name == "key" || positionalKeys > 3:
				param = "keys"
			default:
				param = "key" + string(rune('0'+index))
			}
		}
		expr := acrUnwrapParens(file, k.expr)
		if k.spread || expr == 0 || !acrExprIsUnit(file, expr, 0) {
			continue
		}
		ctx.EmitAtNode(k.expr, "Implicitly passing `Unit` as argument to "+param)
	}
}

func (r *ProduceStateDoesNotAssignValueRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if _, ok := acrRuntimeCall(file, call, acrRuntimePackage, "produceState"); !ok {
		return
	}
	producer := uint32(0)
	if acrHasTrailingLambda(file, call) {
		suffix := file.FlatNextSib(file.FlatFirstChild(call))
		if annotated, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
			producer, _ = file.FlatFindChild(annotated, "lambda_literal")
		}
	} else {
		for _, a := range acrCallArguments(file, call) {
			if a.label == "producer" {
				producer = acrUnwrapParens(file, a.expr)
			}
		}
	}
	if producer == 0 || file.FlatType(producer) != "lambda_literal" || acrProducerMayAssignValue(file, producer) {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "produceState calls should assign `value` inside the producer lambda")
}

// acrProducerKnownCalls are calls inside a producer that neither receive the ProduceStateScope nor a MutableState.
var acrProducerKnownCalls = map[string]bool{
	"delay": true, "launch": true, "async": true, "withContext": true, "awaitDispose": true, "coroutineScope": true,
	"supervisorScope": true, "println": true, "print": true, "repeat": true, "run": true, "check": true,
	"require": true, "error": true, "listOf": true, "mutableListOf": true, "emptyList": true, "setOf": true,
	"mapOf": true, "to": true, "yield": true, "ensureActive": true,
}

// acrProducerMayAssignValue reports whether a producer lambda assigns `value` (`value = x`, `x.value += y`,
// `value++`) anywhere in its subtree, or calls something that might receive the scope or a MutableState, which
// Lint counts as assigning.
func acrProducerMayAssignValue(file *scanner.File, producer uint32) bool {
	found := false
	file.FlatWalkAllNodes(producer, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "assignment":
			if target := file.FlatFirstChild(n); target != 0 && acrRightmostName(file, target) == "value" {
				found = true
			}
		case "postfix_expression", "prefix_expression":
			if acrIncrementOperator(file, n) && acrRightmostName(file, flatFirstNamedChild(file, n)) == "value" {
				found = true
			}
		case "call_expression":
			if !acrIsInnerCallOfTrailingLambda(file, n) && acrCallMayReceiveProducerScope(file, n) {
				found = true
			}
		}
	})
	return found
}

// acrRightmostName returns the last identifier of `value`, `this.value`, `a.b.value` (also in assignable form).
func acrRightmostName(file *scanner.File, expr uint32) string {
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr)
	case "navigation_expression", "directly_assignable_expression":
		if suffix := flatLastNamedChild(file, expr); suffix != 0 {
			if file.FlatType(suffix) == "navigation_suffix" {
				return file.FlatChildTextOrEmpty(suffix, "simple_identifier")
			}
			if file.FlatType(suffix) == "simple_identifier" {
				return file.FlatNodeText(suffix)
			}
		}
	}
	return ""
}

// acrCallMayReceiveProducerScope reports whether a call inside a producer might take the ProduceStateScope or a
// MutableState: it passes `this` or an argument of unknown type, calls an unresolved implicit-receiver function
// (possibly a ProduceStateScope extension), or calls a same-file function with such a parameter or receiver.
func acrCallMayReceiveProducerScope(file *scanner.File, call uint32) bool {
	name, _, hasReceiver := acrCallCallee(file, call)
	if name == "" {
		return true
	}
	// Lint looks at the callee's parameter types; the known calls take neither the scope nor a MutableState.
	if !hasReceiver && acrProducerKnownCalls[name] && !acrFileDeclaresAnyFunction(file, name) {
		return false
	}
	for _, a := range acrCallArguments(file, call) {
		if acrArgumentMayBeState(file, a.expr) {
			return true
		}
	}
	if fn := acrSameFileFunction(file, name); fn != 0 {
		for _, param := range acrFunctionParameters(file, fn) {
			if acrTypeMayBeProducerState(file, acrParameterType(file, param)) {
				return true
			}
		}
		return flatFunctionDeclarationIsExtension(file, fn) && acrTypeMayBeProducerState(file, acrFunctionReceiverType(file, fn))
	}
	if hasReceiver {
		recv := acrUnwrapParens(file, file.FlatFirstChild(acrCalleeExpression(file, call)))
		return file.FlatType(recv) == "this_expression"
	}
	return flatFindSameFileClassLikeDeclaration(file, name) == 0
}

// acrArgumentMayBeState reports whether an argument might be the scope or a MutableState: `this`, or a value whose
// type the file does not prove to be something else.
func acrArgumentMayBeState(file *scanner.File, expr uint32) bool {
	expr = acrUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "this_expression":
		return true
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return true
		}
		t := acrDeclaredType(file, decl)
		if t != 0 {
			return acrTypeMayBeProducerState(file, t)
		}
		init := acrUnwrapParens(file, acrInitializer(file, decl))
		return init == 0 || file.FlatType(init) == "call_expression" || file.FlatType(init) == "simple_identifier" ||
			file.FlatType(init) == "navigation_expression"
	}
	return false
}

// acrTypeMayBeProducerState reports whether a declared type could be a ProduceStateScope or MutableState.
func acrTypeMayBeProducerState(file *scanner.File, typeNode uint32) bool {
	if typeNode == 0 {
		return false
	}
	return acrTypeIs(file, typeNode, acrRuntimePackage, "ProduceStateScope", "MutableState") ||
		!acrTypeIsKnownNonState(file, typeNode)
}

// acrTypeIsKnownNonState reports whether a declared type is plainly not a state: kotlin builtins, strings,
// collections and function types.
func acrTypeIsKnownNonState(file *scanner.File, typeNode uint32) bool {
	switch file.FlatType(typeNode) {
	case "function_type":
		return true
	case "nullable_type":
		return acrTypeIsKnownNonState(file, flatFirstNamedChild(file, typeNode))
	}
	switch acrTypeNodeName(file, typeNode) {
	case "Int", "Long", "Float", "Double", "Boolean", "String", "Char", "Byte", "Short", "Unit", "List", "Set", "Map",
		"Collection", "Iterable", "Sequence":
		return true
	}
	return false
}

// acrParameterType returns a function parameter's type node.
func acrParameterType(file *scanner.File, param uint32) uint32 {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return child
		}
	}
	return 0
}

// acrSaverFactories are runtime-saveable functions returning a Saver.
var acrSaverFactories = map[string]bool{"Saver": true, "listSaver": true, "mapSaver": true, "autoSaver": true}

// acrExprIsSaver proves an expression is a runtime-saveable Saver.
func acrExprIsSaver(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "object_literal":
		return acrClassSupertypeIs(file, expr, 0, acrSaveablePackage, "Saver")
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		if acrSaverFactories[name] {
			if hasReceiver {
				return acrCallTargets(file, expr, acrSaveablePackage, name)
			}
			return acrCalleeResolvesTo(file, expr, acrSaveablePackage, name)
		}
		if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 && !hasReceiver {
			return acrClassSupertypeIs(file, class, 0, acrSaveablePackage, "Saver")
		}
	case "simple_identifier", "navigation_expression":
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return acrTypeIs(file, t, acrSaveablePackage, "Saver")
			}
			return acrExprIsSaver(file, acrInitializer(file, decl), depth+1)
		}
		if file.FlatType(expr) == "simple_identifier" {
			if obj := flatFindSameFileClassLikeDeclaration(file, file.FlatNodeText(expr)); obj != 0 &&
				file.FlatType(obj) == "object_declaration" {
				return acrClassSupertypeIs(file, obj, 0, acrSaveablePackage, "Saver")
			}
		}
	}
	return false
}

func (r *RememberSaveableSaverParameterRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if _, ok := acrRuntimeCall(file, call, acrSaveablePackage, "rememberSaveable"); !ok {
		return
	}
	var inputs []acrCallArgument
	args := acrCallArguments(file, call)
	trailing := acrHasTrailingLambda(file, call)
	for i, a := range args {
		// Without a trailing lambda the last positional argument is `init`.
		isInit := i == len(args)-1 && !trailing
		if a.label == "" && !isInit {
			inputs = append(inputs, a)
		}
	}
	if len(inputs) != 1 || inputs[0].spread || !acrExprIsSaver(file, inputs[0].expr, 0) {
		return
	}
	param := "saver"
	if last := acrTrailingLambdaResult(file, call); last != 0 && file.FlatType(last) == "call_expression" {
		if name, _, _ := acrCallCallee(file, last); acrStateFactories[name] && acrCallTargets(file, last, acrRuntimePackage, name) {
			param = "stateSaver"
		}
	}
	expr := inputs[0].expr
	ctx.Emit(scanner.Finding{
		Line: file.FlatRow(expr) + 1, Col: file.FlatCol(expr) + 1,
		StartByte: int(file.FlatStartByte(expr)), EndByte: int(file.FlatEndByte(expr)),
		Message: "Passing `Saver` instance to vararg `inputs`",
		Fix: &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(expr)), EndByte: int(file.FlatStartByte(expr)),
			Replacement: param + " = "},
	})
}
