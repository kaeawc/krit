package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerLicensingRules() {

	// --- from licensing.go ---
	{
		r := &CopyrightYearOutdatedRule{
			BaseRule:         BaseRule{RuleName: "CopyrightYearOutdated", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects stale copyright years in file header comments."},
			RecentYearCutoff: recentCopyrightYearCutoff,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Fix: api.FixCosmetic, Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &MissingSpdxIdentifierRule{
			BaseRule:       BaseRule{RuleName: "MissingSpdxIdentifier", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects file header comments that are missing a SPDX license identifier."},
			RequiredPrefix: spdxIdentifierPrefix,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &LgplStaticLinkingInApkRule{
			BaseRule: BaseRule{RuleName: "LgplStaticLinkingInApk", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects Android application modules that statically link known-LGPL dependencies into the APK."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &NoticeFileOutOfDateRule{
			BaseRule: BaseRule{RuleName: "NoticeFileOutOfDate", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects projects whose NOTICE file is missing required attribution for declared dependencies."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.StringListOption(api.StringListOptionSpec[NoticeFileOutOfDateRule]{
					Name:        "noticeRequiredArtifacts",
					Default:     []string{},
					Description: "Gradle coordinates (group:name) whose attribution text must appear in NOTICE.",
					Apply:       func(r *NoticeFileOutOfDateRule, v []string) { r.NoticeRequiredArtifacts = v },
				}),
			},
		})
	}
	{
		r := &OssLicensesNotIncludedInAndroidRule{
			BaseRule: BaseRule{RuleName: "OssLicensesNotIncludedInAndroid", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects Android app modules with implementation dependencies but no attribution surface (oss-licenses-plugin or LICENSE file)."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &DependencyLicenseIncompatibleRule{
			BaseRule: BaseRule{RuleName: "DependencyLicenseIncompatible", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects external dependencies whose license is incompatible with the project's declared license."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.StringOption(api.StringOptionSpec[DependencyLicenseIncompatibleRule]{
					Name:        "projectLicense",
					Default:     "",
					Description: "SPDX license identifier for the project; dependencies with licenses incompatible with this are flagged.",
					Apply:       func(r *DependencyLicenseIncompatibleRule, v string) { r.ProjectLicense = v },
				}),
			},
		})
	}
	{
		r := &OptInMarkerNotRecognisedRule{
			BaseRule: BaseRule{RuleName: "OptInMarkerNotRecognised", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects @OptIn marker classes not in the embedded well-known markers list."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.StringListOption(api.StringListOptionSpec[OptInMarkerNotRecognisedRule]{
					Name:        "additionalMarkers",
					Default:     []string{},
					Description: "Additional OptIn marker class names (simple or fully-qualified) to treat as recognised.",
					Apply:       func(r *OptInMarkerNotRecognisedRule, v []string) { r.AdditionalMarkers = v },
				}),
			},
		})
	}
	{
		r := &OptInMarkerExposedPubliclyRule{
			BaseRule: BaseRule{RuleName: "OptInMarkerExposedPublicly", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects @OptIn annotations on public API declarations that propagate the opt-in requirement to callers."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if annotationFinalName(file, idx) != "OptIn" {
					return
				}
				if scanner.IsTestFile(file.Path) {
					return
				}
				decl, ok := optInAnnotationTarget(file, idx)
				if !ok {
					return
				}
				if file.FlatHasModifier(decl, "private") ||
					file.FlatHasModifier(decl, "internal") ||
					file.FlatHasModifier(decl, "protected") {
					return
				}
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"@OptIn on a public declaration propagates the opt-in requirement to callers. Restrict the declaration's visibility or annotate callers explicitly."))
			},
			DefaultActive: true,
		})
	}
	{
		r := &OptInWithoutJustificationRule{
			BaseRule: BaseRule{RuleName: "OptInWithoutJustification", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects @OptIn annotations whose declaration has no preceding KDoc explaining why opting in is safe."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &SuppressedWarningWithoutJustificationRule{
			BaseRule: BaseRule{RuleName: "SuppressedWarningWithoutJustification", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects @Suppress annotations whose declaration has no preceding KDoc explaining why silencing the warning is safe."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &RequiresOptInWithoutMessageRule{
			BaseRule: BaseRule{RuleName: "RequiresOptInWithoutMessage", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects @RequiresOptIn annotations that omit a message argument explaining why callers must opt in."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &RequiresOptInWithoutLevelRule{
			BaseRule: BaseRule{RuleName: "RequiresOptInWithoutLevel", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects custom @RequiresOptIn annotation classes that omit an explicit level = WARNING|ERROR argument."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
		})
	}
	{
		r := &SpdxIdentifierMismatchWithProjectRule{
			BaseRule: BaseRule{RuleName: "SpdxIdentifierMismatchWithProject", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects file SPDX identifiers that disagree with the project's configured license."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.StringOption(api.StringOptionSpec[SpdxIdentifierMismatchWithProjectRule]{
					Name:        "projectLicense",
					Default:     "",
					Description: "SPDX license identifier for the project; file headers whose SPDX id differs from this are flagged.",
					Apply:       func(r *SpdxIdentifierMismatchWithProjectRule, v string) { r.ProjectLicense = v },
				}),
			},
		})
	}
	{
		r := &SpdxIdentifierInvalidRule{
			BaseRule: BaseRule{RuleName: "SpdxIdentifierInvalid", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects file header SPDX-License-Identifier values that are not recognised SPDX short IDs."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.StringListOption(api.StringListOptionSpec[SpdxIdentifierInvalidRule]{
					Name:        "additionalIdentifiers",
					Default:     []string{},
					Description: "Additional SPDX identifiers (or exceptions) to treat as recognised.",
					Apply:       func(r *SpdxIdentifierInvalidRule, v []string) { r.AdditionalIdentifiers = v },
				}),
			},
		})
	}
	{
		r := &DependencyLicenseUnknownRule{
			BaseRule: BaseRule{RuleName: "DependencyLicenseUnknown", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects external dependencies not present in the embedded license registry."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			Options: []api.ConfigOption{
				api.BoolOption(api.BoolOptionSpec[DependencyLicenseUnknownRule]{
					Name:        "requireVerification",
					Default:     false,
					Description: "Require external dependencies to exist in the embedded license api.",
					Apply:       func(r *DependencyLicenseUnknownRule, v bool) { r.RequireVerification = v },
				}),
			},
		})
	}
}
