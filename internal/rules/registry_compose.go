package rules

import (
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerComposeRules() {
	registerComposeColumnRowInScrollable()
	registerComposeDerivedStateMisuse()
	registerComposeLambdaCapturesUnstableState()
	registerComposeModifierFillAfterSize()
	registerComposeModifierBackgroundAfterClip()
	registerComposeModifierClickableBeforePadding()
	registerComposePreviewAnnotationMissing()
	registerComposeMutableDefaultArgument()
	registerComposeStringResourceInsideLambda()
	registerComposeRememberWithoutKey()
	registerComposeLaunchedEffectWithoutKeys()
	registerComposeMutableStateInComposition()
	registerComposeStatefulDefaultParameter()
	registerComposePreviewWithBackingState()
	registerComposeDisposableEffectMissingDispose()
	registerComposeModifierPassedThenChained()
	registerComposeSideEffectInComposition()
	registerComposeUnstableParameter()
	registerComposeRememberSaveableNonParcelable()
}

func registerComposeColumnRowInScrollable() {
	r := &ComposeColumnRowInScrollableRule{BaseRule: BaseRule{RuleName: "ComposeColumnRowInScrollable", RuleSetName: "compose", Sev: "warning", Desc: "Detects nested same-axis scroll containers such as Column with verticalScroll containing LazyColumn."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			call := file.FlatChild(idx, 0)
			if call == 0 || file.FlatType(call) != "call_expression" {
				return
			}
			nameNode, _ := file.FlatFindChild(call, "simple_identifier")
			if nameNode == 0 {
				return
			}
			callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
			if callSuffix == 0 {
				return
			}
			annotatedLambda, _ := file.FlatFindChild(callSuffix, "annotated_lambda")
			if annotatedLambda == 0 {
				return
			}
			lambda, _ := file.FlatFindChild(annotatedLambda, "lambda_literal")
			if lambda == 0 {
				return
			}
			if file.FlatNodeTextEquals(nameNode, "Column") {
				if !composeFlatNodeMayContain(file, call, composeVerticalScrollToken) ||
					!composeFlatNodeMayContain(file, lambda, composeLazyColumnToken) ||
					!composeFlatCallContainsCall(file, call, "verticalScroll") {
					return
				}
				if !composeFlatCallContainsCall(file, lambda, "LazyColumn") {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Column with verticalScroll should not contain LazyColumn; use a single LazyColumn for the scrollable content.")
			} else if file.FlatNodeTextEquals(nameNode, "Row") {
				if !composeFlatNodeMayContain(file, call, composeHorizontalScrollToken) ||
					!composeFlatNodeMayContain(file, lambda, composeLazyRowToken) ||
					!composeFlatCallContainsCall(file, call, "horizontalScroll") {
					return
				}
				if !composeFlatCallContainsCall(file, lambda, "LazyRow") {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Row with horizontalScroll should not contain LazyRow; use a single LazyRow for the scrollable content.")
			}
		},
	})
}

func registerComposeDerivedStateMisuse() {
	r := &ComposeDerivedStateMisuseRule{BaseRule: BaseRule{RuleName: "ComposeDerivedStateMisuse", RuleSetName: "compose", Sev: "warning", Desc: "Detects unnecessary derivedStateOf wrapping a direct boolean comparison of Compose state reads."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, LexicalCalleeNames: []string{"derivedStateOf"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "derivedStateOf" {
				return
			}
			body := composeDerivedStateBodyFlat(file, idx)
			if body == 0 {
				return
			}
			if t := file.FlatType(body); t != "comparison_expression" && t != "equality_expression" {
				return
			}
			delegatedReads, stateHolders := composeStateBindingsInScopeFlat(file, idx)
			if !composeIsDirectStateComparisonFlat(file, body, delegatedReads, stateHolders) {
				return
			}
			// A comparison reduces a changing value to fewer distinct values, which is what
			// derivedStateOf is for. It is redundant only for a Boolean state or a remember keyed on the state.
			name := composeComparedStateName(file, body, delegatedReads, stateHolders)
			if name == "" || !(composeStateIsBooleanFlat(file, idx, name) || composeRememberKeyedOnFlat(file, idx, name, delegatedReads)) {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"derivedStateOf around a direct state comparison is unnecessary; the state read already drives recomposition.")
		},
	})
}

func registerComposeLambdaCapturesUnstableState() {
	r := &ComposeLambdaCapturesUnstableStateRule{BaseRule: BaseRule{RuleName: "ComposeLambdaCapturesUnstableState", RuleSetName: "compose", Sev: "warning", Desc: "Detects inline onClick lambdas in lazy item builders that capture the current item without hoisting through remember."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"lambda_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Strong skipping remembers lambdas automatically, so the capture is harmless.
			if composeStrongSkippingEnabled(ctx.LibraryFacts) {
				return
			}
			if !composeIsNamedArgumentLambdaFlat(file, idx, "onClick") {
				return
			}
			itemsLambda := composeNearestAncestorFlat(file, idx, "lambda_literal")
			if itemsLambda == 0 || !composeLambdaBelongsToCallFlat(file, itemsLambda, "items", "itemsIndexed") {
				return
			}
			own := composeLambdaParameterNamesFlat(file, idx)
			for _, paramName := range composeLambdaParameterNamesFlat(file, itemsLambda) {
				// An onClick parameter with the same name shadows the item.
				if slices.Contains(own, paramName) {
					continue
				}
				if composeLambdaReferencesIdentifierFlat(file, idx, paramName) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"inline Compose callback captures lazy item "+paramName+"; hoist it behind remember("+paramName+") before passing it to onClick.")
					return
				}
			}
		},
	})
}

