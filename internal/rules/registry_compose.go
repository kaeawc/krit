package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerComposeRules() {

	// --- from compose.go ---
	{
		r := &ComposeColumnRowInScrollableRule{BaseRule: BaseRule{RuleName: "ComposeColumnRowInScrollable", RuleSetName: "compose", Sev: "warning", Desc: "Detects nested same-axis scroll containers such as Column with verticalScroll containing LazyColumn."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
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
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Row with horizontalScroll should not contain LazyRow; use a single LazyRow for the scrollable content.")
				}
			},
		})
	}
	{
		r := &ComposeDerivedStateMisuseRule{BaseRule: BaseRule{RuleName: "ComposeDerivedStateMisuse", RuleSetName: "compose", Sev: "warning", Desc: "Detects unnecessary derivedStateOf wrapping a direct boolean comparison of Compose state reads."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"derivedStateOf around a direct state comparison is unnecessary; the state read already drives recomposition.")
			},
		})
	}
	{
		r := &ComposeLambdaCapturesUnstableStateRule{BaseRule: BaseRule{RuleName: "ComposeLambdaCapturesUnstableState", RuleSetName: "compose", Sev: "warning", Desc: "Detects inline onClick lambdas in lazy item builders that capture the current item without hoisting through remember."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !composeIsNamedArgumentLambdaFlat(file, idx, "onClick") {
					return
				}
				itemsLambda := composeNearestAncestorFlat(file, idx, "lambda_literal")
				if itemsLambda == 0 || !composeLambdaBelongsToCallFlat(file, itemsLambda, "items", "itemsIndexed") {
					return
				}
				for _, paramName := range composeLambdaParameterNamesFlat(file, itemsLambda) {
					if composeLambdaReferencesIdentifierFlat(file, idx, paramName) {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							"inline Compose callback captures the current lazy item; hoist it behind remember(item) before passing it to onClick.")
						return
					}
				}
			},
		})
	}
	{
		r := &ComposeModifierFillAfterSizeRule{BaseRule: BaseRule{RuleName: "ComposeModifierFillAfterSize", RuleSetName: "compose", Sev: "info", Desc: "Detects Modifier chains where fillMaxWidth/Height/Size follows size(), overriding the explicit size."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if _, ok := composeFillMaxAxisNames[flatCallExpressionName(file, idx)]; !ok {
					return
				}
				prev := composeChainedCallPrev(file, idx)
				if prev == 0 || flatCallExpressionName(file, prev) != "size" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"fillMax* after size() overrides the explicit size on one or both axes; drop the size() or swap the order so fillMax*() is applied first.")
			},
		})
	}
	{
		r := &ComposeModifierBackgroundAfterClipRule{BaseRule: BaseRule{RuleName: "ComposeModifierBackgroundAfterClip", RuleSetName: "compose", Sev: "warning", Desc: "Detects Modifier chains where background() is applied before clip(), painting outside the clip shape."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "clip" {
					return
				}
				prev := composeChainedCallPrev(file, idx)
				if prev == 0 || flatCallExpressionName(file, prev) != "background" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"background() applied before clip() paints the un-clipped rectangle; put .clip(...) before .background(...) so the background respects the clip shape.")
			},
		})
	}
	{
		r := &ComposeModifierClickableBeforePaddingRule{BaseRule: BaseRule{RuleName: "ComposeModifierClickableBeforePadding", RuleSetName: "compose", Sev: "warning", Desc: "Detects Modifier chains where clickable is applied before padding, excluding the padding region from the click area."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "padding" {
					return
				}
				prev := composeChainedCallPrev(file, idx)
				if prev == 0 || flatCallExpressionName(file, prev) != "clickable" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"clickable { } before padding() makes the click area exclude the padding region; put .padding(...) first so the click hit test covers the padded bounds.")
			},
		})
	}
	{
		r := &ComposePreviewAnnotationMissingRule{BaseRule: BaseRule{RuleName: "ComposePreviewAnnotationMissing", RuleSetName: "compose", Sev: "info", Desc: "Detects @Composable functions whose name ends in Preview but lack the @Preview annotation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"@Composable function named "+name+" is missing @Preview; add @Preview so Android Studio renders it in the preview pane.")
			},
		})
	}
	{
		r := &ComposeMutableDefaultArgumentRule{BaseRule: BaseRule{RuleName: "ComposeMutableDefaultArgument", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable parameters with mutable collection defaults that re-evaluate on each recomposition."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &ComposeStringResourceInsideLambdaRule{BaseRule: BaseRule{RuleName: "ComposeStringResourceInsideLambda", RuleSetName: "compose", Sev: "warning", Desc: "Detects stringResource() calls inside callback lambdas where the composition-only API will crash at runtime."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"stringResource() is composition-only and will crash when invoked from a "+label+" callback; hoist the resource lookup above the lambda.")
			},
		})
	}
	{
		r := &ComposeRememberWithoutKeyRule{BaseRule: BaseRule{RuleName: "ComposeRememberWithoutKey", RuleSetName: "compose", Sev: "warning", Desc: "Detects remember blocks that reference enclosing parameters but have no keys, causing stale cached values."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				paramName, ok := composeLambdaReferencesEnclosingParam(file, lambda)
				if !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"remember { } reads enclosing parameter "+paramName+" but has no keys; the cached value won't update when "+paramName+" changes. Pass remember("+paramName+") { ... }.")
			},
		})
	}
	{
		r := &ComposeLaunchedEffectWithoutKeysRule{BaseRule: BaseRule{RuleName: "ComposeLaunchedEffectWithoutKeys", RuleSetName: "compose", Sev: "warning", Desc: "Detects LaunchedEffect blocks keyed only on constants whose body reads enclosing parameters that change."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				paramName, ok := composeLambdaReferencesEnclosingParam(file, lambda)
				if !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"LaunchedEffect body reads enclosing parameter "+paramName+" but is keyed only on constants; the effect won't re-run when "+paramName+" changes. Pass LaunchedEffect("+paramName+") { ... }.")
			},
		})
	}
	{
		r := &ComposeMutableStateInCompositionRule{BaseRule: BaseRule{RuleName: "ComposeMutableStateInComposition", RuleSetName: "compose", Sev: "warning", Desc: "Detects mutableStateOf() used as a plain local without remember, causing state loss on every recomposition."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					if flatCallExpressionName(file, child) != "mutableStateOf" {
						return
					}
					ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
						"mutableStateOf() as a plain local in a @Composable creates a fresh state on every recomposition; wrap it in remember { mutableStateOf(...) } or use `by remember { mutableStateOf(...) }`.")
					return
				}
			},
		})
	}
	{
		r := &ComposeStatefulDefaultParameterRule{BaseRule: BaseRule{RuleName: "ComposeStatefulDefaultParameter", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable parameters with constructor-call defaults that allocate fresh instances every recomposition."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					first := name[0]
					if first < 'A' || first > 'Z' {
						continue
					}
					ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
						"default `"+name+"()` on a @Composable parameter allocates a fresh instance every recomposition; use `remember { "+name+"() }` or a `remember"+name+"()` helper so the state persists across composes.")
				}
			},
		})
	}
	{
		r := &ComposePreviewWithBackingStateRule{BaseRule: BaseRule{RuleName: "ComposePreviewWithBackingState", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Preview functions that call runtime state holders like hiltViewModel() or collectAsState() which fail in the preview renderer."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &ComposeDisposableEffectMissingDisposeRule{BaseRule: BaseRule{RuleName: "ComposeDisposableEffectMissingDispose", RuleSetName: "compose", Sev: "warning", Desc: "Detects DisposableEffect blocks whose body does not end with onDispose, causing resource leaks."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				if file.FlatType(last) == "call_expression" && flatCallNameAny(file, last) == "onDispose" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"DisposableEffect body must end with onDispose { ... }; without it the resource registered in the block leaks when the composable leaves composition.")
			},
		})
	}
	{
		r := &ComposeModifierPassedThenChainedRule{BaseRule: BaseRule{RuleName: "ComposeModifierPassedThenChained", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable functions that declare a modifier parameter but never use it, starting fresh Modifier chains instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &ComposeSideEffectInCompositionRule{BaseRule: BaseRule{RuleName: "ComposeSideEffectInComposition", RuleSetName: "compose", Sev: "warning", Desc: "Detects direct assignments inside @Composable function bodies that are not wrapped in a recognized effect scope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"assignment"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok || !flatHasAnnotationNamed(file, fn, "Composable") {
					return
				}
				if composeFunctionHasPreviewAnnotation(file, fn) {
					return
				}
				if !composeFileHasRuntimeComposableEvidence(file) {
					return
				}
				if composeAssignmentIsMutableTransitionTargetState(file, idx, fn) {
					return
				}
				if composeAssignmentSynchronizesRememberedObject(file, idx, fn) {
					return
				}
				for cur, ok := file.FlatParent(idx); ok && cur != fn; cur, ok = file.FlatParent(cur) {
					if file.FlatType(cur) != "lambda_literal" {
						continue
					}
					if composeSideEffectAllowedLambdaBoundary(file, cur, fn) {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"assignment inside a @Composable function body runs on every recomposition; wrap the mutation in LaunchedEffect / SideEffect / DisposableEffect so it only runs when intended.")
			},
		})
	}
	{
		r := &ComposeUnstableParameterRule{BaseRule: BaseRule{RuleName: "ComposeUnstableParameter", RuleSetName: "compose", Sev: "warning", Desc: "Detects @Composable function parameters using mutable collection types that prevent recomposition skipping."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !flatHasAnnotationNamed(file, idx, "Composable") {
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
	{
		r := &ComposeRememberSaveableNonParcelableRule{BaseRule: BaseRule{RuleName: "ComposeRememberSaveableNonParcelable", RuleSetName: "compose", Sev: "warning", Desc: "Detects rememberSaveable blocks constructing non-primitive types without an explicit saver argument."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"rememberSaveable { "+name+"() } with no `saver`/`stateSaver` falls back to AutoSaver, which only handles primitives and Parcelable/Serializable. If "+name+" isn't @Parcelize, pass an explicit saver; otherwise suppress this warning.")
			},
		})
	}
}
