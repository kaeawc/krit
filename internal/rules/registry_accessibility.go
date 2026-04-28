package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerAccessibilityRules() {

	// --- from accessibility.go ---
	{
		r := &AnimatorDurationIgnoresScaleRule{
			BaseRule: BaseRule{RuleName: "AnimatorDurationIgnoresScale", RuleSetName: "a11y", Sev: "info", Desc: "Detects animator durations that ignore the system ANIMATOR_DURATION_SCALE accessibility setting."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "assignment"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{
				"ofArgb", "ofFloat", "ofInt", "ofObject", "setDuration",
			}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if animatorDurationScaleReferenced(ctx, idx) {
					return
				}
				switch file.FlatType(idx) {
				case "call_expression":
					if !animatorReceiverConfirmed(ctx, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Animator duration ignores the system animation scale. Read Settings.Global.ANIMATOR_DURATION_SCALE and scale the duration before starting the animation.")
				case "assignment":
					if !animatorAssignmentTargetConfirmed(ctx, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Animator duration ignores the system animation scale. Read Settings.Global.ANIMATOR_DURATION_SCALE and scale the duration before starting the animation.")
				}
			},
		})
	}
	{
		r := &ComposeClickableWithoutMinTouchTargetRule{
			BaseRule: BaseRule{RuleName: "ComposeClickableWithoutMinTouchTarget", RuleSetName: "a11y", Sev: "warning", Desc: "Detects clickable Compose modifiers with explicit touch target dimensions below the 48dp minimum."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallNameAny(file, idx) != "clickable" {
					return
				}
				navExpr, _ := accessibilityCallExpressionParts(file, idx)
				if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "clickable" {
					return
				}
				chain, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
				if !rootedAtModifier || !composeModifierCallChainImportsConfirmed(file, chain) ||
					composeModifierChainContainsCall(chain, "minimumInteractiveComponentSize") {
					return
				}
				minDp, hasExplicitSize := composeModifierChainSmallestDpFlat(file, chain)
				if !hasExplicitSize || minDp >= 48 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"clickable Compose modifier has a touch target below 48.dp; use at least 48.dp or add minimumInteractiveComponentSize().")
			},
		})
	}
	{
		r := &ComposeDecorativeImageContentDescriptionRule{
			BaseRule: BaseRule{RuleName: "ComposeDecorativeImageContentDescription", RuleSetName: "a11y", Sev: "warning", Desc: "Detects decorative images with null contentDescription that are not hidden from TalkBack."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallName(file, idx)
				if !composeImageCallNames[name] {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				cdArg := flatNamedValueArgument(file, args, "contentDescription")
				if cdArg == 0 {
					return
				}
				if !namedArgRHSIsNullLiteral(file, cdArg) {
					return
				}
				if subtreeHasCalleeIn(file, idx, composeSemanticsEscapeHatches) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Decorative image with `contentDescription = null` should use `Modifier.clearAndSetSemantics {}` or `semantics { invisibleToUser() }` to hide from TalkBack.")
			},
		})
	}
	{
		r := &ComposeIconButtonMissingContentDescriptionRule{
			BaseRule: BaseRule{RuleName: "ComposeIconButtonMissingContentDescription", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Icon or IconButton composables missing a contentDescription for screen readers."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !composeContentDescriptionFileMayUse(file) {
					return
				}
				// In the trailing-lambda idiom `IconButton(args) { ... }`,
				// tree-sitter emits an outer call_expression whose first
				// named child is the inner `IconButton(args)` call_expression.
				// Skip the inner call so we evaluate each user-visible call
				// exactly once, at the outer form where the trailing-lambda
				// children are actually reachable.
				if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
					first := file.FlatFirstChild(parent)
					for first != 0 && !file.FlatIsNamed(first) {
						first = file.FlatNextSib(first)
					}
					if first == idx {
						return
					}
				}
				name := flatCallNameAny(file, idx)
				if !composeContentDescriptionCalls[name] {
					return
				}
				if !composeContentDescriptionCallConfirmed(file, idx, name) {
					return
				}
				if name != "IconButton" && composeHasConfirmedIconButtonAncestor(file, idx) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					// Outer trailing-lambda form — args live on the nested inner call.
					args = flatCallKeyArguments(file, idx)
				}
				if name == "IconButton" {
					// The IconButton itself rarely declares contentDescription —
					// the Icon child inside its content lambda carries it. Fire
					// only when NO descendant call_expression anywhere in the
					// IconButton's subtree uses a contentDescription arg.
					if callSubtreeHasNamedArgument(file, idx, "contentDescription") {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"IconButton's Icon is missing `contentDescription`. Set a description for accessibility.")
					return
				}
				if args == 0 {
					return
				}
				cdArg := flatNamedValueArgument(file, args, "contentDescription")
				if cdArg != 0 {
					return
				}
				if subtreeHasCalleeIn(file, idx, composeSemanticsEscapeHatches) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					name+" is missing `contentDescription`. Set a description for accessibility or mark as decorative.")
			},
		})
	}
	{
		r := &ComposeRawTextLiteralRule{
			BaseRule: BaseRule{RuleName: "ComposeRawTextLiteral", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Compose Text() calls using hardcoded string literals instead of stringResource() for i18n."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallName(file, idx)
				if name != "Text" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				expr := flatValueArgumentExpression(file, firstArg)
				if expr == 0 || file.FlatType(expr) != "string_literal" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if flatHasAnnotationNamed(file, fn, "Preview") {
					return
				}
				fnName := flatFunctionName(file, fn)
				if strings.Contains(fnName, "Preview") || strings.Contains(fnName, "Sample") {
					return
				}
				if strings.HasSuffix(file.Path, "Preview.kt") || strings.HasSuffix(file.Path, "Sample.kt") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Text() uses a hardcoded string literal. Use `stringResource()` for internationalization and accessibility.")
			},
		})
	}
	{
		r := &ComposeSemanticsMissingRoleRule{
			BaseRule: BaseRule{RuleName: "ComposeSemanticsMissingRole", RuleSetName: "a11y", Sev: "warning", Desc: "Detects interactive Compose modifiers (clickable, toggleable, selectable) without an explicit accessibility role."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				name := flatNavigationExpressionLastIdentifier(file, navExpr)
				if !composeInteractionModifiers[name] {
					return
				}
				if isInsidePreviewOrSampleFunctionFlat(file, idx) {
					return
				}
				_, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
				if !rootedAtModifier {
					return
				}
				if args != 0 && flatNamedBooleanArgumentIsFalse(file, args, "enabled") {
					return
				}
				if args != 0 && flatNamedValueArgument(file, args, "role") != 0 {
					return
				}
				outerCall := findOutermostModifierChainCall(file, idx)
				if outerCall != 0 {
					fullText := file.FlatNodeText(outerCall)
					if strings.Contains(fullText, "semantics") && strings.Contains(fullText, "role") {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Modifier."+name+" without an explicit `role`. Add `role = Role.X` or a `Modifier.semantics { role = ... }` for screen readers.")
			},
		})
	}
	{
		r := &ComposeTextFieldMissingLabelRule{
			BaseRule: BaseRule{RuleName: "ComposeTextFieldMissingLabel", RuleSetName: "a11y", Sev: "warning", Desc: "Detects TextField or OutlinedTextField composables missing a label parameter for accessibility."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallName(file, idx)
				if !composeTextFieldCalls[name] {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				if flatNamedValueArgument(file, args, "label") != 0 {
					return
				}
				if flatNamedValueArgument(file, args, "placeholder") != 0 {
					return
				}
				parent, ok := file.FlatParent(idx)
				if ok && hasSiblingTextCall(file, parent, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					name+" is missing a `label` parameter. Add a label for accessibility.")
			},
		})
	}
	{
		r := &ToastForAccessibilityAnnouncementRule{
			BaseRule: BaseRule{RuleName: "ToastForAccessibilityAnnouncement", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Toast.makeText used in accessibility-related functions instead of announceForAccessibility."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				callName := flatNavigationExpressionLastIdentifier(file, navExpr)
				if callName != "makeText" {
					return
				}
				receiver := file.FlatNamedChild(navExpr, 0)
				if receiver == 0 || !file.FlatNodeTextEquals(receiver, "Toast") {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				fnName := strings.ToLower(flatFunctionName(file, fn))
				isA11yContext := false
				for _, pattern := range a11yFunctionPatterns {
					if strings.Contains(fnName, pattern) {
						isA11yContext = true
						break
					}
				}
				if !isA11yContext {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Toast used in an accessibility context. Use `View.announceForAccessibility()` or `AccessibilityManager` instead.")
			},
		})
	}
}
