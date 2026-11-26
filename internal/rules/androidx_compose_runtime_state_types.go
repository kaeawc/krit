package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// AutoboxingStateValuePropertyRule ports runtime-lint's AutoboxingStateValueProperty: the generic `value` of a
// primitive-specialized State (IntState, MutableLongState, ...) read or written, which boxes the value.
type AutoboxingStateValuePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// AutoboxingStateCreationRule ports runtime-lint's AutoboxingStateCreation: `mutableStateOf` of a non-null Int,
// Long, Float or Double, where the specialized factory avoids boxing.
type AutoboxingStateCreationRule struct {
	FlatDispatchBase
	BaseRule
}

// MutableCollectionMutableStateRule ports runtime-lint's MutableCollectionMutableState: `mutableStateOf` holding
// a mutable collection, whose in-place mutations the state cannot observe.
type MutableCollectionMutableStateRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports medium confidence: the state type is proven from same-file declarations and literals.
func (r *AutoboxingStateValuePropertyRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the state type is proven from type arguments, declarations and literals.
func (r *AutoboxingStateCreationRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the collection type is proven from type arguments, declarations and calls.
func (r *MutableCollectionMutableStateRule) Confidence() float64 { return api.ConfidenceMedium }

// fixLevel declares the `value` -> `intValue` rewrite, which reads and writes the same state without boxing.
func (r *AutoboxingStateValuePropertyRule) fixLevel() api.FixLevel { return api.FixIdiomatic }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *AutoboxingStateValuePropertyRule) fileMentions() []string {
	return []string{acrRuntimePackage}
}

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *AutoboxingStateCreationRule) fileMentions() []string { return []string{acrRuntimePackage} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *MutableCollectionMutableStateRule) fileMentions() []string {
	return []string{acrRuntimePackage}
}

func (r *AutoboxingStateValuePropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AutoboxingStateValueProperty", RuleSet: "compose", DefaultActive: true,
		FixLevel: "idiomatic",
		KnownLimitations: []string{
			"The receiver must be proven to be IntState/LongState/FloatState/DoubleState (or the Mutable variants) by a same-file declaration or initializer (mutableIntStateOf, remember { mutableIntStateOf(...) }, asIntState(), ...).",
		}}
}

func (r *AutoboxingStateCreationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AutoboxingStateCreation", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The state type is taken from an explicit type argument, the declared type of the property it initializes, or a literal or same-file typed argument; other inferred types are not checked.",
		}}
}

func (r *MutableCollectionMutableStateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MutableCollectionMutableState", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The collection type is taken from an explicit type argument, the declared type of the property it initializes, or a mutable-collection factory, constructor or same-file declaration passed as the value.",
		}}
}

func registerAndroidxComposeRuntimeStateTypeRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	value := &AutoboxingStateValuePropertyRule{BaseRule: BaseRule{RuleName: "AutoboxingStateValueProperty", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects the generic value property of IntState, LongState, FloatState or DoubleState, which autoboxes; use intValue and friends."}}
	registerAcrRule(value.BaseRule, value, []string{"navigation_expression", "directly_assignable_expression"}, kotlin, value.check)

	creation := &AutoboxingStateCreationRule{BaseRule: BaseRule{RuleName: "AutoboxingStateCreation", RuleSetName: "compose",
		Sev: "info", Desc: "Detects mutableStateOf holding a non-null Int, Long, Float or Double, where mutableIntStateOf and friends avoid autoboxing."}}
	registerAcrRule(creation.BaseRule, creation, []string{"call_expression"}, kotlin, creation.check)

	collection := &MutableCollectionMutableStateRule{BaseRule: BaseRule{RuleName: "MutableCollectionMutableState",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects mutableStateOf holding a mutable collection, whose in-place changes the state does not observe."}}
	registerAcrRule(collection.BaseRule, collection, []string{"call_expression"}, kotlin, collection.check)
}

// acrPrimitiveStates maps the androidx.compose.runtime primitive state types to their specialized property.
var acrPrimitiveStates = map[string]string{
	"IntState": "intValue", "MutableIntState": "intValue", "LongState": "longValue", "MutableLongState": "longValue",
	"FloatState": "floatValue", "MutableFloatState": "floatValue", "DoubleState": "doubleValue",
	"MutableDoubleState": "doubleValue",
}

// acrPrimitiveStateFactories maps the runtime functions that return a primitive state to its specialized property.
var acrPrimitiveStateFactories = map[string]string{
	"mutableIntStateOf": "intValue", "mutableLongStateOf": "longValue", "mutableFloatStateOf": "floatValue",
	"mutableDoubleStateOf": "doubleValue", "asIntState": "intValue", "asLongState": "longValue",
	"asFloatState": "floatValue", "asDoubleState": "doubleValue",
}

func (r *AutoboxingStateValuePropertyRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	suffix := flatLastNamedChild(file, node)
	if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
		return
	}
	valueNode, _ := file.FlatFindChild(suffix, "simple_identifier")
	if valueNode == 0 || !file.FlatNodeTextEquals(valueNode, "value") {
		return
	}
	var preferred, access string
	if file.FlatType(node) == "directly_assignable_expression" {
		// Lint reports `state.value = x` but not compound assignments such as `state.value += x`.
		if assignment, ok := file.FlatParent(node); !ok || !file.FlatHasChildOfType(assignment, "=") {
			return
		}
		if decl := acrAssignableReceiverDeclaration(file, node, suffix); decl != 0 {
			preferred = acrDeclPrimitiveState(file, decl, 0)
		}
		access = "Assigning"
	} else {
		if parent, ok := file.FlatParent(node); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == node {
			return
		}
		preferred = acrExprPrimitiveState(file, file.FlatFirstChild(node), 0)
		access = "Reading"
		if parent, ok := file.FlatParent(node); ok {
			if t := file.FlatType(parent); t == "postfix_expression" || t == "prefix_expression" {
				if op := acrIncrementOperator(file, parent); op {
					access = "Assigning"
				}
			}
		}
	}
	if preferred == "" {
		return
	}
	ctx.Emit(scanner.Finding{
		Line: file.FlatRow(valueNode) + 1, Col: file.FlatCol(valueNode) + 1,
		StartByte: int(file.FlatStartByte(valueNode)), EndByte: int(file.FlatEndByte(valueNode)),
		Message: access + " `value` will cause an autoboxing operation. Use `" + preferred + "` to avoid unnecessary allocations.",
		Fix: &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(valueNode)),
			EndByte: int(file.FlatEndByte(valueNode)), Replacement: preferred},
	})
}

// acrIncrementOperator reports whether a prefix or postfix expression applies ++ or --.
func acrIncrementOperator(file *scanner.File, expr uint32) bool {
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if t := file.FlatType(child); t == "++" || t == "--" {
			return true
		}
	}
	return false
}

// acrExprPrimitiveState returns the specialized property name when an expression is proven to be a primitive
// runtime state, or "".
func acrExprPrimitiveState(file *scanner.File, expr uint32, depth int) string {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return ""
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression":
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			return acrDeclPrimitiveState(file, decl, depth)
		}
		if file.FlatType(expr) == "simple_identifier" {
			if recv := acrScopeLambdaReceiver(file, expr); recv != 0 {
				return acrExprPrimitiveState(file, recv, depth+1)
			}
		}
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, expr) {
			return ""
		}
		name, _, hasReceiver := acrCallCallee(file, expr)
		if preferred, ok := acrPrimitiveStateFactories[name]; ok && acrCallResolvesToRuntime(file, expr, name, hasReceiver) {
			return preferred
		}
		if (name == "remember" || name == "rememberSaveable") && !hasReceiver {
			pkg := acrRuntimePackage
			if name == "rememberSaveable" {
				pkg += ".saveable"
			}
			if acrCalleeResolvesTo(file, expr, pkg, name) {
				if last := acrTrailingLambdaResult(file, expr); last != 0 {
					return acrExprPrimitiveState(file, last, depth+1)
				}
			}
		}
	}
	return ""
}

// acrCallResolvesToRuntime reports whether a call of name targets androidx.compose.runtime: a simple or
// qualified call, or an extension call (asIntState) whose name resolves there.
func acrCallResolvesToRuntime(file *scanner.File, call uint32, name string, hasReceiver bool) bool {
	if acrCallTargets(file, call, acrRuntimePackage, name) {
		return true
	}
	return hasReceiver && strings.HasPrefix(name, "as") && !acrFileDeclaresAnyFunction(file, name) &&
		acrNameResolvesTo(file, acrRuntimePackage, name)
}

