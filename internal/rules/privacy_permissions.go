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
	ut, _ := file.FlatFindChild(idx, "user_type")
	if ut == 0 {
		if call, ok := file.FlatFindChild(idx, "constructor_invocation"); ok {
			ut, _ = file.FlatFindChild(call, "user_type")
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
	body, _ := file.FlatFindChild(fn, "function_body")
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


var loginScreenNamePattern = regexp.MustCompile(`(?i)(Login|Password|Pin|Secure|Payment|Card)`)

// ScreenshotNotBlockedOnLoginScreenRule flags Activity classes or @Composable
// functions whose name suggests a sensitive screen but do not set FLAG_SECURE.
type ScreenshotNotBlockedOnLoginScreenRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ScreenshotNotBlockedOnLoginScreenRule) Confidence() float64 { return 0.75 }


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

