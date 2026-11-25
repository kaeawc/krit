package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *ProguardAndroidTxtUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ProguardAndroidTxtUsage", RuleSet: "android-lint", DefaultActive: true}
}

func (r *NotShrinkingResourcesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NotShrinkingResources", RuleSet: "android-lint", DefaultActive: true}
}

func (r *KaptUsageInsteadOfKspRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "KaptUsageInsteadOfKsp", RuleSet: "android-lint", DefaultActive: true}
}

func (r *LifecycleAnnotationProcessorWithJava8Rule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LifecycleAnnotationProcessorWithJava8", RuleSet: "android-lint", DefaultActive: true}
}
