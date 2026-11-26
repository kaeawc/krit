package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxStartupRules registers the checks from androidx.startup:startup-runtime 1.2.0. Both look at every
// main-source class: EnsureInitializerNoArgConstr in the parsed-files scope, EnsureInitializerMetadata in the
// manifest-source scope.
func registerAndroidxStartupRules() {
	languages := []scanner.Language{scanner.LangKotlin, scanner.LangJava}

	noArg := &EnsureInitializerNoArgConstrRule{BaseRule: BaseRule{RuleName: "EnsureInitializerNoArgConstr",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects androidx.startup Initializers without a no-argument constructor, which the startup runtime cannot create."}}
	api.Register(&api.Rule{
		ID: noArg.RuleName, Category: noArg.RuleSetName, Description: noArg.Desc, Sev: api.Severity(noArg.Sev),
		Needs: api.NeedsParsedFiles, Languages: languages, Confidence: api.ConfidenceHigh, Implementation: noArg,
		Check: noArg.check, LintIssue: true,
	})

	metadata := &EnsureInitializerMetadataRule{BaseRule: BaseRule{RuleName: "EnsureInitializerMetadata",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects androidx.startup Initializers that no manifest <meta-data> entry or dependencies() list reaches."}}
	api.Register(&api.Rule{
		ID: metadata.RuleName, Category: metadata.RuleSetName, Description: metadata.Desc, Sev: api.Severity(metadata.Sev),
		Needs: api.NeedsManifest | api.NeedsParsedFiles, AndroidDeps: uint32(AndroidDepManifest), Languages: languages,
		Confidence: api.ConfidenceHigh, Implementation: metadata, Check: metadata.check, LintIssue: true,
	})
}
