// Descriptor metadata for internal/rules/style_braces.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *BracesOnIfStatementsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BracesOnIfStatements",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects if/else statements that are missing braces around their bodies.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "multiLine",
				Type:        v2.OptString,
				Default:     "",
				Description: "Brace policy for multi-line if: never/always/necessary/consistent.",
				Apply: func(target interface{}, value interface{}) {
					target.(*BracesOnIfStatementsRule).MultiLine = value.(string)
				},
			},
			{
				Name:        "singleLine",
				Type:        v2.OptString,
				Default:     "",
				Description: "Brace policy for single-line if: never/always/necessary/consistent.",
				Apply: func(target interface{}, value interface{}) {
					target.(*BracesOnIfStatementsRule).SingleLine = value.(string)
				},
			},
		},
	}
}

func (r *BracesOnWhenStatementsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BracesOnWhenStatements",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects when branches that are missing braces around their bodies.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "multiLine",
				Type:        v2.OptString,
				Default:     "",
				Description: "Brace policy for multi-line when branches.",
				Apply: func(target interface{}, value interface{}) {
					target.(*BracesOnWhenStatementsRule).MultiLine = value.(string)
				},
			},
			{
				Name:        "singleLine",
				Type:        v2.OptString,
				Default:     "",
				Description: "Brace policy for single-line when branches.",
				Apply: func(target interface{}, value interface{}) {
					target.(*BracesOnWhenStatementsRule).SingleLine = value.(string)
				},
			},
		},
	}
}

func (r *MandatoryBracesLoopsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MandatoryBracesLoops",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects for, while, and do-while loops that are missing braces around their bodies.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}
