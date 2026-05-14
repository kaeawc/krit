// Descriptor metadata for internal/rules/testing_quality.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AssertEqualsArgumentOrderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AssertEqualsArgumentOrder",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *AssertNullableWithNotNullAssertionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AssertNullableWithNotNullAssertion",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *AssertTrueOnComparisonRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AssertTrueOnComparison",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *UntestedPublicAPIRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UntestedPublicApi",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *MixedAssertionLibrariesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MixedAssertionLibraries",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *MockWithoutVerifyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MockWithoutVerify",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *RelaxedMockUsedForValueClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RelaxedMockUsedForValueClass",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *RunBlockingInTestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RunBlockingInTest",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *RunTestWithDelayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RunTestWithDelay",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *RunTestWithThreadSleepRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RunTestWithThreadSleep",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *SharedMutableStateInObjectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SharedMutableStateInObject",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *SpyOnDataClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpyOnDataClass",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *TestDispatcherNotInjectedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestDispatcherNotInjected",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *TestFunctionReturnValueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestFunctionReturnValue",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *TestInheritanceDepthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestInheritanceDepth",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonThresholdTuning,
	}
}

func (r *TestNameContainsUnderscoreRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestNameContainsUnderscore",
		RuleSet:       "testing-quality",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *TestWithOnlyTodoRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestWithOnlyTodo",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}

func (r *TestWithoutAssertionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestWithoutAssertion",
		RuleSet:       "testing-quality",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[TestWithoutAssertionRule]{
				Name:        "allowNoAssertionTests",
				Default:     false,
				Description: "Allow @Test functions with no explicit assertion or verification call.",
				Apply:       func(r *TestWithoutAssertionRule, v bool) { r.AllowNoAssertionTests = v },
			}),
			api.StringListOption(api.StringListOptionSpec[TestWithoutAssertionRule]{
				Name:        "assertionMethodPatterns",
				Description: "Additional method-name glob patterns that should count as assertion-bearing test DSL calls.",
				Apply:       func(r *TestWithoutAssertionRule, v []string) { r.AssertionMethodPatterns = v },
			}),
		},
	}
}

func (r *VerifyWithoutMockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VerifyWithoutMock",
		RuleSet:       "testing-quality",
		DefaultActive: true,
	}
}
