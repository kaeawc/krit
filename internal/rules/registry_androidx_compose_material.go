package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxComposeMaterialRules registers the checks from androidx.compose.material:material-android 1.13.0
// and androidx.compose.material3:material3-android 1.5.0.
func registerAndroidxComposeMaterialRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	m2 := &UnusedMaterialScaffoldPaddingParameterRule{BaseRule: BaseRule{RuleName: "UnusedMaterialScaffoldPaddingParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects material Scaffold content lambdas that ignore the padding parameter, so app bars can cover the content."}}
	registerAxlRule(m2.BaseRule, m2, []string{"lambda_literal"}, kotlin, axlRuleOptions{},
		func(ctx *api.Context) { axlCheckScaffoldPadding(ctx, axlMaterialPackage) })

	m3 := &UnusedMaterial3ScaffoldPaddingParameterRule{BaseRule: BaseRule{RuleName: "UnusedMaterial3ScaffoldPaddingParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects material3 Scaffold content lambdas that ignore the padding parameter, so app bars can cover the content."}}
	registerAxlRule(m3.BaseRule, m3, []string{"lambda_literal"}, kotlin, axlRuleOptions{},
		func(ctx *api.Context) { axlCheckScaffoldPadding(ctx, axlMaterial3Package) })

	colors := &ConflictingOnColorRule{BaseRule: BaseRule{RuleName: "ConflictingOnColor", RuleSetName: "compose",
		Sev: "error", Desc: "Detects material Colors whose background colors share a value but not the same 'on' color."}}
	registerAxlRule(colors.BaseRule, colors, []string{"call_expression"}, kotlin, axlRuleOptions{}, colors.check)

	imports := &UsingMaterialAndMaterial3LibrariesRule{BaseRule: BaseRule{RuleName: "UsingMaterialAndMaterial3Libraries",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects androidx.compose.material imports in code that also uses the incompatible material3 library."}}
	registerAxlRule(imports.BaseRule, imports, []string{"import_header", "import_declaration"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, axlRuleOptions{NeedsLibraryFacts: true}, imports.check)
}