func registerComposeModifierFillAfterSize() {
	r := &ComposeModifierFillAfterSizeRule{BaseRule: BaseRule{RuleName: "ComposeModifierFillAfterSize", RuleSetName: "compose", Sev: "info", Desc: "Detects Modifier chains where fillMaxWidth/Height/Size follows size(), which makes the fillMax call a no-op."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, LexicalCalleeNames: []string{"fillMaxHeight", "fillMaxSize", "fillMaxWidth"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if _, ok := composeFillMaxAxisNames[flatCallExpressionName(file, idx)]; !ok {
				return
			}
			prev := composeChainedCallPrev(file, idx)
			if prev == 0 || flatCallExpressionName(file, prev) != "size" {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"fillMax* after size() has no effect because size() already fixes the constraints on those axes; remove one of the two.")
		},
	})
}

func registerComposeModifierBackgroundAfterClip() {
	r := &ComposeModifierBackgroundAfterClipRule{BaseRule: BaseRule{RuleName: "ComposeModifierBackgroundAfterClip", RuleSetName: "compose", Sev: "warning", Desc: "Detects Modifier chains where background() is applied before clip(), painting outside the clip shape."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, LexicalCalleeNames: []string{"clip"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "clip" {
				return
			}
			prev := composeChainedCallPrev(file, idx)
			if prev == 0 || flatCallExpressionName(file, prev) != "background" {
				return
			}
			// background(color, shape) already clips its own paint to the shape.
			if _, bgArgs := flatCallExpressionParts(file, prev); bgArgs != 0 &&
				(flatNamedValueArgument(file, bgArgs, "shape") != 0 || flatPositionalValueArgument(file, bgArgs, 1) != 0) {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"background() applied before clip() paints the un-clipped rectangle; put .clip(...) before .background(...) so the background respects the clip shape.")
		},
	})
}

func registerComposeModifierClickableBeforePadding() {
	r := &ComposeModifierClickableBeforePaddingRule{BaseRule: BaseRule{RuleName: "ComposeModifierClickableBeforePadding", RuleSetName: "compose", Sev: "info", Desc: "Detects Modifier chains where padding is applied before clickable, which leaves the padding outside the click area and ripple."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, LexicalCalleeNames: []string{"clickable"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "clickable" {
				return
			}
			prev := composeChainedCallPrev(file, idx)
			if prev == 0 || flatCallExpressionName(file, prev) != "padding" {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"padding() before clickable { } keeps the padding outside the click area and ripple; put .clickable(...) first if the padded bounds should be tappable.")
		},
	})
}

