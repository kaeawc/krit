package rules

import (
	"regexp"
	"strconv"
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
	return contactsQualifiedReferenceMatchesFlat(file, idx, []string{"ContactsContract", "CommonDataKinds", "Phone", "CONTENT_URI"})
}

func contactsQueryCallHasResolverTargetFlat(file *scanner.File, idx uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "query" {
		return false
	}
	path := contactsIdentifierPathFlat(file, navExpr)
	if len(path) < 2 {
		return false
	}
	receiverTail := path[len(path)-2]
	if receiverTail == "contentResolver" || receiverTail == "getContentResolver" {
		return true
	}
	if len(path) == 2 && contactsSameOwnerDeclarationHasTypeFlat(file, idx, path[0], "ContentResolver") {
		return true
	}
	return false
}

func contactsQueryHasPermissionUiPathFlat(file *scanner.File, idx uint32) bool {
	lambda, ok := flatEnclosingAncestor(file, idx, "lambda_literal")
	if !ok {
		return false
	}
	registrationCall, ok := lambdaBelongsToRequestPermissionRegistrationFlat(file, lambda)
	if !ok {
		return false
	}
	launcherName, ok := contactsRegistrationLauncherNameFlat(file, registrationCall)
	if !ok {
		return false
	}
	owner := contactsDeclarationOwnerFlat(file, registrationCall)
	return contactsOwnerLaunchesReadContactsFlat(file, owner, launcherName)
}

func lambdaBelongsToRequestPermissionRegistrationFlat(file *scanner.File, idx uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "call_expression" {
			continue
		}
		if contactsCallRegistersRequestPermissionFlat(file, cur) {
			return cur, true
		}
	}
	return 0, false
}

func contactsCallRegistersRequestPermissionFlat(file *scanner.File, idx uint32) bool {
	if flatCallNameAny(file, idx) != "registerForActivityResult" {
		return false
	}
	args := flatCallKeyArguments(file, idx)
	contractArg := flatPositionalValueArgument(file, args, 0)
	if contractArg == 0 {
		contractArg = flatNamedValueArgument(file, args, "contract")
	}
	contractExpr := flatValueArgumentExpression(file, contractArg)
	return contactsQualifiedCallMatchesFlat(file, contractExpr, []string{"ActivityResultContracts", "RequestPermission"})
}

func contactsRegistrationLauncherNameFlat(file *scanner.File, registrationCall uint32) (string, bool) {
	for cur, ok := file.FlatParent(registrationCall); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "property_declaration", "variable_declaration":
			name := extractIdentifierFlat(file, cur)
			return name, name != ""
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return "", false
		}
	}
	return "", false
}

func contactsDeclarationOwnerFlat(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration", "object_declaration":
			return cur
		}
	}
	return 0
}

func contactsOwnerLaunchesReadContactsFlat(file *scanner.File, owner uint32, launcherName string) bool {
	if file == nil || launcherName == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(owner, "call_expression", func(call uint32) {
		if found || flatCallExpressionName(file, call) != "launch" {
			return
		}
		if !contactsCallReceiverNameMatchesFlat(file, call, launcherName) {
			return
		}
		_, args := flatCallExpressionParts(file, call)
		permissionArg := flatPositionalValueArgument(file, args, 0)
		if permissionArg == 0 {
			permissionArg = flatNamedValueArgument(file, args, "input")
		}
		permissionExpr := flatValueArgumentExpression(file, permissionArg)
		if contactsExpressionIsReadContactsPermissionFlat(file, permissionExpr) {
			found = true
		}
	})
	return found
}

func contactsCallReceiverNameMatchesFlat(file *scanner.File, call uint32, receiverName string) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	path := contactsIdentifierPathFlat(file, navExpr)
	return len(path) == 2 && path[0] == receiverName && path[1] == "launch"
}

