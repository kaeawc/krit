// Descriptor metadata for internal/rules/style_idiomatic.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *UseAnyOrNoneInsteadOfFindRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseAnyOrNoneInsteadOfFind",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseCheckNotNullRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseCheckNotNull",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseCheckOrErrorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseCheckOrError",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseEmptyCounterpartRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseEmptyCounterpart",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseIsNullOrEmptyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseIsNullOrEmpty",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseOrEmptyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseOrEmpty",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseRequireNotNullRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseRequireNotNull",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UseRequireRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseRequire",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}