func registerComposePreviewAnnotationMissing() {
	r := &ComposePreviewAnnotationMissingRule{BaseRule: BaseRule{RuleName: "ComposePreviewAnnotationMissing", RuleSetName: "compose", Sev: "info", Desc: "Detects @Composable functions whose name ends in Preview but lack the @Preview annotation."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			if flatHasAnnotationNamed(file, idx, "Preview") {
				return
			}
			nameNode, _ := file.FlatFindChild(idx, "simple_identifier")
			if nameNode == 0 {
				return
			}
			name := file.FlatNodeText(nameNode)
			if !strings.HasSuffix(name, "Preview") {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx),
				"@Composable function named "+name+" is missing @Preview; add @Preview so Android Studio renders it in the preview pane.")
		},
	})
}

func registerComposeMutableDefaultArgument() {
	r := &ComposeMutableDefaultArgumentRule{BaseRule: BaseRule{RuleName: "ComposeMutableDefaultArgument", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable parameters with mutable collection defaults that re-evaluate on each recomposition."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			params, _ := file.FlatFindChild(idx, "function_value_parameters")
			if params == 0 {
				return
			}
			sawEquals := false
			for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
				typ := file.FlatType(child)
				switch typ {
				case "=":
					sawEquals = true
					continue
				case ",", "parameter", "(", ")":
					sawEquals = false
					continue
				}
				if !sawEquals {
					continue
				}
				sawEquals = false
				if typ != "call_expression" {
					continue
				}
				name := flatCallExpressionName(file, child)
				if _, ok := composeMutableCollectionFactories[name]; !ok {
					continue
				}
				ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
					"mutable collection default ("+name+"()) on a @Composable parameter re-evaluates each recomposition; use an immutable default like emptyList()/emptyMap() or hoist the collection to state.")
			}
		},
	})
}

func registerComposeStringResourceInsideLambda() {
	r := &ComposeStringResourceInsideLambdaRule{BaseRule: BaseRule{RuleName: "ComposeStringResourceInsideLambda", RuleSetName: "compose", Sev: "warning", Desc: "Detects stringResource() calls inside callback lambdas where the composition-only API will crash at runtime."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "stringResource" {
				return
			}
			label := composeEnclosingLambdaArgumentName(file, idx)
			if label == "" {
				return
			}
			if len(label) < 3 || label[0] != 'o' || label[1] != 'n' || label[2] < 'A' || label[2] > 'Z' {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"stringResource() is composition-only and will crash when invoked from a "+label+" callback; hoist the resource lookup above the lambda.")
		},
	})
}

