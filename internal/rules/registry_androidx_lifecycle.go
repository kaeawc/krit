package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxLifecycleRules registers the checks from androidx.lifecycle 2.11.1 (lifecycle-runtime,
// lifecycle-runtime-compose, lifecycle-runtime-testing and lifecycle-viewmodel-compose).
func registerAndroidxLifecycleRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}

	state := &LifecycleCurrentStateInCompositionRule{BaseRule: BaseRule{RuleName: "LifecycleCurrentStateInComposition",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects Lifecycle.currentState read in composition, which does not observe changes; use currentStateAsState()."}}
	registerAxlRule(state.BaseRule, state, []string{"navigation_expression"}, kotlin, axlRuleOptions{}, state.check)

	vm := &ViewModelConstructorInComposableRule{BaseRule: BaseRule{RuleName: "ViewModelConstructorInComposable",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects ViewModels constructed directly in composition instead of through viewModel()."}}
	registerAxlRule(vm.BaseRule, vm, []string{"call_expression"}, kotlin, axlRuleOptions{OptionalResolver: true}, vm.check)

	owner := &TestLifecycleOwnerInCoroutineRule{BaseRule: BaseRule{RuleName: "TestLifecycleOwnerInCoroutine",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects currentState set inside runTest, whose blocking setter hangs in a coroutine; use setCurrentState()."}}
	registerAxlRule(owner.BaseRule, owner, []string{"lambda_literal"}, kotlin, axlRuleOptions{}, owner.check)

	repeat := &RepeatOnLifecycleWrongUsageRule{BaseRule: BaseRule{RuleName: "RepeatOnLifecycleWrongUsage",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects repeatOnLifecycle reached from onStart or onResume of a Fragment or Activity instead of onCreate/onViewCreated."}}
	registerAxlRule(repeat.BaseRule, repeat, []string{"class_declaration"}, kotlin, axlRuleOptions{OptionalResolver: true},
		repeat.check)
}
