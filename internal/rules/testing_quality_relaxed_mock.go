package rules

// Testing-quality rule: RelaxedMockUsedForValueClass. Flags
// mockk(relaxed = true) calls whose type argument is a primitive /
// value-class type — those should use literal values instead.
//
// Extracted from testing_quality.go and registry_testing_quality.go as
// part of the god-file split. Pairs the rule struct, its Confidence()
// method, the primitiveTypes table, the testingQualityTypeArgument
// helper, and the registration closure that drives the check.

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type RelaxedMockUsedForValueClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RelaxedMockUsedForValueClassRule) Confidence() float64 { return api.ConfidenceMedium }

var primitiveTypes = map[string]bool{
	"Int": true, "Long": true, "Float": true, "Double": true,
	"Boolean": true, "String": true, "Byte": true, "Short": true, "Char": true,
}

func testingQualityTypeArgument(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	result := ""
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if result != "" {
			return
		}
		if file.FlatType(n) == "type_arguments" {
			for gc := file.FlatFirstChild(n); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "type_projection" || file.FlatType(gc) == "user_type" || file.FlatType(gc) == "type_identifier" {
					result = strings.TrimSpace(file.FlatNodeText(gc))
					return
				}
			}
		}
	})
	return result
}

func registerTestingQualityRelaxedMockUsedForValueClass() {
	r := &RelaxedMockUsedForValueClassRule{
		BaseRule: BaseRule{RuleName: "RelaxedMockUsedForValueClass", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects relaxed mocks of primitive or value types where literal values should be used instead."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallNameAny(file, idx) != "mockk" {
				return
			}
			relaxedArg := flatNamedValueArgument(file, flatCallKeyArguments(file, idx), "relaxed")
			if relaxedArg == 0 || !callArgHasBoolean(file, relaxedArg, true) {
				return
			}
			typeArg := testingQualityTypeArgument(file, idx)
			if typeArg == "" {
				return
			}
			if !primitiveTypes[typeArg] {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Don't mock primitives/value types; use literal values.")
		},
	})
}
