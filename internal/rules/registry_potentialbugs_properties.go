package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerPotentialbugsPropertiesRules() {

	// --- from potentialbugs_properties.go ---
	{
		r := &PropertyUsedBeforeDeclarationRule{BaseRule: BaseRule{RuleName: "PropertyUsedBeforeDeclaration", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects class properties referenced in initializers or init blocks before they are declared."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_body"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					case "function_declaration", "lambda_literal":
						return nil
					}
					if file.FlatType(n) == "simple_identifier" {
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
						for _, ref := range refs {
							if ref == propName {
								continue
							}
							declIdx, ok := propByName[ref]
							if ok && declIdx > i {
								ctx.EmitAt(file.FlatRow(child)+1, 1,
									fmt.Sprintf("Property '%s' uses '%s' which is declared later.", propName, ref))
								break // one finding per property
							}
						}
					case "anonymous_initializer":
						// init {} blocks execute eagerly in declaration order.
						refs := collectIdentifiers(child)
						for _, ref := range refs {
							declIdx, ok := propByName[ref]
							if ok && declIdx > i {
								ctx.EmitAt(file.FlatRow(child)+1, 1,
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
		r := &UnconditionalJumpStatementInLoopRule{BaseRule: BaseRule{RuleName: "UnconditionalJumpStatementInLoop", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects loops containing an unconditional return, break, or throw that causes the loop to execute only once."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement", "while_statement", "do_while_statement"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				body, _ := file.FlatFindChild(idx, "statements")
				if body == 0 {
					for i := 0; i < file.FlatChildCount(idx); i++ {
						child := file.FlatChild(idx, i)
						if file.FlatType(child) == "control_structure_body" {
							body = child
							break
						}
					}
				}
				if body == 0 {
					return
				}
				text := file.FlatNodeText(body)
				trimmed := strings.TrimSpace(text)
				if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
					trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
				}
				lines := strings.Split(trimmed, "\n")
				for _, l := range lines {
					lt := strings.TrimSpace(l)
					if lt == "" {
						continue
					}
					if strings.HasPrefix(lt, "return") || strings.HasPrefix(lt, "break") || strings.HasPrefix(lt, "continue") || strings.HasPrefix(lt, "throw") {
						if !strings.Contains(trimmed, "if ") && !strings.Contains(trimmed, "if(") && !strings.Contains(trimmed, "when") {
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								"Unconditional jump statement in loop. The loop will only execute once.")
						}
					}
					break // Only check the first non-empty statement
				}
			},
		})
	}
	{
		r := &UnnamedParameterUseRule{BaseRule: BaseRule{RuleName: "UnnamedParameterUse", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects function calls with many unnamed parameters where named parameters would improve readability."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
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
				if count < 5 || hasNamed {
					return
				}
				if flatCallForwardsEnclosingFunctionParameters(file, idx, args) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Function call with many unnamed parameters. Consider using named parameters for clarity.")
			},
		})
	}
	{
		r := &UnusedUnaryOperatorRule{BaseRule: BaseRule{RuleName: "UnusedUnaryOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects standalone unary +x or -x expressions whose result is never used."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"prefix_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 2 {
					return
				}
				op := file.FlatNodeText(file.FlatChild(idx, 0))
				if op != "+" && op != "-" {
					return
				}

				topExpr := idx
				for {
					parent, ok := file.FlatParent(topExpr)
					if !ok || file.FlatType(parent) != "binary_expression" {
						break
					}
					topExpr = parent
				}

				stmts, ok := file.FlatParent(topExpr)
				if !ok {
					return
				}

				if file.FlatType(stmts) != "statements" {
					return
				}

				if flatIsLastNamedChildOf(file, topExpr, stmts) && flatIsExpressionBlock(file, stmts) {
					return
				}

				text := file.FlatNodeText(idx)
				if file.FlatType(topExpr) == "binary_expression" {
					text = file.FlatNodeText(topExpr)
				}

				row := file.FlatRow(idx) + 1
				col := file.FlatCol(idx) + 1
				ctx.EmitAt(row, col,
					fmt.Sprintf("Unused unary operator. The result of '%s' is not used.", text))
			},
		})
	}
	{
		r := &UselessPostfixExpressionRule{BaseRule: BaseRule{RuleName: "UselessPostfixExpression", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects postfix increment or decrement in return statements where the operation has no effect."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"jump_expression"}, Confidence: r.Confidence(), Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: r.checkUselessPostfixFlat,
		})
	}
}
