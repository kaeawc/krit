package rules

import "testing"

func TestLooksLikeHardcodedJwt(t *testing.T) {
	// A real-shaped JWT — header is base64 of {"alg":"HS256","typ":"JWT"}.
	realJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	cases := []struct {
		name string
		body string
		want bool
	}{
		{"genuine JWT", realJWT, true},
		{"two segments", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0", false},
		{"missing eyJ prefix", "abcdefgh.abcdefgh.abcdefgh", false},
		{"placeholder marker", "eyJhbGciOiJIUzI1NiJ9.PLACEHOLDER_HERE.eyJzdWIiOiIxIn0", false},
		{"non base64url chars", "eyJhbGciOiJIUzI1NiJ9.bad@@chars==.signature", false},
		{"too short segments", "eyJhbGc.eyJzdWI.SflKxw", false},
		{"interpolated literal token", `${"` + realJWT + `"}`, true},
		{"interpolated dynamic value", "${someVariable}", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeHardcodedJwt(c.body); got != c.want {
				t.Errorf("looksLikeHardcodedJwt(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

func TestSecretLooksLikePlaceholder(t *testing.T) {
	cases := map[string]bool{
		"":                        true,
		"YOUR_API_TOKEN":          true,
		"changeme":                true,
		"REPLACE-ME":              true,
		"<token>":                 true,
		"example_secret_value":    true,
		"AKIAIOSFODNN7EXAMPLE":    false, // not in marker list (AWS-style)
		"genuineRandomTokenAbcd1": false,
	}
	for input, want := range cases {
		if got := secretLooksLikePlaceholder(input); got != want {
			t.Errorf("secretLooksLikePlaceholder(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestSecretIsBase64URLSegment(t *testing.T) {
	cases := map[string]bool{
		"abcDEF123-_": true,
		"":            false,
		"with space":  false,
		"plus+sign":   false,
		"slash/here":  false,
		"equals=pad":  false,
	}
	for input, want := range cases {
		if got := secretIsBase64URLSegment(input); got != want {
			t.Errorf("secretIsBase64URLSegment(%q) = %v, want %v", input, got, want)
		}
	}
}
