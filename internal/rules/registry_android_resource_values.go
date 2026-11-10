package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerAndroidResourceValuesRules() {

	// --- from android_resource_values.go ---
	{
		r := &ReferenceTypeRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ReferenceType", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ReferenceType",
			Brief:      "Resource alias references a different resource type",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &WebViewInScrollViewResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WebViewInScrollViewResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "WebViewLayout",
			Brief:      "WebView inside ScrollView",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &OnClickResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "OnClickResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "OnClick",
			Brief:      "android:onClick in layout XML is discouraged",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TextFieldsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TextFieldsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TextFields",
			Brief:      "Missing `inputType`",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TextViewEditsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TextViewEditsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TextViewEdits",
			Brief:      "TextView should probably be an EditText instead",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InsecureBaseConfigurationResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InsecureBaseConfigurationResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InsecureBaseConfiguration",
			Brief:      "Insecure Base Configuration",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &WebViewLayoutResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WebViewLayoutResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "WebViewLayout",
			Brief:      "WebViews in wrap_content parents",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnusedAttributeResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnusedAttributeResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnusedAttribute",
			Brief:      "Attribute unused on older platforms",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			NeedsLibraryFacts: true,
			Check:             r.check,
		})
	}
	{
		r := &WrongRegionResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WrongRegionResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "WrongRegion",
			Brief:      "Suspicious Language/Region Combination",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LocaleConfigStaleResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LocaleConfigStale", RuleSetName: androidRuleSet, Sev: "info"},
			IssueID:    "LocaleConfigStale",
			Brief:      "locales_config.xml is out of sync with locale-specific values folders",
			Category:   ALCI18N,
			ALSeverity: ALSInformational,
			Priority:   3,
			Origin:     "Krit roadmap",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingQuantityResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingQuantityResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingQuantity",
			Brief:      "Plural missing required quantity",
			Category:   ALCMessages,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnusedQuantityResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnusedQuantityResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnusedQuantity",
			Brief:      "Plural defines quantity unused for language",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ImpliedQuantityResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ImpliedQuantityResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ImpliedQuantity",
			Brief:      "Plural 'one' without %d placeholder",
			Category:   ALCMessages,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatInvalidResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatInvalidResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "StringFormatInvalid",
			Brief:      "Invalid format string",
			Category:   ALCMessages,
			ALSeverity: ALSError,
			Priority:   9,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatCountResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatCountResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringFormatCount",
			Brief:      "Formatting argument types incomplete or inconsistent",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatMatchesResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatMatchesResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "StringFormatMatches",
			Brief:      "`String.format` string doesn't match the XML format string",
			Category:   ALCMessages,
			ALSeverity: ALSError,
			Priority:   9,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &PluralsCandidateResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "PluralsCandidateResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "PluralsCandidate",
			Brief:      "Potential Plurals",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatTrivialResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatTrivialResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringFormatTrivial",
			Brief:      "Placeholder-only string format",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringNotLocalizableResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringNotLocalizableResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringNotLocalizable",
			Brief:      "String resource should not be localized",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &GoogleAPIKeyInResourcesRule{BaseRule: BaseRule{
			RuleName: "GoogleApiKeyInResources", RuleSetName: "security", Sev: "warning",
			Desc: "Detects Google API keys embedded directly in XML resource files",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringResourceMissingPositionalRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringResourceMissingPositional", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringResourceMissingPositional",
			Brief:      "String resource has multiple non-positional format specifiers",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "Krit roadmap",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringTrailingWhitespaceResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringTrailingWhitespace", RuleSetName: androidRuleSet, Sev: "info"},
			IssueID:    "StringTrailingWhitespace",
			Brief:      "String resource has trailing whitespace",
			Category:   ALCI18N,
			ALSeverity: ALSInformational,
			Priority:   3,
			Origin:     "Krit roadmap",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InconsistentArraysResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InconsistentArraysResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InconsistentArrays",
			Brief:      "Inconsistencies in array element counts",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ExtraTextResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ExtraTextResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ExtraText",
			Brief:      "Extraneous text in resource files",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LocaleFolderRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LocaleFolder", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "LocaleFolder",
			Brief:      "Wrong locale folder naming",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UseAlpha2Rule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UseAlpha2", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UseAlpha2",
			Brief:      "3-letter ISO code in locale folder",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
