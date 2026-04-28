package rules

import (
	"strconv"
	"strings"
	"unicode"

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
			if isBooleanLiteralTrue(file, flatValueArgumentExpression(file, arg)) {
				allowsFallback = true
			}
		case "setAllowedAuthenticators":
			_, args := flatCallExpressionParts(file, candidate)
			if args == 0 {
				return
			}
			// Walk the argument AST looking for a `simple_identifier`
			// node named DEVICE_CREDENTIAL. This matches the flag
			// whether it appears bare, fully qualified
			// (BiometricManager.Authenticators.DEVICE_CREDENTIAL), or
			// OR'd with other constants — without substring matching
			// the node text.
			file.FlatWalkAllNodes(args, func(inner uint32) {
				if allowsFallback {
					return
				}
				if file.FlatType(inner) == "simple_identifier" &&
					file.FlatNodeText(inner) == "DEVICE_CREDENTIAL" {
					allowsFallback = true
				}
			})
		}
	})
	return allowsFallback
}

func biometricAuthenticateReceiverMatchesFlat(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "authenticate" || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 {
		return false
	}
	if file.FlatType(receiver) == "call_expression" && flatCallExpressionName(file, receiver) == "BiometricPrompt" {
		return true
	}
	name := flatReferenceSimpleName(file, receiver)
	return name != "" && contactsSameOwnerDeclarationHasTypeFlat(file, call, name, "BiometricPrompt")
}

func biometricPromptInfoBuilderExpressionFlat(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" || flatCallExpressionName(file, idx) != "build" {
		return false
	}
	foundBuilder := false
	file.FlatWalkNodes(idx, "call_expression", func(call uint32) {
		if foundBuilder || flatCallExpressionName(file, call) != "Builder" {
			return
		}
		navExpr, _ := flatCallExpressionParts(file, call)
		path := contactsIdentifierPathFlat(file, navExpr)
		if contactsIdentifierPathEquals(path, []string{"PromptInfo", "Builder"}) ||
			contactsIdentifierPathEquals(path, []string{"BiometricPrompt", "PromptInfo", "Builder"}) {
			foundBuilder = true
		}
	})
	return foundBuilder
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

func contactsIdentifierPathHasSuffix(got, want []string) bool {
	if len(got) < len(want) {
		return false
	}
	got = got[len(got)-len(want):]
	return contactsIdentifierPathEquals(got, want)
}

// LocationBackgroundWithoutRationaleRule flags requestPermissions calls for
// ACCESS_BACKGROUND_LOCATION when the file has no shouldShowRequestPermissionRationale call.
type LocationBackgroundWithoutRationaleRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LocationBackgroundWithoutRationaleRule) Confidence() float64 { return 0.75 }

func privacyExpressionIsBackgroundLocationPermissionFlat(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		value, ok := contactsStringLiteralValueFlat(file, idx)
		return ok && value == "android.permission.ACCESS_BACKGROUND_LOCATION"
	case "simple_identifier":
		name := file.FlatNodeString(idx, nil)
		return contactsSameFileConstantIsBackgroundLocationFlat(file, idx, name)
	case "navigation_expression":
		path := contactsIdentifierPathFlat(file, idx)
		return contactsIdentifierPathHasSuffix(path, []string{"Manifest", "permission", "ACCESS_BACKGROUND_LOCATION"}) ||
			contactsIdentifierPathEquals(path, []string{"android", "Manifest", "permission", "ACCESS_BACKGROUND_LOCATION"})
	case "call_expression":
		return flatCallExpressionName(file, idx) == "arrayOf" && privacyValueArgumentsContainBackgroundLocationFlat(file, flatCallKeyArguments(file, idx))
	case "value_argument", "value_arguments":
		return privacyValueArgumentsContainBackgroundLocationFlat(file, idx)
	}
	found := false
	file.FlatWalkAllNodes(idx, func(node uint32) {
		if found || node == idx {
			return
		}
		found = privacyExpressionIsBackgroundLocationPermissionFlat(file, node)
	})
	return found
}

func privacyValueArgumentsContainBackgroundLocationFlat(file *scanner.File, args uint32) bool {
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
		if privacyExpressionIsBackgroundLocationPermissionFlat(file, expr) {
			return true
		}
	}
	return false
}

func contactsSameFileConstantIsBackgroundLocationFlat(file *scanner.File, useIdx uint32, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if found || extractIdentifierFlat(file, prop) != name {
			return
		}
		if privacyExpressionIsBackgroundLocationPermissionFlat(file, contactsPropertyInitializerFlat(file, prop)) {
			found = true
		}
	})
	return found
}

