// Descriptor metadata for internal/rules/exceptions.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *ErrorUsageWithThrowableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ErrorUsageWithThrowable",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects error() calls that pass a Throwable argument instead of using throw directly.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExceptionRaisedInUnexpectedLocationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExceptionRaisedInUnexpectedLocation",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects throw statements inside equals, hashCode, toString, or finalize methods.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "methodNames",
				Type:        v2.OptStringList,
				Default:     []string{"equals", "hashCode", "toString", "finalize"},
				Description: "Method names where exceptions are unexpected.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ExceptionRaisedInUnexpectedLocationRule).MethodNames = value.([]string)
				},
			},
		},
	}
}

func (r *InstanceOfCheckForExceptionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InstanceOfCheckForException",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects instanceof/is checks for exception types inside catch blocks instead of using specific catch clauses.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NotImplementedDeclarationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NotImplementedDeclaration",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects TODO() calls that throw NotImplementedError at runtime.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ObjectExtendsThrowableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObjectExtendsThrowable",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects Kotlin object declarations that extend Throwable, which are singletons that lose stack trace information.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RethrowCaughtExceptionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RethrowCaughtException",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects catch blocks whose only statement is rethrowing the caught exception, making the catch block useless.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ReturnFromFinallyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ReturnFromFinally",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects return statements inside finally blocks that can swallow exceptions from the try/catch.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoreLabeled",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore labeled returns.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ReturnFromFinallyRule).IgnoreLabeled = value.(bool)
				},
			},
		},
	}
}

func (r *SwallowedExceptionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SwallowedException",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects catch blocks that silently swallow the caught exception without logging, handling, or rethrowing.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedExceptionNameRegex",
				Type:        v2.OptRegex,
				Default:     "^_$",
				Description: "Regex for exception names allowed to be swallowed.",
				Apply: func(target interface{}, value interface{}) {
					target.(*SwallowedExceptionRule).AllowedExceptionNameRegex = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "ignoredExceptionTypes",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Exception types allowed to be swallowed.",
				Apply: func(target interface{}, value interface{}) {
					target.(*SwallowedExceptionRule).IgnoredExceptionTypes = value.([]string)
				},
			},
		},
	}
}

func (r *ThrowingExceptionFromFinallyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ThrowingExceptionFromFinally",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects throw statements inside finally blocks that can mask exceptions from the try/catch.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *ThrowingExceptionInMainRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ThrowingExceptionInMain",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects throw statements inside the main() function instead of graceful error handling.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ThrowingExceptionsWithoutMessageOrCause",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects common exception types thrown without a descriptive message or cause argument.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "exceptions",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Exception types to check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ThrowingExceptionsWithoutMessageOrCauseRule).Exceptions = value.([]string)
				},
			},
		},
	}
}

func (r *ThrowingNewInstanceOfSameExceptionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ThrowingNewInstanceOfSameException",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects catch blocks that throw a new instance of the same exception type instead of rethrowing the original.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
