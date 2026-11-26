package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxLifecycleRuntimeRules registers the checks from androidx.lifecycle:lifecycle-livedata-core and
// lifecycle-runtime 2.11.1 that need value and suspend-call reasoning.
func registerAndroidxLifecycleRuntimeRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}

	nullSafe := &NullSafeMutableLiveDataRule{BaseRule: BaseRule{RuleName: "NullSafeMutableLiveData",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects null or nullable values set on a LiveData whose type argument is non-nullable."}}
	registerAxlRule(nullSafe.BaseRule, nullSafe, []string{"assignment", "call_expression"}, kotlin, axlRuleOptions{},
		nullSafe.check)

	when := &UnsafeLifecycleWhenUsageRule{BaseRule: BaseRule{RuleName: "UnsafeLifecycleWhenUsage",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects View access in the finally block of a suspending try inside Lifecycle.whenStarted and similar."}}
	registerAxlRule(when.BaseRule, when, []string{"call_expression"}, kotlin, axlRuleOptions{}, when.check)
}
