// Descriptor metadata for internal/rules/android_security.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AddJavascriptInterfaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AddJavascriptInterface",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ByteOrderMarkRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ByteOrderMark",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *DrawAllocationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DrawAllocation",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *EasterEggRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EasterEgg",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedContentProviderRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedContentProvider",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedReceiverRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedReceiver",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FieldGetterRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FieldGetter",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FloatMathRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FloatMath",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GetInstanceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GetInstance",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *GrantAllUrisRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GrantAllUris",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HandlerLeakRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HandlerLeak",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RecycleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Recycle",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SecureRandomRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SecureRandom",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "Flags java.util.Random for security-sensitive code and deterministic SecureRandom.setSeed(long) calls.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *TrustedServerRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TrustedServer",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *WorldReadableFilesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WorldReadableFiles",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *WorldWriteableFilesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WorldWriteableFiles",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
