// Descriptor metadata for internal/rules/di_hygiene.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AnvilContributesBindingWithoutScopeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnvilContributesBindingWithoutScope",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *AnvilMergeComponentEmptyScopeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnvilMergeComponentEmptyScope",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *BindsMismatchedArityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BindsMismatchedArity",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *DeadBindingsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeadBindings",
		RuleSet:       "di-hygiene",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *DiCycleDetectionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DiCycleDetection",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects cycles in the constructor-injected DI binding graph.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		Confidence:    0.75,
	}
}

func (r *HiltInstallInMismatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HiltInstallInMismatch",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *InjectOnAbstractClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InjectOnAbstractClass",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *SingletonOnMutableClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SingletonOnMutableClass",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *MetroFactoryDeclarationShapeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MetroFactoryDeclarationShape",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects Metro factory annotations on concrete or sealed declarations; Metro factories must be interfaces or non-sealed abstract classes.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ScopeOnParameterizedClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ScopeOnParameterizedClass",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *MissingJvmSuppressWildcardsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingJvmSuppressWildcards",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *ModuleWithNonStaticProvidesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ModuleWithNonStaticProvides",
		RuleSet:       "di-hygiene",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *IntoMapMissingKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IntoMapMissingKey",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *IntoSetOnNonSetReturnRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IntoSetOnNonSetReturn",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *SubcomponentNotInstalledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SubcomponentNotInstalled",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @Subcomponent declarations not returned from any parent component method; the subcomponent is orphaned.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.5,
	}
}

func (r *BindsInsteadOfProvidesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BindsInsteadOfProvides",
		RuleSet:       "di-hygiene",
		Severity:      "info",
		Description:   "Detects @Provides functions that return their single parameter unchanged; @Binds is cheaper.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.7,
	}
}

func (r *BindsReturnTypeMatchesParamRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BindsReturnTypeMatchesParam",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @Binds functions whose parameter type equals the return type; a no-op binding.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComponentMissingModuleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComponentMissingModule",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @Component(modules = [...]) declarations whose listed modules do not transitively cover every reachable binding.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.5,
	}
}

func (r *IntoMapDuplicateKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IntoMapDuplicateKey",
		RuleSet:       "di-hygiene",
		Severity:      "warning",
		Description:   "Detects @IntoMap providers that share the same key in the same module/component; duplicate map keys create conflicting contributions.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.7,
	}
}

func (r *IntoSetDuplicateTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IntoSetDuplicateType",
		RuleSet:       "di-hygiene",
		Severity:      "info",
		Description:   "Detects @IntoSet providers that contribute the same concrete impl in the same module/component; the set dedupes, dropping contributions.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.5,
	}
}

func (r *ProviderInsteadOfLazyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ProviderInsteadOfLazy",
		RuleSet:       "di-hygiene",
		Severity:      "info",
		Description:   "Detects Provider<T> constructor params whose .get() is called exactly once; Lazy<T> matches the intent and is cheaper.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *LazyInsteadOfDirectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LazyInsteadOfDirect",
		RuleSet:       "di-hygiene",
		Severity:      "info",
		Description:   "Detects Lazy<T> constructor params whose .get() is called eagerly at class init; direct injection is cheaper.",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *HiltSingletonWithActivityDepRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HiltSingletonWithActivityDep",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}

func (r *HiltEntryPointOnNonInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HiltEntryPointOnNonInterface",
		RuleSet:       "di-hygiene",
		DefaultActive: true,
	}
}
