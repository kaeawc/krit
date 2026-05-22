package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerStyleIdiomaticRules() {

	// --- from style_idiomatic.go ---
	{
		r := &UseCheckNotNullRule{BaseRule: BaseRule{RuleName: "UseCheckNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects check(x != null) calls that should use checkNotNull(x) instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				argText, suffixText, ok := flatNonNullCheckText(file, idx, "check")
				if !ok {
					return
				}
				if ctx.Resolver != nil {
					resolved := ctx.Resolver.ResolveByNameFlat(argText, idx, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Use 'checkNotNull(x)' instead of 'check(x != null)'.")
				replacement := "checkNotNull(" + argText + ")"
				if suffixText != "" {
					replacement += " " + suffixText
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseRequireNotNullRule{BaseRule: BaseRule{RuleName: "UseRequireNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects require(x != null) calls that should use requireNotNull(x) instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				argText, suffixText, ok := flatNonNullCheckText(file, idx, "require")
				if !ok {
					return
				}
				if ctx.Resolver != nil {
					resolved := ctx.Resolver.ResolveByNameFlat(argText, idx, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Use 'requireNotNull(x)' instead of 'require(x != null)'.")
				replacement := "requireNotNull(" + argText + ")"
				if suffixText != "" {
					replacement += " " + suffixText
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseCheckOrErrorRule{BaseRule: BaseRule{RuleName: "UseCheckOrError", RuleSetName: "style", Sev: "warning", Desc: "Detects if (!cond) throw IllegalStateException patterns that should use check()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				file := ctx.File
				flatThrowPattern(ctx, file.FlatType(ctx.Idx), file.FlatNodeText(ctx.Idx), "IllegalStateException", "check", r.BaseRule)
			},
		})
	}
	{
		r := &UseRequireRule{BaseRule: BaseRule{RuleName: "UseRequire", RuleSetName: "style", Sev: "warning", Desc: "Detects if (!cond) throw IllegalArgumentException patterns that should use require()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				file := ctx.File
				flatThrowPattern(ctx, file.FlatType(ctx.Idx), file.FlatNodeText(ctx.Idx), "IllegalArgumentException", "require", r.BaseRule)
			},
		})
	}
	{
		r := &UseIsNullOrEmptyRule{BaseRule: BaseRule{RuleName: "UseIsNullOrEmpty", RuleSetName: "style", Sev: "warning", Desc: "Detects x == null || x.isEmpty() patterns that should use isNullOrEmpty()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"disjunction_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs:  api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{Identifiers: []string{"isEmpty", "count", ".size", ".length", "\"\""}},
			OracleCallTargets: &api.OracleCallTargetFilter{
				CalleeNames: []string{"count", "isEmpty"},
				LexicalSkipByCallee: map[string][]string{
					"count":   {"*"},
					"isEmpty": {"*"},
				},
			},
			// Matches the receiver FQN against known collection/String types via
			// the expressions map; no class declarations needed.
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Implementation:         r,
			Check: func(ctx *api.Context) {
				flatUseIsNullOrEmpty(ctx, r.BaseRule)
			},
		})
	}
	{
		r := &UseOrEmptyRule{BaseRule: BaseRule{RuleName: "UseOrEmpty", RuleSetName: "style", Sev: "warning", Desc: "Detects x ?: emptyList() patterns that should use .orEmpty() instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"elvis_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				right := file.FlatChild(idx, 2)
				if left == 0 || right == 0 {
					return
				}
				if !flatIsEmptyRHS(file, right) {
					return
				}
				if hasFlatAncestorTypeName(file, idx, "string_literal", "raw_string_literal", "line_string_literal", "multi_line_string_literal") {
					return
				}
				leftText := file.FlatNodeText(left)
				if strings.Contains(leftText, "?.") {
					return
				}
				if strings.Contains(leftText, "?.let") {
					return
				}
				rightText := strings.TrimSpace(file.FlatNodeText(right))
				if strings.HasPrefix(rightText, "emptyArray(") {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Use '.orEmpty()' instead of '?: %s'.", rightText))
				var replacement string
				if lhsNeedsParensFlat(file, left) {
					replacement = "(" + leftText + ").orEmpty()"
				} else {
					replacement = leftText + ".orEmpty()"
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseAnyOrNoneInsteadOfFindRule{BaseRule: BaseRule{RuleName: "UseAnyOrNoneInsteadOfFind", RuleSetName: "style", Sev: "warning", Desc: "Detects .find {} != null patterns that should use .any {} or .none {} instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				op := file.FlatChild(idx, 1)
				right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
				if left == 0 || op == 0 || right == 0 {
					return
				}
				opText := file.FlatNodeText(op)
				leftText := strings.TrimSpace(file.FlatNodeText(left))
				rightText := strings.TrimSpace(file.FlatNodeText(right))
				isNullLeft := leftText == "null"
				isNullRight := rightText == "null"
				if !isNullLeft && !isNullRight {
					return
				}
				var replacement string
				switch opText {
				case "!=":
					replacement = "any"
				case "==":
					replacement = "none"
				default:
					return
				}
				callSideIdx := left
				if isNullLeft {
					callSideIdx = right
				}
				if file.FlatType(callSideIdx) != "call_expression" {
					return
				}
				nav, _ := file.FlatFindChild(callSideIdx, "navigation_expression")
				if nav == 0 {
					return
				}
				if flatLastChildOfType(file, nav, "navigation_suffix") == 0 {
					return
				}
				funcName := flatNavigationExpressionLastIdentifier(file, nav)
				if !anyOrNoneFindFuncs[funcName] {
					return
				}
				callSuffix, _ := file.FlatFindChild(callSideIdx, "call_suffix")
				if callSuffix == 0 {
					return
				}
				lambda := flatCallSuffixLambdaNode(file, callSuffix)
				if lambda == 0 {
					return
				}
				receiver := file.FlatNamedChild(nav, 0)
				if receiver == 0 {
					return
				}
				receiverText := file.FlatNodeText(receiver)
				lambdaText := file.FlatNodeText(lambda)
				msg := fmt.Sprintf("Use '.%s {}' instead of '.%s {} %s null'.", replacement, funcName, opText)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: receiverText + "." + replacement + " " + lambdaText,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseEmptyCounterpartRule{BaseRule: BaseRule{RuleName: "UseEmptyCounterpart", RuleSetName: "style", Sev: "warning", Desc: "Detects listOf(), setOf(), and similar calls with no arguments that should use emptyList(), emptySet(), etc."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) == 0 {
					return
				}
				callee := file.FlatChild(idx, 0)
				if callee == 0 || file.FlatType(callee) != "simple_identifier" {
					return
				}
				calleeName := file.FlatNodeText(callee)
				replacement, ok := emptyCounterparts[calleeName]
				if !ok {
					return
				}
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				args := flatCallSuffixValueArgs(file, suffix)
				if args == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) == "value_argument" {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Use '%s()' instead of '%s()'.", replacement, calleeName))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement + "()",
				}
				ctx.Emit(f)
			},
		})
	}
}
