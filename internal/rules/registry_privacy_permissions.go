package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPrivacyPermissionsRules() {

	// --- from privacy_permissions.go ---
	{
		r := &AdMobInitializedBeforeConsentRule{BaseRule: BaseRule{RuleName: "AdMobInitializedBeforeConsent", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects MobileAds.initialize() in Application.onCreate before any consent info update call."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "initialize" {
					return
				}
				if flatReceiverNameFromCall(file, idx) != "MobileAds" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok || extractIdentifierFlat(file, fn) != "onCreate" {
					return
				}
				classNode, ok := flatEnclosingAncestor(file, idx, "class_declaration")
				if !ok || !privacyClassDirectlyExtendsFlat(file, classNode, "Application") {
					return
				}
				if privacyHasPrecedingConsentUpdateCallFlat(file, fn, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"MobileAds.initialize(...) runs in Application.onCreate before any consentInformation.requestConsentInfoUpdate(...) call. Request consent info before initializing AdMob.")
			},
		})
	}
	{
		r := &BiometricAuthNotFallingBackToDeviceCredentialRule{BaseRule: BaseRule{RuleName: "BiometricAuthNotFallingBackToDeviceCredential", RuleSetName: privacyRuleSet, Sev: "info", Desc: "Detects BiometricPrompt.authenticate() calls whose PromptInfo lacks device credential fallback."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !biometricAuthenticateReceiverMatchesFlat(file, idx) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				promptInfoArg := flatPositionalValueArgument(file, args, 0)
				if promptInfoArg == 0 {
					promptInfoArg = flatNamedValueArgument(file, args, "promptInfo")
				}
				promptInfoExpr := flatValueArgumentExpression(file, promptInfoArg)
				if !biometricPromptInfoBuilderExpressionFlat(file, promptInfoExpr) {
					return
				}
				if biometricPromptAllowsDeviceCredentialFlat(file, promptInfoExpr) {
					return
				}
				ctx.EmitAt(file.FlatRow(promptInfoExpr)+1, file.FlatCol(promptInfoExpr)+1,
					"BiometricPrompt.authenticate(...) builds PromptInfo without device credential fallback. Add setDeviceCredentialAllowed(true) or include DEVICE_CREDENTIAL in setAllowedAuthenticators(...).")
			},
		})
	}
	{
		r := &ContactsAccessWithoutPermissionUiRule{BaseRule: BaseRule{RuleName: "ContactsAccessWithoutPermissionUi", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects contacts queries not gated behind a RequestPermission activity-result callback."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "query" {
					return
				}
				if !contactsQueryCallHasResolverTargetFlat(file, idx) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				uriArg := flatNamedValueArgument(file, args, "uri")
				if uriArg == 0 {
					uriArg = flatPositionalValueArgument(file, args, 0)
				}
				uriExpr := flatValueArgumentExpression(file, uriArg)
				if uriExpr == 0 {
					return
				}
				if !isContactsPhoneContentURIFlat(file, uriExpr) {
					return
				}
				if contactsQueryHasPermissionUiPathFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(uriExpr)+1, file.FlatCol(uriExpr)+1,
					"Contacts phone query without an obvious RequestPermission callback path. Request READ_CONTACTS before querying ContactsContract.CommonDataKinds.Phone.CONTENT_URI.")
			},
		})
	}
	{
		r := &LocationBackgroundWithoutRationaleRule{BaseRule: BaseRule{RuleName: "LocationBackgroundWithoutRationale", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects ACCESS_BACKGROUND_LOCATION requests without a shouldShowRequestPermissionRationale call."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "requestPermissions" && name != "launch" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				if !privacyValueArgumentsContainBackgroundLocationFlat(file, args) {
					return
				}
				if privacyOwnerHasBackgroundLocationRationaleFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"ACCESS_BACKGROUND_LOCATION requested without shouldShowRequestPermissionRationale. Show a rationale dialog before requesting background location access.")
			},
		})
	}
	{
		r := &ScreenshotNotBlockedOnLoginScreenRule{BaseRule: BaseRule{RuleName: "ScreenshotNotBlockedOnLoginScreen", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects sensitive screens (login, payment, PIN) that do not set FLAG_SECURE to block screenshots."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "function_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeType := file.FlatType(idx)
				name := extractIdentifierFlat(file, idx)
				if name == "" || !privacySensitiveScreenDeclaration(file, idx, name) {
					return
				}
				if nodeType == "class_declaration" {
					if !privacyClassExtendsActivity(file, idx) {
						return
					}
					if privacyNodeContainsFlagSecureReferenceFlat(file, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Sensitive screen \""+name+"\" does not set FLAG_SECURE. Add window.setFlags(FLAG_SECURE, FLAG_SECURE) to prevent screenshots and screen recording.")
					return
				}
				if nodeType == "function_declaration" {
					if !privacyHasComposableAnnotation(file, idx) {
						return
					}
					if !privacyComposableLooksLikeScreen(name) {
						return
					}
					if privacyHasPreviewAnnotation(file, idx) {
						return
					}
					if privacyNodeContainsFlagSecureReferenceFlat(file, idx) || privacyNodeContainsScreenshotBlockerReferenceFlat(file, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Sensitive composable \""+name+"\" does not block screenshots. Apply FLAG_SECURE or a ScreenshotBlocker modifier.")
				}
			},
		})
	}
	{
		r := &ClipboardOnSensitiveInputTypeRule{BaseRule: BaseRule{RuleName: "ClipboardOnSensitiveInputType", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects clipboard writes from variables whose names suggest passwords or credentials."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "setPrimaryClip" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				if !privacyClipboardCallHasSensitiveSourceFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Clipboard write from a variable that looks like a password or credential. Avoid copying sensitive data to the clipboard.")
			},
		})
	}
}
