// Descriptor metadata for internal/rules/potentialbugs_exceptions.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *PrintStackTraceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PrintStackTrace",
		RuleSet:       "exceptions",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *TooGenericExceptionCaughtRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TooGenericExceptionCaught",
		RuleSet:       "exceptions",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[TooGenericExceptionCaughtRule]{
				Name:        "allowedExceptionNameRegex",
				Default:     "",
				Description: "Regex for exception variable names to allow.",
				Apply:       func(r *TooGenericExceptionCaughtRule, v *regexp.Regexp) { r.AllowedExceptionNameRegex = v },
			}),
			api.StringListOption(api.StringListOptionSpec[TooGenericExceptionCaughtRule]{
				Name:        "exceptionNames",
				Description: "Generic exception types to flag.",
				Apply:       func(r *TooGenericExceptionCaughtRule, v []string) { r.ExceptionNames = v },
			}),
		},
	}
}

func (r *TooGenericExceptionThrownRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TooGenericExceptionThrown",
		RuleSet:       "exceptions",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[TooGenericExceptionThrownRule]{
				Name:        "exceptionNames",
				Description: "Generic exception types to flag.",
				Apply:       func(r *TooGenericExceptionThrownRule, v []string) { r.ExceptionNames = v },
			}),
		},
	}
}

func (r *UnreachableCatchBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnreachableCatchBlock",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}
