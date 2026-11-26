package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// composeLintsRuleSet is the rule set of the compose-lint-checks (slackhq/compose-lints) ports. They share the
// compose rule set with the AndroidX Compose ports; the Lint IDs keep them apart.
const composeLintsRuleSet = "compose"

// registerComposeLintsRules registers the ports of com.slack.lint.compose:compose-lint-checks 1.6.1. compose-lints
// analyzes Kotlin only. Each rule uses its Lint issue ID, message and severity, and ships experimental and off by
// default: the checks encode the compose-lints style guide, and several overlap AndroidX Compose ports that are on.
func registerComposeLintsRules() {
	usage := &ComposeCompositionLocalUsageRule{BaseRule: BaseRule{RuleName: "ComposeCompositionLocalUsage",
		RuleSetName: composeLintsRuleSet, Sev: "warning", Desc: "Detects new CompositionLocal declarations, which are implicit dependencies."}}
	registerComposeLintsRule(usage.BaseRule, usage, []string{"property_declaration"}, composeLintsMentionsCompositionLocal, usage.check)

	getter := &ComposeCompositionLocalGetterRule{BaseRule: BaseRule{RuleName: "ComposeCompositionLocalGetter",
		RuleSetName: composeLintsRuleSet, Sev: "error", Desc: "Detects CompositionLocal properties declared with a getter, which creates a new instance on every read."}}
	registerComposeLintsRule(getter.BaseRule, getter, []string{"getter"}, composeLintsMentionsCompositionLocal, getter.check)

	m2 := &ComposeM2ApiRule{BaseRule: BaseRule{RuleName: "ComposeM2Api", RuleSetName: composeLintsRuleSet, Sev: "error",
		Desc: "Detects calls and references to Compose Material 2 (androidx.compose.material) APIs."}}
	registerComposeLintsProjectRule(m2.BaseRule, m2, m2.check)

	itemKey := &ComposeItemKeyHashCodeRule{BaseRule: BaseRule{RuleName: "ComposeItemKeyHashCode", RuleSetName: composeLintsRuleSet,
		Sev: "warning", Desc: "Detects hashCode() used in the key argument of a call in a @Composable function."}}
	registerComposeLintsRule(itemKey.BaseRule, itemKey, []string{"value_argument"}, composeLintsMentions("hashCode"), itemKey.check)

	composed := &ComposeModifierComposedRule{BaseRule: BaseRule{RuleName: "ComposeModifierComposed", RuleSetName: composeLintsRuleSet,
		Sev: "error", Desc: "Detects Modifier.composed { }, which should be migrated to the Modifier.Node API."}}
	registerComposeLintsRule(composed.BaseRule, composed, []string{"call_expression"}, composeLintsMentions("composed"), composed.check)

	previewPublic := &ComposePreviewPublicRule{BaseRule: BaseRule{RuleName: "ComposePreviewPublic", RuleSetName: composeLintsRuleSet,
		Sev: "error", Desc: "Detects public @Composable functions annotated with @Preview or a multipreview annotation."}}
	registerComposeLintsProjectRule(previewPublic.BaseRule, previewPublic, previewPublic.check)

	previewNaming := &ComposePreviewNamingRule{BaseRule: BaseRule{RuleName: "ComposePreviewNaming", RuleSetName: composeLintsRuleSet,
		Sev: "error", Desc: "Detects multipreview annotation classes not named with the Preview or Previews suffix."}}
	registerComposeLintsProjectRule(previewNaming.BaseRule, previewNaming, previewNaming.check)
}

func registerComposeLintsRule(rule BaseRule, impl any, nodeTypes []string, gate func(*scanner.File) bool, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceHigh,
		Implementation: impl, Check: check, FileGate: gate, LintIssue: true,
		DefaultActive: false, OptInReason: api.OptInReasonOpinionated, Maturity: api.MaturityExperimental,
	})
}

// registerComposeLintsProjectRule registers a port that needs the project's files, such as multipreview
// annotations declared in another file. It runs once per analysis over the cross-file index.
func registerComposeLintsProjectRule(rule BaseRule, impl any, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		Needs: api.NeedsCrossFile, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceHigh,
		Implementation: impl, Check: check, LintIssue: true,
		DefaultActive: false, OptInReason: api.OptInReasonOpinionated, Maturity: api.MaturityExperimental,
	})
}

// composeLintsMentions gates a rule to files containing text.
func composeLintsMentions(text string) func(*scanner.File) bool {
	return func(file *scanner.File) bool { return fileFactsCache().Mentions(file, text) }
}

// composeLintsMentionsCompositionLocal gates the CompositionLocal rules to files naming one of the factories.
func composeLintsMentionsCompositionLocal(file *scanner.File) bool {
	facts := fileFactsCache()
	return facts.Mentions(file, "compositionLocalOf") || facts.Mentions(file, "CompositionLocalOf")
}
