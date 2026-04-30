// Descriptor metadata for internal/rules/potentialbugs_exceptions.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *PrintStackTraceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PrintStackTrace",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects printStackTrace() calls that should use a logger instead.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *TooGenericExceptionCaughtRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TooGenericExceptionCaught",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects catching overly generic exception types like Exception or Throwable.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedExceptionNameRegex",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for exception variable names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooGenericExceptionCaughtRule).AllowedExceptionNameRegex = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "exceptionNames",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Generic exception types to flag.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooGenericExceptionCaughtRule).ExceptionNames = value.([]string)
				},
			},
		},
	}
}

func (r *TooGenericExceptionThrownRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TooGenericExceptionThrown",
		RuleSet:       "exceptions",
		Severity:      "warning",
		Description:   "Detects throwing overly generic exception types like Exception or Throwable.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "exceptionNames",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Generic exception types to flag.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooGenericExceptionThrownRule).ExceptionNames = value.([]string)
				},
			},
		},
	}
}

func (r *UnreachableCatchBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnreachableCatchBlock",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects catch blocks that are unreachable because a more general exception type is caught above.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnreachableCodeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnreachableCode",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects code after return, throw, break, or continue statements that can never execute.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
