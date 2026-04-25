// Descriptor metadata for internal/rules/testing_quality.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AssertEqualsArgumentOrderRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AssertEqualsArgumentOrder",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects assertEquals calls with reversed argument order (actual, expected) instead of (expected, actual).",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AssertNullableWithNotNullAssertionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AssertNullableWithNotNullAssertion",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects non-null assertions (!!) inside assertion calls where assertNotNull should be used instead.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AssertTrueOnComparisonRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AssertTrueOnComparison",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects assertTrue(a == b) calls that should use assertEquals for better failure messages.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MixedAssertionLibrariesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MixedAssertionLibraries",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects files that import both JUnit Assert and Google Truth assertion APIs.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MockWithoutVerifyRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MockWithoutVerify",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects mock objects created in test functions that are never verified or stubbed.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RelaxedMockUsedForValueClassRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RelaxedMockUsedForValueClass",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects relaxed mocks of primitive or value types where literal values should be used instead.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RunBlockingInTestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RunBlockingInTest",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects runBlocking usage in test functions where runTest provides better coroutine test support.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RunTestWithDelayRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RunTestWithDelay",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects delay() calls inside runTest blocks where advanceTimeBy should be used instead.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RunTestWithThreadSleepRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RunTestWithThreadSleep",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects Thread.sleep() calls inside runTest blocks where advanceTimeBy should be used instead.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SharedMutableStateInObjectRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "SharedMutableStateInObject",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects mutable var properties in companion objects or object declarations shared across tests.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SpyOnDataClassRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "SpyOnDataClass",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects spying on data class instances where value-based equality breaks spy semantics.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *TestDispatcherNotInjectedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestDispatcherNotInjected",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects production dispatchers (Dispatchers.IO, Default, Main) used directly in test functions.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TestFunctionReturnValueRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestFunctionReturnValue",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects @Test functions that return a non-Unit type, since JUnit ignores return values.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TestInheritanceDepthRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestInheritanceDepth",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects test class inheritance hierarchies deeper than two levels that should be flattened.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *TestNameContainsUnderscoreRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestNameContainsUnderscore",
		RuleSet:       "testing-quality",
		Severity:      "info",
		Description:   "Detects test function names using underscores where backtick-quoted names are preferred.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *TestWithOnlyTodoRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestWithOnlyTodo",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects @Test functions whose body is only a TODO() or fail() call without @Ignore.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TestWithoutAssertionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestWithoutAssertion",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects @Test functions that contain no assertion or verification calls.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *VerifyWithoutMockRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "VerifyWithoutMock",
		RuleSet:       "testing-quality",
		Severity:      "warning",
		Description:   "Detects verify or coVerify calls on objects that are not declared as mocks in the test.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.6,
	}
}
