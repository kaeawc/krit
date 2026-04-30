package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPrivacyAnalyticsRules() {

	// --- from privacy_analytics.go ---
	{
		r := &AnalyticsEventWithPiiParamNameRule{BaseRule: BaseRule{RuleName: "AnalyticsEventWithPiiParamName", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects analytics event parameters whose key names match PII patterns like email, phone, or SSN."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !isAnalyticsEventMethod(name) {
					return
				}
				if !privacyCallHasReceiverType(ctx, idx, analyticsReceiverTypes) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				var emitted bool
				file.FlatWalkNodes(args, "infix_expression", func(infix uint32) {
					if !infixOperatorIs(file, infix, "to") {
						return
					}
					keyText := infixLeftStringLiteralContent(file, infix)
					if keyText == "" {
						return
					}
					if piiKeyPattern.MatchString(keyText) {
						ctx.EmitAt(file.FlatRow(infix)+1, file.FlatCol(infix)+1, "Analytics event parameter \""+keyText+"\" looks like PII. Avoid sending personally identifiable information to analytics services.")
						emitted = true
					}
				})
				if emitted {
					return
				}
				file.FlatWalkNodes(args, "string_literal", func(strNode uint32) {
					parent, ok := file.FlatParent(strNode)
					if !ok {
						return
					}
					if file.FlatType(parent) == "value_argument" || file.FlatType(parent) == "infix_expression" {
						return
					}
					body, ok := kotlinStringLiteralBody(file.FlatNodeText(strNode))
					if !ok {
						return
					}
					if piiKeyPattern.MatchString(body) {
						ctx.EmitAt(file.FlatRow(strNode)+1, file.FlatCol(strNode)+1, "Analytics event parameter \""+body+"\" looks like PII. Avoid sending personally identifiable information to analytics services.")
					}
				})
			},
		})
	}
	{
		r := &AnalyticsUserIdFromPiiRule{BaseRule: BaseRule{RuleName: "AnalyticsUserIdFromPii", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects user-ID setter calls whose argument is a PII property like email or phoneNumber."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !isUserIdSetterMethod(name) {
					return
				}
				if !privacyCallHasReceiverType(ctx, idx, analyticsReceiverTypes) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				arg := flatPositionalValueArgument(file, args, 0)
				if arg == 0 {
					return
				}
				argExpr := flatValueArgumentExpression(file, arg)
				if argExpr == 0 {
					return
				}
				argText := file.FlatNodeText(argExpr)
				lastProp := privacyLastPropertyName(argText)
				if !piiPropertyNames[lastProp] {
					return
				}
				ctx.EmitAt(file.FlatRow(argExpr)+1, file.FlatCol(argExpr)+1, "User ID set from PII property \""+lastProp+"\". User IDs should be opaque identifiers, not personally identifiable information.")
			},
		})
	}
	{
		r := &CrashlyticsCustomKeyWithPiiRule{BaseRule: BaseRule{RuleName: "CrashlyticsCustomKeyWithPii", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects Crashlytics setCustomKey calls where the key name matches PII patterns."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "setCustomKey" {
					return
				}
				if !privacyCallHasReceiverType(ctx, idx, crashlyticsReceiverTypes) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				arg := flatPositionalValueArgument(file, args, 0)
				if arg == 0 {
					return
				}
				argExpr := flatValueArgumentExpression(file, arg)
				if argExpr == 0 {
					return
				}
				body, ok := kotlinStringLiteralBody(file.FlatNodeText(argExpr))
				if !ok {
					return
				}
				if !piiKeyPattern.MatchString(body) {
					return
				}
				ctx.EmitAt(file.FlatRow(argExpr)+1, file.FlatCol(argExpr)+1, "Crashlytics custom key \""+body+"\" looks like PII. Crash reports should not carry personally identifiable information.")
			},
		})
	}
	{
		r := &FirebaseRemoteConfigDefaultsWithPiiRule{BaseRule: BaseRule{RuleName: "FirebaseRemoteConfigDefaultsWithPii", RuleSetName: privacyRuleSet, Sev: "info", Desc: "Detects Firebase Remote Config default map keys that match PII patterns."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "setDefaults" && name != "setDefaultsAsync" {
					return
				}
				if !privacyCallHasReceiverType(ctx, idx, remoteConfigReceiverTypes) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				file.FlatWalkNodes(args, "infix_expression", func(infix uint32) {
					if !infixOperatorIs(file, infix, "to") {
						return
					}
					keyText := infixLeftStringLiteralContent(file, infix)
					if keyText == "" {
						return
					}
					if piiKeyPattern.MatchString(keyText) {
						ctx.EmitAt(file.FlatRow(infix)+1, file.FlatCol(infix)+1, "Remote Config default key \""+keyText+"\" looks like PII. Remote Config values are not encrypted at rest.")
					}
				})
			},
		})
	}
	{
		r := &AnalyticsCallWithoutConsentGateRule{BaseRule: BaseRule{RuleName: "AnalyticsCallWithoutConsentGate", RuleSetName: privacyRuleSet, Sev: "info", Desc: "Detects analytics event calls that are not guarded by a consent or GDPR check."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !isAnalyticsEventMethod(name) {
					return
				}
				if !privacyCallHasReceiverType(ctx, idx, analyticsReceiverTypes) {
					return
				}
				if privacyCallIsInsideConsentGuard(file, idx) {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if ok && privacyHasConsentEarlyReturn(file, fn, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Analytics call without a visible consent gate. Guard analytics events behind a consent check (e.g. if (consent.analyticsAllowed) { ... }).")
			},
		})
	}
}
