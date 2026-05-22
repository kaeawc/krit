package rules

// Helpers for the TLS/SSL family of security rules:
//   - OkHTTPDisableSslValidationRule
//   - DisableCertificatePinningRule
//   - AllowAllHostnameVerifierRule
//   - InsecureTrustManagerRule
//
// Generic text utilities (matchingParenIndex, firstBraceBody,
// stripLineAndBlockComments) live here for proximity to their main
// consumers. The android_security_helpers_test.go test file still
// exercises them — same Go package, no import change.
//
// Extracted from android_security.go as part of the god-file split.
// The rule structs and check methods remain in android_security.go.

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/strutil"
)

func okHTTPDisableSslValidationChainText(file *scanner.File, idx uint32) string {
	best := file.FlatNodeText(idx)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		typ := file.FlatType(cur)
		if typ == "function_declaration" || typ == "method_declaration" || typ == "class_declaration" || typ == "source_file" {
			break
		}
		text := file.FlatNodeText(cur)
		if strings.Contains(text, "OkHttpClient.Builder") {
			best = text
		}
	}
	return best
}

func disableCertificatePinningEmptyBuilder(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || javaAwareCallName(file, idx) != "build" {
		return false
	}
	if !sourceImportsOrMentions(file, "okhttp3.CertificatePinner") &&
		!sourceImportsOrMentions(file, "okhttp3.CertificatePinner.Builder") {
		return false
	}
	chainText := disableCertificatePinningChainText(file, idx)
	compact := strings.Join(strings.Fields(chainText), "")
	if strings.Contains(compact, ".add(") {
		return false
	}
	return strings.Contains(compact, "CertificatePinner.Builder().build(") ||
		strings.Contains(compact, "newCertificatePinner.Builder().build(") ||
		(sourceImportsOrMentions(file, "okhttp3.CertificatePinner.Builder") &&
			(strings.Contains(compact, "Builder().build(") || strings.Contains(compact, "newBuilder().build(")))
}

func disableCertificatePinningChainText(file *scanner.File, idx uint32) string {
	best := file.FlatNodeText(idx)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		typ := file.FlatType(cur)
		if typ == "function_declaration" || typ == "method_declaration" || typ == "class_declaration" || typ == "source_file" {
			break
		}
		text := file.FlatNodeText(cur)
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "CertificatePinner.Builder") ||
			strings.Contains(compact, "Builder().") ||
			strings.Contains(compact, "newBuilder().") {
			best = text
		}
	}
	return best
}

func okHTTPDisableSslValidationAlwaysTrueVerifier(chainText string) bool {
	text := strings.ReplaceAll(chainText, " ", "")
	return strings.Contains(text, "->true") ||
		strings.Contains(text, "returntrue;") ||
		strings.Contains(text, "returntrue}")
}

func okHTTPDisableSslValidationUnsafeTrustManager(chainText string) bool {
	text := strings.ToLower(chainText)
	return strings.Contains(text, "trustall") ||
		strings.Contains(text, "unsafe") ||
		(strings.Contains(text, "x509trustmanager") &&
			(strings.Contains(text, "checkservertrusted") || strings.Contains(text, "checkclienttrusted")))
}

func allowAllHostnameVerifierClass(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	if !sourceImportsOrMentions(file, "javax.net.ssl.HostnameVerifier") {
		return false
	}
	if allowAllHostnameVerifierHasLocalShadow(file, idx) {
		return false
	}
	header := allowAllHostnameVerifierClassHeader(file, idx)
	if header == "" {
		return false
	}
	return insecureTrustManagerTextHasTypeToken(header, "HostnameVerifier") ||
		strings.Contains(header, "javax.net.ssl.HostnameVerifier")
}

func allowAllHostnameVerifierClassHeader(file *scanner.File, idx uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if open := strings.Index(text, "{"); open >= 0 {
		return text[:open]
	}
	return text
}

func allowAllHostnameVerifierHasLocalShadow(file *scanner.File, owner uint32) bool {
	shadowed := false
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if candidate != owner && extractIdentifierFlat(file, candidate) == "HostnameVerifier" {
			shadowed = true
		}
	})
	file.FlatWalkNodes(0, "interface_declaration", func(candidate uint32) {
		if candidate != owner && extractIdentifierFlat(file, candidate) == "HostnameVerifier" {
			shadowed = true
		}
	})
	return shadowed
}

func allowAllHostnameVerifierVerifyMethod(file *scanner.File, owner uint32) uint32 {
	var match uint32
	check := func(method uint32) {
		if match != 0 || !allowAllHostnameVerifierMethodOwnedBy(file, method, owner) {
			return
		}
		if allowAllHostnameVerifierMethodName(file, method) != "verify" {
			return
		}
		text := file.FlatNodeText(method)
		if allowAllHostnameVerifierParamCount(text) != 2 {
			return
		}
		if allowAllHostnameVerifierMethodReturnsTrue(text) {
			match = method
		}
	}
	file.FlatWalkNodes(owner, "function_declaration", check)
	file.FlatWalkNodes(owner, "method_declaration", check)
	return match
}

