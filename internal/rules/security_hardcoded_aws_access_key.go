package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// HardcodedAwsAccessKeyRule detects string literals that look like an AWS
// access-key ID committed directly into source. AWS access-key IDs are
// exactly 20 characters long and start with a known prefix that identifies
// the credential type (long-term IAM user, temporary STS, role, etc.).
type HardcodedAwsAccessKeyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedAwsAccessKeyRule) Confidence() float64 { return api.ConfidenceHigher }

// awsAccessKeyPrefixes is the canonical list of AWS access-key ID prefixes.
// See https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html
var awsAccessKeyPrefixes = [...]string{
	"AKIA", // long-term IAM user
	"ASIA", // temporary STS
	"AGPA", // user group
	"AIDA", // IAM user (legacy unique ID)
	"AROA", // IAM role
	"AIPA", // EC2 instance profile
	"ANPA", // managed policy
	"ANVA", // policy version
}

// looksLikeHardcodedAwsAccessKey reports whether `body` (an unwrapped
// string-literal body, possibly a Kotlin string template) is an AWS
// access-key ID that is not a known placeholder.
//
// An AWS access-key ID is exactly 20 characters: a 4-character type prefix
// followed by 16 uppercase alphanumeric characters. AWS documentation marks
// fake keys with a literal `EXAMPLE` suffix, which we treat as a placeholder.
func looksLikeHardcodedAwsAccessKey(body string) bool {
	token, ok := secretFromInterpolation(body)
	if !ok || token == "" {
		return false
	}
	token = strings.TrimSpace(token)
	if secretLooksLikePlaceholder(token) {
		return false
	}
	if strings.Contains(strings.ToLower(token), "example") {
		return false
	}
	if len(token) != 20 {
		return false
	}
	prefix := token[:4]
	hasPrefix := false
	for _, p := range awsAccessKeyPrefixes {
		if prefix == p {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		return false
	}
	for i := 4; i < 20; i++ {
		c := token[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}
