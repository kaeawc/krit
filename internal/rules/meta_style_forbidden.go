// Descriptor metadata for internal/rules/style_forbidden.go.
// NOTE: Meta() for ForbiddenImportRule is hand-written in meta_forbidden_import.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *ForbiddenAnnotationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenAnnotation",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenAnnotationRule]{
				Name:        "annotations",
				Default:     []string{"SuppressWarnings"},
				Description: "Forbidden annotation names.",
				Apply:       func(r *ForbiddenAnnotationRule, v []string) { r.Annotations = v },
			}),
		},
	}
}

func (r *ForbiddenCommentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenComment",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[ForbiddenCommentRule]{
				Name:        "allowedPatterns",
				Default:     "",
				Description: "Regex pattern for allowed comments.",
				Apply:       func(r *ForbiddenCommentRule, v *regexp.Regexp) { r.AllowedPatterns = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ForbiddenCommentRule]{
				Name:        "comments",
				Default:     []string{"TODO:", "FIXME:", "STOPSHIP:"},
				Description: "Forbidden comment markers.",
				Apply:       func(r *ForbiddenCommentRule, v []string) { r.Comments = v },
			}),
		},
	}
}

func (r *ForbiddenMethodCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenMethodCall",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenMethodCallRule]{
				Name:        "methods",
				Default:     []string{"print(", "println("},
				Description: "Forbidden method calls.",
				Apply:       func(r *ForbiddenMethodCallRule, v []string) { r.Methods = v },
			}),
		},
	}
}

func (r *ForbiddenNamedParamRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenNamedParam",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenNamedParamRule]{
				Name:        "methods",
				Default:     []string{"require", "check", "assert"},
				Description: "Methods where named parameters are forbidden.",
				Apply:       func(r *ForbiddenNamedParamRule, v []string) { r.Methods = v },
			}),
		},
	}
}

func (r *ForbiddenOptInRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenOptIn",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenOptInRule]{
				Name:        "markerClasses",
				Description: "Specific @OptIn marker classes to forbid (empty = all @OptIn).",
				Apply:       func(r *ForbiddenOptInRule, v []string) { r.MarkerClasses = v },
			}),
		},
	}
}

func (r *ForbiddenSuppressRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenSuppress",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenSuppressRule]{
				Name:        "rules",
				Description: "Specific suppressed rules to forbid (empty = all @Suppress).",
				Apply:       func(r *ForbiddenSuppressRule, v []string) { r.Rules = v },
			}),
		},
	}
}

func (r *ForbiddenVoidRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenVoid",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ForbiddenVoidRule]{
				Name:        "ignoreOverridden",
				Default:     false,
				Description: "Ignore overridden functions.",
				Apply:       func(r *ForbiddenVoidRule, v bool) { r.IgnoreOverridden = v },
			}),
			api.BoolOption(api.BoolOptionSpec[ForbiddenVoidRule]{
				Name:        "ignoreUsageInGenerics",
				Default:     false,
				Description: "Ignore Void in generic type arguments.",
				Apply:       func(r *ForbiddenVoidRule, v bool) { r.IgnoreUsageInGenerics = v },
			}),
		},
	}
}

func (r *MagicNumberRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MagicNumber",
		RuleSet:       "style",
		DefaultActive: true,
		KnownLimitations: []string{
			"Heuristic literal scan: numeric arguments threaded through helper functions or builders may still be flagged at the call site.",
			"Preview / Compose scaffolding annotations are excluded via ignoreAnnotated; custom annotation conventions may need additional configuration.",
		},
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MagicNumberRule]{
				Name:        "ignoreAnnotated",
				Description: "Annotations that suppress this rule on a declaration.",
				Apply:       func(r *MagicNumberRule, v []string) { r.IgnoreAnnotated = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *MagicNumberRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[MagicNumberRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *MagicNumberRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreAnnotation",
				Default:     false,
				Description: "Ignore numbers inside annotations.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreAnnotation = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreColorLiterals",
				Default:     true,
				Description: "Ignore Color(0x...) hex literals.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreColorLiterals = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreCompanionObjectPropertyDeclaration",
				Default:     true,
				Description: "Ignore numbers in companion object property declarations.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreCompanionObjectPropertyDeclaration = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreComposeUnits",
				Default:     true,
				Description: "Ignore Compose unit literals (dp, sp, em).",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreComposeUnits = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreConstantDeclaration",
				Default:     true,
				Description: "Ignore numbers in const val declarations.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreConstantDeclaration = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreEnums",
				Default:     true,
				Description: "Ignore numbers in enum entries.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreEnums = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreExtensionFunctions",
				Default:     true,
				Description: "Ignore numbers in extension functions.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreExtensionFunctions = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreHashCodeFunction",
				Default:     true,
				Description: "Ignore numbers in hashCode() functions.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreHashCodeFunction = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreLocalVariableDeclaration",
				Default:     false,
				Description: "Ignore numbers in local variable declarations.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreLocalVariableDeclaration = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreNamedArgument",
				Default:     true,
				Description: "Ignore numbers used as named arguments.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreNamedArgument = v },
			}),
			api.StringListOption(api.StringListOptionSpec[MagicNumberRule]{
				Name:        "ignoreNumbers",
				Default:     []string{"-1", "0", "1", "2", "0f", "0.0f", "0.5f", "1f", "1.0f", "-1f", "0.5", ".5", "90f", "180f", "270f", "360f", "100", "100f", "1000", "1000L", "10000", "10000L", "255", "255f", "60", "60f", "60L", "60000", "60000L", "24", "24L", "1024", "1024L", "16", "16f", "8", "8f", "4", "4f"},
				Description: "Numbers to ignore.",
				Apply:       func(r *MagicNumberRule, v []string) { r.IgnoreNumbers = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignorePropertyDeclaration",
				Default:     false,
				Description: "Ignore numbers in property declarations.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnorePropertyDeclaration = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MagicNumberRule]{
				Name:        "ignoreRanges",
				Default:     true,
				Description: "Ignore numbers in range expressions.",
				Apply:       func(r *MagicNumberRule, v bool) { r.IgnoreRanges = v },
			}),
		},
	}
}

func (r *WildcardImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WildcardImport",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[WildcardImportRule]{
				Name:        "excludeImports",
				Default:     []string{"java.util.*", "platform.**", "kotlinx.cinterop.*"},
				Description: "Wildcard imports to exclude from this rule.",
				Apply:       func(r *WildcardImportRule, v []string) { r.ExcludeImports = v },
			}),
		},
	}
}
