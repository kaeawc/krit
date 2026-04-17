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

func (r *AnalyticsEventWithPiiParamNameRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *AnalyticsEventWithPiiParamNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !isAnalyticsEventMethod(name) {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	var findings []scanner.Finding
	file.FlatWalkNodes(args, "infix_expression", func(infix uint32) {
		text := file.FlatNodeText(infix)
		if !strings.Contains(text, " to ") {
			return
		}
		parts := strings.SplitN(text, " to ", 2)
		keyText := strings.Trim(strings.TrimSpace(parts[0]), "\"")
		if piiKeyPattern.MatchString(keyText) {
			findings = append(findings, r.Finding(
				file,
				file.FlatRow(infix)+1,
				file.FlatCol(infix)+1,
				"Analytics event parameter \""+keyText+"\" looks like PII. Avoid sending personally identifiable information to analytics services.",
			))
		}
	})

	if len(findings) > 0 {
		return findings
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
			findings = append(findings, r.Finding(
				file,
				file.FlatRow(strNode)+1,
				file.FlatCol(strNode)+1,
				"Analytics event parameter \""+body+"\" looks like PII. Avoid sending personally identifiable information to analytics services.",
			))
		}
	})

	return findings
}

// AnalyticsUserIdFromPiiRule flags calls to user-ID setters whose argument
// is a property named email, phoneNumber, username, etc.
type AnalyticsUserIdFromPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsUserIdFromPiiRule) Confidence() float64 { return 0.75 }

func (r *AnalyticsUserIdFromPiiRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *AnalyticsUserIdFromPiiRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !isUserIdSetterMethod(name) {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return nil
	}
	argExpr := flatValueArgumentExpression(file, arg)
	if argExpr == 0 {
		return nil
	}

	argText := file.FlatNodeText(argExpr)
	lastProp := privacyLastPropertyName(argText)
	if !piiPropertyNames[lastProp] {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(argExpr)+1,
		file.FlatCol(argExpr)+1,
		"User ID set from PII property \""+lastProp+"\". User IDs should be opaque identifiers, not personally identifiable information.",
	)}
}

// CrashlyticsCustomKeyWithPiiRule flags setCustomKey calls where the key
// name matches PII patterns.
type CrashlyticsCustomKeyWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CrashlyticsCustomKeyWithPiiRule) Confidence() float64 { return 0.75 }

func (r *CrashlyticsCustomKeyWithPiiRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *CrashlyticsCustomKeyWithPiiRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "setCustomKey" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return nil
	}
	argExpr := flatValueArgumentExpression(file, arg)
	if argExpr == 0 {
		return nil
	}

	body, ok := kotlinStringLiteralBody(file.FlatNodeText(argExpr))
	if !ok {
		return nil
	}

	if !piiKeyPattern.MatchString(body) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(argExpr)+1,
		file.FlatCol(argExpr)+1,
		"Crashlytics custom key \""+body+"\" looks like PII. Crash reports should not carry personally identifiable information.",
	)}
}

// FirebaseRemoteConfigDefaultsWithPiiRule flags setDefaults/setDefaultsAsync
// calls whose map keys match PII patterns.
type FirebaseRemoteConfigDefaultsWithPiiRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) Confidence() float64 { return 0.75 }

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "setDefaults" && name != "setDefaultsAsync" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	var findings []scanner.Finding
	file.FlatWalkNodes(args, "infix_expression", func(infix uint32) {
		text := file.FlatNodeText(infix)
		if !strings.Contains(text, " to ") {
			return
		}
		parts := strings.SplitN(text, " to ", 2)
		keyText := strings.Trim(strings.TrimSpace(parts[0]), "\"")
		if piiKeyPattern.MatchString(keyText) {
			findings = append(findings, r.Finding(
				file,
				file.FlatRow(infix)+1,
				file.FlatCol(infix)+1,
				"Remote Config default key \""+keyText+"\" looks like PII. Remote Config values are not encrypted at rest.",
			))
		}
	})
	return findings
}

// AnalyticsCallWithoutConsentGateRule flags analytics event calls that are not
// guarded by a consent/privacy/GDPR check.
type AnalyticsCallWithoutConsentGateRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AnalyticsCallWithoutConsentGateRule) Confidence() float64 { return 0.75 }

func (r *AnalyticsCallWithoutConsentGateRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *AnalyticsCallWithoutConsentGateRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !isAnalyticsEventMethod(name) {
		return nil
	}

	if privacyCallIsInsideConsentGuard(file, idx) {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if ok && privacyHasConsentEarlyReturn(file, fn, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Analytics call without a visible consent gate. Guard analytics events behind a consent check (e.g. if (consent.analyticsAllowed) { ... }).",
	)}
}

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
	body := file.FlatFindChild(fn, "function_body")
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
