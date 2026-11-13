package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerStyleRedundantRules() {
	registerStyleRedundantVisibilityModifier()
	registerStyleRedundantConstructorKeyword()
	registerStyleRedundantExplicitType()
	registerStyleUnnecessaryParentheses()
	registerStyleUnnecessaryInheritance()
	registerStyleUnnecessaryInnerClass()
	registerStyleOptionalUnit()
	registerStyleUnnecessaryBackticks()
	registerStyleUnnecessaryAnnotationUseSiteTarget()
	registerStyleUselessCallOnNotNull()
}

func registerStyleRedundantVisibilityModifier() {
	r := &RedundantVisibilityModifierRule{BaseRule: BaseRule{RuleName: "RedundantVisibilityModifier", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit public modifier which is redundant since public is the default visibility in Kotlin."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"modifiers"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: r.check,
	})
}

func registerStyleRedundantConstructorKeyword() {
	r := &RedundantConstructorKeywordRule{BaseRule: BaseRule{RuleName: "RedundantConstructorKeyword", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary constructor keyword on primary constructors without annotations or visibility modifiers."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
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
			f := r.Finding(file, file.FlatRow(ctor)+1, file.FlatCol(ctor)+1,
				"Redundant 'constructor' keyword. Remove it when there are no annotations or visibility modifiers.")
			// Auto-fix: remove " constructor" keeping only the parameter list.
			// Walk back from constructor start to consume preceding whitespace.
			// Line breaks go too, so `class A\nconstructor(` becomes `class A(` instead of a dangling `(`.
			startByte := int(file.FlatStartByte(ctor))
			for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t' ||
				file.Content[startByte-1] == '\n' || file.Content[startByte-1] == '\r') {
				startByte--
			}
			// Locate the `(` that opens the constructor parameter list.
			// The Kotlin tree-sitter grammar emits the parameters as
			// individual `class_parameter` children alongside the `(`,
			// `,`, and `)` token children — there is no
			// `class_parameters` wrapper to find.
			parenStart := 0
			for c := file.FlatFirstChild(ctor); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "(" {
					parenStart = int(file.FlatStartByte(c))
					break
				}
			}
			if parenStart > startByte {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     parenStart,
					Replacement: "",
				}
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleRedundantExplicitType() {
	r := &RedundantExplicitTypeRule{BaseRule: BaseRule{RuleName: "RedundantExplicitType", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit type annotations that can be inferred from the initializer."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceMedium, Needs: api.NeedsResolver, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// detekt checks local variables only; a property of a class or file states its type for readers and ABI.
			if p, ok := file.FlatParent(idx); !ok || file.FlatType(p) != "statements" {
				return
			}
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
			// detekt only looks at literals, a constructor call of the stated type and a reference to an object.
			if !redundantTypeInitializerKind(file, initExpr, typeNode) {
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
					// Dropping `List<Any>` or `MutableList<T>` changes or breaks inference unless the arguments agree.
					if match && strings.Contains(file.FlatNodeText(typeNode), "<") {
						match = resolvedTypeArgsEqual(declaredType, inferredType)
					}
					if match {
						f := r.Finding(file, declLine(file, idx), declCol(file, idx),
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
			case "integer_literal", "hex_literal", "bin_literal":
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
				f := r.Finding(file, declLine(file, idx), declCol(file, idx),
					"Redundant explicit type. Type can be inferred from the initializer.")
				f.Fix = r.buildFixFlat(typeNode, file)
				ctx.Emit(f)
			}
		},
	})
}

// redundantTypeInitializerKind reports whether the initializer is one detekt's RedundantExplicitType examines: a
// literal, a call to a bare name equal to the stated type, or a bare name equal to the stated type.
func redundantTypeInitializerKind(file *scanner.File, init, typeNode uint32) bool {
	switch file.FlatType(init) {
	case "string_literal", "boolean_literal", "character_literal", "integer_literal", "long_literal", "real_literal",
		"hex_literal", "bin_literal":
		return true
	case "call_expression":
		callee := file.FlatFirstChild(init)
		if callee == 0 || file.FlatType(callee) != "simple_identifier" || !file.FlatNodeTextEquals(callee, file.FlatNodeText(typeNode)) {
			return false
		}
		return !flatSubtreeHasNodeType(file, init, "type_arguments")
	case "simple_identifier":
		return file.FlatNodeTextEquals(init, file.FlatNodeText(typeNode))
	}
	return false
}

func registerStyleUnnecessaryParentheses() {
	r := &UnnecessaryParenthesesRule{BaseRule: BaseRule{RuleName: "UnnecessaryParentheses", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary parentheses around expressions that add no value."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"parenthesized_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			parent, ok := file.FlatParent(idx)
			if !ok {
				return
			}
			inner := parenInnerExpression(file, idx)
			if inner == 0 {
				return
			}
			parentType := file.FlatType(parent)
			// The grammar splits `@Suppress("X")` before an assignment into an annotation plus a parenthesized
			// expression; those parentheses are the annotation's argument list.
			if prev, ok := file.FlatPrevSibling(idx); ok && file.FlatType(prev) == "annotation" && file.FlatEndByte(prev) == file.FlatStartByte(idx) {
				return
			}
			// Never flag parens inside delegated_super_type.
			if parentType == "delegation_specifier" || parentType == "delegated_super_type" {
				return
			}
			// For nested parens like ((x)), detekt reports both pairs but the outer fix removes them in one pass.
			// A fix on the inner pair would overlap it, and the conflict resolver would drop the outer one
			// (see TestFixableFixturesIdempotent in fix_idempotency_test.go), so only the outermost has a fix.
			nested := parentType == "parenthesized_expression"
			redundant := !parenthesesNecessary(file, idx, inner)
			if !redundant {
				return
			}
			// If AllowForUnclearPrecedence is set, keep parens that clarify
			// operator precedence (inner is binary op with a binary-op parent).
			if r.AllowForUnclearPrecedence && parenPrecedenceUnclear(file, idx, inner) {
				return
			}
			// For the replacement, descend through any nested
			// parenthesized_expression chain to the deepest non-paren
			// expression. This makes ((x)) -> x converge in one pass.
			// Redundancy/lambda checks above still use the immediate
			// child so that, e.g., f((lambda)) preserves today's
			// semantics around lambdas in value_argument context.
			replacementNode := inner
			for file.FlatType(replacementNode) == "parenthesized_expression" {
				var next uint32
				for i := 0; i < file.FlatNamedChildCount(replacementNode); i++ {
					c := file.FlatNamedChild(replacementNode, i)
					if c != 0 {
						next = c
						break
					}
				}
				if next == 0 {
					break
				}
				replacementNode = next
			}
			innerText := file.FlatNodeText(replacementNode)
			nodeText := file.FlatNodeText(idx)
			msg := fmt.Sprintf("Parentheses in %s are unnecessary and can be replaced with: %s", nodeText, innerText)
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
			// Auto-fix: replace the parenthesized_expression bytes with the inner expression text.
			if !nested {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: innerText,
				}
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleUnnecessaryInheritance() {
	r := &UnnecessaryInheritanceRule{BaseRule: BaseRule{RuleName: "UnnecessaryInheritance", RuleSetName: "style", Sev: "warning", Desc: "Detects unnecessary explicit inheritance from Any which is implicit in Kotlin."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration", "object_declaration", "object_literal"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Look for a delegation_specifier that is `Any()`, anywhere in the supertype list.
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "delegation_specifier" || file.FlatNodeText(child) != "Any()" {
					continue
				}
				row, col := declLine(file, idx), declCol(file, idx)
				if file.FlatType(idx) == "object_literal" {
					row, col = file.FlatRow(idx)+1, file.FlatCol(idx)+1
				}
				f := r.Finding(file, row, col, "Unnecessary inheritance of 'Any'.")
				// Remove the `Any()` entry together with the colon or comma that joins it.
				startByte := int(file.FlatStartByte(child))
				endByte := int(file.FlatEndByte(child))
				others := 0
				for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
					if c != child && file.FlatType(c) == "delegation_specifier" {
						others++
					}
				}
				if others == 0 {
					for sb := startByte - 1; sb >= 0; sb-- {
						ch := file.Content[sb]
						if ch == ':' {
							startByte = sb
							break
						}
						if ch != ' ' && ch != '\t' {
							break
						}
					}
					// `object : Any() { }` keeps one space before the body.
					if file.FlatType(idx) == "object_literal" {
						for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
							endByte++
						}
						for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
							startByte--
						}
						f.Fix = &scanner.Fix{ByteMode: true, StartByte: startByte, EndByte: endByte, Replacement: " "}
						ctx.Emit(f)
						return
					}
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: startByte, EndByte: endByte, Replacement: ""}
					ctx.Emit(f)
					return
				}
				// Several supertypes: leave the rewrite to the author.
				ctx.Emit(f)
				return
			}
		},
	})
}

