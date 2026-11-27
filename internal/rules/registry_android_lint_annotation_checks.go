package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintAnnotationChecks registers the Android Lint 32.4.1 ReturnThis, UseSdkSuppress and
// AnnotateVersionCheck checks.
func registerAndroidLintAnnotationChecks() {
	returnThis := &ReturnThisRule{AndroidRule: lintAndroidRule("ReturnThis", "Method must return `this`", ALCCorrectness, ALSError, 4)}
	registerLintRule(returnThis.AndroidRule, returnThis, returnThis.Confidence(), returnThis.check,
		[]string{"function_declaration", "method_declaration"}, 0, nil)

	// UseSdkSuppress only looks at test sources, which registerLintRule skips.
	sdk := &UseSdkSuppressRule{AndroidRule: lintAndroidRule("UseSdkSuppress", "Using `@SdkSuppress` instead of `@RequiresApi`",
		ALCCorrectness, ALSError, 4)}
	api.Register(&api.Rule{
		ID: sdk.RuleName, Category: sdk.RuleSetName, Description: sdk.Description(), Sev: api.Severity(sdk.Sev),
		NodeTypes:  []string{"class_declaration", "object_declaration", "function_declaration", "method_declaration"},
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: sdk.Confidence(), Implementation: sdk, Check: sdk.check,
	})

	version := &AnnotateVersionCheckRule{AndroidRule: lintAndroidRule("AnnotateVersionCheck", "Annotate SDK_INT checks",
		ALCCorrectness, ALSWarning, 6)}
	registerLintRule(version.AndroidRule, version, version.Confidence(), version.check,
		[]string{"comparison_expression", "binary_expression"}, 0, nil)
}