func registerComposeRememberWithoutKey() {
	r := &ComposeRememberWithoutKeyRule{BaseRule: BaseRule{RuleName: "ComposeRememberWithoutKey", RuleSetName: "compose", Sev: "warning", Desc: "Detects remember blocks that reference enclosing parameters but have no keys, causing stale cached values."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallNameAny(file, idx) != "remember" {
				return
			}
			if flatCallKeyArguments(file, idx) != 0 {
				return
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			names, fn := composeLambdaReferencedParams(file, lambda)
			if fn == 0 || !flatHasAnnotationNamed(file, fn, "Composable") || composeKeyRuleExemptFunction(file, fn) {
				return
			}
			// A local computed from a parameter goes stale with it, so it counts as a captured value too.
			derived := map[string]struct{}{}
			if paramNames := flatFunctionParameterNames(file, fn); len(paramNames) > 0 {
				paramSet := make(map[string]struct{}, len(paramNames))
				for _, n := range paramNames {
					paramSet[n] = struct{}{}
				}
				derived = composeLocalsDerivedFromParams(file, fn, lambda, paramSet)
				if len(derived) > 0 {
					all := make(map[string]struct{}, len(paramSet)+len(derived))
					for n := range paramSet {
						all[n] = struct{}{}
					}
					for n := range derived {
						all[n] = struct{}{}
					}
					names = composeLambdaReferencedNames(file, lambda, all)
				}
			}
			// A state-seeding factory copies the parameter into local state on purpose.
			if composeSeedsMutableState(file, lambda) {
				return
			}
			if composeRememberDocumentedFirstCapture(file, idx, lambda) {
				return
			}
			if composeRememberIsUnkeyedFallbackArm(file, idx) {
				return
			}
			keyed := composeEnclosingKeyBlockParams(file, idx)
			variable := composeRememberedVariableName(file, idx)
			reported := map[string]bool{}
			// A factory parameter does not trigger a report alone, but it is a captured name the key list must keep.
			factories := map[string]bool{}
			paramTypes := composeParamTypeTexts(file, fn)
			for _, name := range names {
				if strings.HasPrefix(name, "initial") || strings.HasPrefix(name, "default") {
					continue
				}
				if keyed[name] || composeParamIsStableNavigator(paramTypes[name]) {
					continue
				}
				if composeParamReadOnlyInTrackingBlock(file, lambda, name, paramTypes[name]) {
					continue
				}
				if composeResyncsStateFromParam(file, fn, lambda, variable, name) {
					continue
				}
				// An invoke-only factory lambda builds a one-shot holder; keying on it would rebuild the object.
				if composeParamIsInvokeOnlyFactory(file, lambda, name, paramTypes[name]) {
					continue
				}
				if composeParamIsInjectedFactory(file, lambda, name, paramTypes[name]) {
					factories[name] = true
					continue
				}
				reported[name] = true
			}
			if len(reported) == 0 {
				return
			}
			var captured []string
			noun := "parameter"
			for _, name := range names {
				if reported[name] || factories[name] {
					captured = append(captured, name)
					if _, isDerived := derived[name]; isDerived {
						noun = "value"
					}
				}
			}
			// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
			var oracleLookup oracle.Lookup
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				oracleLookup = cr.Oracle()
			}
			if !composeRememberOracleConfirmed(oracleLookup, file, idx) {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"remember { } reads enclosing "+composeNounPlural(noun, len(captured))+" "+composeNameList(captured)+" but has no keys; the cached value won't update when "+composeChangeVerb(captured)+". Pass remember("+composeNameList(captured)+") { ... }.")
		},
	})
}

// composeRememberOracleConfirmed accepts the call when no oracle is
// available or it resolves the `remember` callee to the
// `androidx.compose.runtime.remember` family. A project-local
// `remember` function resolves to a different FQN and is filtered
// out — the AST token match alone would have flagged it. When the oracle
// is available but cannot resolve the call, stay silent (do not treat as confirmed).
func composeRememberOracleConfirmed(lookup oracle.Lookup, file *scanner.File, idx uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, idx)
	if target == "" {
		return false
	}
	return strings.HasPrefix(target, "androidx.compose.runtime.remember") ||
		strings.HasPrefix(target, "androidx.compose.runtime.RememberKt.remember")
}

func registerComposeLaunchedEffectWithoutKeys() {
	r := &ComposeLaunchedEffectWithoutKeysRule{BaseRule: BaseRule{RuleName: "ComposeLaunchedEffectWithoutKeys", RuleSetName: "compose", Sev: "warning", Desc: "Detects LaunchedEffect blocks keyed only on constants whose body reads enclosing parameters that change."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallNameAny(file, idx) != "LaunchedEffect" {
				return
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			args := flatCallKeyArguments(file, idx)
			if args == 0 {
				return
			}
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				text := file.FlatNodeText(arg)
				if _, ok := composeConstantKeyTexts[text]; !ok {
					return
				}
			}
			if fn, ok := flatEnclosingFunction(file, idx); ok && composeKeyRuleExemptFunction(file, fn) {
				return
			}
			// One-shot effects and callback reads are intentional with constant keys.
			if composeLambdaHasOnceOnlyShape(file, lambda) || composeEffectDocumentedOneShot(file, idx) {
				return
			}
			// A receiver-only parameter alone is not reported, but once one is, the fix keys on every captured parameter.
			if len(composeLambdaReadsDataParams(file, lambda, false)) == 0 {
				return
			}
			params := composeLambdaReadsDataParams(file, lambda, true)
			list := strings.Join(params, ", ")
			noun := "parameter "
			verb := " changes"
			if len(params) > 1 {
				noun, verb = "parameters ", " change"
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"LaunchedEffect body reads enclosing "+noun+list+" but is keyed only on constants; the effect won't re-run when "+list+verb+". Pass LaunchedEffect("+list+") { ... }.")
		},
	})
}

