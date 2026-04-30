// Descriptor metadata for internal/rules/coroutines.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *ChannelReceiveWithoutCloseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ChannelReceiveWithoutClose",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects Channel properties in a class that are never closed, leaking the receiver coroutine.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CollectInOnCreateWithoutLifecycleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CollectInOnCreateWithoutLifecycle",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects Flow.collect calls in lifecycle callbacks that are not wrapped by repeatOnLifecycle.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CollectionsSynchronizedListIterationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CollectionsSynchronizedListIteration",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects iteration over Collections.synchronized* wrappers without external synchronization.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ConcurrentModificationIterationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ConcurrentModificationIteration",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects collection mutation inside for loops that causes ConcurrentModificationException.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CoroutineLaunchedInTestWithoutRunTestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CoroutineLaunchedInTestWithoutRunTest",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects launch/async calls in @Test functions that are not wrapped in runTest.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CoroutineScopeCreatedButNeverCancelled",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects CoroutineScope properties in a class that are never cancelled, leaking coroutines.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeferredAwaitInFinallyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeferredAwaitInFinally",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects Deferred.await() calls inside finally blocks that can throw and mask the original exception.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FlowWithoutFlowOnRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FlowWithoutFlowOn",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects flow chains with a terminal operator but no flowOn, risking execution on the wrong dispatcher.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GlobalCoroutineUsageRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GlobalCoroutineUsage",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects GlobalScope.launch/async usage instead of structured concurrency with a proper CoroutineScope.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *GlobalScopeLaunchInViewModelRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GlobalScopeLaunchInViewModel",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects GlobalScope.launch/async inside ViewModel or Presenter classes instead of viewModelScope.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InjectDispatcherRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InjectDispatcher",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects hardcoded Dispatchers.IO/Default/Unconfined passed as arguments instead of injected dispatchers.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "dispatcherNames",
				Type:        v2.OptStringList,
				Default:     []string{"IO", "Default", "Unconfined", "Main"},
				Description: "Dispatcher names to flag.",
				Apply: func(target interface{}, value interface{}) {
					target.(*InjectDispatcherRule).DispatcherNames = value.([]string)
				},
			},
		},
	}
}

func (r *LaunchWithoutCoroutineExceptionHandlerRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LaunchWithoutCoroutineExceptionHandler",
		RuleSet:       "coroutines",
		Severity:      "info",
		Description:   "Detects launch blocks containing throw statements but no CoroutineExceptionHandler to catch uncaught exceptions.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MainDispatcherInLibraryCodeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MainDispatcherInLibraryCode",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects Dispatchers.Main usage in library modules that lack the kotlinx-coroutines-android dependency.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MutableStateInObjectRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MutableStateInObject",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects mutable var properties inside object declarations that are shared mutable state without synchronization.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RedundantSuspendModifierRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RedundantSuspendModifier",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects suspend functions that contain no suspend calls in their body.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *SharedFlowWithoutReplayRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SharedFlowWithoutReplay",
		RuleSet:       "coroutines",
		Severity:      "info",
		Description:   "Detects MutableSharedFlow() created with default configuration that has no replay or buffer capacity.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SleepInsteadOfDelayRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SleepInsteadOfDelay",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects Thread.sleep() usage inside suspend functions or coroutine builder lambdas instead of delay().",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *StateFlowCompareByReferenceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StateFlowCompareByReference",
		RuleSet:       "coroutines",
		Severity:      "info",
		Description:   "Detects redundant .distinctUntilChanged() after .map{} on StateFlow, which already deduplicates by equality.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StateFlowMutableLeakRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StateFlowMutableLeak",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects publicly exposed MutableStateFlow properties that should be private with a read-only StateFlow accessor.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SupervisorScopeInEventHandlerRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SupervisorScopeInEventHandler",
		RuleSet:       "coroutines",
		Severity:      "info",
		Description:   "Detects supervisorScope with a single child operation where supervisor semantics provide no benefit.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SuspendFunInFinallySectionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SuspendFunInFinallySection",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects suspend function calls inside finally blocks that may not execute if the coroutine is cancelled.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SuspendFunSwallowedCancellationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SuspendFunSwallowedCancellation",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects catch blocks that catch CancellationException without rethrowing, breaking structured concurrency.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *SuspendFunWithCoroutineScopeReceiverRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SuspendFunWithCoroutineScopeReceiver",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects functions that are both suspend and extension on CoroutineScope, which should be one or the other.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *SuspendFunWithFlowReturnTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SuspendFunWithFlowReturnType",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects suspend functions that return a Flow type, since Flow builders are cold and do not require suspend.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *SynchronizedOnBoxedPrimitiveRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SynchronizedOnBoxedPrimitive",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects synchronized() blocks using a boxed primitive value as the lock monitor.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SynchronizedOnNonFinalRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SynchronizedOnNonFinal",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects synchronized() blocks using a var property as the lock, which can change the monitor object.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SynchronizedOnStringRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SynchronizedOnString",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects synchronized() blocks using a string literal as the lock monitor.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *VolatileMissingOnDclRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "VolatileMissingOnDcl",
		RuleSet:       "coroutines",
		Severity:      "warning",
		Description:   "Detects double-checked locking patterns on a var property without @Volatile annotation.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WithContextInSuspendFunctionNoopRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WithContextInSuspendFunctionNoop",
		RuleSet:       "coroutines",
		Severity:      "info",
		Description:   "Detects nested withContext calls using the same dispatcher as the parent, which is redundant.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
