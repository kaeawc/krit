package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidResourceA11yRules() {

	// --- from android_resource_a11y.go ---
	{
		r := &HardcodedValuesResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "HardcodedValuesResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "HardcodedText",
			Brief:      "Hardcoded text in layout XML",
			Category:   ALCI18N,
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
		r := &MissingContentDescriptionResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingContentDescriptionResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ContentDescription",
			Brief:      "Image without contentDescription",
			Category:   ALCAccessibility,
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
		r := &LabelForResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LabelForResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "LabelFor",
			Brief:      "EditText without a corresponding labelFor",
			Category:   ALCAccessibility,
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
		r := &ClickableViewAccessibilityResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ClickableViewAccessibilityResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ClickableViewAccessibility",
			Brief:      "Clickable view missing contentDescription",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &BackButtonResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "BackButtonResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "BackButton",
			Brief:      "Explicit back button in layout",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ButtonCaseResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ButtonCaseResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ButtonCase",
			Brief:      "OK/Cancel button with wrong capitalization",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ButtonOrderResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ButtonOrderResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ButtonOrder",
			Brief:      "Cancel button should appear before OK button",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ButtonStyleResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ButtonStyleResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ButtonStyle",
			Brief:      "Dialog button without borderless style",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutClickableWithoutMinSizeRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LayoutClickableWithoutMinSize", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ClickableMinSize",
			Brief:      "Clickable view below 48dp",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutEditTextMissingImportanceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LayoutEditTextMissingImportance", RuleSetName: androidRuleSet, Sev: "info"},
			IssueID:    "AutofillImportance",
			Brief:      "EditText missing importantForAutofill",
			Category:   ALCAccessibility,
			ALSeverity: ALSInformational,
			Priority:   4,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutImportantForAccessibilityNoRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LayoutImportantForAccessibilityNo", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ImportantForAccessibility",
			Brief:      "Interactive view hidden from accessibility",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutAutofillHintMismatchRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LayoutAutofillHintMismatch", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AutofillHintMismatch",
			Brief:      "inputType without matching autofillHints",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutMinTouchTargetInButtonRowRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LayoutMinTouchTargetInButtonRow", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MinTouchTargetInButtonRow",
			Brief:      "Button in row without 48dp min height",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringNotSelectableRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringNotSelectable", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TextNotSelectable",
			Brief:      "Non-selectable text with URLs or phone numbers",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringRepeatedInContentDescriptionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringRepeatedInContentDescription", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RepeatedContentDescription",
			Brief:      "contentDescription duplicates visible text",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringSpanInContentDescriptionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringSpanInContentDescription", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "SpanInContentDescription",
			Brief:      "String with HTML used in contentDescription",
			Category:   ALCAccessibility,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "krit",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
