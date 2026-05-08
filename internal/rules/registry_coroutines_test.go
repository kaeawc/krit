package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterCoroutinesRules_AllRulesPresent asserts that every coroutines
// rule whose registration was previously inlined into the
// registerCoroutinesRules() god-function is still registered after the
// per-rule split. If a rule is dropped from the dispatch table during a
// future refactor, this test fails loudly.
func TestRegisterCoroutinesRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"CollectInOnCreateWithoutLifecycle",
		"GlobalCoroutineUsage",
		"InjectDispatcher",
		"RedundantSuspendModifier",
		"SleepInsteadOfDelay",
		"SuspendFunWithFlowReturnType",
		"CoroutineLaunchedInTestWithoutRunTest",
		"SuspendFunInFinallySection",
		"SuspendFunSwallowedCancellation",
		"SuspendFunWithCoroutineScopeReceiver",
		"ChannelReceiveWithoutClose",
		"CollectionsSynchronizedListIteration",
		"ConcurrentModificationIteration",
		"CoroutineScopeCreatedButNeverCancelled",
		"DeferredAwaitInFinally",
		"FlowWithoutFlowOn",
		"SynchronizedOnString",
		"SynchronizedOnBoxedPrimitive",
		"SynchronizedOnNonFinal",
		"VolatileMissingOnDcl",
		"MutableStateInObject",
		"StateFlowMutableLeak",
		"SharedFlowWithoutReplay",
		"StateFlowCompareByReference",
		"GlobalScopeLaunchInViewModel",
		"SupervisorScopeInEventHandler",
		"WithContextInSuspendFunctionNoop",
		"LaunchWithoutCoroutineExceptionHandler",
		"MainDispatcherInLibraryCode",
	}

	registered := make(map[string]*api.Rule, len(api.Registry))
	for _, r := range api.Registry {
		if r.Category == "coroutines" {
			registered[r.ID] = r
		}
	}

	for _, id := range expected {
		rule, ok := registered[id]
		if !ok {
			t.Errorf("expected coroutines rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("coroutines rule %q has empty Description", id)
		}
		// Aggregate rules keep their lifecycle on r.Aggregate; everything
		// in the coroutines split is a per-node check rule.
		if rule.Check == nil {
			t.Errorf("coroutines rule %q has no Check function", id)
		}
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, id := range expected {
		expectedSet[id] = struct{}{}
	}
	for id := range registered {
		if _, ok := expectedSet[id]; !ok {
			t.Errorf("unexpected coroutines rule %q registered (update test if intentionally added)", id)
		}
	}
}
