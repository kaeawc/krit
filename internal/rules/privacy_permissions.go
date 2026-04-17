package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

const privacyRuleSet = "privacy"

// AdMobInitializedBeforeConsentRule flags MobileAds.initialize(...) inside an
// Application.onCreate override when no earlier consent update call is present.
type AdMobInitializedBeforeConsentRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Privacy/permissions rule. Detection pattern-matches sensitive API call
// shapes and annotation names without cross-checking project permission
// contracts. Classified per roadmap/17.
func (r *AdMobInitializedBeforeConsentRule) Confidence() float64 { return 0.75 }

func (r *AdMobInitializedBeforeConsentRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *AdMobInitializedBeforeConsentRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "initialize" {
		return nil
	}
	if flatReceiverNameFromCall(file, idx) != "MobileAds" {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || extractIdentifierFlat(file, fn) != "onCreate" {
		return nil
	}

	classNode, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok || !privacyClassDirectlyExtendsFlat(file, classNode, "Application") {
		return nil
	}
	if privacyHasPrecedingConsentUpdateCallFlat(file, fn, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"MobileAds.initialize(...) runs in Application.onCreate before any consentInformation.requestConsentInfoUpdate(...) call. Request consent info before initializing AdMob.",
	)}
}

func privacyClassDirectlyExtendsFlat(file *scanner.File, idx uint32, typeName string) bool {
	if file == nil || idx == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if privacyDelegationSupertypeNameFlat(file, child) == typeName {
			return true
		}
	}
	return false
}

func privacyDelegationSupertypeNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	ut := file.FlatFindChild(idx, "user_type")
	if ut == 0 {
		if call := file.FlatFindChild(idx, "constructor_invocation"); call != 0 {
			ut = file.FlatFindChild(call, "user_type")
		}
	}
	if ut == 0 {
		return ""
	}

	var lastIdent string
	for i := 0; i < file.FlatChildCount(ut); i++ {
		child := file.FlatChild(ut, i)
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return lastIdent
}

func privacyHasPrecedingConsentUpdateCallFlat(file *scanner.File, fn, target uint32) bool {
	body := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}

	found := false
	targetStart := file.FlatStartByte(target)
	file.FlatWalkNodes(body, "call_expression", func(candidate uint32) {
		if found || file.FlatStartByte(candidate) >= targetStart {
			return
		}
		if flatCallExpressionName(file, candidate) == "requestConsentInfoUpdate" {
			found = true
		}
	})
	return found
}

// BiometricAuthNotFallingBackToDeviceCredentialRule flags direct
// BiometricPrompt.authenticate(...) calls whose inline PromptInfo.Builder chain
// never enables device credential fallback.
type BiometricAuthNotFallingBackToDeviceCredentialRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Privacy/permissions rule. Detection pattern-matches sensitive API call
// shapes and annotation names without cross-checking project permission
// contracts. Classified per roadmap/17.
func (r *BiometricAuthNotFallingBackToDeviceCredentialRule) Confidence() float64 { return 0.75 }

func (r *BiometricAuthNotFallingBackToDeviceCredentialRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *BiometricAuthNotFallingBackToDeviceCredentialRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 || args == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "authenticate" {
		return nil
	}

	navText := file.FlatNodeText(navExpr)
	if !strings.Contains(navText, "BiometricPrompt") {
		return nil
	}

	promptInfoArg := flatPositionalValueArgument(file, args, 0)
	if promptInfoArg == 0 {
		promptInfoArg = flatNamedValueArgument(file, args, "promptInfo")
	}
	promptInfoExpr := flatValueArgumentExpression(file, promptInfoArg)
	if promptInfoExpr == 0 || file.FlatType(promptInfoExpr) != "call_expression" {
		return nil
	}

	promptInfoText := file.FlatNodeText(promptInfoExpr)
	if !strings.Contains(promptInfoText, "PromptInfo.Builder()") || !strings.Contains(promptInfoText, ".build()") {
		return nil
	}

	if biometricPromptAllowsDeviceCredentialFlat(file, promptInfoExpr) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(promptInfoExpr)+1,
		file.FlatCol(promptInfoExpr)+1,
		"BiometricPrompt.authenticate(...) builds PromptInfo without device credential fallback. Add setDeviceCredentialAllowed(true) or include DEVICE_CREDENTIAL in setAllowedAuthenticators(...).",
	)}
}

func biometricPromptAllowsDeviceCredentialFlat(file *scanner.File, idx uint32) bool {
	allowsFallback := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if allowsFallback || file.FlatType(candidate) != "call_expression" {
			return
		}

		switch flatCallExpressionName(file, candidate) {
		case "setDeviceCredentialAllowed":
			_, args := flatCallExpressionParts(file, candidate)
			arg := flatPositionalValueArgument(file, args, 0)
			if arg == 0 {
				return
			}
			if compactKotlinExpr(file.FlatNodeText(arg)) == "true" {
				allowsFallback = true
			}
		case "setAllowedAuthenticators":
			_, args := flatCallExpressionParts(file, candidate)
			if args == 0 {
				return
			}
			if strings.Contains(compactKotlinExpr(file.FlatNodeText(args)), "DEVICE_CREDENTIAL") {
				allowsFallback = true
			}
		}
	})
	return allowsFallback
}

// ContactsAccessWithoutPermissionUiRule flags contacts queries that are not
// gated behind a RequestPermission activity-result callback.
type ContactsAccessWithoutPermissionUiRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Privacy/permissions rule. Detection pattern-matches sensitive API call
// shapes and annotation names without cross-checking project permission
// contracts. Classified per roadmap/17.
func (r *ContactsAccessWithoutPermissionUiRule) Confidence() float64 { return 0.75 }

