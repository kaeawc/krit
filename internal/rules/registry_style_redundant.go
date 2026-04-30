package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	"strings"
)

func registerStyleRedundantRules() {

	// --- from style_redundant.go ---
	{
		r := &RedundantVisibilityModifierRule{BaseRule: BaseRule{RuleName: "RedundantVisibilityModifier", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit public modifier which is redundant since public is the default visibility in Kotlin."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"modifiers"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Check for "public" and absence of "override" using AST child walking.
				// This node IS a "modifiers" node, so walk its children directly.
				hasPublic := false
				hasOverride := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					childText := file.FlatNodeText(child)
					if childText == "public" {
						hasPublic = true
					}
					if childText == "override" {
						hasOverride = true
					}
					// Modifier keywords may be wrapped (e.g. visibility_modifier > "public")
					for j := 0; j < file.FlatChildCount(child); j++ {
						gcText := file.FlatNodeText(file.FlatChild(child, j))
						if gcText == "public" {
							hasPublic = true
						}
						if gcText == "override" {
							hasOverride = true
						}
					}
				}
				if hasPublic && !hasOverride {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Redundant 'public' modifier. Public is the default visibility in Kotlin.")
					// Find the visibility_modifier child with "public"
					for i := 0; i < file.FlatChildCount(idx); i++ {
						child := file.FlatChild(idx, i)
						if file.FlatType(child) == "visibility_modifier" && file.FlatNodeTextEquals(child, "public") {
							startByte := int(file.FlatStartByte(child))
							endByte := int(file.FlatEndByte(child))
							// Also consume trailing whitespace
							for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
								endByte++
							}
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   startByte,
								EndByte:     endByte,
								Replacement: "",
							}
							break
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &RedundantConstructorKeywordRule{BaseRule: BaseRule{RuleName: "RedundantConstructorKeyword", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary constructor keyword on primary constructors without annotations or visibility modifiers."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				if ctor == 0 {
					return
				}
				if file.FlatHasChildOfType(ctor, "modifiers") {
					return
				}
				// Check whether the constructor text contains the explicit "constructor" keyword.
				ctorText := file.FlatNodeText(ctor)
				if !strings.Contains(ctorText, "constructor") {
					return
				}
				f := r.Finding(file, file.FlatRow(ctor)+1, 1,
					"Redundant 'constructor' keyword. Remove it when there are no annotations or visibility modifiers.")
				// Auto-fix: remove " constructor" keeping only the parameter list.
				// Walk back from constructor start to consume preceding whitespace.
				startByte := int(file.FlatStartByte(ctor))
				for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '	') {
					startByte--
				}
				// Find the parameter list (class_parameters) inside the constructor node.
				paramList, _ := file.FlatFindChild(ctor, "class_parameters")
				if paramList != 0 {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
						EndByte:     int(file.FlatStartByte(paramList)),
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &RedundantExplicitTypeRule{BaseRule: BaseRule{RuleName: "RedundantExplicitType", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit type annotations that can be inferred from the initializer."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Needs: v2.NeedsResolver, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Must have both an explicit type annotation and an initializer
				var typeNode uint32
				var initExpr uint32
				hasEquals := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "user_type", "nullable_type":
						typeNode = child
					case "=":
						hasEquals = true
					default:
						if hasEquals && initExpr == 0 && file.FlatType(child) != "property_delegate" {
							initExpr = child
						}
					}
					// Also check inside variable_declaration
					if file.FlatType(child) == "variable_declaration" {
						for j := 0; j < file.FlatChildCount(child); j++ {
							gc := file.FlatChild(child, j)
							if t := file.FlatType(gc); t == "user_type" || t == "nullable_type" {
								typeNode = gc
							}
						}
					}
				}
				if typeNode == 0 || initExpr == 0 {
					return
				}
				// --- Resolver-based matching (preferred) ---
				if ctx.Resolver != nil {
					declaredType := ctx.Resolver.ResolveFlatNode(typeNode, file)
					inferredType := ctx.Resolver.ResolveFlatNode(initExpr, file)
					if declaredType.Kind != typeinfer.TypeUnknown && inferredType.Kind != typeinfer.TypeUnknown {
						match := false
						if declaredType.FQN != "" && inferredType.FQN != "" {
							match = declaredType.FQN == inferredType.FQN && declaredType.Nullable == inferredType.Nullable
						} else if declaredType.Name != "" && inferredType.Name != "" {
							match = declaredType.Name == inferredType.Name && declaredType.Nullable == inferredType.Nullable
						}
						if match {
							f := r.Finding(file, file.FlatRow(idx)+1, 1,
								"Redundant explicit type. Type can be inferred from the initializer.")
							f.Fix = r.buildFixFlat(typeNode, file)
							ctx.Emit(f)
						}
						return
					}
					// Fall through to literal matching if resolver returned unknown
				}
				// --- Fallback: literal pattern matching via AST nodes ---
				typeText := file.FlatNodeText(typeNode)
				initType := file.FlatType(initExpr)
				initText := file.FlatNodeText(initExpr)
				matched := false
				switch initType {
				case "string_literal":
					matched = typeText == "String"
				case "boolean_literal":
					matched = typeText == "Boolean"
				case "character_literal":
					matched = typeText == "Char"
				case "integer_literal":
					if strings.HasSuffix(initText, "L") || strings.HasSuffix(initText, "l") {
						matched = typeText == "Long"
					} else {
						matched = typeText == "Int"
					}
				case "real_literal":
					if strings.HasSuffix(initText, "f") || strings.HasSuffix(initText, "F") {
						matched = typeText == "Float"
					} else {
						matched = typeText == "Double"
					}
				case "call_expression":
					// val x: Foo = Foo(...) — constructor call matches type name
					callee, _ := file.FlatFindChild(initExpr, "simple_identifier")
					if callee != 0 && file.FlatNodeTextEquals(callee, typeText) {
						matched = true
					}
				case "simple_identifier":
					// val x: Foo = SomeRef — only match if name reference equals type text
					if initText == typeText {
						matched = true
					}
				}
				if matched {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Redundant explicit type. Type can be inferred from the initializer.")
					f.Fix = r.buildFixFlat(typeNode, file)
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &UnnecessaryParenthesesRule{BaseRule: BaseRule{RuleName: "UnnecessaryParentheses", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary parentheses around expressions that add no value."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"parenthesized_expression"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				parent, ok := file.FlatParent(idx)
				if !ok {
					return
				}
				// Find the inner expression (skip "(" and ")" tokens).
				var inner uint32
				for i := 0; i < file.FlatNamedChildCount(idx); i++ {
					child := file.FlatNamedChild(idx, i)
					if child != 0 {
						inner = child
						break
					}
				}
				if inner == 0 {
					return
				}
				parentType := file.FlatType(parent)
				// Never flag parens inside delegated_super_type (matches detekt).
				if parentType == "delegation_specifier" || parentType == "delegated_super_type" {
					return
				}
				redundant := false
				switch parentType {
				case "jump_expression":
					// return (x), throw (x) — parens always unnecessary.
					redundant = true
				case "parenthesized_expression":
					// Double parens: ((x)) — inner parens always unnecessary.
					redundant = true
				case "property_declaration", "variable_declaration":
					// val x = (expr) — parens unnecessary around entire RHS.
					redundant = true
				case "assignment":
					// x = (expr) — parens unnecessary around entire RHS.
					redundant = true
				case "value_argument", "value_arguments":
					// foo((expr)) — parens unnecessary around a single argument
					// unless it's a lambda (parenthesized lambda prevents trailing lambda syntax).
					if t := file.FlatType(inner); t == "lambda_literal" || t == "annotated_lambda" {
						redundant = false
					} else {
						redundant = true
					}
				case "if_expression":
					// The condition of an `if` is already wrapped in parens by syntax.
					// if ((x > 0)) — the inner parenthesized_expression is redundant.
					redundant = unnParensIsIfConditionFlat(file, idx, parent)
				case "when_expression":
					// when ((x)) — parens around the subject are unnecessary.
					redundant = unnParensIsWhenSubjectFlat(file, idx, parent)
				case "when_condition":
					// Parens inside a when condition: when (x) { (0) -> ... }
					redundant = true
				case "indexing_expression":
					// a[(expr)] — parens around index are unnecessary.
					redundant = true
				case "statements":
					// Top-level expression statement: (expr) on its own line.
					redundant = true
				default:
					// For other contexts, parens are redundant only if the inner
					// expression is a simple identifier, literal, string, or already
					// grouped (call_expression, navigation_expression, etc.) — i.e.,
					// removing the parens won't change precedence.
					redundant = unnParensInnerIsSafeFlat(file, inner)
				}
				if !redundant {
					return
				}
				// If AllowForUnclearPrecedence is set, keep parens that clarify
				// operator precedence (inner is binary op with a binary-op parent).
				if r.AllowForUnclearPrecedence && unnParensClarifyPrecedenceFlat(file, idx, inner) {
					return
				}
				innerText := file.FlatNodeText(inner)
				nodeText := file.FlatNodeText(idx)
				msg := fmt.Sprintf("Unnecessary parentheses in %s. Can be replaced with: %s", nodeText, innerText)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				// Auto-fix: replace the parenthesized_expression bytes with the inner expression text.
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: innerText,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryInheritanceRule{BaseRule: BaseRule{RuleName: "UnnecessaryInheritance", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary explicit inheritance from Any which is implicit in Kotlin."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Look for delegation_specifier children that are `: Any()`
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) != "delegation_specifier" {
						continue
					}
					text := file.FlatNodeText(child)
					if text != "Any()" {
						continue
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Unnecessary inheritance from 'Any'. All classes extend Any implicitly.")
					// Remove the `: Any()` portion — find the colon before the delegation_specifier
					startByte := int(file.FlatStartByte(child))
					endByte := int(file.FlatEndByte(child))
					// Walk backwards from the delegation_specifier to remove the `: ` prefix
					for sb := startByte - 1; sb >= 0; sb-- {
						ch := file.Content[sb]
						if ch == ':' {
							startByte = sb
							break
						}
						if ch != ' ' && ch != '	' {
							break
						}
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
						EndByte:     endByte,
						Replacement: "",
					}
					ctx.Emit(f)
					return
				}
			},
		})
	}
	{
		r := &UnnecessaryInnerClassRule{BaseRule: BaseRule{RuleName: "UnnecessaryInnerClass", RuleSetName: "style", Sev: "warning", Desc: "Detects inner classes that do not reference the outer class and could remove the inner modifier."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				mods, _ := file.FlatFindChild(idx, "modifiers")
				body, _ := file.FlatFindChild(idx, "class_body")
				if mods == 0 || body == 0 {
					return
				}
				// Verify the "inner" modifier is present.
				if !strings.Contains(file.FlatNodeText(mods), "inner") {
					return
				}
				bodyText := file.FlatNodeText(body)
				// Check if the body references this@OuterClass or the outer class's members
				if !strings.Contains(bodyText, "this@") && !strings.Contains(bodyText, "@") {
					name := extractIdentifierFlat(file, idx)
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Inner class '%s' does not use the outer class reference. Remove 'inner' modifier.", name))
					modsText := file.FlatNodeText(mods)
					newMods := strings.Replace(modsText, "inner ", "", 1)
					if newMods == modsText {
						newMods = strings.Replace(modsText, "inner", "", 1)
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(mods)),
						EndByte:     int(file.FlatEndByte(mods)),
						Replacement: newMods,
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &OptionalUnitRule{BaseRule: BaseRule{RuleName: "OptionalUnit", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit Unit return types and return Unit statements that are redundant."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// 1. Check for explicit `: Unit` return type annotation.
				// In the tree-sitter AST, function_declaration children include a ":"
				// token followed by a user_type node when a return type is specified.
				colonIdx := -1
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == ":" {
						colonIdx = i
					}
					if colonIdx >= 0 && file.FlatType(child) == "user_type" {
						typeText := file.FlatNodeText(child)
						if typeText == "Unit" {
							f := r.Finding(file, file.FlatRow(child)+1,
								file.FlatCol(child)+1,
								"Unit return type is optional and can be omitted.")
							// Remove ": Unit" including the colon and any surrounding whitespace
							colonNode := file.FlatChild(idx, colonIdx)
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(colonNode)),
								EndByte:     int(file.FlatEndByte(child)),
								Replacement: "",
							}
							ctx.Emit(f)
						}
						break
					}
				}
				// 2. Check for `return Unit` statements inside the function body using compiled query.
				body, _ := file.FlatFindChild(idx, "function_body")
				if body != 0 {
					file.FlatWalkNodes(body, "jump_expression", func(jump uint32) {
						if file.FlatChildCount(jump) < 2 {
							return
						}
						first := file.FlatChild(jump, 0)
						if file.FlatType(first) != "return" {
							return
						}
						second := file.FlatChild(jump, 1)
						if file.FlatType(second) == "simple_identifier" && file.FlatNodeTextEquals(second, "Unit") {
							f := r.Finding(file, file.FlatRow(jump)+1,
								file.FlatCol(jump)+1,
								"return Unit is redundant and can be replaced with return.")
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatEndByte(first)),
								EndByte:     int(file.FlatEndByte(second)),
								Replacement: "",
							}
							ctx.Emit(f)
						}
					})
				}
			},
		})
	}
	{
		r := &UnnecessaryBackticksRule{BaseRule: BaseRule{RuleName: "UnnecessaryBackticks", RuleSetName: "style", Sev: "warning", Desc: "Detects backtick-quoted identifiers that do not require backticks."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"simple_identifier"}, Confidence: 0.75, Fix: v2.FixCosmetic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if len(text) < 3 || text[0] != '`' || text[len(text)-1] != '`' {
					return
				}
				inner := text[1 : len(text)-1]
				// Backticks are needed for keywords and all-underscore identifiers.
				if isKotlinKeyword(inner) || isAllUnderscores(inner) {
					return
				}
				// Must be a valid Kotlin identifier without backticks.
				if !isValidKotlinIdentifier(inner) {
					return
				}
				// Inside a string template, removing backticks may merge with adjacent text.
				// e.g. "$`foo`bar" — removing backticks yields "$foobar" (different meaning).
				endByte := int(file.FlatEndByte(idx))
				if endByte < len(file.Content) && isInsideStringTemplateFlat(file, idx) {
					nextCh := file.Content[endByte]
					if isIdentChar(nextCh) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary backticks around '%s'.", inner))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     endByte,
					Replacement: inner,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UselessCallOnNotNullRule{BaseRule: BaseRule{RuleName: "UselessCallOnNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects calls like .orEmpty() or .isNullOrEmpty() on receivers that are already non-null."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Needs: v2.NeedsResolver, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) != "call_expression" {
					return
				}
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr != 0 {
					methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
					replacement, isUseless := uselessNullCalls[methodName]
					if isUseless {
						receiverNode := file.FlatNamedChild(navExpr, 0)
						if receiverNode != 0 {
							nonNull := false
							recType := file.FlatType(receiverNode)
							if recType == "string_literal" || recType == "line_string_literal" || recType == "multi_line_string_literal" {
								nonNull = true
							} else if recType == "call_expression" {
								callText := file.FlatNodeText(receiverNode)
								for _, prefix := range nonNullFactoryCalls {
									if strings.HasPrefix(callText, prefix) {
										nonNull = true
										break
									}
								}
							}
							recText := file.FlatNodeText(receiverNode)
							if strings.Contains(recText, "?.") {
								return
							}
							if !nonNull && ctx.Resolver != nil {
								resolved := ctx.Resolver.ResolveFlatNode(receiverNode, file)
								if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
									validTypes := orEmptyValidTypes
									if methodName == "isNullOrEmpty" || methodName == "isNullOrBlank" {
										validTypes = isNullOrValidTypes
									}
									if !validTypes[resolved.Name] {
										return
									}
									if !resolved.IsNullable() {
										nonNull = true
									}
								}
							}
							if nonNull {
								var msg string
								if replacement == "" {
									msg = fmt.Sprintf("Useless call to %s on non-null type. The value is already non-null.", methodName)
								} else {
									msg = fmt.Sprintf("Replace %s with %s — the receiver is non-null.", methodName, replacement)
								}
								f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
								if replacement == "" {
									if start, _, ok := flatCallExpressionMethodSpan(file, idx, methodName); ok {
										f.Fix = &scanner.Fix{
											ByteMode:    true,
											StartByte:   start - 1,
											EndByte:     int(file.FlatEndByte(idx)),
											Replacement: "",
										}
									}
								} else if start, end, ok := flatCallExpressionMethodSpan(file, idx, methodName); ok {
									f.Fix = &scanner.Fix{
										ByteMode:    true,
										StartByte:   start,
										EndByte:     end,
										Replacement: replacement,
									}
								}
								ctx.Emit(f)
								return
							}
						}
					}
				}
				if args != 0 && ctx.Resolver != nil {
					calleeName := flatCallExpressionName(file, idx)
					replacementName, ok := ofNotNullReplacements[calleeName]
					if !ok {
						return
					}
					allNonNull := true
					argCount := 0
					for i := 0; i < file.FlatChildCount(args); i++ {
						va := file.FlatChild(args, i)
						if file.FlatType(va) != "value_argument" {
							continue
						}
						argCount++
						expr := flatValueArgumentExpression(file, va)
						if expr == 0 {
							allNonNull = false
							break
						}
						exprText := file.FlatNodeText(expr)
						if file.FlatType(expr) == "spread_expression" || strings.Contains(exprText, "?.") || file.FlatType(expr) == "navigation_expression" || containsNullableStdlibCall(exprText) {
							allNonNull = false
							break
						}
						resolved := ctx.Resolver.ResolveFlatNode(expr, file)
						if resolved == nil || resolved.Kind == typeinfer.TypeUnknown || resolved.IsNullable() {
							allNonNull = false
							break
						}
					}
					if allNonNull && argCount > 0 {
						f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Replace %s with %s — all arguments are non-null.", calleeName, replacementName))
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: replacementName,
						}
						ctx.Emit(f)
					}
				}
			},
		})
	}
}
