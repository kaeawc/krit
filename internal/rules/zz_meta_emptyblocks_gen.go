// Descriptor metadata for internal/rules/emptyblocks.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *EmptyCatchBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyCatchBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects catch blocks with an empty body that silently swallow exceptions.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedExceptionNameRegex",
				Type:        v2.OptRegex,
				Default:     "^(_|ignore|expected)$",
				Description: "Regex for exception names that allow empty catch.",
				Apply: func(target interface{}, value interface{}) {
					target.(*EmptyCatchBlockRule).AllowedExceptionNameRegex = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *EmptyClassBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyClassBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects class declarations with an empty body that can have their braces removed.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyDefaultConstructorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyDefaultConstructor",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects explicit empty default constructors that are redundant and can be removed.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyDoWhileBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyDoWhileBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects do-while loops with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyElseBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyElseBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects else blocks with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyFinallyBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyFinallyBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects finally blocks with an empty body that serve no purpose.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyForBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyForBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects for loops with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyFunctionBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyFunctionBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects function declarations with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoreOverridden",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore overridden functions with empty body.",
				Apply: func(target interface{}, value interface{}) {
					target.(*EmptyFunctionBlockRule).IgnoreOverridden = value.(bool)
				},
			},
		},
	}
}

func (r *EmptyIfBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyIfBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects if blocks with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyInitBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyInitBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects init blocks with an empty body that can be removed.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyKotlinFileRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyKotlinFile",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects Kotlin files with no meaningful code beyond package and import declarations.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *EmptySecondaryConstructorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptySecondaryConstructor",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects secondary constructors with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyTryBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyTryBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects try blocks with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyWhenBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyWhenBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects when expressions with no entries.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}

func (r *EmptyWhileBlockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EmptyWhileBlock",
		RuleSet:       "empty-blocks",
		Severity:      "warning",
		Description:   "Detects while loops with an empty body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}