func (r *ContactsAccessWithoutPermissionUiRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ContactsAccessWithoutPermissionUiRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "query" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	uriArg := flatNamedValueArgument(file, args, "uri")
	if uriArg == 0 {
		uriArg = flatPositionalValueArgument(file, args, 0)
	}
	uriExpr := flatValueArgumentExpression(file, uriArg)
	if uriExpr == 0 {
		return nil
	}

	if !isContactsPhoneContentURIFlat(file, uriExpr) {
		return nil
	}

	if contactsQueryHasPermissionUiPathFlat(file, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(uriExpr)+1,
		file.FlatCol(uriExpr)+1,
		"Contacts phone query without an obvious RequestPermission callback path. Request READ_CONTACTS before querying ContactsContract.CommonDataKinds.Phone.CONTENT_URI.",
	)}
}

func isContactsPhoneContentURIFlat(file *scanner.File, idx uint32) bool {
	text := compactKotlinExpr(file.FlatNodeText(idx))
	return strings.Contains(text, "ContactsContract.CommonDataKinds.Phone.CONTENT_URI")
}

func contactsQueryHasPermissionUiPathFlat(file *scanner.File, idx uint32) bool {
	lambda, ok := flatEnclosingAncestor(file, idx, "lambda_literal")
	if !ok || !lambdaBelongsToRequestPermissionRegistrationFlat(file, lambda) {
		return false
	}

	scope, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "source_file")
	if !ok {
		return false
	}

	scopeText := compactKotlinExpr(file.FlatNodeText(scope))
	return strings.Contains(scopeText, "READ_CONTACTS")
}

func lambdaBelongsToRequestPermissionRegistrationFlat(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "call_expression" {
			continue
		}
		callText := compactKotlinExpr(file.FlatNodeText(cur))
		if strings.Contains(callText, "registerForActivityResult(ActivityResultContracts.RequestPermission()") {
			return true
		}
	}
	return false
}

// LocationBackgroundWithoutRationaleRule flags requestPermissions calls for
// ACCESS_BACKGROUND_LOCATION when the file has no shouldShowRequestPermissionRationale call.
type LocationBackgroundWithoutRationaleRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LocationBackgroundWithoutRationaleRule) Confidence() float64 { return 0.75 }

func (r *LocationBackgroundWithoutRationaleRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *LocationBackgroundWithoutRationaleRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "requestPermissions" && name != "launch" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	argsText := compactKotlinExpr(file.FlatNodeText(args))
	if !strings.Contains(argsText, "ACCESS_BACKGROUND_LOCATION") {
		return nil
	}

	content := string(file.Content)
	if strings.Contains(content, "shouldShowRequestPermissionRationale") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"ACCESS_BACKGROUND_LOCATION requested without shouldShowRequestPermissionRationale. Show a rationale dialog before requesting background location access.",
	)}
}

var loginScreenNamePattern = regexp.MustCompile(`(?i)(Login|Password|Pin|Secure|Payment|Card)`)

// ScreenshotNotBlockedOnLoginScreenRule flags Activity classes or @Composable
// functions whose name suggests a sensitive screen but do not set FLAG_SECURE.
type ScreenshotNotBlockedOnLoginScreenRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ScreenshotNotBlockedOnLoginScreenRule) Confidence() float64 { return 0.75 }

func (r *ScreenshotNotBlockedOnLoginScreenRule) NodeTypes() []string {
	return []string{"class_declaration", "function_declaration"}
}

func (r *ScreenshotNotBlockedOnLoginScreenRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	nodeType := file.FlatType(idx)
	name := extractIdentifierFlat(file, idx)
	if name == "" || !loginScreenNamePattern.MatchString(name) {
		return nil
	}

	bodyText := compactKotlinExpr(file.FlatNodeText(idx))

	if nodeType == "class_declaration" {
		if !privacyClassExtendsActivity(file, idx) {
			return nil
		}
		if strings.Contains(bodyText, "FLAG_SECURE") {
			return nil
		}
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"Sensitive screen \""+name+"\" does not set FLAG_SECURE. Add window.setFlags(FLAG_SECURE, FLAG_SECURE) to prevent screenshots and screen recording.",
		)}
	}

	if nodeType == "function_declaration" {
		if !privacyHasComposableAnnotation(file, idx) {
			return nil
		}
		if strings.Contains(bodyText, "FLAG_SECURE") || strings.Contains(bodyText, "ScreenshotBlocker") {
			return nil
		}
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"Sensitive composable \""+name+"\" does not block screenshots. Apply FLAG_SECURE or a ScreenshotBlocker modifier.",
		)}
	}

	return nil
}

func privacyClassExtendsActivity(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		superName := privacyDelegationSupertypeNameFlat(file, child)
		switch superName {
		case "Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity":
			return true
		}
	}
	return false
}

func privacyHasComposableAnnotation(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		if typ == "modifiers" || typ == "annotation" {
			text := file.FlatNodeText(child)
			if strings.Contains(text, "Composable") {
				return true
			}
		}
	}
	return false
}

var passwordVarNamePattern = regexp.MustCompile(`(?i)(password|passwd|pwd|pin|secret|credential)`)

// ClipboardOnSensitiveInputTypeRule flags setPrimaryClip calls where the
// source variable name suggests a password or credential field.
type ClipboardOnSensitiveInputTypeRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ClipboardOnSensitiveInputTypeRule) Confidence() float64 { return 0.75 }

func (r *ClipboardOnSensitiveInputTypeRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ClipboardOnSensitiveInputTypeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "setPrimaryClip" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	argsText := file.FlatNodeText(args)
	if !passwordVarNamePattern.MatchString(argsText) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Clipboard write from a variable that looks like a password or credential. Avoid copying sensitive data to the clipboard.",
	)}
}
