// Descriptor metadata for internal/rules/android_resource_layout.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AdapterViewChildrenResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AdapterViewChildrenResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *IncludeLayoutParamResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IncludeLayoutParamResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InconsistentLayoutResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InconsistentLayout",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *NestedScrollingResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedScrollingResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *OrientationResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OrientationResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RequiredSizeResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RequiredSizeResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ScrollViewCountResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ScrollViewCountResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ScrollViewSizeResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ScrollViewSizeResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *TooDeepLayoutResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TooDeepLayoutResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *TooManyViewsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TooManyViewsResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UseCompoundDrawablesResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseCompoundDrawablesResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UselessLeafResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UselessLeafResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UselessParentResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UselessParentResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}