// acrDeclPrimitiveState classifies a declaration by its declared type or initializer.
func acrDeclPrimitiveState(file *scanner.File, decl uint32, depth int) string {
	if t := acrDeclaredType(file, decl); t != 0 {
		for name, preferred := range acrPrimitiveStates {
			if acrTypeIs(file, t, acrRuntimePackage, name) {
				return preferred
			}
		}
		return ""
	}
	return acrExprPrimitiveState(file, acrInitializer(file, decl), depth+1)
}

// acrTrailingLambdaResult returns the last statement of a call's trailing lambda, or 0.
func acrTrailingLambdaResult(file *scanner.File, call uint32) uint32 {
	suffix := file.FlatNextSib(file.FlatFirstChild(call))
	if suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return 0
	}
	annotated, ok := file.FlatFindChild(suffix, "annotated_lambda")
	if !ok {
		return 0
	}
	lambda, ok := file.FlatFindChild(annotated, "lambda_literal")
	if !ok {
		return 0
	}
	stmts, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return 0
	}
	return flatLastNamedChild(file, stmts)
}

// acrMutableStateOfCall reports whether call is androidx.compose.runtime.mutableStateOf.
func acrMutableStateOfCall(file *scanner.File, call uint32) bool {
	if acrIsInnerCallOfTrailingLambda(file, call) {
		return false
	}
	name, _, _ := acrCallCallee(file, call)
	return name == "mutableStateOf" && acrCallTargets(file, call, acrRuntimePackage, name)
}

// acrStateTypeEvidence is what the source proves about the T of a mutableStateOf<T>(...) call.
type acrStateTypeEvidence struct {
	// typeNode is T's type node when it is written out: an explicit type argument, the argument of the declared
	// MutableState<T>/State<T> type the call initializes, or the declared type of the delegated property.
	typeNode uint32
	// inferred is true when nothing in the context constrains T, so it is the value argument's type.
	inferred bool
}

// acrStateTypeOf finds the evidence for a mutableStateOf call's type argument.
func acrStateTypeOf(file *scanner.File, call uint32) acrStateTypeEvidence {
	if args := acrCallTypeArguments(file, call); args != 0 {
		if proj := flatFirstNamedChild(file, args); proj != 0 && file.FlatType(proj) == "type_projection" {
			return acrStateTypeEvidence{typeNode: flatFirstNamedChild(file, proj)}
		}
		return acrStateTypeEvidence{}
	}
	node := call
	for depth := 0; depth < 4; depth++ {
		parent, ok := file.FlatParent(node)
		if !ok {
			return acrStateTypeEvidence{}
		}
		switch file.FlatType(parent) {
		case "property_declaration":
			if acrInitializer(file, parent) != node {
				return acrStateTypeEvidence{}
			}
			declared := acrDeclaredType(file, parent)
			if declared == 0 {
				return acrStateTypeEvidence{inferred: true}
			}
			if acrTypeIs(file, declared, acrRuntimePackage, "MutableState", "State") {
				return acrStateTypeEvidence{typeNode: acrFirstTypeArgument(file, declared)}
			}
			return acrStateTypeEvidence{}
		case "property_delegate":
			prop, ok := file.FlatParent(parent)
			if !ok {
				return acrStateTypeEvidence{}
			}
			if declared := acrDeclaredType(file, prop); declared != 0 {
				return acrStateTypeEvidence{typeNode: declared}
			}
			return acrStateTypeEvidence{inferred: true}
		case "statements":
			// The last statement of a remember { } lambda is the remembered value; so is the last
			// statement of a lambda passed to a same-file `fun <T> f(block: () -> T): T` helper.
			lambda, ok := file.FlatParent(parent)
			if !ok || file.FlatType(lambda) != "lambda_literal" || flatLastNamedChild(file, parent) != node {
				return acrStateTypeEvidence{}
			}
			arg, ok := acrLambdaArgumentOf(file, lambda)
			if !ok || !arg.Trailing {
				return acrStateTypeEvidence{}
			}
			name, _, hasReceiver := acrCallCallee(file, arg.Call)
			if hasReceiver || acrCallTypeArguments(file, arg.Call) != 0 {
				return acrStateTypeEvidence{}
			}
			isRemember := name == "remember" && acrCalleeResolvesTo(file, arg.Call, acrRuntimePackage, name)
			if !isRemember && !acrGenericLambdaHelper(file, arg, name) {
				return acrStateTypeEvidence{}
			}
			node = arg.Call
			continue
		case "value_argument":
			return acrArgumentStateType(file, node)
		}
		return acrStateTypeEvidence{}
	}
	return acrStateTypeEvidence{}
}

