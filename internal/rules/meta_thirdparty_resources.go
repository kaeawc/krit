// Descriptor metadata for internal/rules/thirdparty_resources.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

const resourceImportAliasesDescription = "R classes and their import aliases, as `package.R as Alias` entries (slack-lints `import-aliases`; comma-separated entries are split too)."

func (r *FullyQualifiedResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FullyQualifiedResource",
		RuleSet:       moshiRuleSet,
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[FullyQualifiedResourceRule]{
				Name:        "importAliases",
				Description: resourceImportAliasesDescription,
				Apply:       func(r *FullyQualifiedResourceRule, v []string) { r.ImportAliases = v },
			}),
		},
	}
}

func (r *MissingResourceImportAliasRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingResourceImportAlias",
		RuleSet:       moshiRuleSet,
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MissingResourceImportAliasRule]{
				Name:        "importAliases",
				Description: resourceImportAliasesDescription,
				Apply:       func(r *MissingResourceImportAliasRule, v []string) { r.ImportAliases = v },
			}),
		},
	}
}

func (r *WrongResourceImportAliasRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongResourceImportAlias",
		RuleSet:       moshiRuleSet,
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[WrongResourceImportAliasRule]{
				Name:        "importAliases",
				Description: resourceImportAliasesDescription,
				Apply:       func(r *WrongResourceImportAliasRule, v []string) { r.ImportAliases = v },
			}),
		},
	}
}