func registerStyleUnnecessaryInnerClass() {
	r := &UnnecessaryInnerClassRule{BaseRule: BaseRule{RuleName: "UnnecessaryInnerClass", RuleSetName: "style", Sev: "warning", Desc: "Detects inner classes that do not reference the outer class and could remove the inner modifier."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			innerNode := file.FlatFindModifierNode(idx, "inner")
			if innerNode == 0 || innerClassMayUseOuterInstance(file, idx) {
				return
			}
			name := extractIdentifierFlat(file, idx)
			f := r.Finding(file, declLine(file, idx), declCol(file, idx),
				fmt.Sprintf("Inner class '%s' does not use the outer class reference. Remove 'inner' modifier.", name))
			endByte := int(file.FlatEndByte(innerNode))
			for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
				endByte++
			}
			f.Fix = &scanner.Fix{
				ByteMode:  true,
				StartByte: int(file.FlatStartByte(innerNode)),
				EndByte:   endByte,
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleOptionalUnit() {
	r := &OptionalUnitRule{BaseRule: BaseRule{RuleName: "OptionalUnit", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit Unit return types and return Unit statements that are redundant."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration", "statements"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: r.check,
	})
}

func registerStyleUnnecessaryBackticks() {
	r := &UnnecessaryBackticksRule{BaseRule: BaseRule{RuleName: "UnnecessaryBackticks", RuleSetName: "style", Sev: "warning", Desc: "Detects backtick-quoted identifiers that do not require backticks."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"simple_identifier", "type_identifier"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			text := file.FlatNodeText(idx)
			if len(text) < 3 || text[0] != '`' || text[len(text)-1] != '`' {
				return
			}
			inner := text[1 : len(text)-1]
			if offsetInsideSourceComment(file, file.FlatStartByte(idx)) {
				return
			}
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
				"Backticks are unnecessary.")
			// Documentation that quotes the backticked name would be left stale, so leave those to a human.
			if !commentQuotesBackticked(file, inner) {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     endByte,
					Replacement: inner,
				}
			}
			ctx.Emit(f)
		},
	})
}