// acrGenericLambdaHelper reports whether arg passes a lambda to a same-file `fun <T> name(block: () -> T): T`
// (or `R.() -> T`), so the lambda's last expression is the call's value, as with remember { } (#1612).
func acrGenericLambdaHelper(file *scanner.File, arg acrLambdaArgument, name string) bool {
	fn := acrSameFileCallee(file, arg.Call, name)
	if fn == 0 || !acrCallArityFits(file, fn, arg.Call) {
		return false
	}
	typeParam := acrReturnTypeParameter(file, fn)
	if typeParam == "" {
		return false
	}
	param := acrParameterForArgument(file, fn, arg)
	if param == 0 {
		return false
	}
	fnType := acrDeclaredType(file, param)
	if fnType == 0 || file.FlatType(fnType) != "function_type" {
		return false
	}
	ret := flatLastNamedChild(file, fnType)
	return ret != 0 && file.FlatType(ret) == "user_type" && strings.TrimSpace(file.FlatNodeText(ret)) == typeParam
}

// acrArgumentStateType finds a mutableStateOf call's type argument when the call is a value argument of a
// same-file function: the parameter's MutableState/State type argument, or inferred from the value when the
// parameter is Any/Any? or an unbounded type parameter, which leave T unconstrained (#1612).
func acrArgumentStateType(file *scanner.File, call uint32) acrStateTypeEvidence {
	arg, ok := acrLambdaArgumentOf(file, call)
	if !ok || arg.Trailing {
		return acrStateTypeEvidence{}
	}
	name, _, hasReceiver := acrCallCallee(file, arg.Call)
	if hasReceiver || acrCallTypeArguments(file, arg.Call) != 0 {
		return acrStateTypeEvidence{}
	}
	fn := acrSameFileCallee(file, arg.Call, name)
	if fn == 0 || !acrCallArityFits(file, fn, arg.Call) {
		return acrStateTypeEvidence{}
	}
	param := acrParameterForArgument(file, fn, arg)
	if param == 0 || acrParameterHasModifier(file, param, "vararg") {
		return acrStateTypeEvidence{}
	}
	typ := acrDeclaredType(file, param)
	if typ == 0 {
		return acrStateTypeEvidence{}
	}
	if file.FlatType(typ) == "nullable_type" {
		typ = flatFirstNamedChild(file, typ)
	}
	if acrTypeIs(file, typ, acrRuntimePackage, "MutableState", "State") {
		return acrStateTypeEvidence{typeNode: acrFirstTypeArgument(file, typ)}
	}
	text := strings.TrimSpace(file.FlatNodeText(typ))
	if text == "Any" && flatFindSameFileClassLikeDeclaration(file, "Any") == 0 || acrUnboundedTypeParameter(file, fn, text) {
		return acrStateTypeEvidence{inferred: true}
	}
	return acrStateTypeEvidence{}
}

// acrSameFileCallee returns the same-file function a receiverless call to name at call resolves to: among
// declarations whose container (block, class body or file) encloses the call, the innermost, as Kotlin
// prefers locals over members over top-level functions. It returns 0 when there is none, when the innermost
// level overloads the name, or when the file imports the name explicitly.
func acrSameFileCallee(file *scanner.File, call uint32, name string) uint32 {
	if name == "" {
		return 0
	}
	if facts := fileFactsCache().Imports(file); facts != nil {
		if _, imported := facts.Aliases[name]; imported {
			return 0
		}
	}
	if acrImplicitReceiverMayShadow(file, call) {
		return 0
	}
	var best uint32
	bestDepth, count := -1, 0
	for _, decl := range acrFunctionDecls(file, name) {
		container, ok := file.FlatParent(decl)
		if !ok {
			continue
		}
		depth := acrAncestorDepth(file, container, call)
		if depth < 0 {
			continue
		}
		switch {
		case depth > bestDepth:
			best, bestDepth, count = decl, depth, 1
		case depth == bestDepth:
			count++
		}
	}
	if count != 1 {
		return 0
	}
	return best
}

