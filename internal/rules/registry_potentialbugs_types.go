package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	"strings"
)

func registerPotentialbugsTypesRules() {

	// --- from potentialbugs_types.go ---
	{
		r := &AvoidReferentialEqualityRule{BaseRule: BaseRule{RuleName: "AvoidReferentialEquality", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of referential equality operators === or !== which compare object identity instead of value."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				left, op, right := equalityOperands(file, idx)
				if op == 0 {
					return
				}
				opType := file.FlatType(op)
				if opType != "===" && opType != "!==" {
					return
				}
				if left == 0 || right == 0 {
					return
				}
				// Skip null checks (referential against null is intentional).
				if file.FlatType(left) == "null" || file.FlatType(right) == "null" {
					return
				}
				// `this === other` inside equals() is idiomatic identity shortcut.
				if file.FlatType(left) == "this_expression" && isInsideEqualsMethodFlatType(file, idx) {
					return
				}
				// Under a boolean expression that already calls an equals-family
				// method, the referential compare is the short-circuit fast path.
				if enclosingBoolExprHasEqualsCall(file, idx, equalsFamilyCallNames) {
					return
				}
				// Enum constants — the identity compare is correct.
				if looksLikeEnumConstantRef(file.FlatNodeText(left)) || looksLikeEnumConstantRef(file.FlatNodeText(right)) {
					return
				}
				if ctx.Resolver != nil {
					leftType := ctx.Resolver.ResolveFlatNode(left, file)
					rightType := ctx.Resolver.ResolveFlatNode(right, file)
					leftKnown := leftType != nil && leftType.Kind != typeinfer.TypeUnknown
					rightKnown := rightType != nil && rightType.Kind != typeinfer.TypeUnknown
					if (leftKnown || rightKnown) && !typeinfer.IsKnownValueType(leftType) && !typeinfer.IsKnownValueType(rightType) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Referential equality (===, !==) should be avoided. Use structural equality (==, !=) instead.")
				repl := "=="
				if opType == "!==" {
					repl = "!="
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(op)),
					EndByte:     int(file.FlatEndByte(op)),
					Replacement: repl,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &DoubleMutabilityForCollectionRule{BaseRule: BaseRule{RuleName: "DoubleMutabilityForCollection", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects var declarations with mutable collection types, creating double mutability."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if !propertyDeclarationIsVar(file, idx) {
					return
				}
				// text is still used downstream for the resolver-less
				// mutable factory check.
				text := file.FlatNodeText(idx)
				mutableTypes := r.configuredMutableTypes()
				varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
				if varDecl == 0 {
					return
				}
				if !doubleMutabilityHasExplicitMutableType(file, varDecl, mutableTypes) {
					if !initializerLooksLikeMutableFactory(text) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Variable with mutable collection type creates double mutability. Use val with a mutable collection or var with an immutable collection.")
				var varKeyword uint32
				file.FlatForEachChild(idx, func(ch uint32) {
					if file.FlatNodeTextEquals(ch, "var") {
						varKeyword = ch
					}
				})
				if varKeyword != 0 {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(varKeyword)),
						EndByte:     int(file.FlatEndByte(varKeyword)),
						Replacement: "val",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EqualsAlwaysReturnsTrueOrFalseRule{BaseRule: BaseRule{RuleName: "EqualsAlwaysReturnsTrueOrFalse", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects equals() implementations that always return true or always return false."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "equals" {
					return
				}
				if !file.FlatHasModifier(idx, "override") {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				bodyText := file.FlatNodeText(body)
				trimmed := strings.TrimSpace(bodyText)
				if trimmed == "= true" || trimmed == "= false" {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"equals() always returns the same value. This is likely a bug.")
					return
				}
				allTrue := true
				allFalse := true
				returnCount := 0
				file.FlatWalkNodes(body, "jump_expression", func(jmp uint32) {
					jmpText := strings.TrimSpace(file.FlatNodeText(jmp))
					if !strings.HasPrefix(jmpText, "return") {
						return
					}
					returnCount++
					val := strings.TrimSpace(strings.TrimPrefix(jmpText, "return"))
					if val != "true" {
						allTrue = false
					}
					if val != "false" {
						allFalse = false
					}
				})
				if returnCount > 0 && (allTrue || allFalse) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"equals() always returns the same value. This is likely a bug.")
				}
			},
		})
	}
	{
		r := &EqualsWithHashCodeExistRule{BaseRule: BaseRule{RuleName: "EqualsWithHashCodeExist", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects classes that override equals() without hashCode() or vice versa."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				body, _ := file.FlatFindChild(idx, "class_body")
				if body == 0 {
					return
				}
				hasEquals := false
				hasHashCode := false
				walkFlatClassMembers(file, body, func(child uint32) {
					if hasEquals && hasHashCode {
						return
					}
					if file.FlatType(child) != "function_declaration" {
						return
					}
					if !file.FlatHasModifier(child, "override") {
						return
					}
					name := extractIdentifierFlat(file, child)
					if name == "" {
						return
					}
					switch name {
					case "equals":
						hasEquals = true
					case "hashCode":
						hasHashCode = true
					}
				})
				if hasEquals && !hasHashCode {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Class overrides equals() but not hashCode().")
				} else if !hasEquals && hasHashCode {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Class overrides hashCode() but not equals().")
				}
			},
		})
	}
	{
		r := &WrongEqualsTypeParameterRule{BaseRule: BaseRule{RuleName: "WrongEqualsTypeParameter", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects equals() with a parameter type other than Any?, which does not properly override the contract."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.85, Fix: v2.FixSemantic, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &CharArrayToStringCallRule{BaseRule: BaseRule{RuleName: "CharArrayToStringCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects charArray.toString() calls that return the array's address instead of its content."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Fix: v2.FixIdiomatic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: r.check,
		})
	}
	{
		r := &DontDowncastCollectionTypesRule{BaseRule: BaseRule{RuleName: "DontDowncastCollectionTypes", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects downcasting read-only collection types to mutable variants like 'as MutableList'."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Fix: v2.FixSemantic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: r.check,
		})
	}
	{
		r := &ImplicitUnitReturnTypeRule{BaseRule: BaseRule{RuleName: "ImplicitUnitReturnType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects functions with expression bodies that implicitly return Unit without an explicit return type."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				if file.FlatChildCount(body) == 0 || file.FlatType(file.FlatChild(body, 0)) != "statements" {
					bodyText := file.FlatNodeText(body)
					if !strings.HasPrefix(strings.TrimSpace(bodyText), "{") {
						return
					}
				}
				hasReturnType := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					ct := file.FlatType(child)
					if ct == "user_type" || ct == "nullable_type" || ct == "type_identifier" {
						hasReturnType = true
						break
					}
					if file.FlatNodeTextEquals(child, ":") {
						hasReturnType = true
						break
					}
				}
				if hasReturnType {
					return
				}
				funcName := extractIdentifierFlat(file, idx)
				if funcName == "" {
					return
				}
				if ctx.Resolver != nil {
					resolved := ctx.Resolver.ResolveByNameFlat(funcName, idx, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown &&
						(resolved.Kind == typeinfer.TypeUnit || resolved.Name == "Unit" || resolved.FQN == "kotlin.Unit") {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Function without explicit return type. Consider adding ': Unit' or the appropriate return type.")
				if body != 0 {
					insertAt := int(file.FlatStartByte(body))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   insertAt,
						EndByte:     insertAt,
						Replacement: ": Unit ",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ElseCaseInsteadOfExhaustiveWhenRule{BaseRule: BaseRule{RuleName: "ElseCaseInsteadOfExhaustiveWhen", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects when expressions on sealed classes or enums that use an else branch instead of exhaustive matching."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: 0.75, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !whenHasElseBranchFlat(file, idx) {
					return
				}
				if ctx.Resolver != nil {
					coveredTypes := make(map[string]bool)
					var subjectTypeName string
					file.FlatForEachChild(idx, func(entry uint32) {
						if file.FlatType(entry) != "when_entry" {
							return
						}
						file.FlatForEachChild(entry, func(cond uint32) {
							if file.FlatType(cond) != "when_condition" {
								return
							}
							if typeName := whenConditionTypeTestName(file, cond); typeName != "" {
								coveredTypes[typeName] = true
							}
						})
					})
					if len(coveredTypes) > 0 {
						for typeName := range coveredTypes {
							info := ctx.Resolver.ClassHierarchy(typeName)
							if info != nil && len(info.Supertypes) > 0 {
								for _, st := range info.Supertypes {
									parts := strings.Split(st, ".")
									simpleName := parts[len(parts)-1]
									variants := ctx.Resolver.SealedVariants(simpleName)
									if len(variants) == 0 {
										variants = ctx.Resolver.SealedVariants(st)
									}
									if len(variants) > 0 {
										subjectTypeName = simpleName
										allCovered := true
										for _, v := range variants {
											vParts := strings.Split(v, ".")
											vSimple := vParts[len(vParts)-1]
											if !coveredTypes[vSimple] && !coveredTypes[v] {
												allCovered = false
												break
											}
										}
										if !allCovered {
											return
										}
										break
									}
								}
								if subjectTypeName != "" {
									break
								}
							}
						}
						if subjectTypeName != "" {
							ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								fmt.Sprintf("When expression on sealed type '%s' uses 'else' but all variants are covered. Remove the else branch.", subjectTypeName))
						}
					}
					return
				}
				// Without type resolution we cannot determine whether the when subject is
				// an enum or sealed class, so skip to avoid false positives.
			},
		})
	}
}
