// Descriptor metadata for internal/rules/android_resource_layout.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AdapterViewChildrenResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AdapterViewChildrenResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IncludeLayoutParamResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "IncludeLayoutParamResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InconsistentLayoutResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InconsistentLayout",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NestedScrollingResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NestedScrollingResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OrientationResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OrientationResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RequiredSizeResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RequiredSizeResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ScrollViewCountResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ScrollViewCountResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ScrollViewSizeResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ScrollViewSizeResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TooDeepLayoutResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TooDeepLayoutResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TooManyViewsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TooManyViewsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UseCompoundDrawablesResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseCompoundDrawablesResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UselessLeafResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UselessLeafResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UselessParentResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UselessParentResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
