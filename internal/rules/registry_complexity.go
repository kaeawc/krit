package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerComplexityRules() {

	// --- from complexity.go ---
	{
		r := &LongMethodRule{BaseRule: BaseRule{RuleName: "LongMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions that exceed a configurable line count threshold."}, AllowedLines: 60}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if strings.HasSuffix(file.Path, "Table.kt") || strings.HasSuffix(file.Path, "Tables.kt") ||
					strings.HasSuffix(file.Path, "Dao.kt") || strings.HasSuffix(file.Path, "DAO.kt") {
					return
				}
				if flatHasAnnotationNamed(file, idx, "Composable") {
					return
				}
				if flatHasAnnotationNamed(file, idx, "Test") ||
					flatHasAnnotationNamed(file, idx, "ParameterizedTest") {
					return
				}
				if strings.Contains(file.Path, "/migration/") ||
					strings.Contains(file.Path, "\\migration\\") ||
					strings.Contains(file.Path, "/migrations/") ||
					strings.Contains(file.Path, "\\migrations\\") {
					return
				}
				if isDSLBuilderBodyFlat(idx, file) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if androidLifecycleMethods[name] {
					return
				}
				if strings.HasSuffix(file.Path, "Job.kt") &&
					(name == "run" || name == "onRun" || name == "doRun" || name == "onHandle") {
					return
				}
				lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx))
				if lines > r.AllowedLines {
					line := longMethodDeclarationLineFlat(file, idx)
					ctx.EmitAt(line, 1,
						fmt.Sprintf("Function '%s' has %d lines (allowed: %d).", name, lines, r.AllowedLines))
				}
			},
		})
	}
	{
		r := &LargeClassRule{BaseRule: BaseRule{RuleName: "LargeClass", RuleSetName: "complexity", Sev: "warning", Desc: "Detects classes that exceed a configurable line count threshold."}, AllowedLines: 600}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if strings.HasSuffix(file.Path, "Table.kt") || strings.HasSuffix(file.Path, "Tables.kt") ||
					strings.HasSuffix(file.Path, "Dao.kt") || strings.HasSuffix(file.Path, "DAO.kt") {
					return
				}
				lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx))
				if lines > r.AllowedLines {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Class '%s' has %d lines (allowed: %d).", name, lines, r.AllowedLines))
				}
			},
		})
	}
	{
		r := &NestedBlockDepthRule{BaseRule: BaseRule{RuleName: "NestedBlockDepth", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions with excessive nesting depth of control flow blocks."}, AllowedDepth: 4}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				depth, line, exceeded := nestedBlockDepthExceedsFlat(file, idx, r.AllowedDepth)
				if exceeded {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(line, 1,
						fmt.Sprintf("Function '%s' has a nested block depth of %d (allowed: %d).", name, depth, r.AllowedDepth))
				}
			},
		})
	}
	{
		r := &CyclomaticComplexMethodRule{BaseRule: BaseRule{RuleName: "CyclomaticComplexMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions whose cyclomatic complexity exceeds a configurable threshold."}, AllowedComplexity: 14, IgnoreSimpleWhenEntries: false}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "override") {
					name := extractIdentifierFlat(file, idx)
					if name == "equals" || name == "hashCode" {
						return
					}
				}
				if isPureBooleanPredicateFlat(file, idx) {
					return
				}
				if lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx)); lines > 60 {
					return
				}
				complexity, line, exceeded := cyclomaticComplexityExceedsFlat(file, idx, r.AllowedComplexity, r.IgnoreSimpleWhenEntries)
				if exceeded {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(line, 1,
						fmt.Sprintf("Function '%s' has a cyclomatic complexity of %d (allowed: %d).", name, complexity, r.AllowedComplexity))
				}
			},
		})
	}
	{
		r := &CognitiveComplexMethodRule{BaseRule: BaseRule{RuleName: "CognitiveComplexMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions whose cognitive complexity exceeds a configurable threshold, weighting nesting depth."}, AllowedComplexity: 15}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				metrics := getComplexityMetricsFlat(idx, file)
				if metrics.cognitive > r.AllowedComplexity {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Function '%s' has a cognitive complexity of %d (allowed: %d).", name, metrics.cognitive, r.AllowedComplexity))
				}
			},
		})
	}
	{
		r := &ComplexConditionRule{BaseRule: BaseRule{RuleName: "ComplexCondition", RuleSetName: "complexity", Sev: "warning", Desc: "Detects conditions with too many mixed logical operators."}, AllowedConditions: 3}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression", "while_statement"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				condOps := countLogicalOperatorsOutsideBodiesFlat(file, idx)
				if condOps > r.AllowedConditions {
					if isPureDisjunctionOrConjunctionFlat(file, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Complex condition with %d logical operators (allowed: %d).", condOps, r.AllowedConditions))
				}
			},
		})
	}
	{
		r := &ComplexInterfaceRule{BaseRule: BaseRule{RuleName: "ComplexInterface", RuleSetName: "complexity", Sev: "warning", Desc: "Detects interfaces with too many member declarations."}, AllowedDefinitions: 10}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasChildOfType(idx, "interface") {
					return
				}
				body, _ := file.FlatFindChild(idx, "class_body")
				if body == 0 {
					return
				}
				members := countDirectClassMembersFlat(file, body)
				if members > r.AllowedDefinitions {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Interface '%s' has %d members (allowed: %d).", name, members, r.AllowedDefinitions))
				}
			},
		})
	}
	{
		r := &LabeledExpressionRule{BaseRule: BaseRule{RuleName: "LabeledExpression", RuleSetName: "complexity", Sev: "warning", Desc: "Detects labeled expressions such as return@label, break@label, and continue@label."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"label"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Labeled expression '%s' detected. Consider refactoring to avoid labels.", strings.TrimSpace(file.FlatNodeText(idx))))
			},
		})
	}
	{
		r := &LongParameterListRule{BaseRule: BaseRule{RuleName: "LongParameterList", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions or constructors with too many parameters."}, AllowedFunctionParameters: 5, AllowedConstructorParameters: 6, IgnoreDefaultParameters: false, IgnoreDataClasses: true}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if file.FlatType(idx) == "function_declaration" {
					summary := getFunctionDeclSummaryFlat(file, idx)
					if summary.hasComposable {
						return
					}
					if summary.hasOverride {
						return
					}
					if lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx)); lines > 60 {
						return
					}
					if summary.paramsNode == 0 {
						return
					}
					params := 0
					limit := r.AllowedFunctionParameters
					for _, p := range summary.params {
						if r.IgnoreDefaultParameters && p.hasDefault {
							continue
						}
						if strings.Contains(file.FlatNodeText(p.idx), "->") {
							continue
						}
						params++
						if params > limit {
							name := summary.name
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								fmt.Sprintf("Function '%s' has %d parameters (allowed: %d).", name, params, limit))
							return
						}
					}
				} else if file.FlatType(idx) == "class_declaration" {
					summary := getClassDeclSummaryFlat(file, idx)
					if r.IgnoreDataClasses && summary.hasData {
						return
					}
					if summary.hasParcelizeLike {
						return
					}
					if len(summary.classParams) > 0 && r.IgnoreDataClasses {
						allProps := true
						for _, p := range summary.classParams {
							if !p.isProperty {
								allProps = false
								break
							}
						}
						if allProps {
							return
						}
					}
					clsName := summary.name
					if strings.HasSuffix(clsName, "ViewModel") || strings.HasSuffix(clsName, "Presenter") {
						return
					}
					params := 0
					limit := r.AllowedConstructorParameters
					for _, p := range summary.classParams {
						if r.IgnoreDefaultParameters && p.hasDefault {
							continue
						}
						if p.isFunctionType {
							continue
						}
						params++
						if params > limit {
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								fmt.Sprintf("Constructor of '%s' has %d parameters (allowed: %d).", clsName, params, limit))
							return
						}
					}
				}
			},
		})
	}
	{
		r := &MethodOverloadingRule{BaseRule: BaseRule{RuleName: "MethodOverloading", RuleSetName: "complexity", Sev: "warning", Desc: "Detects methods with too many overloads of the same name in a scope."}, AllowedOverloads: 6}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				r.checkScopeFlat(ctx, ctx.Idx)
			},
		})
	}
	{
		r := &NamedArgumentsRule{BaseRule: BaseRule{RuleName: "NamedArguments", RuleSetName: "complexity", Sev: "warning", Desc: "Detects function calls with too many unnamed positional arguments."}, AllowedArguments: 3}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				args, _ := file.FlatFindChild(idx, "value_arguments")
				if args == 0 {
					callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
					if callSuffix != 0 {
						args, _ = file.FlatFindChild(callSuffix, "value_arguments")
					}
				}
				if args == 0 {
					return
				}
				unnamed := 0
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) == "value_argument" {
						isNamed := false
						for j := 0; j < file.FlatChildCount(child); j++ {
							childPart := file.FlatChild(child, j)
							ct := file.FlatType(childPart)
							if ct == "value_argument_label" || ct == "simple_identifier" {
								if j+1 < file.FlatChildCount(child) && file.FlatNodeText(file.FlatChild(child, j+1)) == "=" {
									isNamed = true
									break
								}
							}
						}
						if !isNamed {
							unnamed++
						}
					}
				}
				if unnamed > r.AllowedArguments {
					if flatCallForwardsEnclosingFunctionParameters(file, idx, args) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Function call has %d unnamed arguments (allowed: %d). Use named arguments.", unnamed, r.AllowedArguments))
				}
			},
		})
	}
	{
		r := &NestedScopeFunctionsRule{BaseRule: BaseRule{RuleName: "NestedScopeFunctions", RuleSetName: "complexity", Sev: "warning", Desc: "Detects excessively nested Kotlin scope functions like apply, also, let, run, and with."}, AllowedDepth: 1}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractCallNameFlat(file, idx)
				if !scopeFunctions[name] {
					return
				}
				depth := 0
				for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
					if file.FlatType(parent) == "call_expression" {
						pName := extractCallNameFlat(file, parent)
						if scopeFunctions[pName] {
							depth++
						}
					}
				}
				if depth > r.AllowedDepth {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Nested scope function '%s' at depth %d (allowed: %d).", name, depth, r.AllowedDepth))
				}
			},
		})
	}
	{
		r := &ReplaceSafeCallChainWithRunRule{BaseRule: BaseRule{RuleName: "ReplaceSafeCallChainWithRun", RuleSetName: "complexity", Sev: "warning", Desc: "Detects chains of three or more safe calls (?.) that could be simplified with ?.run { }."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if parent, ok := file.FlatParent(idx); ok {
					if file.FlatType(parent) == "navigation_expression" {
						return
					}
					if file.FlatType(parent) == "call_expression" {
						if grandparent, ok := file.FlatParent(parent); ok && file.FlatType(grandparent) == "navigation_expression" {
							return
						}
					}
				}
				count := countSafeCallsInChainFlat(file, idx)
				if count >= 3 {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Chain of %d safe calls. Consider using '?.run { }' to simplify.", count))
				}
			},
		})
	}
	{
		r := &StringLiteralDuplicationRule{BaseRule: BaseRule{RuleName: "StringLiteralDuplication", RuleSetName: "complexity", Sev: "warning", Desc: "Detects string literals that appear more than a configurable number of times in a file."}, AllowedDuplications: 2}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				counts := make(map[string]int)
				firstLine := make(map[string]int)
				file.FlatWalkNodes(idx, "string_literal", func(strNode uint32) {
					text := file.FlatNodeText(strNode)
					if len(text) <= 3 {
						return
					}
					counts[text]++
					if _, ok := firstLine[text]; !ok {
						firstLine[text] = file.FlatRow(strNode) + 1
					}
				})
				for text, count := range counts {
					if count > r.AllowedDuplications {
						ctx.EmitAt(firstLine[text], 1,
							fmt.Sprintf("String literal %s appears %d times (allowed: %d). Consider extracting to a constant.", text, count, r.AllowedDuplications))
					}
				}
			},
		})
	}
	{
		r := &TooManyFunctionsRule{BaseRule: BaseRule{RuleName: "TooManyFunctions", RuleSetName: "complexity", Sev: "warning", Desc: "Detects files or classes that declare too many functions."}, AllowedFunctionsPerFile: 11, IgnoreOverridden: true}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				topLevelCount := 0
				var classDecls []uint32
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "function_declaration":
						if r.shouldCountFunctionFlat(child, file) {
							topLevelCount++
						}
					case "class_declaration":
						classDecls = append(classDecls, child)
					}
				}
				if topLevelCount > r.AllowedFunctionsPerFile {
					ctx.EmitAt(1, 1,
						fmt.Sprintf("File has %d top-level functions (allowed: %d).", topLevelCount, r.AllowedFunctionsPerFile))
				}
				for _, cls := range classDecls {
					if file.FlatHasModifier(cls, "sealed") {
						continue
					}
					if file.FlatHasModifier(cls, "abstract") {
						continue
					}
					if file.FlatHasChildOfType(cls, "interface") {
						continue
					}
					if flatHasAnnotationNamed(file, cls, "Component") ||
						flatHasAnnotationNamed(file, cls, "Subcomponent") ||
						flatHasAnnotationNamed(file, cls, "Module") ||
						flatHasAnnotationNamed(file, cls, "DependencyGraph") ||
						flatHasAnnotationNamed(file, cls, "GraphExtension") ||
						flatHasAnnotationNamed(file, cls, "ContributesTo") ||
						flatHasAnnotationNamed(file, cls, "BindingContainer") {
						continue
					}
					clsName := extractIdentifierFlat(file, cls)
					if strings.HasSuffix(clsName, "Table") || strings.HasSuffix(clsName, "Tables") ||
						strings.HasSuffix(clsName, "Dao") || strings.HasSuffix(clsName, "DAO") ||
						strings.HasSuffix(clsName, "Repository") || strings.HasSuffix(clsName, "Store") ||
						strings.HasSuffix(clsName, "Fragment") || strings.HasSuffix(clsName, "ViewModel") ||
						strings.HasSuffix(clsName, "Activity") || strings.HasSuffix(clsName, "Screen") ||
						strings.HasSuffix(clsName, "View") || strings.HasSuffix(clsName, "Adapter") ||
						strings.HasSuffix(clsName, "Presenter") || strings.HasSuffix(clsName, "Manager") {
						continue
					}
					count := r.countFunctionsInClassFlat(cls, file)
					limit := r.AllowedFunctionsPerFile
					if r.AllowedFunctionsPerClass > 0 {
						limit = r.AllowedFunctionsPerClass
					}
					if r.AllowedFunctionsPerInterface > 0 && file.FlatHasChildOfType(cls, "interface") {
						limit = r.AllowedFunctionsPerInterface
					} else if r.AllowedFunctionsPerObject > 0 && file.FlatHasChildOfType(cls, "object") {
						limit = r.AllowedFunctionsPerObject
					} else if r.AllowedFunctionsPerEnum > 0 && file.FlatHasChildOfType(cls, "enum") {
						limit = r.AllowedFunctionsPerEnum
					}
					if count > limit {
						name := extractIdentifierFlat(file, cls)
						ctx.EmitAt(file.FlatRow(cls)+1, 1,
							fmt.Sprintf("Class '%s' has %d functions (allowed: %d).", name, count, limit))
					}
				}
			},
		})
	}
}
