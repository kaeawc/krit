// Descriptor metadata for internal/rules/emptyblocks.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *EmptyCatchBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyCatchBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "none",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[EmptyCatchBlockRule]{
				Name:        "allowedExceptionNameRegex",
				Default:     "^(_|ignore|expected)$",
				Description: "Regex for exception names that allow empty catch.",
				Apply:       func(r *EmptyCatchBlockRule, v *regexp.Regexp) { r.AllowedExceptionNameRegex = v },
			}),
		},
	}
}

func (r *EmptyClassBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyClassBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyDefaultConstructorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyDefaultConstructor",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyDoWhileBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyDoWhileBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyElseBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyElseBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyFinallyBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyFinallyBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyForBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyForBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyFunctionBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyFunctionBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[EmptyFunctionBlockRule]{
				Name:        "ignoreOverridden",
				Default:     true,
				Description: "Ignore overridden functions with an empty body (intentional framework/interface no-ops). Set to false to also flag empty overrides.",
				Apply:       func(r *EmptyFunctionBlockRule, v bool) { r.IgnoreOverridden = v },
			}),
		},
	}
}

func (r *EmptyIfBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyIfBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyInitBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyInitBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyKotlinFileRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyKotlinFile",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
	}
}

func (r *EmptySecondaryConstructorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptySecondaryConstructor",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyTryBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyTryBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyWhenBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyWhenBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *EmptyWhileBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EmptyWhileBlock",
		RuleSet:       "empty-blocks",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}
