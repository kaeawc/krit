package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerStyleExpressionsRules() {

	// --- from style_expressions.go ---
	{
		r := &ExpressionBodySyntaxRule{BaseRule: BaseRule{RuleName: "ExpressionBodySyntax", RuleSetName: "style", Sev: "warning", Desc: "Detects single-expression functions that could use expression body syntax with the = operator."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				bodyText := strings.TrimSpace(file.FlatNodeText(body))
				if !strings.HasPrefix(bodyText, "{") {
					return
				}
				inner := strings.TrimPrefix(bodyText, "{")
				inner = strings.TrimSuffix(inner, "}")
				inner = strings.TrimSpace(inner)
				if !strings.HasPrefix(inner, "return ") {
					return
				}
				// Single-line `return X` is always eligible. Multi-line bodies
				// (e.g. `return foo()\n  .bar()`) are only eligible when
				// IncludeLineWrapping is true — matches detekt's option.
				if strings.Contains(inner, "\n") && !r.IncludeLineWrapping {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Function body can be written as expression body syntax.")
				expr := strings.TrimPrefix(inner, "return ")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(body)),
					EndByte:     int(file.FlatEndByte(body)),
					Replacement: "= " + expr,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ReturnCountRule{BaseRule: BaseRule{RuleName: "ReturnCount", RuleSetName: "style", Sev: "warning", Desc: "Detects functions with more return statements than the configured maximum."}, Max: 2, ExcludeReturnFromLambda: true}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				for _, excl := range r.ExcludedFunctions {
					if name == excl {
						return
					}
				}
				if lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx)); lines > 60 {
					return
				}
				rawReturns := getJumpMetricsFlat(idx, file).returns
				if rawReturns <= r.Max {
					return
				}
				count := 0
				if !r.ExcludeLabeled && !r.ExcludeReturnFromLambda && !r.ExcludeGuardClauses {
					count = rawReturns
				} else {
					var guardSet map[int]bool
					if r.ExcludeGuardClauses {
						guardSet = collectGuardClauseJumpsFlat(idx, file)
					}
					var whenDispatchSet map[int]bool
					if r.ExcludeGuardClauses {
						whenDispatchSet = collectWhenDispatchJumpsFlat(idx, file)
					}
					sawWhenDispatch := false
					count = countJumpExpressionsFlat(idx, file, "return", r.Max, func(child uint32, text string) bool {
						if r.ExcludeLabeled && strings.Contains(text, "@") {
							return false
						}
						if r.ExcludeReturnFromLambda && isInsideLambdaUnderFlat(child, idx, file) {
							return false
						}
						if guardSet != nil && guardSet[int(file.FlatStartByte(child))] {
							return false
						}
						if r.ExcludeGuardClauses && isInsideInitializerGuardFlat(child, idx, file) {
							return false
						}
						if experiment.Enabled("return-count-skip-when-initializer-guards") &&
							isInsideWhenInitializerGuardFlat(child, idx, file) {
							return false
						}
						if whenDispatchSet != nil && whenDispatchSet[int(file.FlatStartByte(child))] {
							if sawWhenDispatch {
								return false
							}
							sawWhenDispatch = true
						}
						return true
					})
				}
				if count > r.Max {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Function '%s' has %d return statements, max allowed is %d.", name, count, r.Max))
				}
			},
		})
	}
	{
		r := &ThrowsCountRule{BaseRule: BaseRule{RuleName: "ThrowsCount", RuleSetName: "style", Sev: "warning", Desc: "Detects functions with more throw statements than the configured maximum."}, Max: 2}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasAnnotationFlat(file, idx, "Throws") {
					return
				}
				rawThrows := getJumpMetricsFlat(idx, file).throws
				if rawThrows <= r.Max {
					return
				}
				count := rawThrows
				excludeGuards := r.ExcludeGuardClauses || experiment.Enabled("throws-count-exclude-guard-clauses")
				if excludeGuards {
					guardSet := collectGuardClauseJumpsFlat(idx, file)
					count = countJumpExpressionsFlat(idx, file, "throw", r.Max, func(child uint32, _ string) bool {
						return !guardSet[int(file.FlatStartByte(child))]
					})
				}
				if count > r.Max {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Function '%s' has %d throw statements, max allowed is %d.", name, count, r.Max))
				}
			},
		})
	}
	{
		r := &CollapsibleIfStatementsRule{BaseRule: BaseRule{RuleName: "CollapsibleIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects nested if statements without else that can be merged with a logical AND."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Only collapse plain if/then chains — if the if_expression
				// has an `else` token child (or a second control_structure_body
				// for the else branch), bail out so we don't suggest
				// rewriting a conditional with its else branch.
				if ifExpressionHasElse(file, idx) {
					return
				}
				body, _ := file.FlatFindChild(idx, "control_structure_body")
				if body == 0 {
					return
				}
				ifCount := 0
				otherCount := 0
				for i := 0; i < file.FlatChildCount(body); i++ {
					child := file.FlatChild(body, i)
					if file.FlatType(child) == "if_expression" {
						ifCount++
					} else if t := file.FlatType(child); t == "{" || t == "}" || t == "statements" {
						if file.FlatType(child) == "statements" {
							for j := 0; j < file.FlatChildCount(child); j++ {
								sc := file.FlatChild(child, j)
								if file.FlatType(sc) == "if_expression" {
									ifCount++
								} else {
									otherCount++
								}
							}
						}
					} else {
						otherCount++
					}
				}
				if ifCount != 1 || otherCount != 0 {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Collapsible if statements: these nested ifs can be merged with '&&'.")
				outerCond, _ := file.FlatFindChild(idx, "parenthesized_expression")
				if outerCond == 0 {
					for ci := 0; ci < file.FlatChildCount(idx); ci++ {
						ch := file.FlatChild(idx, ci)
						if t := file.FlatType(ch); t != "if" && t != "control_structure_body" && t != "{" && t != "}" {
							if t == "parenthesized_expression" || t == "boolean_literal" || t == "call_expression" || t == "simple_identifier" || t == "comparison_expression" || t == "conjunction_expression" || t == "disjunction_expression" || t == "prefix_expression" {
								outerCond = ch
								break
							}
						}
					}
				}
				var innerIf uint32
				file.FlatWalkNodes(body, "if_expression", func(n uint32) {
					if innerIf == 0 {
						innerIf = n
					}
				})
				if outerCond != 0 && innerIf != 0 {
					outerCondText := file.FlatNodeText(outerCond)
					if strings.HasPrefix(outerCondText, "(") && strings.HasSuffix(outerCondText, ")") {
						outerCondText = outerCondText[1 : len(outerCondText)-1]
					}
					innerCondNode, _ := file.FlatFindChild(innerIf, "parenthesized_expression")
					innerBody, _ := file.FlatFindChild(innerIf, "control_structure_body")
					if innerCondNode != 0 && innerBody != 0 {
						innerCondText := file.FlatNodeText(innerCondNode)
						if strings.HasPrefix(innerCondText, "(") && strings.HasSuffix(innerCondText, ")") {
							innerCondText = innerCondText[1 : len(innerCondText)-1]
						}
						innerBodyText := file.FlatNodeText(innerBody)
						merged := "if (" + outerCondText + " && " + innerCondText + ") " + innerBodyText
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: merged,
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &SafeCastRule{BaseRule: BaseRule{RuleName: "SafeCast", RuleSetName: "style", Sev: "warning", Desc: "Detects is-check followed by unsafe cast patterns that should use safe cast as? instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression", "when_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeType := file.FlatType(idx)
				if nodeType == "if_expression" {
					var condNode uint32
					var thenBody uint32
					foundElse := false
					for i := 0; i < file.FlatChildCount(idx); i++ {
						child := file.FlatChild(idx, i)
						switch file.FlatType(child) {
						case "parenthesized_expression":
							if condNode == 0 {
								condNode = child
							}
						case "check_expression", "conjunction_expression", "disjunction_expression":
							if condNode == 0 {
								condNode = child
							}
						case "control_structure_body":
							if !foundElse && thenBody == 0 {
								thenBody = child
							}
						case "else":
							foundElse = true
						}
					}
					if condNode == 0 || thenBody == 0 {
						return
					}
					var isVar, isType string
					file.FlatWalkAllNodes(condNode, func(n uint32) {
						if file.FlatType(n) == "check_expression" && isVar == "" {
							t := file.FlatNodeText(n)
							parts := strings.SplitN(t, " is ", 2)
							if len(parts) == 2 {
								isVar = strings.TrimSpace(parts[0])
								isType = strings.TrimSpace(parts[1])
							}
						}
					})
					if isVar == "" || isType == "" {
						return
					}
					if strings.ContainsAny(isVar, "()[].") {
						return
					}
					if condNode != 0 && (file.FlatType(condNode) == "conjunction_expression" ||
						file.FlatType(condNode) == "disjunction_expression") {
						return
					}
					found := false
					file.FlatWalkAllNodes(thenBody, func(n uint32) {
						if found {
							return
						}
						if file.FlatType(n) == "as_expression" {
							t := file.FlatNodeText(n)
							if strings.Contains(t, "as?") {
								return
							}
							parts := strings.SplitN(t, " as ", 2)
							if len(parts) == 2 {
								asVar := strings.TrimSpace(parts[0])
								asType := strings.TrimSpace(parts[1])
								if asVar == isVar && asType == isType {
									found = true
								}
							}
						}
					})
					if !found {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Consider using safe cast 'as?' instead of is-check followed by unsafe cast.")
				} else if nodeType == "when_expression" {
					for i := 0; i < file.FlatChildCount(idx); i++ {
						entry := file.FlatChild(idx, i)
						if file.FlatType(entry) != "when_entry" {
							continue
						}
						var isVar, isType string
						var entryBody uint32
						inCondition := true
						for j := 0; j < file.FlatChildCount(entry); j++ {
							child := file.FlatChild(entry, j)
							switch file.FlatType(child) {
							case "->":
								inCondition = false
							case "control_structure_body":
								entryBody = child
							default:
								if inCondition && isVar == "" {
									file.FlatWalkAllNodes(child, func(n uint32) {
										if file.FlatType(n) == "check_expression" && isVar == "" {
											t := file.FlatNodeText(n)
											parts := strings.SplitN(t, " is ", 2)
											if len(parts) == 2 {
												isVar = strings.TrimSpace(parts[0])
												isType = strings.TrimSpace(parts[1])
											}
										}
									})
								}
							}
						}
						if isVar == "" || isType == "" || entryBody == 0 {
							continue
						}
						if strings.ContainsAny(isVar, "()[].") {
							continue
						}
						found := false
						file.FlatWalkAllNodes(entryBody, func(n uint32) {
							if found {
								return
							}
							if file.FlatType(n) == "as_expression" {
								t := file.FlatNodeText(n)
								if strings.Contains(t, "as?") {
									return
								}
								parts := strings.SplitN(t, " as ", 2)
								if len(parts) == 2 {
									asVar := strings.TrimSpace(parts[0])
									asType := strings.TrimSpace(parts[1])
									if asVar == isVar && asType == isType {
										found = true
									}
								}
							}
						})
						if found {
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								"Consider using safe cast 'as?' instead of is-check followed by unsafe cast.")
							return
						}
					}
				}
			},
		})
	}
	{
		r := &VarCouldBeValRule{BaseRule: BaseRule{RuleName: "VarCouldBeVal", RuleSetName: "style", Sev: "warning", Desc: "Detects var properties that are never reassigned and could be declared as val."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				var varKeyword uint32
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "var" || file.FlatNodeTextEquals(child, "var") {
						varKeyword = child
						break
					}
				}
				if varKeyword == 0 {
					return
				}
				if file.FlatHasModifier(idx, "override") {
					return
				}
				if file.FlatHasModifier(idx, "lateinit") {
					return
				}
				if file.FlatHasChildOfType(idx, "property_delegate") {
					return
				}
				if hasFrameworkAnnotationFlat(file, idx) {
					return
				}
				if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "setter" {
					return
				}
				if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "getter" {
					if nextNext, ok := file.FlatNextSibling(nextSib); ok && file.FlatType(nextNext) == "setter" {
						return
					}
				}
				parent, ok := file.FlatParent(idx)
				if !ok {
					return
				}
				isLocal := file.FlatType(parent) == "statements"
				isClassLevel := file.FlatType(parent) == "class_body"
				if isClassLevel {
					if !file.FlatHasModifier(idx, "private") {
						return
					}
				} else if !isLocal {
					if file.FlatType(parent) == "source_file" && !file.FlatHasModifier(idx, "private") {
						return
					}
				}
				varName := ""
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "variable_declaration" {
						varName = extractIdentifierFlat(file, child)
						break
					}
				}
				if varName == "" {
					return
				}
				reassigned := r.reassignedNamesFlat(parent, file)[varName]
				if !reassigned {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("'var %s' is never reassigned. Use 'val' instead.", varName))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(varKeyword)),
						EndByte:     int(file.FlatEndByte(varKeyword)),
						Replacement: "val",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &MayBeConstantRule{BaseRule: BaseRule{RuleName: "MayBeConstant", RuleSetName: "style", Sev: "warning", Desc: "Detects top-level val properties with constant initializers that could be declared as const val."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".kts") {
					return
				}
				if parent, ok := file.FlatParent(idx); !ok {
					return
				} else if pt := file.FlatType(parent); pt != "source_file" && pt != "companion_object" {
					// Top-level, or companion-object-level, only — not
					// inside nested class bodies or function bodies.
					if pt == "class_body" {
						if gp, ok := file.FlatParent(parent); !ok || file.FlatType(gp) != "companion_object" {
							return
						}
					} else {
						return
					}
				}
				// Must be `val` (not `var`) and not already `const`.
				if propertyDeclarationIsVar(file, idx) {
					return
				}
				if file.FlatHasModifier(idx, "const") {
					return
				}
				initExpr := propertyInitializerExpression(file, idx)
				if initExpr == 0 {
					return
				}
				if mayBeConstantExpressionFlat(ctx, initExpr) {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Property may be declared as 'const val'.")
					mods, _ := file.FlatFindChild(idx, "modifiers")
					if mods != 0 {
						modsText := file.FlatNodeText(mods)
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(mods)),
							EndByte:     int(file.FlatEndByte(mods)),
							Replacement: modsText + " const",
						}
					} else {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     int(file.FlatStartByte(idx)) + 3,
							Replacement: "const val",
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &ModifierOrderRule{BaseRule: BaseRule{RuleName: "ModifierOrder", RuleSetName: "style", Sev: "warning", Desc: "Detects modifiers that are not in the recommended Kotlin ordering."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"modifiers"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var mods []string
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "annotation", "line_comment", "multiline_comment":
						continue
					}
					text := strings.TrimSpace(file.FlatNodeText(child))
					if text != "" {
						mods = append(mods, text)
					}
				}
				if len(mods) <= 1 {
					return
				}
				lastIdx := -1
				for _, m := range mods {
					orderIdx := modifierIndex(m)
					if orderIdx < 0 {
						continue
					}
					if orderIdx < lastIdx {
						f := r.Finding(file, file.FlatRow(idx)+1, 1,
							"Modifiers are not in the recommended order.")
						sorted := sortModifiers(mods)
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: strings.Join(sorted, " "),
						}
						ctx.Emit(f)
						return
					}
					lastIdx = orderIdx
				}
			},
		})
	}
	{
		r := &FunctionOnlyReturningConstantRule{BaseRule: BaseRule{RuleName: "FunctionOnlyReturningConstant", RuleSetName: "style", Sev: "warning", Desc: "Detects functions whose body only returns a constant value that could be a const val."}, IgnoreOverridableFunction: true, IgnoreActualFunction: true}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if r.IgnoreOverridableFunction &&
					(file.FlatHasModifier(idx, "override") ||
						file.FlatHasModifier(idx, "open") ||
						file.FlatHasModifier(idx, "abstract")) {
					return
				}
				if r.IgnoreActualFunction && file.FlatHasModifier(idx, "actual") {
					return
				}
				if HasIgnoredAnnotation(file.FlatNodeText(idx),
					[]string{"Provides", "Binds", "BindsInstance", "BindsOptionalOf",
						"IntoSet", "IntoMap", "ElementsIntoSet", "Multibinds",
						"ContributesBinding", "ContributesMultibinding",
						"ContributesTo", "ContributesSubcomponent"}) {
					return
				}
				params, _ := file.FlatFindChild(idx, "function_value_parameters")
				if params != 0 {
					paramText := file.FlatNodeText(params)
					if len(strings.TrimSpace(strings.Trim(paramText, "()"))) > 0 {
						return
					}
				}
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "receiver_type" {
						return
					}
				}
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if t := file.FlatType(p); t == "class_declaration" || t == "object_declaration" {
						for i := 0; i < file.FlatChildCount(p); i++ {
							c := file.FlatChild(p, i)
							if ct := file.FlatType(c); ct == "interface" || (ct == "class" && file.FlatNodeTextEquals(c, "interface")) {
								return
							}
						}
						break
					}
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				bodyText := strings.TrimSpace(file.FlatNodeText(body))
				if strings.HasPrefix(bodyText, "=") {
					expr := strings.TrimSpace(strings.TrimPrefix(bodyText, "="))
					if isConstant(expr) {
						name := extractIdentifierFlat(file, idx)
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Function '%s' only returns a constant. Consider replacing with a const val.", name))
						return
					}
				}
				inner := strings.TrimPrefix(bodyText, "{")
				inner = strings.TrimSuffix(inner, "}")
				inner = strings.TrimSpace(inner)
				if strings.HasPrefix(inner, "return ") && !strings.Contains(inner, "\n") {
					expr := strings.TrimPrefix(inner, "return ")
					if isConstant(expr) {
						name := extractIdentifierFlat(file, idx)
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Function '%s' only returns a constant. Consider replacing with a const val.", name))
					}
				}
			},
		})
	}
	{
		r := &LoopWithTooManyJumpStatementsRule{BaseRule: BaseRule{RuleName: "LoopWithTooManyJumpStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects loops containing more break or continue statements than the configured maximum."}, MaxJumps: 1}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement", "while_statement", "do_while_statement"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				jumpCount := 0
				var walk func(n uint32, depth int)
				walk = func(n uint32, depth int) {
					if n == 0 {
						return
					}
					if depth > 0 {
						switch file.FlatType(n) {
						case "for_statement", "while_statement", "do_while_statement",
							"lambda_literal", "function_declaration", "anonymous_function":
							return
						}
					}
					if file.FlatType(n) == "jump_expression" {
						text := file.FlatNodeText(n)
						if strings.HasPrefix(text, "break") || strings.HasPrefix(text, "continue") {
							jumpCount++
						}
					}
					for i := 0; i < file.FlatChildCount(n); i++ {
						walk(file.FlatChild(n, i), depth+1)
					}
				}
				walk(idx, 0)
				if jumpCount > r.MaxJumps {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Loop has %d jump statements, max allowed is %d.", jumpCount, r.MaxJumps))
				}
			},
		})
	}
	{
		r := &ExplicitItLambdaParameterRule{BaseRule: BaseRule{RuleName: "ExplicitItLambdaParameter", RuleSetName: "style", Sev: "warning", Desc: "Detects single-parameter lambdas that explicitly name their parameter it instead of using the implicit it."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				paramsNode, _ := file.FlatFindChild(idx, "lambda_parameters")
				if paramsNode == 0 {
					return
				}
				var paramNodes []uint32
				for i := 0; i < file.FlatChildCount(paramsNode); i++ {
					child := file.FlatChild(paramsNode, i)
					if t := file.FlatType(child); t == "variable_declaration" || t == "simple_identifier" {
						paramNodes = append(paramNodes, child)
					}
				}
				if len(paramNodes) != 1 {
					return
				}
				param := paramNodes[0]
				var name string
				hasType := false
				if file.FlatType(param) == "simple_identifier" {
					name = file.FlatNodeText(param)
				} else {
					id, _ := file.FlatFindChild(param, "simple_identifier")
					if id != 0 {
						name = file.FlatNodeText(id)
					}
					if file.FlatHasChildOfType(param, "user_type") || file.FlatHasChildOfType(param, "nullable_type") ||
						file.FlatHasChildOfType(param, "function_type") {
						hasType = true
					}
				}
				if name != "it" {
					return
				}
				var msg string
				if hasType {
					msg = "`it` should not be used as name for a lambda parameter."
				} else {
					msg = "Explicit 'it' lambda parameter is redundant. Use implicit 'it'."
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1, msg)
				if !hasType {
					arrowNode := findArrowInLambdaFlat(file, idx)
					if arrowNode != 0 {
						arrowEnd := int(file.FlatEndByte(arrowNode))
						if arrowEnd < len(file.Content) && file.Content[arrowEnd] == ' ' {
							arrowEnd++
						}
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     arrowEnd,
							Replacement: "{ ",
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ExplicitItLambdaMultipleParametersRule{BaseRule: BaseRule{RuleName: "ExplicitItLambdaMultipleParameters", RuleSetName: "style", Sev: "warning", Desc: "Detects multi-parameter lambdas that use it as a parameter name."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				paramsNode, _ := file.FlatFindChild(idx, "lambda_parameters")
				if paramsNode == 0 {
					return
				}
				var names []string
				for i := 0; i < file.FlatChildCount(paramsNode); i++ {
					child := file.FlatChild(paramsNode, i)
					var name string
					switch file.FlatType(child) {
					case "simple_identifier":
						name = file.FlatNodeText(child)
					case "variable_declaration":
						id, _ := file.FlatFindChild(child, "simple_identifier")
						if id != 0 {
							name = file.FlatNodeText(id)
						}
					default:
						continue
					}
					if name != "" {
						names = append(names, name)
					}
				}
				if len(names) <= 1 {
					return
				}
				for _, name := range names {
					if name == "it" {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							"`it` should not be used as name for a lambda parameter.")
						return
					}
				}
			},
		})
	}
}
