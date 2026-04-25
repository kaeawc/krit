// Descriptor metadata for internal/rules/android_security.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AddJavascriptInterfaceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AddJavascriptInterface",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ByteOrderMarkRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ByteOrderMark",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *DrawAllocationRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DrawAllocation",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *EasterEggRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "EasterEgg",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedContentProviderRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ExportedContentProvider",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedReceiverRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ExportedReceiver",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FieldGetterRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "FieldGetter",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FloatMathRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "FloatMath",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GetInstanceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GetInstance",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *GrantAllUrisRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GrantAllUris",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HandlerLeakRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HandlerLeak",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RecycleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "Recycle",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SecureRandomRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "SecureRandom",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "Flags java.util.Random for security-sensitive code and deterministic SecureRandom.setSeed(long) calls.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *TrustedServerRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TrustedServer",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *WorldReadableFilesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WorldReadableFiles",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *WorldWriteableFilesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WorldWriteableFiles",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
