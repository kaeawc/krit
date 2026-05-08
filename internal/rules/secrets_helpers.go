package rules

import (
	"strings"
)

// secretLooksLikePlaceholder reports whether a candidate secret value is
// clearly a placeholder/dummy that should not be flagged. The token is
// compared case-insensitively against a list of conventional dummy markers
// shared across the hardcoded-secret rule family.
//
// Extracted from looksLikeHardcodedBearerToken so that rules like
// HardcodedJwt and HardcodedAwsAccessKey share one
// consistent allowlist of "obviously not a real secret" markers.
func secretLooksLikePlaceholder(token string) bool {
	lower := strings.ToLower(strings.TrimSpace(token))
	if lower == "" {
		return true
	}
	for _, marker := range secretPlaceholderMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// secretPlaceholderMarkers is the shared list of substrings that disqualify a
// candidate from being treated as a real secret. Lowercased; substring match.
var secretPlaceholderMarkers = []string{
	"placeholder",
	"changeme",
	"replace_me",
	"replace-me",
	"your_token",
	"your-token",
	"your_api_token",
	"your-api-token",
	"your_secret",
	"your-secret",
	"token_here",
	"dummy_token",
	"dummy-token",
	"fake_token",
	"fake-token",
	"<token>",
	"<secret>",
	"<secret-token>",
	"example_",
	"example-",
	"sample_",
	"sample-",
}

// secretFromInterpolation returns the inner literal token if `body` is a
// Kotlin string-template expression of the form `${ "actual" }`, or
// returns body unchanged when the body has no interpolation. When body
// contains an interpolation that is NOT a literal (e.g. `$variable`,
// `${someExpr}`), the second return is false — callers should bail out
// because the runtime value cannot be inspected statically.
func secretFromInterpolation(body string) (string, bool) {
	body = strings.TrimSpace(body)
	switch {
	case strings.HasPrefix(body, "${") && strings.HasSuffix(body, "}"):
		inner := strings.TrimSpace(body[2 : len(body)-1])
		token, ok := kotlinStringLiteralBody(inner)
		if !ok {
			return "", false
		}
		return token, true
	case strings.Contains(body, "${") || strings.Contains(body, "$"):
		return "", false
	}
	return body, true
}

// secretIsBase64URLSegment reports whether s is non-empty and contains only
// base64url-safe characters (A-Z, a-z, 0-9, -, _). JWT segments must satisfy
// this; AWS access keys are a stricter subset.
func secretIsBase64URLSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}
