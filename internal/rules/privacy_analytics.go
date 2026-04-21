package rules

import (
	"regexp"
	"strings"

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

func (r *AnalyticsEventWithPiiParamNameRule) Confidence() float64 { return 0.75 }

// AnalyticsUserIdFromPiiRule flags calls to user-ID setters whose argument
// is a property named email, phoneNumber, username, etc.
type AnalyticsUserIdFromPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsUserIdFromPiiRule) Confidence() float64 { return 0.75 }

// CrashlyticsCustomKeyWithPiiRule flags setCustomKey calls where the key
// name matches PII patterns.
type CrashlyticsCustomKeyWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CrashlyticsCustomKeyWithPiiRule) Confidence() float64 { return 0.75 }

// FirebaseRemoteConfigDefaultsWithPiiRule flags setDefaults/setDefaultsAsync
// calls whose map keys match PII patterns.
type FirebaseRemoteConfigDefaultsWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) Confidence() float64 { return 0.75 }

// AnalyticsCallWithoutConsentGateRule flags analytics event calls that are not
// guarded by a consent/privacy/GDPR check.
type AnalyticsCallWithoutConsentGateRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsCallWithoutConsentGateRule) Confidence() float64 { return 0.75 }

func isAnalyticsEventMethod(name string) bool {
	switch name {
	case "logEvent", "trackEvent", "track", "logCustomEvent":
		return true
	}
	return false
}

func isUserIdSetterMethod(name string) bool {
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
		ifText := file.FlatNodeText(ifNode)
		if strings.Contains(ifText, "return") {
			found = true
		}
	})
	return found
}
