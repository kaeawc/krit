package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerStyleExpressionsExtraRules() {

	// --- from style_expressions_extra.go ---
	{
		r := &MultilineLambdaItParameterRule{BaseRule: BaseRule{RuleName: "MultilineLambdaItParameter", RuleSetName: "style", Sev: "warning", Desc: "Detects multiline lambdas that use the implicit it parameter instead of naming it explicitly."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				startLine := file.FlatRow(idx)
				bodyText := stripKotlinComments(file.FlatNodeText(idx))
				endLine := startLine + strings.Count(bodyText, "\n")
				if startLine == endLine {
					return // single-line lambda, ok
				}
				if strings.Contains(bodyText, "->") {
					return
				}
				if strings.Contains(bodyText, " it.") || strings.Contains(bodyText, " it ") ||
					strings.Contains(bodyText, "(it)") || strings.Contains(bodyText, "\tit.") ||
					strings.Contains(bodyText, "\nit.") || strings.Contains(bodyText, "{it") {
					ctx.EmitAt(int(startLine)+1, 1,
						"Multiline lambda should have an explicit parameter instead of 'it'.")
				}
			},
		})
	}
	{
		r := &MultilineRawStringIndentationRule{BaseRule: BaseRule{RuleName: "MultilineRawStringIndentation", RuleSetName: "style", Sev: "warning", Desc: "Detects multiline raw strings that are missing trimIndent() or trimMargin() calls."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &TrimMultilineRawStringRule{BaseRule: BaseRule{RuleName: "TrimMultilineRawString", RuleSetName: "style", Sev: "warning", Desc: "Detects multiline raw strings that should use trimIndent() or trimMargin() for proper indentation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), Fix: v2.FixCosmetic, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringShouldBeRawStringRule{BaseRule: BaseRule{RuleName: "StringShouldBeRawString", RuleSetName: "style", Sev: "warning", Desc: "Detects string literals with many escape characters that would be more readable as raw strings."}, MaxEscapes: 2}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if strings.HasPrefix(text, "\"\"\"") {
					return // already raw
				}
				count := len(escapeCountRe.FindAllString(text, -1))
				if count > r.MaxEscapes {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("String contains %d escape characters. Consider using a raw string.", count))
				}
			},
		})
	}
	{
		r := &CanBeNonNullableRule{BaseRule: BaseRule{RuleName: "CanBeNonNullable", RuleSetName: "style", Sev: "warning", Desc: "Detects nullable types that are initialized with non-null values and never assigned null."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				switch ctx.File.FlatType(ctx.Idx) {
				case "property_declaration":
					r.checkPropertyFlat(ctx)
				case "function_declaration":
					r.checkFunctionParamsFlat(ctx)
				}
			},
		})
	}
	{
		r := &DoubleNegativeExpressionRule{BaseRule: BaseRule{RuleName: "DoubleNegativeExpression", RuleSetName: "style", Sev: "warning", Desc: "Detects double negative expressions like !isNotEmpty() that should use the positive variant."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"prefix_expression"}, Confidence: r.Confidence(), Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: r.checkDoubleNegativeExpressionFlat,
		})
	}
	{
		r := &DoubleNegativeLambdaRule{BaseRule: BaseRule{RuleName: "DoubleNegativeLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects double negative lambda patterns like filterNot { !predicate } that should use filter."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.checkDoubleNegativeLambdaFlat,
		})
	}
	{
		r := &NullableBooleanCheckRule{BaseRule: BaseRule{RuleName: "NullableBooleanCheck", RuleSetName: "style", Sev: "warning", Desc: "Detects equality comparisons against Boolean literals like x == true on nullable Booleans."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				var boolLit, otherNode uint32
				if file.FlatType(right) == "boolean_literal" {
					boolLit = right
					otherNode = left
				} else if file.FlatType(left) == "boolean_literal" {
					boolLit = left
					otherNode = right
				} else {
					return
				}
				boolVal := file.FlatNodeText(boolLit)
				otherText := file.FlatNodeText(otherNode)
				msg := fmt.Sprintf("Nullable boolean check '%s %s %s'. Consider using '%s ?: false' or safe call.",
					otherText, opText, boolVal, otherText)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				if (opText == "==" && boolVal == "true") || (opText == "!=" && boolVal == "false") {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: otherText + " ?: false",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &RangeUntilInsteadOfRangeToRule{BaseRule: BaseRule{RuleName: "RangeUntilInsteadOfRangeTo", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of the until infix function that can be replaced with the ..< range operator."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"infix_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				op := file.FlatChild(idx, 1)
				if op == 0 || !file.FlatNodeTextEquals(op, "until") {
					return
				}
				left := file.FlatChild(idx, 0)
				right := file.FlatChild(idx, 2)
				if left == 0 || right == 0 {
					return
				}
				leftText := file.FlatNodeText(left)
				rightText := file.FlatNodeText(right)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Use '..<' range operator instead of 'until'.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: leftText + "..<" + rightText,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &DestructuringDeclarationWithTooManyEntriesRule{BaseRule: BaseRule{RuleName: "DestructuringDeclarationWithTooManyEntries", RuleSetName: "style", Sev: "warning", Desc: "Detects destructuring declarations with more entries than the configured maximum."}, MaxEntries: 3}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"multi_variable_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				count := 0
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "variable_declaration" {
						count++
					}
				}
				if count > r.MaxEntries {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Destructuring declaration has %d entries, max allowed is %d.", count, r.MaxEntries))
				}
			},
		})
	}
}