func privacyOwnerHasBackgroundLocationRationaleFlat(file *scanner.File, call uint32) bool {
	owner, ok := flatEnclosingAncestor(file, call, "function_declaration")
	if !ok {
		owner = contactsDeclarationOwnerFlat(file, call)
	}
	if owner == 0 {
		return false
	}
	found := false
	targetStart := file.FlatStartByte(call)
	file.FlatWalkNodes(owner, "call_expression", func(candidate uint32) {
		if found || candidate == call || file.FlatStartByte(candidate) >= targetStart {
			return
		}
		if flatCallExpressionName(file, candidate) != "shouldShowRequestPermissionRationale" {
			return
		}
		found = privacyValueArgumentsContainBackgroundLocationFlat(file, flatCallKeyArguments(file, candidate))
	})
	return found
}

func privacySensitiveScreenName(name string) bool {
	tokens := privacyIdentifierTokenSet(name)
	for _, token := range []string{"login", "password", "passwd", "pwd", "pin", "secure", "payment", "payments", "credential", "credentials", "checkout"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func privacySensitiveScreenDeclaration(file *scanner.File, idx uint32, name string) bool {
	if privacySensitiveScreenName(name) {
		return true
	}
	if !privacyNameHasCardToken(name) {
		return false
	}
	text := strings.ToLower(file.FlatNodeText(idx))
	for _, marker := range []string{"storedcard", "creditcard", "credit_card", "paymentcard", "payment_card", "billingcard", "billing_card"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func privacyNameHasCardToken(name string) bool {
	tokens := privacyIdentifierTokenSet(name)
	return tokens["card"] || tokens["cards"]
}

func privacyComposableLooksLikeScreen(name string) bool {
	tokens := privacyIdentifierTokenSet(name)
	for _, token := range []string{"screen", "activity", "page", "route"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func privacyIdentifierTokenSet(name string) map[string]bool {
	out := make(map[string]bool)
	for _, token := range privacyIdentifierTokens(name) {
		out[token] = true
	}
	return out
}

func privacyIdentifierTokens(name string) []string {
	runes := []rune(strings.Trim(name, "`"))
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(string(current)))
		current = current[:0]
	}
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(current) > 0 {
			prev := current[len(current)-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next)) {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()
	return tokens
}

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
	return hasAnnotationNamed(file, idx, "Composable")
}

func privacyHasPreviewAnnotation(file *scanner.File, idx uint32) bool {
	return hasAnnotationNamed(file, idx, "Preview")
}

func privacyNodeContainsFlagSecureReferenceFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatWalkNodes(idx, "simple_identifier", func(ident uint32) {
		if found {
			return
		}
		found = file.FlatNodeString(ident, nil) == "FLAG_SECURE"
	})
	return found
}

func privacyNodeContainsScreenshotBlockerReferenceFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatWalkNodes(idx, "simple_identifier", func(ident uint32) {
		if found {
			return
		}
		found = file.FlatNodeString(ident, nil) == "ScreenshotBlocker"
	})
	return found
}

func privacySensitiveIdentifierName(name string) bool {
	name = strings.ToLower(strings.Trim(name, "`"))
	for _, part := range []string{"password", "passwd", "pwd", "pin", "secret", "credential"} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

func privacyClipboardCallHasSensitiveSourceFlat(file *scanner.File, call uint32) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(args, "simple_identifier", func(ident uint32) {
		if found {
			return
		}
		name := file.FlatNodeString(ident, nil)
		if !privacySensitiveIdentifierName(name) {
			return
		}
		found = contactsSameOwnerDeclarationNamedFlat(file, call, name)
	})
	return found
}

func contactsSameOwnerDeclarationNamedFlat(file *scanner.File, useIdx uint32, name string) bool {
	if file == nil || name == "" {
		return false
	}
	for _, owner := range contactsEnclosingOwnersFlat(file, useIdx) {
		found := false
		file.FlatWalkAllNodes(owner, func(candidate uint32) {
			if found || candidate == useIdx {
				return
			}
			switch file.FlatType(candidate) {
			case "class_parameter", "parameter", "property_declaration", "variable_declaration":
				if contactsDeclarationOwnerFlat(file, candidate) == owner && extractIdentifierFlat(file, candidate) == name {
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

// ClipboardOnSensitiveInputTypeRule flags setPrimaryClip calls where the
// source variable name suggests a password or credential field.
type ClipboardOnSensitiveInputTypeRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ClipboardOnSensitiveInputTypeRule) Confidence() float64 { return 0.75 }
