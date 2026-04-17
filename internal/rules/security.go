package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ContentProviderQueryWithSelectionInterpolationRule detects interpolated
// selection strings passed to ContentResolver.query(...).
type ContentProviderQueryWithSelectionInterpolationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *ContentProviderQueryWithSelectionInterpolationRule) Confidence() float64 { return 0.75 }

func (r *ContentProviderQueryWithSelectionInterpolationRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ContentProviderQueryWithSelectionInterpolationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "query" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	selectionArg := flatNamedValueArgument(file, args, "selection")
	if selectionArg == 0 {
		selectionArg = flatPositionalValueArgument(file, args, 2)
	}
	if selectionArg == 0 || !flatContainsStringInterpolation(file, selectionArg) {
		return nil
	}

	if !isLikelyContentResolverQueryFlat(file, idx, args) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(selectionArg)+1,
		file.FlatCol(selectionArg)+1,
		"Interpolated ContentResolver selection string. Use selectionArgs placeholders instead.",
	)}
}

// HardcodedBearerTokenRule detects bearer authorization strings that embed a
// long token literal directly in source.
type HardcodedBearerTokenRule struct {
	FlatDispatchBase
	BaseRule
}

// HardcodedGcpServiceAccountRule detects embedded GCP service-account JSON and
// private keys committed into source files.
type HardcodedGcpServiceAccountRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *HardcodedGcpServiceAccountRule) Confidence() float64 { return 0.75 }

func (r *HardcodedGcpServiceAccountRule) NodeTypes() []string {
	return []string{"string_literal"}
}

func (r *HardcodedGcpServiceAccountRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	lowerPath := strings.ToLower(file.Path)
	if strings.HasSuffix(lowerPath, ".pem") || strings.HasSuffix(lowerPath, ".json") {
		return nil
	}

	text := file.FlatNodeText(idx)
	body, ok := kotlinStringLiteralBody(text)
	if !ok || !looksLikeHardcodedGcpServiceAccount(body) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Hardcoded GCP service account credential literal. Load it from a file or secret storage instead of embedding it in source.",
	)}
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *HardcodedBearerTokenRule) Confidence() float64 { return 0.75 }

func (r *HardcodedBearerTokenRule) NodeTypes() []string {
	return []string{"string_literal"}
}

func (r *HardcodedBearerTokenRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if _, ok := extractHardcodedBearerToken(text); !ok {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Hardcoded bearer token literal. Load the token from config or secret storage instead of embedding it in source.",
	)}
}

func extractHardcodedBearerToken(text string) (string, bool) {
	body, ok := kotlinStringLiteralBody(text)
	if !ok || !strings.HasPrefix(body, "Bearer ") {
		return "", false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(body, "Bearer "))
	if rest == "" {
		return "", false
	}

	var token string
	switch {
	case strings.HasPrefix(rest, "${") && strings.HasSuffix(rest, "}"):
		inner := strings.TrimSpace(rest[2 : len(rest)-1])
		var literal bool
		token, literal = kotlinStringLiteralBody(inner)
		if !literal {
			return "", false
		}
	case strings.Contains(rest, "${") || strings.Contains(rest, "$"):
		return "", false
	case strings.ContainsAny(rest, " \t\r\n"):
		return "", false
	default:
		token = rest
	}

	if !looksLikeHardcodedBearerToken(token) {
		return "", false
	}

	return token, true
}

func kotlinStringLiteralBody(text string) (string, bool) {
	text = strings.TrimSpace(text)
	switch {
	case len(text) >= 6 && strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`):
		return text[3 : len(text)-3], true
	case len(text) >= 2 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`):
		unquoted, err := strconv.Unquote(text)
		if err == nil {
			return unquoted, true
		}
		return text[1 : len(text)-1], true
	default:
		return "", false
	}
}

func looksLikeHardcodedBearerToken(token string) bool {
	token = strings.TrimSpace(token)
	if len(token) < 16 {
		return false
	}

	lower := strings.ToLower(token)
	for _, marker := range []string{
		"placeholder",
		"changeme",
		"replace_me",
		"replace-me",
		"your_token",
		"your-token",
		"your_api_token",
		"your-api-token",
		"token_here",
		"dummy_token",
		"dummy-token",
		"fake_token",
		"fake-token",
		"<token>",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}

	return true
}

func looksLikeHardcodedGcpServiceAccount(body string) bool {
	trimmed := strings.TrimSpace(body)
	return strings.Contains(body, `"type": "service_account"`) ||
		strings.HasPrefix(trimmed, "-----BEGIN PRIVATE KEY-----")
}

// FileFromUntrustedPathRule detects File(parent, child) construction inside
// extract/upload/download-style functions where child is either a literal with
// parent traversal (`..`) or a non-literal path segment without an obvious
// canonical-path containment check.
type FileFromUntrustedPathRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *FileFromUntrustedPathRule) Confidence() float64 { return 0.75 }

func (r *FileFromUntrustedPathRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *FileFromUntrustedPathRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "File" {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	fnName := strings.ToLower(extractIdentifierFlat(file, fn))
	if !isRiskyFileFromPathFunction(fnName) {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	parentArg := flatPositionalValueArgument(file, args, 0)
	childArg := flatPositionalValueArgument(file, args, 1)
	if parentArg == 0 || childArg == 0 {
		return nil
	}

	parentExpr := valueArgumentExpressionTextFlat(file, parentArg)
	childExpr := valueArgumentExpressionTextFlat(file, childArg)
	if childExpr == "" {
		return nil
	}

	if isStringLiteralExpr(childExpr) {
		if !strings.Contains(childExpr, "..") {
			return nil
		}
	} else if hasCanonicalPathContainmentGuardFlat(file, fn, parentExpr) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(childArg)+1,
		file.FlatCol(childArg)+1,
		"File child path comes from untrusted input in extraction/download code. Reject '..' segments or enforce canonical-path containment before writing.",
	)}
}

func isLikelyContentResolverQueryFlat(file *scanner.File, callExpr, args uint32) bool {
	receiver := strings.ToLower(flatReceiverNameFromCall(file, callExpr))
	if strings.Contains(receiver, "resolver") || strings.Contains(receiver, "contentresolver") {
		return true
	}

	uriArg := flatNamedValueArgument(file, args, "uri")
	if uriArg == 0 {
		uriArg = flatPositionalValueArgument(file, args, 0)
	}
	if uriArg == 0 {
		return false
	}

	return strings.Contains(strings.ToLower(file.FlatNodeText(uriArg)), "uri")
}

func isRiskyFileFromPathFunction(name string) bool {
	for _, fragment := range []string{"upload", "extract", "unzip", "download"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}

func valueArgumentExpressionTextFlat(file *scanner.File, arg uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	if idx := strings.Index(text, "="); idx >= 0 {
		return strings.TrimSpace(text[idx+1:])
	}
	return text
}

func isStringLiteralExpr(text string) bool {
	return strings.HasPrefix(text, "\"") || strings.HasPrefix(text, "\"\"\"")
}

func hasCanonicalPathContainmentGuardFlat(file *scanner.File, fn uint32, parentExpr string) bool {
	if file == nil || parentExpr == "" {
		return false
	}
	fnText := file.FlatNodeText(fn)
	return strings.Contains(fnText, ".canonicalPath.startsWith(") &&
		strings.Contains(fnText, parentExpr+".canonicalPath") &&
		strings.Contains(fnText, "File.separator")
}
