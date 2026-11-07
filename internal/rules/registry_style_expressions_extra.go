package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleExpressionsExtraRules() {

	// --- from style_expressions_extra.go ---
	{
		r := &MultilineLambdaItParameterRule{BaseRule: BaseRule{RuleName: "MultilineLambdaItParameter", RuleSetName: "style", Sev: "warning", Desc: "Detects multiline lambdas that use the implicit it parameter instead of naming it explicitly."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				// detekt: a lambda is multiline with two statements or more, or one statement that spans lines.
				stmts, _ := file.FlatFindChild(idx, "statements")
				if stmts == 0 {
					return
				}
				count, last := 0, uint32(0)
				for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) == ";" || isFlatCommentNode(file, c) {
						continue
					}
					count++
					last = c
				}
				if count == 0 || (count == 1 && !strings.Contains(file.FlatNodeText(last), "\n")) {
					return
				}
				if params, ok := file.FlatFindChild(idx, "lambda_parameters"); ok && params != 0 {
					if file.FlatNodeTextEquals(params, "it") {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							"The parameter name in a multiline lambda should not be an explicit `it`. Consider giving your parameter a readable and descriptive name.")
					}
					return
				}
				if !lambdaUsesImplicitIt(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"The implicit `it` should not be used in a multiline lambda. Consider giving your parameter a readable and descriptive name.")
			},
		})
	}
	{
		r := &MultilineRawStringIndentationRule{BaseRule: BaseRule{RuleName: "MultilineRawStringIndentation", RuleSetName: "style", Sev: "warning", Desc: "Detects trimmed multiline raw strings whose content or closing quotes are not indented one step from the line that opens the string."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TrimMultilineRawStringRule{BaseRule: BaseRule{RuleName: "TrimMultilineRawString", RuleSetName: "style", Sev: "warning", Desc: "Detects multiline raw strings that should use trimIndent() or trimMargin() for proper indentation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &StringShouldBeRawStringRule{BaseRule: BaseRule{RuleName: "StringShouldBeRawString", RuleSetName: "style", Sev: "warning", Desc: "Detects string literals with many escape characters that would be more readable as raw strings."}, MaxEscapes: 2}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"prefix_expression"}, Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.checkDoubleNegativeExpressionFlat,
		})
	}
	{
		r := &DoubleNegativeLambdaRule{BaseRule: BaseRule{RuleName: "DoubleNegativeLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects negative functions such as none and takeUnless whose lambda also contains a negation."}, NegativeFunctions: []string{"takeUnless", "none", "filterNot"}, NegativeFunctionNameParts: []string{"not", "non"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.checkDoubleNegativeLambdaFlat,
		})
	}
	{
		r := &NullableBooleanCheckRule{BaseRule: BaseRule{RuleName: "NullableBooleanCheck", RuleSetName: "style", Sev: "warning", Desc: "Detects elvis checks like x ?: false on nullable Booleans that should use equality (x == true), per the Kotlin coding conventions."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"elvis_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
				if left == 0 || right == 0 || file.FlatType(right) != "boolean_literal" {
					return
				}
				if !nullableBooleanOperand(ctx, left) {
					return
				}
				// `x ?: false` is `x == true` and `x ?: true` is `x != false` for a Boolean? operand.
				boolVal := file.FlatNodeText(right)
				op, want := "==", "true"
				if boolVal == "true" {
					op, want = "!=", "false"
				}
				leftText := file.FlatNodeText(left)
				if t := file.FlatType(left); t != "simple_identifier" && t != "navigation_expression" && t != "call_expression" && t != "parenthesized_expression" {
					leftText = "(" + leftText + ")"
				}
				replacement := leftText + " " + op + " " + want
				// Equality binds looser than elvis, so a surrounding operator needs parentheses to keep its operands.
				if parent, ok := file.FlatParent(idx); ok {
					switch file.FlatType(parent) {
					case "parenthesized_expression", "if_expression", "while_statement", "do_while_statement", "value_argument",
						"property_declaration", "assignment", "jump_expression", "statements", "conjunction_expression",
						"disjunction_expression", "control_structure_body", "when_condition", "function_body":
					default:
						replacement = "(" + replacement + ")"
					}
				}
				msg := fmt.Sprintf("Nullable boolean check '%s ?: %s'. Use '%s' instead.", file.FlatNodeText(left), boolVal, replacement)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
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
		r := &UntilInsteadOfRangeToRule{BaseRule: BaseRule{RuleName: "UntilInsteadOfRangeTo", RuleSetName: "style", Sev: "warning", Desc: "Detects a..b - 1 and a.rangeTo(b - 1), which can be written as a until b."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"range_expression", "call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &RangeUntilInsteadOfRangeToRule{BaseRule: BaseRule{RuleName: "RangeUntilInsteadOfRangeTo", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of the until infix function that can be replaced with the ..< range operator."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"infix_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"multi_variable_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// Only real destructuring sites; error recovery can yield this node for parameter lists.
				parent, ok := file.FlatParent(idx)
				if !ok {
					return
				}
				switch file.FlatType(parent) {
				case "property_declaration", "for_statement", "lambda_parameters":
				default:
					return
				}
				count := 0
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "variable_declaration" {
						count++
					}
				}
				if count > r.MaxEntries {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("The destructuring declaration contains %d but only %d are allowed.", count, r.MaxEntries))
				}
			},
		})
	}
}
