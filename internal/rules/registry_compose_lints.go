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
}

func registerComposeLintsRule(rule BaseRule, impl any, nodeTypes []string, gate func(*scanner.File) bool, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceHigh,
		Implementation: impl, Check: check, FileGate: gate, LintIssue: true,
		DefaultActive: false, OptInReason: api.OptInReasonOpinionated, Maturity: api.MaturityExperimental,
	})
}

// composeLintsMentionsCompositionLocal gates the CompositionLocal rules to files naming one of the factories.
func composeLintsMentionsCompositionLocal(file *scanner.File) bool {
	facts := fileFactsCache()
	return facts.Mentions(file, "compositionLocalOf") || facts.Mentions(file, "CompositionLocalOf")
}
