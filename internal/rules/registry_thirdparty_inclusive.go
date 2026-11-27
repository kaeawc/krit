package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerThirdPartyInclusiveRules() {
	r := &InclusiveNamingRule{BaseRule: BaseRule{RuleName: slackInclusiveNaming, RuleSetName: moshiRuleSet,
		Sev: string(api.SeverityError), Desc: "Use inclusive naming."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
		NodeTypes: []string{
			// Kotlin
			"source_file", "class_declaration", "object_declaration", "companion_object", "enum_entry",
			"function_declaration", "secondary_constructor", "anonymous_function", "property_declaration", "setter",
			"lambda_parameters", "catch_block", "when_subject", "label",
			// Java
			"program", "interface_declaration", "enum_declaration", "annotation_type_declaration", "record_declaration",
			"method_declaration", "annotation_type_element_declaration", "field_declaration", "local_variable_declaration",
			"resource", "enum_constant", "formal_parameter", "spread_parameter", "catch_formal_parameter",
			"lambda_expression", "labeled_statement",
		},
		Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          r.check,
		LintIssue:      true,
		DefaultActive:  false,
		OptInReason:    api.OptInReasonDomainSpecific,
		Maturity:       api.MaturityExperimental,
	})
}
