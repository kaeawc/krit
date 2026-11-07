package rules

import (
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerPotentialbugsPropertiesRules() {

	// --- from potentialbugs_properties.go ---
	{
		r := &PropertyUsedBeforeDeclarationRule{BaseRule: BaseRule{RuleName: "PropertyUsedBeforeDeclaration", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects class properties referenced in initializers or init blocks before they are declared."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_body"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				type propInfo struct {
					name  string
					node  uint32
					index int // order among direct children
				}

				// First pass: collect class-level property declarations (direct children of class_body).
				var props []propInfo
				propByName := map[string]int{} // property name -> index in class_body children
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) != "property_declaration" {
						continue
					}
					name := propertyDeclarationNameFlat(file, child)
					if name == "" {
						continue
					}
					props = append(props, propInfo{name, child, i})
					propByName[name] = i
				}
				if len(props) == 0 {
					return
				}

				// collectIdentifiers gathers all simple_identifier text from a subtree,
				// but does NOT descend into function_declaration or lambda_literal nodes
				// (those execute lazily, so references there are fine).
				var collectIdentifiers func(n uint32) []string
				collectIdentifiers = func(n uint32) []string {
					var ids []string
					switch file.FlatType(n) {
					case "function_declaration", "lambda_literal", "getter", "setter":
						return nil
					}
					if file.FlatType(n) == "simple_identifier" && propertyRefCouldBeClassProperty(file, n) {
						ids = append(ids, file.FlatNodeText(n))
					}
					for i := 0; i < file.FlatChildCount(n); i++ {
						ids = append(ids, collectIdentifiers(file.FlatChild(n, i))...)
					}
					return ids
				}

				// Second pass: for each class-level property initializer AND each init block,
				// check if it references a property declared later.
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "property_declaration":
						propName := propertyDeclarationNameFlat(file, child)
						if propName == "" {
							continue
						}
						refs := collectIdentifiers(child)
						locals := localDeclaredNames(file, child)
						for _, ref := range refs {
							if ref == propName || locals[ref] {
								continue
							}
							declIdx, ok := propByName[ref]
							if ok && declIdx > i {
								ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
									fmt.Sprintf("Property '%s' uses '%s' which is declared later.", propName, ref))
								break // one finding per property
							}
						}
					case "anonymous_initializer":
						// init {} blocks execute eagerly in declaration order.
						refs := collectIdentifiers(child)
						locals := localDeclaredNames(file, child)
						for _, ref := range refs {
							if locals[ref] {
								continue
							}
							declIdx, ok := propByName[ref]
							if ok && declIdx > i {
								ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
									fmt.Sprintf("Init block uses '%s' which is declared later.", ref))
								break
							}
						}
					}
				}
			},
		})
	}
	{
		r := &UnconditionalJumpStatementInLoopRule{BaseRule: BaseRule{RuleName: "UnconditionalJumpStatementInLoop", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects loops whose body has an unconditional return, break, or throw statement, so the loop executes at most once."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"for_statement", "while_statement", "do_while_statement"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if loopHasUnconditionalJump(file, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"This loop contains an unconditional jump expression which essentially renders it useless as it will exit the loop during the first iteration.")
				}
			},
		})
	}
	{
		r := &UnnamedParameterUseRule{BaseRule: BaseRule{RuleName: "UnnamedParameterUse", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects function calls with many unnamed parameters where named parameters would improve readability."}, AllowSingleParamUse: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				count := 0
				hasNamed := false
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) == "value_argument" {
						count++
						hasNamed = hasNamed || flatHasValueArgumentLabel(file, child)
					}
				}
				if hasNamed {
					return
				}
				// Single unnamed argument: only fire when the user has
				// explicitly disabled the single-param allowance.
				if count == 1 && !r.AllowSingleParamUse {
					if flatCallForwardsEnclosingFunctionParameters(file, idx, args) {
						return
					}
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						"Function call with a single unnamed parameter. Consider using a named parameter for clarity.")
					return
				}
				if count < 5 {
					return
				}
				if flatCallForwardsEnclosingFunctionParameters(file, idx, args) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Function call with many unnamed parameters. Consider using named parameters for clarity.")
			},
		})
	}
	{
		r := &UnusedUnaryOperatorRule{BaseRule: BaseRule{RuleName: "UnusedUnaryOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects a unary + or - that starts a line and whose result is never used, such as the broken continuation `+ 3` below a statement."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"prefix_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) { checkUnusedUnaryOperator(ctx, r) },
		})
	}
	{
		r := &UselessPostfixExpressionRule{BaseRule: BaseRule{RuleName: "UselessPostfixExpression", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects postfix increment or decrement whose effect is lost: returning a local variable's postfix expression, or assigning x++ back to x."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"jump_expression", "assignment"}, Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.checkUselessPostfixFlat,
		})
	}
}
