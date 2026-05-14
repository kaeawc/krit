// Descriptor metadata for internal/rules/style_expressions.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CollapsibleIfStatementsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CollapsibleIfStatements",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *ExplicitItLambdaMultipleParametersRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExplicitItLambdaMultipleParameters",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "cosmetic",
	}
}

func (r *ExplicitItLambdaParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExplicitItLambdaParameter",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "cosmetic",
	}
}

func (r *ExpressionBodySyntaxRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExpressionBodySyntax",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ExpressionBodySyntaxRule]{
				Name:        "includeLineWrapping",
				Default:     false,
				Description: "Suggest expression body for multi-line returns.",
				Apply:       func(r *ExpressionBodySyntaxRule, v bool) { r.IncludeLineWrapping = v },
			}),
		},
	}
}

func (r *FunctionOnlyReturningConstantRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FunctionOnlyReturningConstant",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[FunctionOnlyReturningConstantRule]{
				Name:        "excludedFunctions",
				Description: "Functions excluded from this rule.",
				Apply:       func(r *FunctionOnlyReturningConstantRule, v []string) { r.ExcludedFunctions = v },
			}),
			api.BoolOption(api.BoolOptionSpec[FunctionOnlyReturningConstantRule]{
				Name:        "ignoreActualFunction",
				Default:     false,
				Description: "Ignore actual functions.",
				Apply:       func(r *FunctionOnlyReturningConstantRule, v bool) { r.IgnoreActualFunction = v },
			}),
			api.BoolOption(api.BoolOptionSpec[FunctionOnlyReturningConstantRule]{
				Name:        "ignoreOverridableFunction",
				Default:     false,
				Description: "Ignore open/override functions.",
				Apply:       func(r *FunctionOnlyReturningConstantRule, v bool) { r.IgnoreOverridableFunction = v },
			}),
		},
	}
}

func (r *LoopWithTooManyJumpStatementsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LoopWithTooManyJumpStatements",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[LoopWithTooManyJumpStatementsRule]{
				Name:        "maxJumpCount",
				Aliases:     []string{"threshold"},
				Default:     1,
				Description: "Maximum jump statements allowed in a loop.",
				Apply:       func(r *LoopWithTooManyJumpStatementsRule, v int) { r.MaxJumps = v },
			}),
		},
	}
}

func (r *MayBeConstantRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MayBeConstant",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *ModifierOrderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ModifierOrder",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "cosmetic",
	}
}

func (r *ReturnCountRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ReturnCount",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ReturnCountRule]{
				Name:        "excludeGuardClauses",
				Default:     false,
				Description: "Exclude guard clause returns from the count.",
				Apply:       func(r *ReturnCountRule, v bool) { r.ExcludeGuardClauses = v },
			}),
			api.BoolOption(api.BoolOptionSpec[ReturnCountRule]{
				Name:        "excludeLabeled",
				Default:     false,
				Description: "Exclude labeled returns from the count.",
				Apply:       func(r *ReturnCountRule, v bool) { r.ExcludeLabeled = v },
			}),
			api.BoolOption(api.BoolOptionSpec[ReturnCountRule]{
				Name:        "excludeReturnFromLambda",
				Default:     true,
				Description: "Exclude return@lambda from the count.",
				Apply:       func(r *ReturnCountRule, v bool) { r.ExcludeReturnFromLambda = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ReturnCountRule]{
				Name:        "excludedFunctions",
				Description: "Functions excluded from this rule.",
				Apply:       func(r *ReturnCountRule, v []string) { r.ExcludedFunctions = v },
			}),
			api.IntOption(api.IntOptionSpec[ReturnCountRule]{
				Name:        "max",
				Aliases:     []string{"threshold"},
				Default:     2,
				Description: "Maximum allowed return statements.",
				Apply:       func(r *ReturnCountRule, v int) { r.Max = v },
			}),
		},
	}
}

func (r *SafeCastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SafeCast",
		RuleSet:       "style",
		DefaultActive: true,
	}
}

func (r *ThrowsCountRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ThrowsCount",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ThrowsCountRule]{
				Name:        "excludeGuardClauses",
				Default:     false,
				Description: "Exclude guard clause throws from the count.",
				Apply:       func(r *ThrowsCountRule, v bool) { r.ExcludeGuardClauses = v },
			}),
			api.IntOption(api.IntOptionSpec[ThrowsCountRule]{
				Name:        "max",
				Aliases:     []string{"threshold"},
				Default:     2,
				Description: "Maximum allowed throw statements.",
				Apply:       func(r *ThrowsCountRule, v int) { r.Max = v },
			}),
		},
	}
}

func (r *VarCouldBeValRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VarCouldBeVal",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[VarCouldBeValRule]{
				Name:        "ignoreLateinitVar",
				Default:     false,
				Description: "Ignore lateinit var declarations.",
				Apply:       func(r *VarCouldBeValRule, v bool) { r.IgnoreLateinitVar = v },
			}),
		},
	}
}