// commentQuotesBackticked reports whether any comment or KDoc in the file contains `name` in backticks.
func commentQuotesBackticked(file *scanner.File, name string) bool {
	quoted := "`" + name + "`"
	found := false
	for _, kind := range []string{"multiline_comment", "line_comment"} {
		file.FlatWalkNodes(0, kind, func(c uint32) {
			if !found && strings.Contains(file.FlatNodeText(c), quoted) {
				found = true
			}
		})
	}
	return found
}

func registerStyleUselessCallOnNotNull() {
	r := &UselessCallOnNotNullRule{BaseRule: BaseRule{RuleName: "UselessCallOnNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects calls like .orEmpty() or .isNullOrEmpty() on receivers that are already non-null."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Needs: api.NeedsResolver | api.NeedsOracleExprType, Fix: api.FixIdiomatic, Implementation: r,
		Check: func(ctx *api.Context) {
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
							resolved := uselessCallReceiverType(ctx, receiverNode)
							if resolved != nil && resolved.Resolved && resolved.Kind != typeinfer.TypeUnknown {
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
								msg = fmt.Sprintf("Remove redundant call to %s", methodName)
							} else {
								msg = fmt.Sprintf("Replace %s with %s", methodName, replacement)
							}
							f := r.Finding(file, callLine(file, idx), callCol(file, idx), msg)
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
				// A receiver means a user-defined member or extension, not the stdlib top-level function.
				if !ok || file.FlatType(file.FlatFirstChild(idx)) != "simple_identifier" {
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
					if file.FlatType(expr) == "simple_identifier" && !uselessOfNotNullIdentifierIsNonNull(file, file.FlatNodeText(expr)) {
						allNonNull = false
						break
					}
					resolved := ctx.Resolver.ResolveFlatNode(expr, file)
					if resolved == nil || !resolved.Resolved || resolved.Kind == typeinfer.TypeUnknown || resolved.IsNullable() {
						allNonNull = false
						break
					}
				}
				if allNonNull && argCount > 0 {
					f := r.Finding(file, callLine(file, idx), callCol(file, idx),
						fmt.Sprintf("Replace %s with %s", calleeName, replacementName))
					// Replace only the callee name so the arguments survive.
					if calleeNode := file.FlatFirstChild(idx); calleeNode != 0 {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(calleeNode)),
							EndByte:     int(file.FlatEndByte(calleeNode)),
							Replacement: replacementName,
						}
					}
					ctx.Emit(f)
				}
			}
		},
	})
}