var composeMutableStateFactories = map[string]bool{
	"mutableStateOf": true, "mutableIntStateOf": true, "mutableLongStateOf": true, "mutableFloatStateOf": true,
	"mutableDoubleStateOf": true, "mutableStateListOf": true, "mutableStateMapOf": true,
}

func registerComposeMutableStateInComposition() {
	r := &ComposeMutableStateInCompositionRule{BaseRule: BaseRule{RuleName: "ComposeMutableStateInComposition", RuleSetName: "compose", Sev: "warning", Desc: "Detects mutableStateOf() used as a plain local without remember, causing state loss on every recomposition."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			fn, ok := flatEnclosingFunction(file, idx)
			if !ok || !flatHasAnnotationNamed(file, fn, "Composable") {
				return
			}
			sawEquals := false
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "=":
					sawEquals = true
					continue
				case "property_delegate":
					return
				}
				if !sawEquals {
					continue
				}
				if file.FlatType(child) != "call_expression" {
					continue
				}
				callee := flatCallExpressionName(file, child)
				if !composeMutableStateFactories[callee] {
					return
				}
				// A callback or effect body runs outside composition.
				if composeRefInDeferredLambda(file, fn, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
					callee+"() as a plain local in a @Composable creates a fresh state on every recomposition; wrap it in remember { "+callee+"(...) } or use `by remember { "+callee+"(...) }`.")
				return
			}
		},
	})
}

func registerComposeStatefulDefaultParameter() {
	r := &ComposeStatefulDefaultParameterRule{BaseRule: BaseRule{RuleName: "ComposeStatefulDefaultParameter", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable parameters with constructor-call defaults that allocate fresh instances every recomposition."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			params, _ := file.FlatFindChild(idx, "function_value_parameters")
			if params == 0 {
				return
			}
			sawEquals := false
			for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
				typ := file.FlatType(child)
				switch typ {
				case "=":
					sawEquals = true
					continue
				case ",", "parameter", "(", ")":
					sawEquals = false
					continue
				}
				if !sawEquals {
					continue
				}
				sawEquals = false
				if typ != "call_expression" {
					continue
				}
				name := flatCallExpressionName(file, child)
				if name == "" {
					continue
				}
				if !composeIsStateHolderTypeFlat(file, name) {
					continue
				}
				ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
					"default `"+name+"()` on a @Composable parameter allocates a fresh instance every recomposition; use `remember { "+name+"() }` or a `remember"+name+"()` helper so the state persists across composes.")
			}
		},
	})
}

func registerComposePreviewWithBackingState() {
	r := &ComposePreviewWithBackingStateRule{BaseRule: BaseRule{RuleName: "ComposePreviewWithBackingState", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Preview functions that call runtime state holders like hiltViewModel() or collectAsState() which fail in the preview renderer."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Preview") {
				return
			}
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
			}
			file.FlatWalkAllNodes(body, func(n uint32) {
				if file.FlatType(n) != "call_expression" {
					return
				}
				name := flatCallExpressionName(file, n)
				if _, ok := composeRuntimeStateHolderCalls[name]; !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(n)+1, file.FlatCol(n)+1,
					"@Preview body calls "+name+"() which depends on a runtime state holder; previews should render with fake/injected data so the Studio preview renderer can show them.")
			})
		},
	})
}

