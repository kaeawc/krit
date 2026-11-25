package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintSecurityManifestSourcePorts registers the Android Lint 32.4.1 Security checks that join project
// sources with the manifests. They run once per project in the manifest-source scope.
func registerAndroidLintSecurityManifestSourcePorts() {
	provider := &ProviderReadPermissionOnlyRule{AndroidRule: lintAndroidRule("ProviderReadPermissionOnly",
		"Provider with readPermission only and implemented write APIs", ALCSecurity, ALSWarning, 5)}
	registerLintManifestSourceRule(provider.AndroidRule, provider, provider.Confidence(), provider.check)
	implicit := &UnsafeImplicitIntentLaunchRule{AndroidRule: lintAndroidRule("UnsafeImplicitIntentLaunch",
		"Implicit intent matches an internal non-exported component", ALCSecurity, ALSError, 6)}
	registerLintManifestSourceRule(implicit.AndroidRule, implicit, implicit.Confidence(), implicit.check)
	launch := &UnsafeIntentLaunchRule{AndroidRule: lintAndroidRule("UnsafeIntentLaunch", "Launched Unsafe Intent",
		ALCSecurity, ALSWarning, 6)}
	registerLintManifestSourceRule(launch.AndroidRule, launch, launch.Confidence(), launch.check)
}

// registerLintManifestSourceRule registers a Lint port that reads every project manifest and every parsed Kotlin and
// Java file.
func registerLintManifestSourceRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs:      api.NeedsManifest | api.NeedsParsedFiles,
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, Check: check, NeedsLibraryFacts: true,
	})
}