// acrImplicitReceiverMayShadow reports whether call sits where an implicit receiver's members, which Kotlin
// resolves before outer and top-level functions, may declare the callee: inside an extension function, a lambda of
// with/apply/run or of a same-file function taking a receiver function type, or a class or object with supertypes
// whose inherited members are unknown here (#1751 review).
func acrImplicitReceiverMayShadow(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			if flatFunctionReceiverTypeName(file, p) != "" {
				return true
			}
		case "class_declaration", "object_declaration", "companion_object", "object_literal":
			if _, ok := file.FlatFindChild(p, "delegation_specifier"); ok {
				return true
			}
			if specs, ok := file.FlatFindChild(p, "delegation_specifiers"); ok && specs != 0 {
				return true
			}
		case "lambda_literal":
			if acrReceiverLambda(file, p) {
				return true
			}
		}
	}
	return false
}

// acrReceiverLambda reports a lambda whose body has an implicit receiver: the block of with, apply or run, or a
// lambda passed to a same-file function parameter of a receiver function type (`T.() -> R`).
func acrReceiverLambda(file *scanner.File, lambda uint32) bool {
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok {
		return false
	}
	name, _, _ := acrCallCallee(file, arg.Call)
	switch name {
	case "with", "apply", "run":
		return true
	}
	fn := acrSameFileFunction(file, name)
	if fn == 0 {
		return false
	}
	param := acrParameterForArgument(file, fn, arg)
	if param == 0 {
		return false
	}
	typ := acrDeclaredType(file, param)
	return typ != 0 && file.FlatType(typ) == "function_type" && strings.Contains(file.FlatNodeText(typ), ".(")
}

// acrCallArityFits reports whether fn's value parameters can take the call's arguments.
func acrCallArityFits(file *scanner.File, fn, call uint32) bool {
	params := acrFunctionParameters(file, fn)
	required, vararg := 0, false
	for _, p := range params {
		switch {
		case acrParameterHasModifier(file, p, "vararg"):
			vararg = true
		case !acrParameterHasDefault(file, p):
			required++
		}
	}
	args := acrCallArgumentCount(file, call)
	return args >= required && (vararg || args <= len(params))
}

// acrParameterHasDefault reports whether a function parameter is followed by `= default`.
func acrParameterHasDefault(file *scanner.File, param uint32) bool {
	for next := file.FlatNextSib(param); next != 0; next = file.FlatNextSib(next) {
		switch file.FlatType(next) {
		case ",", ")":
			return false
		case "=":
			return true
		}
	}
	return false
}

// acrCallArgumentCount counts a call's value arguments, including a trailing lambda.
func acrCallArgumentCount(file *scanner.File, call uint32) int {
	n := 0
	file.FlatWalkNodes(call, "value_argument", func(arg uint32) {
		if parent, ok := file.FlatParent(arg); ok {
			if suffix, ok := file.FlatParent(parent); ok {
				if owner, ok := file.FlatParent(suffix); ok && (owner == call || file.FlatFirstChild(call) == owner) {
					n++
				}
			}
		}
	})
	file.FlatWalkNodes(call, "annotated_lambda", func(l uint32) {
		if suffix, ok := file.FlatParent(l); ok {
			if owner, ok := file.FlatParent(suffix); ok && owner == call {
				n++
			}
		}
	})
	return n
}

// acrAncestorDepth returns the depth of container below the file root when it is node's ancestor (or the
// source file), else -1.
func acrAncestorDepth(file *scanner.File, container, node uint32) int {
	if file.FlatType(container) == "source_file" {
		return 0
	}
	found := false
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		if p == container {
			found = true
			break
		}
	}
	if !found {
		return -1
	}
	depth := 0
	for p, ok := file.FlatParent(container); ok; p, ok = file.FlatParent(p) {
		depth++
	}
	return depth
}

