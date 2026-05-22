package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// HardcodedJwtRule detects string literals that look like a structurally
// valid JSON Web Token (header.payload.signature, all base64url) committed
// directly into source. JWTs in source code typically indicate copy-pasted
// real tokens used for testing — they need to come from a secret store, not
// the repository.
type HardcodedJwtRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedJwtRule) Confidence() float64 { return api.ConfidenceHigh }

// looksLikeHardcodedJwt reports whether `body` (an unwrapped string-literal
// body, possibly a Kotlin string template) is a JWT-shaped token that is
// not a known placeholder.
//
// A JWT has three base64url segments separated by `.`, with a header that
// base64-decodes to JSON starting with `{"alg"`. We do a structural check
// (length thresholds + base64url alphabet) rather than full base64 decoding
// to keep the matcher cheap; the `eyJ` prefix on the header (which is the
// base64 prefix of `{"`) provides additional precision.
func looksLikeHardcodedJwt(body string) bool {
	token, ok := secretFromInterpolation(body)
	if !ok || token == "" {
		return false
	}
	token = strings.TrimSpace(token)
	if secretLooksLikePlaceholder(token) {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	header, payload, signature := parts[0], parts[1], parts[2]
	if !strings.HasPrefix(header, "eyJ") {
		return false
	}
	if len(header) < 8 || len(payload) < 8 || len(signature) < 8 {
		return false
	}
	if !secretIsBase64URLSegment(header) ||
		!secretIsBase64URLSegment(payload) ||
		!secretIsBase64URLSegment(signature) {
		return false
	}
	return true
}
