package rules

import "testing"

func TestSpdxExpressionMentions(t *testing.T) {
	cases := []struct {
		expr, license string
		want          bool
	}{
		{"MIT", "MIT", true},
		{"MIT OR Apache-2.0", "Apache-2.0", true},
		{"apache-2.0", "Apache-2.0", true},
		{"GPL-2.0+", "GPL-2.0", true},
		{"GPL-3.0-only", "MIT", false},
	}
	for _, c := range cases {
		if got := spdxExpressionMentions(c.expr, c.license); got != c.want {
			t.Errorf("spdxExpressionMentions(%q, %q) = %v, want %v", c.expr, c.license, got, c.want)
		}
	}
}

func TestSpdxIdentifierInvalidAcceptsLicenseRef(t *testing.T) {
	r := &SpdxIdentifierInvalidRule{}
	if bad := r.invalidIdentifiers("LicenseRef-Acme-Proprietary OR MIT"); len(bad) != 0 {
		t.Fatalf("expected LicenseRef to be valid, got %v", bad)
	}
	if bad := r.invalidIdentifiers("NotALicense"); len(bad) != 1 {
		t.Fatalf("expected one invalid identifier, got %v", bad)
	}
}
