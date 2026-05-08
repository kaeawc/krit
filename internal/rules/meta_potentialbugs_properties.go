// Descriptor metadata for internal/rules/potentialbugs_properties.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *PropertyUsedBeforeDeclarationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PropertyUsedBeforeDeclaration",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}

func (r *UnconditionalJumpStatementInLoopRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnconditionalJumpStatementInLoop",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}

func (r *UnnamedParameterUseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnamedParameterUse",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UnnamedParameterUseRule]{
				Name:        "allowSingleParamUse",
				Default:     false,
				Description: "Allow unnamed use of single parameter.",
				Apply:       func(r *UnnamedParameterUseRule, v bool) { r.AllowSingleParamUse = v },
			}),
		},
	}
}

func (r *UnusedUnaryOperatorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedUnaryOperator",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *UselessPostfixExpressionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UselessPostfixExpression",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}
