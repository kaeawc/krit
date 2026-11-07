package rules

import (
	"strings"
	"unicode"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerAccessibilityRules() {

	// --- from accessibility.go ---
	{
		r := &AnimatorDurationIgnoresScaleRule{
			BaseRule: BaseRule{RuleName: "AnimatorDurationIgnoresScale", RuleSetName: "a11y", Sev: "info", Desc: "Detects animator durations that ignore the system ANIMATOR_DURATION_SCALE accessibility setting."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "assignment"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, LexicalCalleeNames: []string{"clickable"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
				ctx.EmitAtNode(calleeNameAnchor(file, idx),
					"clickable Compose modifier has a touch target below 48.dp; use at least 48.dp or add minimumInteractiveComponentSize().")
			},
		})
	}
	{
		r := &ComposeDecorativeImageContentDescriptionRule{
			BaseRule: BaseRule{RuleName: "ComposeDecorativeImageContentDescription", RuleSetName: "a11y", Sev: "warning", Desc: "Detects decorative images with null contentDescription that are not hidden from TalkBack."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
				if !composeContentDescriptionCallConfirmed(file, idx, name) {
					return
				}
				if subtreeHasCalleeIn(file, idx, composeSemanticsEscapeHatches) {
					return
				}
				if !subtreeHasCalleeIn(file, idx, composeInteractionModifiers) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Interactive image with `contentDescription = null` is hidden from TalkBack; add a meaningful description or remove the interaction.")
			},
		})
	}
	{
		r := &ComposeIconButtonMissingContentDescriptionRule{
			BaseRule: BaseRule{RuleName: "ComposeIconButtonMissingContentDescription", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Icon or IconButton composables missing a contentDescription for screen readers."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
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
					if composeSubtreeSetsContentDescription(file, idx) {
						return
					}
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						"IconButton's Icon is missing `contentDescription`. Set a description for accessibility.")
					return
				}
				if args == 0 {
					return
				}
				// The second positional argument of Icon/Image/AsyncImage is contentDescription.
				if composeCallSetsContentDescription(file, idx) {
					return
				}
				if subtreeHasCalleeIn(file, idx, composeSemanticsEscapeHatches) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					name+" is missing `contentDescription`. Set a description for accessibility or mark as decorative.")
			},
		})
	}
	{
		r := &ComposeRawTextLiteralRule{
			BaseRule:              BaseRule{RuleName: "ComposeRawTextLiteral", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Compose Text() calls using hardcoded string literals instead of stringResource() for i18n."},
			CustomPreviewWildcard: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallName(file, idx)
				if name != "Text" {
					return
				}
				if composeRawTextLiteralNonProductionFile(file.Path, r.NonProductionPathMarkers) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				textArg := flatNamedValueArgument(file, args, "text")
				if textArg == 0 {
					textArg = flatPositionalValueArgument(file, args, 0)
				}
				if textArg == 0 {
					return
				}
				expr := flatValueArgumentExpression(file, textArg)
				if expr == 0 || file.FlatType(expr) != "string_literal" {
					return
				}
				// Strings without a letter (spaces, bullets, separators) have nothing to translate.
				// A pure template such as `"$x"` has no literal text either.
				if !strings.ContainsFunc(composeRawTextLiteralText(file, expr), unicode.IsLetter) {
					return
				}
				if !composeTextCallMayBeCompose(file) {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if composePrivateFunctionOnlyCalledFromPreviews(file, fn) {
					return
				}
				if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) ||
					composeInsideInspectionModeBranch(file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Text() uses a hardcoded string literal. Use `stringResource()` for internationalization and accessibility.")
			},
		})
	}
	{
		r := &ComposeSemanticsMissingRoleRule{
			BaseRule:              BaseRule{RuleName: "ComposeSemanticsMissingRole", RuleSetName: "a11y", Sev: "warning", Desc: "Detects interactive Compose modifiers (clickable, toggleable, selectable) without an explicit accessibility role."},
			CustomPreviewWildcard: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				name := flatNavigationExpressionLastIdentifier(file, navExpr)
				if !composeInteractionModifiers[name] {
					return
				}
				// Test composables don't ship to users.
				if composeIsTestSourceOrFakeAt(file, idx) {
					return
				}
				if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
					return
				}
				if fn, ok := flatEnclosingFunction(file, idx); ok && composePrivateFunctionOnlyCalledFromPreviews(file, fn) {
					return
				}
				_, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
				if !rootedAtModifier {
					return
				}
				if args != 0 && flatNamedBooleanArgumentIsFalse(file, args, "enabled") {
					return
				}
				if args != 0 && (flatNamedValueArgument(file, args, "role") != 0 || composePassesPositionalRole(file, args)) {
					return
				}
				if args != 0 && composeIndicationIsNullFlat(file, args) {
					return
				}
				outerCall := findOutermostModifierChainCall(file, idx)
				if outerCall != 0 {
					fullText := file.FlatNodeText(outerCall)
					if strings.Contains(fullText, "semantics") && strings.Contains(fullText, "role") {
						return
					}
					// Scrims hide from accessibility or click without a visible indication.
					if strings.Contains(fullText, "hideFromAccessibility") || strings.Contains(fullText, "invisibleToUser") ||
						strings.Contains(fullText, "clearAndSetSemantics") {
						return
					}
				}
				ctx.EmitAtNode(calleeNameAnchor(file, idx),
					"Modifier."+name+" without an explicit `role`. Add `role = Role.X` or a `Modifier.semantics { role = ... }` for screen readers.")
			},
		})
	}
	{
		r := &ComposeTextFieldMissingLabelRule{
			BaseRule: BaseRule{RuleName: "ComposeTextFieldMissingLabel", RuleSetName: "a11y", Sev: "warning", Desc: "Detects TextField or OutlinedTextField composables missing a label parameter for accessibility."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallName(file, idx)
				if !composeTextFieldCalls[name] {
					return
				}
				if !composeTextFieldCallResolvesToMaterial(file, idx, name) {
					return
				}
				if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(true, nil)) {
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
				// A read-only field is display text, and debug tooling screens are not user-facing.
				if readOnly := flatNamedValueArgument(file, args, "readOnly"); readOnly != 0 && !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(readOnly)), "false") {
					return
				}
				if composeDebugToolingPath(file.Path, r.NonProductionPathMarkers) {
					return
				}
				parent, ok := file.FlatParent(idx)
				if ok && hasSiblingTextCall(file, parent, idx) {
					return
				}
				if composeTextFieldIsDescribedGridCell(file, idx, args) {
					return
				}
				if composeTextFieldIsSingleCharBox(file, idx) {
					if m := flatNamedValueArgument(file, args, "modifier"); m != 0 && strings.Contains(file.FlatNodeText(m), "contentDescription") {
						return
					}
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						name+" is a single-character code box without a description. A `label` would not fit; add `Modifier.semantics { contentDescription = ... }` for accessibility.")
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					name+" is missing a `label` parameter. Add a label for accessibility.")
			},
		})
	}
	{
		r := &ToastForAccessibilityAnnouncementRule{
			BaseRule: BaseRule{RuleName: "ToastForAccessibilityAnnouncement", RuleSetName: "a11y", Sev: "warning", Desc: "Detects Toast.makeText used in accessibility-related functions instead of announceForAccessibility."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
				if receiver == 0 {
					return
				}
				if recvText := strings.TrimSpace(file.FlatNodeText(receiver)); recvText != "Toast" && recvText != "android.widget.Toast" {
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
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Toast used in an accessibility context. Use `View.announceForAccessibility()` or `AccessibilityManager` instead.")
			},
		})
	}
}
