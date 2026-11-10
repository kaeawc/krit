package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerComplexityRules() {

	// --- from complexity.go ---
	{
		// AllowedLines is detekt's threshold minus one, because detekt reports at lines >= threshold.
		r := &LongMethodRule{BaseRule: BaseRule{RuleName: "LongMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions that exceed a configurable line count threshold."}, AllowedLines: 59}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				lines, ok := functionBodyCodeLines(file, idx)
				if !ok || lines <= r.AllowedLines {
					return
				}
				ctx.EmitAt(declLine(file, idx), declCol(file, idx),
					fmt.Sprintf("The function %s is too long (%d). The maximum length is %d.", detektFunctionName(file, idx), lines, r.AllowedLines+1))
			},
		})
	}
	{
		// AllowedLines is detekt's threshold minus one, because detekt reports at lines >= threshold.
		r := &LargeClassRule{BaseRule: BaseRule{RuleName: "LargeClass", RuleSetName: "complexity", Sev: "warning", Desc: "Detects classes that exceed a configurable line count threshold."}, AllowedLines: 599}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "companion_object"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if lines := classCodeLines(file, idx); lines > r.AllowedLines {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Class %s is too large. Consider splitting it into smaller pieces.", detektClassName(file, idx)))
				}
			},
		})
	}
	{
		// AllowedDepth is detekt's threshold minus one, because detekt reports at depth >= threshold.
		r := &NestedBlockDepthRule{BaseRule: BaseRule{RuleName: "NestedBlockDepth", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions with excessive nesting depth of control flow blocks."}, AllowedDepth: 3}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasEnclosingFunction(file, idx) {
					return
				}
				if detektNestedBlockDepth(file, idx) > r.AllowedDepth {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Function %s is nested too deeply.", extractIdentifierFlat(file, idx)))
				}
			},
		})
	}
	{
		// AllowedComplexity is detekt's threshold minus one, because detekt reports at complexity >= threshold.
		r := &CyclomaticComplexMethodRule{
			BaseRule:          BaseRule{RuleName: "CyclomaticComplexMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions whose cyclomatic complexity exceeds a configurable threshold."},
			AllowedComplexity: 14,
			NestingFunctions:  []string{"also", "apply", "forEach", "isNotNull", "ifNull", "let", "run", "use", "with"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasEnclosingFunction(file, idx) {
					return
				}
				if r.IgnoreSingleWhenExpression && functionBodyIsSingleWhenExpressionFlat(file, idx) {
					return
				}
				opts := cyclomaticOptions{
					ignoreSimpleWhenEntries: r.IgnoreSimpleWhenEntries,
					ignoreNestingFunctions:  r.IgnoreNestingFunctions,
					nestingFunctions:        make(map[string]bool, len(r.NestingFunctions)),
				}
				for _, name := range r.NestingFunctions {
					opts.nestingFunctions[strings.TrimSpace(name)] = true
				}
				if complexity := detektCyclomaticComplexity(file, idx, opts); complexity > r.AllowedComplexity {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("The function %s appears to be too complex based on Cyclomatic Complexity (complexity: %d). Defined complexity threshold for methods is set to '%d'",
							extractIdentifierFlat(file, idx), complexity, r.AllowedComplexity+1))
				}
			},
		})
	}
	{
		// AllowedComplexity is detekt's threshold minus one, because detekt reports at complexity >= threshold.
		r := &CognitiveComplexMethodRule{BaseRule: BaseRule{RuleName: "CognitiveComplexMethod", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions whose cognitive complexity exceeds a configurable threshold, weighting nesting depth."}, AllowedComplexity: 14}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasEnclosingFunction(file, idx) {
					return
				}
				if complexity := detektCognitiveComplexity(file, idx); complexity > r.AllowedComplexity {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("The function %s appears to be too complex based on Cognitive Complexity (complexity: %d). Defined complexity threshold for methods is set to '%d'",
							extractIdentifierFlat(file, idx), complexity, r.AllowedComplexity+1))
				}
			},
		})
	}
	{
		// AllowedConditions is detekt's threshold (4) minus one, because detekt reports at count >= threshold.
		r := &ComplexConditionRule{BaseRule: BaseRule{RuleName: "ComplexCondition", RuleSetName: "complexity", Sev: "warning", Desc: "Detects conditions with too many mixed logical operators."}, AllowedConditions: 3}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression", "while_statement", "do_while_statement"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				cond := conditionExpressionOf(file, idx)
				if cond == 0 {
					return
				}
				// detekt: with more than one binary expression, take the longest one by text and count the
				// `&&` and `||` in its text (strings and lambdas included), plus one.
				var longest uint32
				binaries := 0
				var walk func(n uint32)
				walk = func(n uint32) {
					if isPsiBinaryExpression(file, n) {
						binaries++
						if longest == 0 || file.FlatEndByte(n)-file.FlatStartByte(n) > file.FlatEndByte(longest)-file.FlatStartByte(longest) {
							longest = n
						}
					}
					for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
						walk(c)
					}
				}
				walk(cond)
				if binaries <= 1 {
					return
				}
				text := file.FlatNodeText(longest)
				count := strings.Count(text, "&&") + strings.Count(text, "||") + 1
				if count > r.AllowedConditions {
					ctx.EmitAt(file.FlatRow(cond)+1, file.FlatCol(cond)+1,
						fmt.Sprintf("This condition is too complex (%d). Defined complexity threshold for conditions is set to '%d'", count, r.AllowedConditions+1))
				}
			},
		})
	}
	{
		r := &ComplexInterfaceRule{BaseRule: BaseRule{RuleName: "ComplexInterface", RuleSetName: "complexity", Sev: "warning", Desc: "Detects interfaces with too many member declarations."}, AllowedDefinitions: 9, IncludePrivateDeclarations: false, IncludeStaticDeclarations: false}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasChildOfType(idx, "interface") {
					return
				}
				body, _ := file.FlatFindChild(idx, "class_body")
				if body == 0 {
					return
				}
				members := countComplexInterfaceMembersFlat(file, body, r.IncludePrivateDeclarations, r.IncludeStaticDeclarations, r.IgnoreOverloaded)
				if members > r.AllowedDefinitions {
					name := extractIdentifierFlat(file, idx)
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("The interface %s is too complex. Consider splitting it up.", name))
				}
			},
		})
	}
	{
		r := &LabeledExpressionRule{BaseRule: BaseRule{RuleName: "LabeledExpression", RuleSetName: "complexity", Sev: "warning", Desc: "Detects labeled expressions such as return@label, break@label, and continue@label."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"label", "jump_expression", "this_expression", "super_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := ""
				switch file.FlatType(idx) {
				case "label":
					// A label after return, break or continue is reported with its jump expression.
					if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "jump_expression" {
						return
					}
					name = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(idx)), "@")
				case "jump_expression":
					if l, ok := file.FlatFindChild(idx, "label"); ok {
						name = strings.TrimSpace(file.FlatNodeText(l))
					}
				default:
					for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
						if t := file.FlatType(c); t == "type_identifier" || t == "label" {
							name = file.FlatNodeText(c)
						}
					}
					if name == "" {
						return
					}
					if file.FlatType(idx) == "this_expression" && !thisLabelReportable(file, idx, name) {
						return
					}
				}
				if name == "" || labeledExpressionLabelIgnored(name, r.IgnoredLabels) {
					return
				}
				row, col := file.FlatRow(idx)+1, file.FlatCol(idx)+1
				if file.FlatType(idx) == "jump_expression" {
					// detekt anchors a jump at its label qualifier, `@name`.
					if l, ok := file.FlatFindChild(idx, "label"); ok {
						row, col = file.FlatRow(l)+1, file.FlatCol(l)
					}
				}
				ctx.EmitAt(row, col, "Expression with labels increase complexity and affect maintainability.")
			},
		})
	}
	{
		// The allowed counts are detekt's thresholds (6 and 7) minus one, because detekt reports at count >= threshold.
		r := &LongParameterListRule{BaseRule: BaseRule{RuleName: "LongParameterList", RuleSetName: "complexity", Sev: "warning", Desc: "Detects functions or constructors with too many parameters."}, AllowedFunctionParameters: 5, AllowedConstructorParameters: 6, IgnoreDefaultParameters: false, IgnoreDataClasses: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "class_declaration", "secondary_constructor"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasEnclosingFunction(file, idx) {
					return
				}
				res := compileAnnotationPatterns(r.IgnoreAnnotatedParameter)
				emit := func(list uint32, entries []longParamEntry, description string, allowed int) {
					if countLongParams(entries, r.IgnoreDefaultParameters) <= allowed {
						return
					}
					anchor := file.FlatFirstChild(list)
					for c := anchor; c != 0; c = file.FlatNextSib(c) {
						if file.FlatType(c) == "(" {
							anchor = c
							break
						}
					}
					ctx.EmitAt(file.FlatRow(anchor)+1, file.FlatCol(anchor)+1,
						longParameterListMessage(description, entries, allowed+1))
				}
				switch file.FlatType(idx) {
				case "function_declaration":
					if file.FlatHasModifier(idx, "override") {
						return
					}
					params, ok := file.FlatFindChild(idx, "function_value_parameters")
					if !ok {
						return
					}
					emit(params, functionParamEntries(file, params, res), "function "+extractIdentifierFlat(file, idx), r.AllowedFunctionParameters)
				case "class_declaration":
					if r.IgnoreDataClasses && file.FlatHasModifier(idx, "data") {
						return
					}
					if ctor, ok := file.FlatFindChild(idx, "primary_constructor"); ok {
						emit(ctor, classParamEntries(file, ctor, res), "constructor", r.AllowedConstructorParameters)
					}
				case "secondary_constructor":
					if cls, ok := file.FlatParent(idx); ok {
						if cls, ok = file.FlatParent(cls); ok && r.IgnoreDataClasses && file.FlatHasModifier(cls, "data") {
							return
						}
					}
					if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
						emit(params, functionParamEntries(file, params, res), "constructor", r.AllowedConstructorParameters)
					}
				}
			},
		})
	}
	{
		r := &MethodOverloadingRule{BaseRule: BaseRule{RuleName: "MethodOverloading", RuleSetName: "complexity", Sev: "warning", Desc: "Detects methods with too many overloads of the same name in a scope."}, AllowedOverloads: 5}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"source_file", "class_declaration", "object_declaration", "companion_object"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				r.checkScopeFlat(ctx, ctx.Idx)
			},
		})
	}
	{
		r := &NamedArgumentsRule{BaseRule: BaseRule{RuleName: "NamedArguments", RuleSetName: "complexity", Sev: "warning", Desc: "Detects function calls with too many unnamed positional arguments."}, AllowedArguments: 3}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
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
				unnamed, total := 0, 0
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) == "value_argument" {
						total++
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
				// detekt counts named arguments toward the limit and reports when any argument is unnamed.
				if total > r.AllowedArguments && unnamed > 0 {
					if flatCallForwardsEnclosingFunctionParameters(file, idx, args) || namedArgumentsVarargCalls[extractCallNameFlat(file, idx)] {
						return
					}
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						fmt.Sprintf("This function call has %d arguments. To call a function with more than %d arguments you should set the name of each argument.", total, r.AllowedArguments))
				}
			},
		})
	}
	{
		r := &NestedScopeFunctionsRule{BaseRule: BaseRule{RuleName: "NestedScopeFunctions", RuleSetName: "complexity", Sev: "warning", Desc: "Detects excessively nested Kotlin scope functions like apply, also, let, run, and with."}, AllowedDepth: 1}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				active := nestedScopeFunctionsActiveSet(r.Functions)
				name := extractCallNameFlat(file, idx)
				if !active[name] {
					return
				}
				depth := 0
				for child, parent := idx, firstParent(file, idx); parent != 0; child, parent = parent, firstParent(file, parent) {
					// Only a call inside the outer call's lambda is nested; a call in its receiver is a chain.
					if file.FlatType(parent) == "call_expression" && file.FlatType(child) == "call_suffix" &&
						active[extractCallNameFlat(file, parent)] {
						depth++
					}
				}
				if depth > r.AllowedDepth {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						fmt.Sprintf("The scope function '%s' is nested too deeply ('%d'). Complexity threshold is set to '%d'.", name, depth, r.AllowedDepth))
				}
			},
		})
	}
	{
		r := &ReplaceSafeCallChainWithRunRule{BaseRule: BaseRule{RuleName: "ReplaceSafeCallChainWithRun", RuleSetName: "complexity", Sev: "warning", Desc: "Detects chains of three or more safe calls (?.) that could be simplified with ?.run { }."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
		r := &StringLiteralDuplicationRule{BaseRule: BaseRule{RuleName: "StringLiteralDuplication", RuleSetName: "complexity", Sev: "warning", Desc: "Detects string literals that appear more than a configurable number of times in a file."}, AllowedDuplications: 2, AllowedWithLengthLessThan: 5, IgnoreAnnotation: true, IgnoreStringsRegex: "$^"}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// detekt matches the whole content of the literal against ignoreStringsRegex.
				var ignoreRe *regexp.Regexp
				if r.IgnoreStringsRegex != "" {
					if compiled, err := regexp.Compile("^(?:" + r.IgnoreStringsRegex + ")$"); err == nil {
						ignoreRe = compiled
					}
				}
				// Short literals are skipped by UTF-16 length, like Kotlin's String.length.
				minContentLen := r.AllowedWithLengthLessThan
				counts := make(map[string]int)
				firstLine := make(map[string]int)
				firstCol := make(map[string]int)
				file.FlatWalkNodes(idx, "string_literal", func(strNode uint32) {
					content := stringLiteralUnquote(file.FlatNodeText(strNode))
					if minContentLen > 0 && len(utf16.Encode([]rune(content))) < minContentLen {
						return
					}
					if ignoreRe != nil && ignoreRe.MatchString(content) {
						return
					}
					if r.IgnoreAnnotation && stringLiteralInsideAnnotationFlat(file, strNode) {
						return
					}
					counts[content]++
					if _, ok := firstLine[content]; !ok {
						firstLine[content] = file.FlatRow(strNode) + 1
						firstCol[content] = file.FlatCol(strNode) + 1
					}
				})
				for text, count := range counts {
					if count > r.AllowedDuplications {
						ctx.EmitAt(firstLine[text], firstCol[text],
							"Multiple occurrences of the same string literal within a single file detected. Prefer extracting the string literal into a property or constant.")
					}
				}
			},
		})
	}
	{
		// The allowed counts are detekt's thresholds (11) minus one, because detekt reports at count >= threshold.
		r := &TooManyFunctionsRule{BaseRule: BaseRule{RuleName: "TooManyFunctions", RuleSetName: "complexity", Sev: "warning", Desc: "Detects files or classes that declare too many functions."},
			AllowedFunctionsPerFile: 10, AllowedFunctionsPerClass: 10, AllowedFunctionsPerInterface: 10, AllowedFunctionsPerObject: 10, AllowedFunctionsPerEnum: 10}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"source_file", "class_declaration", "object_declaration", "companion_object"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "source_file" {
					// detekt does not count a script's functions toward the file threshold.
					if file.IsScript() {
						return
					}
					count := 0
					for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
						if file.FlatType(c) == "function_declaration" && r.shouldCountFunctionFlat(c, file) {
							count++
						}
					}
					if count > r.AllowedFunctionsPerFile {
						row, col := 1, 1
						if hdr, ok := file.FlatFindChild(idx, "package_header"); ok {
							row, col = file.FlatRow(hdr)+1, file.FlatCol(hdr)+1
						} else {
							for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
								switch file.FlatType(c) {
								case "import_list", "import_header", "line_comment", "multiline_comment", "file_annotation":
									continue
								}
								row, col = file.FlatRow(c)+1, file.FlatCol(c)+1
								break
							}
						}
						ctx.EmitAt(row, col, fmt.Sprintf("File '%s' with '%d' functions detected. Defined threshold inside files is set to '%d'",
							filepath.Base(file.Path), count, r.AllowedFunctionsPerFile+1))
					}
					return
				}
				count := r.countFunctionsInClassFlat(idx, file)
				kind, limit, plural := "Class", r.AllowedFunctionsPerClass, "classes"
				switch {
				case file.FlatType(idx) != "class_declaration":
					kind, limit, plural = "Object", r.AllowedFunctionsPerObject, "objects"
				case file.FlatHasChildOfType(idx, "interface"):
					kind, limit, plural = "Interface", r.AllowedFunctionsPerInterface, "interfaces"
				case classIsEnum(file, idx):
					kind, limit, plural = "Enum class", r.AllowedFunctionsPerEnum, "enum classes"
				}
				if count > limit {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("%s '%s' with '%d' functions detected. Defined threshold inside %s is set to '%d'",
							kind, detektClassName(file, idx), count, plural, limit+1))
				}
			},
		})
	}
}

// namedArgumentsVarargCalls are variadic library functions. Their elements share one vararg parameter, so
// naming them makes no sense.
var namedArgumentsVarargCalls = map[string]bool{
	"listOf": true, "mutableListOf": true, "listOfNotNull": true, "arrayListOf": true,
	"setOf": true, "mutableSetOf": true, "setOfNotNull": true, "hashSetOf": true, "linkedSetOf": true, "sortedSetOf": true,
	"mapOf": true, "mutableMapOf": true, "hashMapOf": true, "linkedMapOf": true, "sortedMapOf": true,
	"arrayOf": true, "arrayOfNulls": true, "intArrayOf": true, "longArrayOf": true, "doubleArrayOf": true,
	"floatArrayOf": true, "booleanArrayOf": true, "charArrayOf": true, "byteArrayOf": true, "shortArrayOf": true,
	"sequenceOf": true, "format": true, "printf": true, "asList": true, "hash": true,
}

func firstParent(file *scanner.File, idx uint32) uint32 {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return 0
	}
	return parent
}
