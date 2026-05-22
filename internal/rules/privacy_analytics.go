package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

var piiKeyPattern = regexp.MustCompile(`(?i)(email|phone|ssn|dob|birth|address|lat[^a-z]|lng[^a-z]|location)`)

var piiPropertyNames = map[string]bool{
	"email":       true,
	"phoneNumber": true,
	"username":    true,
	"phone":       true,
	"ssn":         true,
}

// AnalyticsEventWithPiiParamNameRule flags analytics event calls whose bundle
// argument includes a key matching PII patterns like email, phone, ssn.
type AnalyticsEventWithPiiParamNameRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsEventWithPiiParamNameRule) Confidence() float64 { return api.ConfidenceMedium }

// AnalyticsUserIDFromPiiRule flags calls to user-ID setters whose argument
// is a property named email, phoneNumber, username, etc.
type AnalyticsUserIDFromPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsUserIDFromPiiRule) Confidence() float64 { return api.ConfidenceMedium }

// CrashlyticsCustomKeyWithPiiRule flags setCustomKey calls where the key
// name matches PII patterns.
type CrashlyticsCustomKeyWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CrashlyticsCustomKeyWithPiiRule) Confidence() float64 { return api.ConfidenceMedium }

// FirebaseRemoteConfigDefaultsWithPiiRule flags setDefaults/setDefaultsAsync
// calls whose map keys match PII patterns.
type FirebaseRemoteConfigDefaultsWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) Confidence() float64 { return api.ConfidenceMedium }

// AnalyticsCallWithoutConsentGateRule flags analytics event calls that are not
// guarded by a consent/privacy/GDPR check.
type AnalyticsCallWithoutConsentGateRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsCallWithoutConsentGateRule) Confidence() float64 { return api.ConfidenceMedium }

func isAnalyticsEventMethod(name string) bool {
	switch name {
	case "logEvent", "trackEvent", "track", "logCustomEvent":
		return true
	}
	return false
}

var analyticsReceiverTypes = []string{
	"com.google.firebase.analytics.FirebaseAnalytics",
	"FirebaseAnalytics",
	"Analytics",
}

var crashlyticsReceiverTypes = []string{
	"com.google.firebase.crashlytics.FirebaseCrashlytics",
	"FirebaseCrashlytics",
}

var remoteConfigReceiverTypes = []string{
	"com.google.firebase.remoteconfig.FirebaseRemoteConfig",
	"FirebaseRemoteConfig",
	"RemoteConfig",
}

func privacyCallHasReceiverType(ctx *api.Context, call uint32, allowed []string) bool {
	if ctx.File == nil {
		return false
	}
	if ctx.Resolver != nil && semantics.MatchQualifiedReceiver(ctx, call, allowed...) {
		return true
	}
	file := ctx.File
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return false
	}
	if privacyNavigationMentionsAllowedType(file, nav, allowed) {
		return true
	}
	receiverName := flatReceiverNameFromCall(file, call)
	return receiverName != "" && privacySameFileReceiverType(file, call, receiverName, allowed)
}

func privacyNavigationMentionsAllowedType(file *scanner.File, nav uint32, allowed []string) bool {
	found := false
	file.FlatWalkAllNodes(nav, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			found = privacyAllowedTypeName(file.FlatNodeString(candidate, nil), allowed)
		}
	})
	return found
}

func privacySameFileReceiverType(file *scanner.File, ref uint32, name string, allowed []string) bool {
	found := false
	file.FlatWalkAllNodes(0, func(decl uint32) {
		if found || file.FlatStartByte(decl) > file.FlatStartByte(ref) {
			return
		}
		switch file.FlatType(decl) {
		case "property_declaration", "parameter", "class_parameter":
		default:
			return
		}
		if extractIdentifierFlat(file, decl) != name || !declarationVisibleFromReference(file, decl, ref) {
			return
		}
		typeName := flatLastIdentifierInNode(file, decl)
		if privacyAllowedTypeName(typeName, allowed) {
			found = true
		}
	})
	return found
}

func privacyAllowedTypeName(name string, allowed []string) bool {
	if name == "" {
		return false
	}
	for _, candidate := range allowed {
		if name == candidate || strings.HasSuffix(candidate, "."+name) {
			return true
		}
	}
	return false
}

func isUserIDSetterMethod(name string) bool {
	switch name {
	case "setUserId", "setUserIdentifier", "identify":
		return true
	}
	return false
}

func privacyLastPropertyName(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.LastIndex(text, "."); idx >= 0 {
		return text[idx+1:]
	}
	return text
}

var consentTokenPattern = regexp.MustCompile(`(?i)(consent|gdpr|privacy|tracking)`)

func privacyCallIsInsideConsentGuard(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		typ := file.FlatType(cur)
		if typ == "if_expression" {
			condText := privacyIfConditionText(file, cur)
			if consentTokenPattern.MatchString(condText) {
				return true
			}
		}
		if typ == "when_expression" {
			subjText := file.FlatNodeText(cur)
			if consentTokenPattern.MatchString(subjText) {
				return true
			}
		}
	}
	return false
}

func privacyIfConditionText(file *scanner.File, ifNode uint32) string {
	for child := file.FlatFirstChild(ifNode); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		if typ == "control_structure_body" || typ == "statements" {
			break
		}
		if typ != "if" && typ != "(" && typ != ")" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func privacyHasConsentEarlyReturn(file *scanner.File, fn, target uint32) bool {
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}
	targetStart := file.FlatStartByte(target)
	found := false
	file.FlatWalkNodes(body, "if_expression", func(ifNode uint32) {
		if found || file.FlatStartByte(ifNode) >= targetStart {
			return
		}
		condText := privacyIfConditionText(file, ifNode)
		if !consentTokenPattern.MatchString(condText) {
			return
		}
		if privacyContainsReturnStatement(file, ifNode) {
			found = true
		}
	})
	return found
}

// privacyContainsReturnStatement reports whether the AST subtree rooted
// at node holds a real `return` jump_expression. Substring matches
// against identifiers like `returnedValue` or text inside string
// literals must not count.
func privacyContainsReturnStatement(file *scanner.File, node uint32) bool {
	found := false
	file.FlatWalkNodes(node, "jump_expression", func(jump uint32) {
		if found {
			return
		}
		first := file.FlatChild(jump, 0)
		if first == 0 {
			return
		}
		switch file.FlatType(first) {
		case "return", "return@":
			found = true
		}
	})
	return found
}
