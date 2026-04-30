package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerStyleIdiomaticDataRules() {

	// --- from style_idiomatic_data.go ---
	{
		r := &UseArrayLiteralsInAnnotationsRule{BaseRule: BaseRule{RuleName: "UseArrayLiteralsInAnnotations", RuleSetName: "style", Sev: "warning", Desc: "Detects arrayOf() calls in annotations that should use array literal [] syntax."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: 0.9, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Require an actual call to `arrayOf` somewhere under this
				// annotation — not just the substring, which fired on
				// `@Foo("arrayOf(x)" as String)` or KDoc-like usages.
				var arrayOfCall uint32
				file.FlatWalkNodes(idx, "call_expression", func(call uint32) {
					if arrayOfCall != 0 {
						return
					}
					if flatCallExpressionName(file, call) == "arrayOf" {
						arrayOfCall = call
					}
				})
				if arrayOfCall == 0 {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1, "Use array literal '[]' syntax in annotations instead of 'arrayOf()'.")
				// Fix: replace the `arrayOf(args)` span with `[args]`. The
				// args range is precisely the value_arguments node minus
				// its outer parens, which we reconstruct from the AST
				// rather than string-scanning for matching parens.
				args := flatCallKeyArguments(file, arrayOfCall)
				if args != 0 {
					argsStart := int(file.FlatStartByte(args))
					argsEnd := int(file.FlatEndByte(args))
					inner := string(file.Content[argsStart+1 : argsEnd-1]) // strip the ( )
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(arrayOfCall)),
						EndByte:     int(file.FlatEndByte(arrayOfCall)),
						Replacement: "[" + inner + "]",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseSumOfInsteadOfFlatMapSizeRule{BaseRule: BaseRule{RuleName: "UseSumOfInsteadOfFlatMapSize", RuleSetName: "style", Sev: "warning", Desc: "Detects flatMap/map followed by size/count/sum chains that should use sumOf instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !sumOfSourceCalls[name] {
					return
				}
				parent, ok := file.FlatParent(idx)
				if !ok {
					return
				}
				var selectorName string
				var chainEnd uint32
				if file.FlatType(parent) == "navigation_expression" {
					suffix := sumOfNavSelectorFlat(file, parent)
					gp, ok := file.FlatParent(parent)
					if ok && file.FlatType(gp) == "call_expression" {
						if outerName := flatCallExpressionName(file, gp); outerName != "" {
							selectorName = outerName
							chainEnd = gp
						}
					}
					if selectorName == "" && suffix != "" {
						selectorName = suffix
						chainEnd = parent
					}
				}
				if selectorName == "" {
					return
				}
				var msg string
				switch selectorName {
				case "size":
					msg = fmt.Sprintf("Use 'sumOf' instead of '%s' and 'size'.", name)
				case "count":
					msg = fmt.Sprintf("Use 'sumOf' instead of '%s' and 'count'.", name)
				case "sum":
					if name != "map" {
						return
					}
					msg = "Use 'sumOf' instead of 'map' and 'sum'."
				default:
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				if name == "flatMap" && selectorName == "size" && chainEnd != 0 {
					if lambdaSuffix, ok := file.FlatFindChild(idx, "call_suffix"); ok {
						if lambdaNode, ok := file.FlatFindChild(lambdaSuffix, "annotated_lambda"); ok {
							body := strings.TrimSpace(file.FlatNodeText(lambdaNode))
							if len(body) >= 2 && body[0] == '{' && body[len(body)-1] == '}' {
								body = strings.TrimSpace(body[1 : len(body)-1])
							}
							var receiverText string
							for i := 0; i < file.FlatChildCount(idx); i++ {
								child := file.FlatChild(idx, i)
								if file.FlatType(child) == "navigation_expression" {
									if file.FlatChildCount(child) > 0 {
										receiverText = file.FlatNodeText(file.FlatChild(child, 0))
									}
									break
								} else if file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, "flatMap") {
									break
								}
							}
							if receiverText != "" {
								f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(chainEnd)), EndByte: int(file.FlatEndByte(chainEnd)), Replacement: receiverText + ".sumOf { " + body + ".size }"}
							} else {
								f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(chainEnd)), EndByte: int(file.FlatEndByte(chainEnd)), Replacement: "sumOf { " + body + ".size }"}
							}
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseLetRule{BaseRule: BaseRule{RuleName: "UseLet", RuleSetName: "style", Sev: "warning", Desc: "Detects null checks that could be replaced with ?.let {} scope function."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if strings.Contains(text, "!= null") && !strings.Contains(text, "else") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Null check could be replaced with ?.let { }.")
				}
			},
		})
	}
	{
		r := &UseDataClassRule{BaseRule: BaseRule{RuleName: "UseDataClass", RuleSetName: "style", Sev: "warning", Desc: "Detects classes with only properties in the constructor that could be data classes."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "data") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "sealed") || file.FlatHasModifier(idx, "enum") || file.FlatHasModifier(idx, "annotation") {
					return
				}
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				if ctor == 0 {
					return
				}
				paramCount := 0
				file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
					trimmed := strings.TrimSpace(file.FlatNodeText(p))
					if strings.HasPrefix(trimmed, "val ") || strings.HasPrefix(trimmed, "var ") {
						paramCount++
					}
				})
				if paramCount == 0 {
					return
				}
				body, _ := file.FlatFindChild(idx, "class_body")
				if body != 0 {
					for i := 0; i < file.FlatChildCount(body); i++ {
						if file.FlatType(file.FlatChild(body, i)) == "function_declaration" {
							return
						}
					}
				}
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Class '%s' could be a data class.", name))
			},
		})
	}
	{
		r := &UseIfInsteadOfWhenRule{BaseRule: BaseRule{RuleName: "UseIfInsteadOfWhen", RuleSetName: "style", Sev: "warning", Desc: "Detects when expressions with two or fewer branches that could be replaced with if."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				entryCount := 0
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "when_entry" {
						entryCount++
					}
				}
				if entryCount <= 2 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "When expression with two or fewer branches could be replaced with if.")
				}
			},
		})
	}
	{
		r := &UseIfEmptyOrIfBlankRule{BaseRule: BaseRule{RuleName: "UseIfEmptyOrIfBlank", RuleSetName: "style", Sev: "warning", Desc: "Detects manual isEmpty/isBlank checks that could use .ifEmpty {} or .ifBlank {} instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var condNode, thenNode, elseNode uint32
				sawElse := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "if", "(", ")", "{", "}":
						continue
					case "else":
						sawElse = true
						continue
					default:
						if condNode == 0 {
							condNode = child
						} else if !sawElse {
							thenNode = child
						} else if elseNode == 0 {
							elseNode = child
						}
					}
				}
				if condNode == 0 || thenNode == 0 || elseNode == 0 {
					return
				}
				if file.FlatType(elseNode) == "if_expression" {
					return
				}
				condText := strings.TrimSpace(file.FlatNodeText(condNode))
				isNegatedPrefix := false
				innerCondText := condText
				if file.FlatType(condNode) == "prefix_expression" && file.FlatChildCount(condNode) >= 2 {
					if opNode := file.FlatChild(condNode, 0); opNode != 0 && file.FlatNodeTextEquals(opNode, "!") {
						isNegatedPrefix = true
						if argNode := file.FlatChild(condNode, 1); argNode != 0 {
							innerCondText = strings.TrimSpace(file.FlatNodeText(argNode))
						}
					}
				}
				parenIdx := strings.LastIndex(innerCondText, "()")
				if parenIdx < 0 {
					return
				}
				beforeParen := innerCondText[:parenIdx]
				dotIdx := strings.LastIndex(beforeParen, ".")
				if dotIdx < 0 {
					return
				}
				receiver := beforeParen[:dotIdx]
				methodName := beforeParen[dotIdx+1:]
				info, ok := ifEmptyOrBlankMethods[methodName]
				if !ok {
					return
				}
				negated := info.negated != isNegatedPrefix
				var selfBranch, defaultBranch uint32
				if negated {
					selfBranch = thenNode
					defaultBranch = elseNode
				} else {
					selfBranch = elseNode
					defaultBranch = thenNode
				}
				selfText := strings.TrimSpace(file.FlatNodeText(selfBranch))
				if strings.HasPrefix(selfText, "{") && strings.HasSuffix(selfText, "}") {
					selfText = strings.TrimSpace(selfText[1 : len(selfText)-1])
				}
				if selfText != receiver {
					return
				}
				defaultText := strings.TrimSpace(file.FlatNodeText(defaultBranch))
				if strings.HasPrefix(defaultText, "{") && strings.HasSuffix(defaultText, "}") {
					defaultText = strings.TrimSpace(defaultText[1 : len(defaultText)-1])
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Use '.%s {}' instead of manual %s() check.", info.replacement, methodName))
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiver + "." + info.replacement + " { " + defaultText + " }"}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ExplicitCollectionElementAccessMethodRule{BaseRule: BaseRule{RuleName: "ExplicitCollectionElementAccessMethod", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit .get() and .set() calls that should use index operator syntax."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var navNode uint32
				var methodName string
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "navigation_expression" {
						navText := file.FlatNodeText(child)
						if strings.HasSuffix(navText, ".get") {
							navNode = child
							methodName = "get"
							break
						}
						if strings.HasSuffix(navText, ".set") {
							navNode = child
							methodName = "set"
							break
						}
					}
				}
				if navNode == 0 {
					return
				}
				argsNode := uint32(0)
				if callSuffix, ok := file.FlatFindChild(idx, "call_suffix"); ok {
					argsNode, _ = file.FlatFindChild(callSuffix, "value_arguments")
				}
				if argsNode == 0 {
					return
				}
				argCount := 0
				for i := 0; i < file.FlatChildCount(argsNode); i++ {
					if file.FlatType(file.FlatChild(argsNode, i)) == "value_argument" {
						argCount++
					}
				}
				if methodName == "get" && argCount < 1 {
					return
				}
				if methodName == "set" && argCount < 2 {
					return
				}
				row := file.FlatRow(idx) + 1
				col := file.FlatCol(idx) + 1
				var argTexts []string
				for i := 0; i < file.FlatChildCount(argsNode); i++ {
					child := file.FlatChild(argsNode, i)
					if file.FlatType(child) == "value_argument" {
						argTexts = append(argTexts, file.FlatNodeText(child))
					}
				}
				receiver := file.FlatChild(navNode, 0)
				if receiver == 0 {
					return
				}
				receiverText := file.FlatNodeText(receiver)
				if methodName == "get" {
					f := r.Finding(file, row, col, "Use index operator instead of explicit 'get' call.")
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + strings.Join(argTexts, ", ") + "]"}
					ctx.Emit(f)
					return
				}
				keys := strings.Join(argTexts[:len(argTexts)-1], ", ")
				value := argTexts[len(argTexts)-1]
				f := r.Finding(file, row, col, "Use index operator instead of explicit 'set' call.")
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + keys + "] = " + value}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &AlsoCouldBeApplyRule{BaseRule: BaseRule{RuleName: "AlsoCouldBeApply", RuleSetName: "style", Sev: "warning", Desc: "Detects .also {} blocks with multiple it. references that could use .apply {} instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "also" {
					return
				}
				lambda := flatCallTrailingLambda(file, idx)
				if lambda == 0 {
					return
				}
				// Count `it.x` dereferences: a simple_identifier `it` whose
				// NEXT named sibling is a navigation_suffix. This catches
				// both `navigation_expression` (`it.name` read) and
				// `directly_assignable_expression` (`it.name = …` LHS)
				// forms. Two or more means "apply" would read more
				// naturally than "also".
				itDotCount := 0
				file.FlatWalkNodes(lambda, "simple_identifier", func(ident uint32) {
					if file.FlatNodeText(ident) != "it" {
						return
					}
					next := file.FlatNextSib(ident)
					for next != 0 && !file.FlatIsNamed(next) {
						next = file.FlatNextSib(next)
					}
					if next != 0 && file.FlatType(next) == "navigation_suffix" {
						itDotCount++
					}
				})
				if itDotCount >= 2 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "'also' with multiple 'it.' references could be replaced with 'apply'.")
				}
			},
		})
	}
	{
		r := &EqualsNullCallRule{BaseRule: BaseRule{RuleName: "EqualsNullCall", RuleSetName: "style", Sev: "warning", Desc: "Detects .equals(null) calls that should use == null instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var navNode uint32
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "navigation_expression" && strings.HasSuffix(file.FlatNodeText(child), ".equals") {
						navNode = child
						break
					}
				}
				if navNode == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if (file.FlatType(child) == "call_suffix" || file.FlatType(child) == "value_arguments") && strings.Contains(file.FlatNodeText(child), "null") {
						f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use '== null' instead of '.equals(null)'.")
						navText := file.FlatNodeText(navNode)
						if dotIdx := strings.LastIndex(navText, ".equals"); dotIdx >= 0 {
							f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(navNode)) + dotIdx, EndByte: int(file.FlatEndByte(idx)), Replacement: " == null"}
						}
						ctx.Emit(f)
						return
					}
				}
			},
		})
	}
}