// acrReturnTypeParameter returns T when fn declares `<T>` and returns exactly T, else "".
func acrReturnTypeParameter(file *scanner.File, fn uint32) string {
	params := acrTypeParameterNames(file, fn)
	if len(params) == 0 {
		return ""
	}
	seenParams := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters":
			seenParams = true
		case "user_type":
			if !seenParams {
				continue
			}
			name := strings.TrimSpace(file.FlatNodeText(child))
			if params[name] {
				return name
			}
			return ""
		case "function_body":
			return ""
		}
	}
	return ""
}

// acrUnboundedTypeParameter reports whether name is a type parameter of fn without an upper bound.
func acrUnboundedTypeParameter(file *scanner.File, fn uint32, name string) bool {
	tps, ok := file.FlatFindChild(fn, "type_parameters")
	if !ok {
		return false
	}
	for tp := file.FlatFirstChild(tps); tp != 0; tp = file.FlatNextSib(tp) {
		if file.FlatType(tp) != "type_parameter" || file.FlatChildTextOrEmpty(tp, "type_identifier") != name {
			continue
		}
		_, bounded := file.FlatFindChild(tp, "user_type")
		_, nullableBound := file.FlatFindChild(tp, "nullable_type")
		return !bounded && !nullableBound && !acrHasTypeConstraint(file, fn, name)
	}
	return false
}

// acrTypeParameterNames returns the names of fn's type parameters.
func acrTypeParameterNames(file *scanner.File, fn uint32) map[string]bool {
	tps, ok := file.FlatFindChild(fn, "type_parameters")
	if !ok {
		return nil
	}
	names := map[string]bool{}
	for tp := file.FlatFirstChild(tps); tp != 0; tp = file.FlatNextSib(tp) {
		if file.FlatType(tp) == "type_parameter" {
			if name := file.FlatChildTextOrEmpty(tp, "type_identifier"); name != "" {
				names[name] = true
			}
		}
	}
	return names
}

// acrHasTypeConstraint reports a `where name : Bound` clause on fn.
func acrHasTypeConstraint(file *scanner.File, fn uint32, name string) bool {
	constraints, ok := file.FlatFindChild(fn, "type_constraints")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(constraints, "type_constraint", func(c uint32) {
		if !found && file.FlatChildTextOrEmpty(c, "type_identifier") == name {
			found = true
		}
	})
	return found
}

// acrCallTypeArguments returns the explicit type_arguments of a call (`f<T>(...)`), or 0.
func acrCallTypeArguments(file *scanner.File, call uint32) uint32 {
	inner := call
	for {
		first := file.FlatFirstChild(inner)
		if first == 0 || file.FlatType(first) != "call_expression" || !acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(first)) {
			break
		}
		inner = first
	}
	suffix := file.FlatNextSib(file.FlatFirstChild(inner))
	if suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return 0
	}
	args, _ := file.FlatFindChild(suffix, "type_arguments")
	return args
}

// acrFirstTypeArgument returns the first type argument's type node of `Foo<T>`, or 0.
func acrFirstTypeArgument(file *scanner.File, typeNode uint32) uint32 {
	if file.FlatType(typeNode) == "nullable_type" {
		typeNode = flatFirstNamedChild(file, typeNode)
	}
	args, ok := file.FlatFindChild(typeNode, "type_arguments")
	if !ok {
		return 0
	}
	proj := flatFirstNamedChild(file, args)
	if proj == 0 || file.FlatType(proj) != "type_projection" {
		return 0
	}
	return flatFirstNamedChild(file, proj)
}

// acrCallValueArgument returns the expression of the positional (or `value =`) first argument of a call.
func acrCallValueArgument(file *scanner.File, call uint32, index int, label string) uint32 {
	inner := call
	if first := file.FlatFirstChild(call); first != 0 && file.FlatType(first) == "call_expression" &&
		acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(first)) {
		inner = first
	}
	suffix := file.FlatNextSib(file.FlatFirstChild(inner))
	if suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return 0
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return 0
	}
	position := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if l := flatValueArgumentLabel(file, arg); l != "" {
			if l == label {
				return flatValueArgumentExpression(file, arg)
			}
			continue
		}
		if position == index {
			return flatValueArgumentExpression(file, arg)
		}
		position++
	}
	return 0
}

