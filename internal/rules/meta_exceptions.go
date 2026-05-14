// Descriptor metadata for internal/rules/exceptions.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *ErrorUsageWithThrowableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ErrorUsageWithThrowable",
		RuleSet:       "exceptions",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *ExceptionRaisedInUnexpectedLocationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExceptionRaisedInUnexpectedLocation",
		RuleSet:       "exceptions",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ExceptionRaisedInUnexpectedLocationRule]{
				Name:        "methodNames",
				Default:     []string{"equals", "hashCode", "toString", "finalize"},
				Description: "Method names where exceptions are unexpected.",
				Apply:       func(r *ExceptionRaisedInUnexpectedLocationRule, v []string) { r.MethodNames = v },
			}),
		},
	}
}

func (r *InstanceOfCheckForExceptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InstanceOfCheckForException",
		RuleSet:       "exceptions",
		DefaultActive: true,
	}
}

func (r *NotImplementedDeclarationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NotImplementedDeclaration",
		RuleSet:       "exceptions",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *ObjectExtendsThrowableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObjectExtendsThrowable",
		RuleSet:       "exceptions",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *RethrowCaughtExceptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RethrowCaughtException",
		RuleSet:       "exceptions",
		DefaultActive: true,
	}
}

func (r *ReturnFromFinallyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ReturnFromFinally",
		RuleSet:       "exceptions",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ReturnFromFinallyRule]{
				Name:        "ignoreLabeled",
				Default:     false,
				Description: "Ignore labeled returns.",
				Apply:       func(r *ReturnFromFinallyRule, v bool) { r.IgnoreLabeled = v },
			}),
		},
	}
}

func (r *SwallowedExceptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SwallowedException",
		RuleSet:       "exceptions",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[SwallowedExceptionRule]{
				Name:        "allowedExceptionNameRegex",
				Default:     "^_$",
				Description: "Regex for exception names allowed to be swallowed.",
				Apply:       func(r *SwallowedExceptionRule, v *regexp.Regexp) { r.AllowedExceptionNameRegex = v },
			}),
			api.StringListOption(api.StringListOptionSpec[SwallowedExceptionRule]{
				Name:        "ignoredExceptionTypes",
				Description: "Exception types allowed to be swallowed.",
				Apply:       func(r *SwallowedExceptionRule, v []string) { r.IgnoredExceptionTypes = v },
			}),
			api.BoolOption(api.BoolOptionSpec[SwallowedExceptionRule]{
				Name:        "loggingCountsAsHandling",
				Default:     true,
				Description: "Treat logging calls that include the caught exception as meaningful handling.",
				Apply:       func(r *SwallowedExceptionRule, v bool) { r.LoggingCountsAsHandling = v },
			}),
		},
	}
}

func (r *ThrowingExceptionFromFinallyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ThrowingExceptionFromFinally",
		RuleSet:       "exceptions",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *ThrowingExceptionInMainRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ThrowingExceptionInMain",
		RuleSet:       "exceptions",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ThrowingExceptionsWithoutMessageOrCause",
		RuleSet:       "exceptions",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ThrowingExceptionsWithoutMessageOrCauseRule]{
				Name:        "exceptions",
				Description: "Exception types to check.",
				Apply:       func(r *ThrowingExceptionsWithoutMessageOrCauseRule, v []string) { r.Exceptions = v },
			}),
		},
	}
}

func (r *ThrowingNewInstanceOfSameExceptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ThrowingNewInstanceOfSameException",
		RuleSet:       "exceptions",
		DefaultActive: true,
	}
}
