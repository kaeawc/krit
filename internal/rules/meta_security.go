// Descriptor metadata for internal/rules/security.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *ContentProviderQueryWithSelectionInterpolationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ContentProviderQueryWithSelectionInterpolation",
		RuleSet:       "security",
		Severity:      "info",
		Description:   "Detects interpolated selection strings passed to ContentResolver.query() that may enable SQL injection.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FileFromUntrustedPathRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FileFromUntrustedPath",
		RuleSet:       "security",
		Severity:      "info",
		Description:   "Detects File construction from untrusted input in extraction or download functions without path traversal guards.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedBearerTokenRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HardcodedBearerToken",
		RuleSet:       "security",
		Severity:      "warning",
		Description:   "Detects bearer authorization strings with hardcoded tokens embedded directly in source code.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedGcpServiceAccountRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HardcodedGcpServiceAccount",
		RuleSet:       "security",
		Severity:      "warning",
		Description:   "Detects embedded GCP service-account JSON or private keys committed into source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