func registerComposeDisposableEffectMissingDispose() {
	r := &ComposeDisposableEffectMissingDisposeRule{BaseRule: BaseRule{RuleName: "ComposeDisposableEffectMissingDispose", RuleSetName: "compose", Sev: "warning", Desc: "Detects DisposableEffect blocks whose body does not end with onDispose, causing resource leaks."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallNameAny(file, idx) != "DisposableEffect" {
				return
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			stmts, _ := file.FlatFindChild(lambda, "statements")
			if stmts == 0 {
				return
			}
			var last uint32
			for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
				if file.FlatIsNamed(child) {
					last = child
				}
			}
			if last == 0 {
				return
			}
			// `return@DisposableEffect onDispose { }` ends the block the same way.
			if file.FlatType(last) == "jump_expression" {
				if v := file.FlatNamedChild(last, file.FlatNamedChildCount(last)-1); v != 0 && file.FlatType(v) == "call_expression" {
					last = v
				}
			}
			if file.FlatType(last) == "call_expression" && flatCallNameAny(file, last) == "onDispose" {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"DisposableEffect body must end with onDispose { ... }; without it the resource registered in the block leaks when the composable leaves composition.")
		},
	})
}

func registerComposeModifierPassedThenChained() {
	r := &ComposeModifierPassedThenChainedRule{BaseRule: BaseRule{RuleName: "ComposeModifierPassedThenChained", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable functions that declare a modifier parameter but never use it, starting fresh Modifier chains instead."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			if !composeHasModifierParameter(file, idx) {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
			}
			referenced := false
			file.FlatWalkAllNodes(body, func(n uint32) {
				if referenced {
					return
				}
				if file.FlatType(n) != "simple_identifier" {
					return
				}
				if !file.FlatNodeTextEquals(n, "modifier") {
					return
				}
				parent, ok := file.FlatParent(n)
				if !ok {
					return
				}
				if file.FlatType(parent) == "navigation_suffix" {
					return
				}
				referenced = true
			})
			if referenced {
				return
			}
			file.FlatWalkAllNodes(body, func(n uint32) {
				if file.FlatType(n) != "navigation_expression" {
					return
				}
				if file.FlatNamedChildCount(n) == 0 {
					return
				}
				first := file.FlatNamedChild(n, 0)
				if file.FlatType(first) != "simple_identifier" {
					return
				}
				if !file.FlatNodeTextEquals(first, "Modifier") {
					return
				}
				ctx.EmitAt(file.FlatRow(n)+1, file.FlatCol(n)+1,
					"this @Composable declares a `modifier: Modifier` parameter but the body starts a fresh `Modifier.X()` chain and never uses it; chain off the passed-in `modifier` instead so the caller's modifiers aren't dropped.")
			})
		},
	})
}

func registerComposeSideEffectInComposition() {
	r := &ComposeSideEffectInCompositionRule{
		BaseRule:              BaseRule{RuleName: "ComposeSideEffectInComposition", RuleSetName: "compose", Sev: "warning", Desc: "Detects direct assignments inside @Composable function bodies that are not wrapped in a recognized effect scope."},
		CustomPreviewWildcard: true,
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"assignment"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			fn, ok := flatEnclosingFunction(file, idx)
			if !ok || !flatHasAnnotationNamed(file, fn, "Composable") {
				return
			}
			if composeFunctionHasPreviewAnnotationWithConfig(file, fn, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
				return
			}
			if !composeFileHasRuntimeComposableEvidence(file) {
				return
			}
			// Test doubles and test sources are not production composition.
			if composeIsTestSourceOrFake(file, fn) {
				return
			}
			if !composeAssignmentIsDirectFunctionBodyStatement(file, idx, fn) {
				return
			}
			if composeAssignmentInitializesFreshRememberedState(file, idx, fn) {
				return
			}
			if composeAssignmentTargetsPlainLocal(file, idx, fn) {
				return
			}
			if composeAssignmentIsMutableTransitionTargetState(file, idx, fn) {
				return
			}
			// Writing state after reading it in the same composition is still a side effect.
			afterRead := composeAssignmentIsStateWriteAfterRead(file, idx, fn)
			if !afterRead && composeAssignmentSynchronizesRememberedObject(file, idx, fn) {
				return
			}
			if composeAssignmentCapturesComposableParameter(file, idx, fn) {
				return
			}
			if composeAssignmentDocumentedIdempotent(file, idx) {
				return
			}
			if composeAssignmentRHSIsRememberOrAnimateCall(file, idx) {
				return
			}
			if !afterRead && composeAssignmentUpdatesMutableStateValue(file, idx, fn) {
				return
			}
			subject := "assignment"
			if lhs, _, ok := composeAssignmentTextParts(file, idx); ok {
				lhs = strings.TrimSpace(lhs)
				switch {
				case strings.Contains(lhs, ".current"):
					subject = "state write through a CompositionLocal (`" + lhs + "`)"
				case strings.HasSuffix(lhs, ".value"):
					subject = "MutableState write (`" + lhs + "`)"
				case strings.HasSuffix(lhs, "]"):
					subject = "state map write (`" + lhs + "`)"
				}
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				subject+" inside a @Composable function body runs on every recomposition; wrap the mutation in LaunchedEffect / SideEffect / DisposableEffect so it only runs when intended.")
		},
	})
}

