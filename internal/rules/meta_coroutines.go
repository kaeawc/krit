// Descriptor metadata for internal/rules/coroutines.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ChannelReceiveWithoutCloseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ChannelReceiveWithoutClose",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *CollectInOnCreateWithoutLifecycleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CollectInOnCreateWithoutLifecycle",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *CollectionsSynchronizedListIterationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CollectionsSynchronizedListIteration",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *ConcurrentModificationIterationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConcurrentModificationIteration",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *CoroutineLaunchedInTestWithoutRunTestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CoroutineLaunchedInTestWithoutRunTest",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *CoroutineScopeCreatedButNeverCancelledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CoroutineScopeCreatedButNeverCancelled",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *DeferredAwaitInFinallyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeferredAwaitInFinally",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *FlowWithoutFlowOnRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FlowWithoutFlowOn",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *GlobalCoroutineUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GlobalCoroutineUsage",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		FixLevel:      "semantic",
	}
}

func (r *GlobalScopeLaunchInViewModelRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GlobalScopeLaunchInViewModel",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *InjectDispatcherRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InjectDispatcher",
		RuleSet:       "coroutines",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[InjectDispatcherRule]{
				Name:        "dispatcherNames",
				Default:     []string{"IO", "Default", "Unconfined", "Main"},
				Description: "Dispatcher names to flag.",
				Apply:       func(r *InjectDispatcherRule, v []string) { r.DispatcherNames = v },
			}),
		},
	}
}

func (r *LaunchWithoutCoroutineExceptionHandlerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LaunchWithoutCoroutineExceptionHandler",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *MainDispatcherInLibraryCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MainDispatcherInLibraryCode",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *MutableStateInObjectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MutableStateInObject",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *RedundantSuspendModifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RedundantSuspendModifier",
		RuleSet:       "coroutines",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *SharedFlowWithoutReplayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SharedFlowWithoutReplay",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *SleepInsteadOfDelayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SleepInsteadOfDelay",
		RuleSet:       "coroutines",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *StateFlowCompareByReferenceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StateFlowCompareByReference",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *StateFlowMutableLeakRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StateFlowMutableLeak",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *SupervisorScopeInEventHandlerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SupervisorScopeInEventHandler",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *SuspendFunInFinallySectionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SuspendFunInFinallySection",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *SuspendFunSwallowedCancellationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SuspendFunSwallowedCancellation",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		FixLevel:      "semantic",
	}
}

func (r *SuspendFunWithCoroutineScopeReceiverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SuspendFunWithCoroutineScopeReceiver",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		FixLevel:      "idiomatic",
	}
}

func (r *SynchronizedOnBoxedPrimitiveRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SynchronizedOnBoxedPrimitive",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *SynchronizedOnNonFinalRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SynchronizedOnNonFinal",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *SynchronizedOnStringRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SynchronizedOnString",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *VolatileMissingOnDclRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VolatileMissingOnDcl",
		RuleSet:       "coroutines",
		DefaultActive: true,
	}
}

func (r *WithContextInSuspendFunctionNoopRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WithContextInSuspendFunctionNoop",
		RuleSet:       "coroutines",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}