// acrDefaultImported reports whether a simple Kotlin default-import name (Int, MutableList, ...) is not rebound by
// an import or a same-file top-level declaration.
func acrDefaultImported(file *scanner.File, name string) bool {
	if facts := fileFactsCache().Imports(file); facts != nil {
		if _, ok := facts.Aliases[name]; ok {
			return false
		}
	}
	return !acrFileDeclaresTopLevel(file, name)
}

// acrKotlinPrimitive returns "Int", "Long", "Float" or "Double" when a type node is that non-null kotlin type.
func acrKotlinPrimitive(file *scanner.File, typeNode uint32) string {
	if typeNode == 0 || file.FlatType(typeNode) != "user_type" {
		return ""
	}
	name := acrTypeNodeName(file, typeNode)
	short := strings.TrimPrefix(name, "kotlin.")
	switch short {
	case "Int", "Long", "Float", "Double":
		if name == "kotlin."+short || acrDefaultImported(file, short) {
			return short
		}
	}
	return ""
}

// acrLiteralPrimitive returns the kotlin type of a numeric literal argument (Int, Long, Float, Double), or "".
func acrLiteralPrimitive(file *scanner.File, expr uint32) string {
	expr = acrUnwrapParens(file, expr)
	if expr != 0 && file.FlatType(expr) == "prefix_expression" {
		op := file.FlatFirstChild(expr)
		if t := file.FlatType(op); t != "-" && t != "+" {
			return ""
		}
		expr = flatLastNamedChild(file, expr)
	}
	switch file.FlatType(expr) {
	case "integer_literal", "hex_literal", "bin_literal":
		return "Int"
	case "long_literal":
		return "Long"
	case "real_literal":
		if text := file.FlatNodeText(expr); strings.HasSuffix(text, "f") || strings.HasSuffix(text, "F") {
			return "Float"
		}
		return "Double"
	}
	return ""
}

// acrUsesStructuralPolicy mirrors usesStructuralEqualityPolicy: no policy argument, or a direct
// structuralEqualityPolicy() call.
func acrUsesStructuralPolicy(file *scanner.File, call uint32) bool {
	policy := acrCallValueArgument(file, call, 1, "policy")
	if policy == 0 {
		return true
	}
	policy = acrUnwrapParens(file, policy)
	return file.FlatType(policy) == "call_expression" && acrCallTargets(file, policy, acrRuntimePackage, "structuralEqualityPolicy")
}

func (r *AutoboxingStateCreationRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrMutableStateOfCall(file, call) || !acrUsesStructuralPolicy(file, call) {
		return
	}
	evidence := acrStateTypeOf(file, call)
	primitive := ""
	switch {
	case evidence.typeNode != 0:
		primitive = acrKotlinPrimitive(file, evidence.typeNode)
	case evidence.inferred:
		value := acrCallValueArgument(file, call, 0, "value")
		if primitive = acrLiteralPrimitive(file, value); primitive == "" {
			if decl := acrExprDeclaration(file, value); decl != 0 {
				primitive = acrKotlinPrimitive(file, acrDeclaredType(file, decl))
			}
		}
	}
	if primitive == "" {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "Prefer `mutable"+primitive+"StateOf` instead of `mutableStateOf`")
}

// acrMutableCollectionTypes are the kotlin.collections (default-imported) and java.util mutable collection types.
var acrMutableCollectionTypes = map[string]bool{
	"MutableList": true, "MutableSet": true, "MutableMap": true, "MutableCollection": true,
	"ArrayList": true, "HashMap": true, "HashSet": true, "LinkedHashMap": true, "LinkedHashSet": true,
}

// acrJavaMutableCollectionTypes are further java.util collection classes, which need an import.
var acrJavaMutableCollectionTypes = []string{
	"ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet", "LinkedList", "TreeMap", "TreeSet",
	"ArrayDeque", "Vector", "Stack", "List", "Set", "Map", "Collection", "Deque", "Queue", "SortedMap", "SortedSet",
	"NavigableMap", "NavigableSet",
}