func allowAllHostnameVerifierMethodOwnedBy(file *scanner.File, method, owner uint32) bool {
	actual, ok := flatEnclosingAncestor(file, method, "class_declaration")
	return ok && actual == owner
}

func allowAllHostnameVerifierMethodName(file *scanner.File, method uint32) string {
	switch file.FlatType(method) {
	case "function_declaration":
		return flatFunctionName(file, method)
	case "method_declaration":
		text := file.FlatNodeText(method)
		if strings.Contains(text, "verify(") {
			return "verify"
		}
	}
	return ""
}

func allowAllHostnameVerifierParamCount(methodText string) int {
	name := strings.Index(methodText, "verify")
	if name < 0 {
		return -1
	}
	openRel := strings.Index(methodText[name:], "(")
	if openRel < 0 {
		return -1
	}
	open := name + openRel
	closeParen := matchingParenIndex(methodText, open)
	if closeParen < 0 {
		return -1
	}
	params := strings.TrimSpace(methodText[open+1 : closeParen])
	if params == "" {
		return 0
	}
	depth := 0
	count := 1
	for _, r := range params {
		switch r {
		case '(', '<', '[', '{':
			depth++
		case ')', '>', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

func allowAllHostnameVerifierMethodReturnsTrue(methodText string) bool {
	cleaned := stripLineAndBlockComments(methodText)
	if body, ok := firstBraceBody(cleaned); ok {
		body = strings.TrimSpace(stripLineAndBlockComments(body))
		return body == "return true" || body == "return true;"
	}
	if eq := strings.LastIndex(cleaned, "="); eq >= 0 {
		expr := strings.TrimSpace(cleaned[eq+1:])
		return expr == "true"
	}
	return false
}

func matchingParenIndex(text string, open int) int {
	if open < 0 || open >= len(text) || text[open] != '(' {
		return -1
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func insecureTrustManagerDecl(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	typ := file.FlatType(idx)
	if typ != "class_declaration" && typ != "object_literal" && typ != "object_creation_expression" {
		return false
	}
	if !sourceImportsOrMentions(file, "javax.net.ssl.X509TrustManager") &&
		!sourceImportsOrMentions(file, "javax.net.ssl.TrustManager") {
		return false
	}
	text := file.FlatNodeText(idx)
	if !insecureTrustManagerTextHasTypeToken(text, "X509TrustManager") &&
		!insecureTrustManagerTextHasTypeToken(text, "TrustManager") {
		return false
	}
	if strings.Contains(text, " by ") {
		return false
	}
	return true
}

func insecureTrustManagerTextHasTypeToken(text, token string) bool {
	return strutil.ContainsTokenWordBoundary(text, token)
}

func insecureTrustManagerTrivialChecks(file *scanner.File, owner uint32) []uint32 {
	var findings []uint32
	check := func(method uint32) {
		if !insecureTrustManagerMethodOwnedBy(file, method, owner) {
			return
		}
		name := insecureTrustManagerMethodName(file, method)
		if name != "checkServerTrusted" && name != "checkClientTrusted" {
			return
		}
		if insecureTrustManagerMethodBodyTrivial(file.FlatNodeText(method), name) {
			findings = append(findings, method)
		}
	}
	file.FlatWalkNodes(owner, "function_declaration", check)
	file.FlatWalkNodes(owner, "method_declaration", check)
	return findings
}

func insecureTrustManagerMethodOwnedBy(file *scanner.File, method, owner uint32) bool {
	actual, ok := flatEnclosingAncestor(file, method, "class_declaration", "object_literal", "object_creation_expression")
	return ok && actual == owner
}

func insecureTrustManagerMethodName(file *scanner.File, method uint32) string {
	switch file.FlatType(method) {
	case "function_declaration":
		return flatFunctionName(file, method)
	case "method_declaration":
		text := file.FlatNodeText(method)
		for _, name := range []string{"checkServerTrusted", "checkClientTrusted"} {
			if strings.Contains(text, name+"(") {
				return name
			}
		}
	}
	return ""
}

func insecureTrustManagerMethodBodyTrivial(methodText, name string) bool {
	open := strings.Index(methodText, name)
	if open < 0 {
		return false
	}
	body, ok := firstBraceBody(methodText[open:])
	if !ok {
		return false
	}
	body = stripLineAndBlockComments(body)
	body = strings.TrimSpace(body)
	return body == "" || body == "return" || body == "return;"
}

func firstBraceBody(text string) (string, bool) {
	start := strings.Index(text, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start+1 : i], true
			}
		}
	}
	return "", false
}

func stripLineAndBlockComments(text string) string {
	for {
		start := strings.Index(text, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(text[start+2:], "*/")
		if end < 0 {
			return text[:start]
		}
		text = text[:start] + text[start+2+end+2:]
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