func registerComposeUnstableParameter() {
	r := &ComposeUnstableParameterRule{BaseRule: BaseRule{RuleName: "ComposeUnstableParameter", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable function parameters using mutable collection types that prevent recomposition skipping."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !flatHasAnnotationNamed(file, idx, "Composable") {
				return
			}
			if composeUnstableParameterShouldSkipFunctionFlat(file, idx) {
				return
			}
			// Strong skipping makes unstable parameters skippable, and only Unit-returning restartable composables skip at all.
			if composeStrongSkippingEnabled(ctx.LibraryFacts) || !composeIsSkippableRestartableFlat(file, idx) {
				return
			}
			params, _ := file.FlatFindChild(idx, "function_value_parameters")
			if params == 0 {
				return
			}
			for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "parameter" {
					continue
				}
				typeNode, _ := file.FlatFindChild(child, "user_type")
				if typeNode == 0 {
					continue
				}
				typeIdent, _ := file.FlatFindChild(typeNode, "type_identifier")
				if typeIdent == 0 {
					continue
				}
				name := file.FlatNodeText(typeIdent)
				if _, unstable := composeUnstableCollectionTypes[name]; !unstable {
					continue
				}
				paramName := ""
				if idNode, ok := file.FlatFindChild(child, "simple_identifier"); ok {
					paramName = file.FlatNodeText(idNode)
				}
				ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
					"@Composable parameter "+paramName+": "+name+"<...> is an unstable type for recomposition skipping; use an ImmutableList/PersistentList from kotlinx.collections.immutable or wrap the collection in an @Immutable data class.")
			}
		},
	})
}

func registerComposeRememberSaveableNonParcelable() {
	r := &ComposeRememberSaveableNonParcelableRule{BaseRule: BaseRule{RuleName: "ComposeRememberSaveableNonParcelable", RuleSetName: "compose", Sev: "warning", Desc: "Detects rememberSaveable blocks constructing non-primitive types without an explicit saver argument."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallNameAny(file, idx) != "rememberSaveable" {
				return
			}
			if args := flatCallKeyArguments(file, idx); args != 0 {
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					if file.FlatNamedChildCount(arg) < 2 {
						continue
					}
					label := file.FlatNamedChild(arg, 0)
					if label == 0 || file.FlatType(label) != "simple_identifier" {
						continue
					}
					text := file.FlatNodeText(label)
					if text == "saver" || text == "stateSaver" {
						return
					}
				}
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			stmts, _ := file.FlatFindChild(lambda, "statements")
			if stmts == 0 {
				return
			}
			var last uint32
			for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
				if file.FlatIsNamed(child) {
					last = child
				}
			}
			if last == 0 || file.FlatType(last) != "call_expression" {
				return
			}
			name := flatCallExpressionName(file, last)
			if name == "" {
				return
			}
			first := name[0]
			if first < 'A' || first > 'Z' {
				return
			}
			if _, safe := composeSaverSafeBuiltins[name]; safe {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx),
				"rememberSaveable { "+name+"() } with no `saver`/`stateSaver` falls back to AutoSaver, which only handles primitives and Parcelable/Serializable. If "+name+" isn't @Parcelize, pass an explicit saver; otherwise suppress this warning.")
		},
	})
}
