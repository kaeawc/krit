package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidResourceIdsRules() {

	// --- from android_resource_ids.go ---
	{
		r := &DuplicateIdsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DuplicateIdsResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "DuplicateIds",
			Brief:      "Duplicate android:id in layout",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InvalidIdResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InvalidIdResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "InvalidId",
			Brief:      "Malformed android:id value",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingIdResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingIdResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingId",
			Brief:      "Fragments should specify an id or tag",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &CutPasteIdResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "CutPasteIdResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "CutPasteId",
			Brief:      "Likely cut & paste mistakes",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DuplicateIncludedIdsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DuplicateIncludedIdsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DuplicateIncludedIds",
			Brief:      "Duplicate ids across included layouts",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingPrefixResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingPrefixResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingPrefix",
			Brief:      "Attribute missing android: namespace prefix",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NamespaceTypoResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NamespaceTypoResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "NamespaceTypo",
			Brief:      "Misspelled namespace URI",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ResAutoResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ResAutoResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ResAuto",
			Brief:      "Namespace used in resource files should be res-auto",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnusedNamespaceResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnusedNamespaceResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "UnusedNamespace",
			Brief:      "Unused namespace",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &IllegalResourceRefResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "IllegalResourceRefResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "IllegalResourceRef",
			Brief:      "Name is not a valid resource reference format",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &WrongCaseResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WrongCaseResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "WrongCase",
			Brief:      "Wrong case in view tag",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &WrongFolderResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WrongFolderResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "WrongFolder",
			Brief:      "Resource file in the wrong res folder",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InvalidResourceFolderResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InvalidResourceFolderResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "InvalidResourceFolder",
			Brief:      "Invalid resource folder name",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &AppCompatResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AppCompatResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AppCompatResource",
			Brief:      "Using android:showAsAction instead of app:showAsAction",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
