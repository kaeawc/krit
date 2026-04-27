package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidResourceValuesRules() {

	// --- from android_resource_values.go ---
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &OnClickResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "OnClickResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "OnClick",
			Brief:      "android:onClick in layout XML is discouraged",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &TextFieldsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TextFieldsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TextFields",
			Brief:      "EditText missing inputType or hint",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ImpliedQuantityResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ImpliedQuantityResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ImpliedQuantity",
			Brief:      "Plural 'one' without %d placeholder",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatMatchesResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatMatchesResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringFormatMatches",
			Brief:      "String.format string doesn't match the XML format string",
			Category:   ALCMessages,
			ALSeverity: ALSError,
			Priority:   9,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringFormatTrivialResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringFormatTrivialResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StringFormatTrivial",
			Brief:      "Trivial string format with single %s",
			Category:   ALCMessages,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GoogleApiKeyInResourcesRule{BaseRule: BaseRule{
			RuleName: "GoogleApiKeyInResources", RuleSetName: "security", Sev: "warning",
			Desc: "Detects Google API keys embedded directly in XML resource files",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