// uselessCallReceiverType resolves the receiver's type. When the compiler analyzed the file only its fact
// counts, because the name-based fallback guesses wrong for properties declared in other modules.
func uselessCallReceiverType(ctx *api.Context, receiver uint32) *typeinfer.ResolvedType {
	// Detekt reads the declared type, so a name declared nullable is not useless even when smart-cast non-null here.
	if ctx.File.FlatType(receiver) == "simple_identifier" && identifierDeclaredNullable(ctx.File, receiver) {
		return nil
	}
	if !compilerAnalyzedFile(ctx) {
		return ctx.Resolver.ResolveFlatNode(receiver, ctx.File)
	}
	resolver, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return nil
	}
	return resolver.OracleExpressionFlat(ctx.File, receiver)
}

// resolvedTypeArgsEqual reports whether both types carry the same, fully known type arguments.
func resolvedTypeArgsEqual(a, b *typeinfer.ResolvedType) bool {
	if a == nil || b == nil || len(a.TypeArgs) == 0 || len(a.TypeArgs) != len(b.TypeArgs) {
		return false
	}
	for i := range a.TypeArgs {
		x, y := &a.TypeArgs[i], &b.TypeArgs[i]
		if x.Kind == typeinfer.TypeUnknown || y.Kind == typeinfer.TypeUnknown || x.Name != y.Name || x.Nullable != y.Nullable {
			return false
		}
		if len(x.TypeArgs) > 0 || len(y.TypeArgs) > 0 {
			if !resolvedTypeArgsEqual(x, y) {
				return false
			}
		}
	}
	return true
}

// uselessOfNotNullIdentifierIsNonNull reports whether a bare identifier argument is declared in this file and no
// declaration of that name has a nullable type. A name declared elsewhere (a member of another class, an implicit
// receiver property) cannot be checked here, so it counts as possibly null.
func uselessOfNotNullIdentifierIsNonNull(file *scanner.File, name string) bool {
	found, nullable := false, false
	for _, kind := range []string{"parameter", "class_parameter", "variable_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			ident, ok := file.FlatFindChild(decl, "simple_identifier")
			if !ok || !file.FlatNodeTextEquals(ident, name) {
				return
			}
			found = true
			if file.FlatHasChildOfType(decl, "nullable_type") {
				nullable = true
			}
		})
	}
	return found && !nullable
}

// identifierDeclaredNullable reports whether the function around the identifier declares a parameter or variable
// of that name with an explicit nullable type.
func identifierDeclaredNullable(file *scanner.File, ident uint32) bool {
	scope, ok := flatEnclosingAncestor(file, ident, "function_declaration")
	if !ok {
		return false
	}
	name := file.FlatNodeText(ident)
	nullable := false
	for _, kind := range []string{"parameter", "variable_declaration"} {
		file.FlatWalkNodes(scope, kind, func(decl uint32) {
			if nullable || !file.FlatHasChildOfType(decl, "nullable_type") {
				return
			}
			if id, ok := file.FlatFindChild(decl, "simple_identifier"); ok && file.FlatNodeTextEquals(id, name) {
				nullable = true
			}
		})
	}
	return nullable
}