// acrMutableCollectionFactories are kotlin.collections functions returning a mutable collection.
var acrMutableCollectionFactories = map[string]bool{
	"mutableListOf": true, "mutableSetOf": true, "mutableMapOf": true, "arrayListOf": true, "hashSetOf": true,
	"hashMapOf": true, "linkedSetOf": true, "linkedMapOf": true, "sortedSetOf": true, "sortedMapOf": true,
	"toMutableList": true, "toMutableSet": true, "toMutableMap": true,
}

// acrTypeIsMutableCollection reports whether a type node is a mutable collection type.
func acrTypeIsMutableCollection(file *scanner.File, typeNode uint32) bool {
	if typeNode == 0 {
		return false
	}
	if file.FlatType(typeNode) == "nullable_type" {
		typeNode = flatFirstNamedChild(file, typeNode)
	}
	name := acrTypeNodeName(file, typeNode)
	if name == "" {
		return false
	}
	short := name[strings.LastIndexByte(name, '.')+1:]
	switch {
	case strings.HasPrefix(name, "kotlin.collections."):
		return acrMutableCollectionTypes[short]
	case strings.HasPrefix(name, "java.util."):
		return acrIsJavaMutableCollection(short)
	case strings.Contains(name, "."):
		return false
	}
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			pkg, cls := fqn[:max(strings.LastIndexByte(fqn, '.'), 0)], fqn[strings.LastIndexByte(fqn, '.')+1:]
			return pkg == "java.util" && acrIsJavaMutableCollection(cls) ||
				pkg == "kotlin.collections" && acrMutableCollectionTypes[cls]
		}
	}
	if acrMutableCollectionTypes[name] && acrDefaultImported(file, name) {
		return true
	}
	if class := acrSameFileClass(file, typeNode); class != 0 {
		return acrClassSupertypeIsMutableCollection(file, class, 0)
	}
	return false
}

func acrIsJavaMutableCollection(name string) bool {
	for _, n := range acrJavaMutableCollectionTypes {
		if n == name {
			return true
		}
	}
	return false
}

// acrClassSupertypeIsMutableCollection reports whether a same-file class extends a mutable collection type.
func acrClassSupertypeIsMutableCollection(file *scanner.File, class uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "delegation_specifier" &&
			acrTypeIsMutableCollection(file, acrDelegationSpecifierType(file, child)) {
			return true
		}
	}
	return false
}

// acrExprIsMutableCollection proves a value is a mutable collection: a factory or constructor call, or a
// same-file declaration typed or initialized as one.
func acrExprIsMutableCollection(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		if acrMutableCollectionFactories[name] {
			return !acrFileDeclaresAnyFunction(file, name) && (hasReceiver || acrDefaultImported(file, name))
		}
		if hasReceiver {
			callee := acrCalleeExpression(file, expr)
			text := strings.Join(strings.Fields(file.FlatNodeText(callee)), "")
			if cls, ok := strings.CutPrefix(text, "java.util."); ok {
				return acrIsJavaMutableCollection(cls)
			}
			return false
		}
		if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 {
			return acrClassSupertypeIsMutableCollection(file, class, 0)
		}
		if facts := fileFactsCache().Imports(file); facts != nil {
			if fqn, ok := facts.Aliases[name]; ok {
				return strings.HasPrefix(fqn, "java.util.") && acrIsJavaMutableCollection(strings.TrimPrefix(fqn, "java.util."))
			}
		}
		// ArrayList(), HashMap(), ... are kotlin.collections type aliases of the java.util classes.
		return acrMutableCollectionTypes[name] && !strings.HasPrefix(name, "Mutable") && acrDefaultImported(file, name)
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return false
		}
		if t := acrDeclaredType(file, decl); t != 0 {
			return acrTypeIsMutableCollection(file, t)
		}
		return acrExprIsMutableCollection(file, acrInitializer(file, decl), depth+1)
	}
	return false
}

func (r *MutableCollectionMutableStateRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrMutableStateOfCall(file, call) {
		return
	}
	evidence := acrStateTypeOf(file, call)
	mutable := false
	switch {
	case evidence.typeNode != 0:
		mutable = acrTypeIsMutableCollection(file, evidence.typeNode)
	case evidence.inferred:
		mutable = acrExprIsMutableCollection(file, acrCallValueArgument(file, call, 0, "value"), 0)
	}
	if !mutable {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "Creating a MutableState object with a mutable collection type")
}