func contactsExpressionIsReadContactsPermissionFlat(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		value, ok := contactsStringLiteralValueFlat(file, idx)
		return ok && value == "android.permission.READ_CONTACTS"
	case "simple_identifier":
		name := file.FlatNodeString(idx, nil)
		return contactsSameFileConstantIsReadContactsFlat(file, idx, name)
	case "navigation_expression":
		return contactsQualifiedReferenceMatchesFlat(file, idx, []string{"Manifest", "permission", "READ_CONTACTS"}) ||
			contactsQualifiedReferenceMatchesFlat(file, idx, []string{"android", "Manifest", "permission", "READ_CONTACTS"})
	case "call_expression":
		return false
	case "value_argument", "value_arguments":
		return contactsValueArgumentsContainReadContactsFlat(file, idx)
	}
	return false
}

func contactsValueArgumentsContainReadContactsFlat(file *scanner.File, args uint32) bool {
	if file == nil || args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if !file.FlatIsNamed(arg) {
			continue
		}
		expr := arg
		if file.FlatType(arg) == "value_argument" {
			expr = flatValueArgumentExpression(file, arg)
		}
		if contactsExpressionIsReadContactsPermissionFlat(file, expr) {
			return true
		}
	}
	return false
}

func contactsStringLiteralValueFlat(file *scanner.File, idx uint32) (string, bool) {
	text := file.FlatNodeText(idx)
	if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) {
		return strings.TrimSuffix(strings.TrimPrefix(text, `"""`), `"""`), !flatContainsStringInterpolation(file, idx)
	}
	value, err := strconv.Unquote(text)
	if err != nil {
		return "", false
	}
	return value, !flatContainsStringInterpolation(file, idx)
}

func contactsSameFileConstantIsReadContactsFlat(file *scanner.File, useIdx uint32, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if found || extractIdentifierFlat(file, prop) != name {
			return
		}
		initExpr := contactsPropertyInitializerFlat(file, prop)
		if contactsExpressionIsReadContactsPermissionFlat(file, initExpr) {
			found = true
		}
	})
	return found
}

func contactsSameOwnerDeclarationHasTypeFlat(file *scanner.File, useIdx uint32, name, typeName string) bool {
	if file == nil || name == "" || typeName == "" {
		return false
	}
	for _, owner := range contactsEnclosingOwnersFlat(file, useIdx) {
		found := false
		file.FlatWalkAllNodes(owner, func(candidate uint32) {
			if found {
				return
			}
			switch file.FlatType(candidate) {
			case "class_parameter", "parameter", "property_declaration", "variable_declaration":
				if contactsDeclarationOwnerFlat(file, candidate) == owner &&
					extractIdentifierFlat(file, candidate) == name &&
					contactsNodeMentionsTypeFlat(file, candidate, typeName) {
					found = true
				}
			}
		})
		if found {
			return true
		}
	}
	return false
}

func contactsEnclosingOwnersFlat(file *scanner.File, idx uint32) []uint32 {
	if file == nil {
		return nil
	}
	var owners []uint32
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration", "object_declaration":
			owners = append(owners, cur)
		}
	}
	owners = append(owners, 0)
	return owners
}

func contactsNodeMentionsTypeFlat(file *scanner.File, idx uint32, typeName string) bool {
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "type_identifier", "simple_identifier":
			found = file.FlatNodeString(candidate, nil) == typeName
		}
	})
	return found
}

func contactsPropertyInitializerFlat(file *scanner.File, prop uint32) uint32 {
	seenEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			if file.FlatNodeTextEquals(child, "=") {
				seenEquals = true
			}
			continue
		}
		if seenEquals {
			return child
		}
	}
	return 0
}

func contactsQualifiedCallMatchesFlat(file *scanner.File, idx uint32, want []string) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return contactsIdentifierPathEquals(contactsIdentifierPathFlat(file, idx), want)
	}
	return contactsIdentifierPathEquals(contactsIdentifierPathFlat(file, navExpr), want)
}

func contactsQualifiedReferenceMatchesFlat(file *scanner.File, idx uint32, want []string) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 {
		return false
	}
	return contactsIdentifierPathEquals(contactsIdentifierPathFlat(file, idx), want)
}

func contactsIdentifierPathFlat(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var path []string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			path = append(path, file.FlatNodeString(candidate, nil))
		}
	})
	return path
}

func contactsIdentifierPathEquals(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
