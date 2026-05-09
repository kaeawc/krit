package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerPotentialbugsTypesRules() {

	// --- from potentialbugs_types.go ---
	{
		r := &AvoidReferentialEqualityRule{
			BaseRule:              BaseRule{RuleName: "AvoidReferentialEquality", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of referential equality operators === or !== which compare object identity instead of value."},
			ForbiddenTypePatterns: []string{"kotlin.String"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
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
				// Comparator/Comparable implementations commonly short-circuit
				// identical object references before doing deeper ordering work.
				if isComparatorIdentityFastPath(file, idx) {
					return
				}
				// Enum constants — the identity compare is correct.
				if looksLikeEnumConstantRef(file.FlatNodeText(left)) || looksLikeEnumConstantRef(file.FlatNodeText(right)) {
					return
				}
				// Sentinel singleton objects are intentionally compared by identity.
				if looksLikeSentinelObjectRef(file, left) || looksLikeSentinelObjectRef(file, right) {
					return
				}
				// Android view identity checks are intentional even when type
				// information is unavailable.
				if looksLikeViewIdentityCheck(file, left, right) {
					return
				}
				// Local aliases to singleton sentinels are still identity
				// checks against the sentinel instance.
				if looksLikeSentinelAliasRef(file, left) || looksLikeSentinelAliasRef(file, right) {
					return
				}
				// Delegate/state fields are often compared by identity against
				// the active local/parameter instance.
				if looksLikeThisFieldIdentityCheck(file, left, right) {
					return
				}
				// Iteration lambdas sometimes use identity to find the current
				// object instance, not an equal value.
				if looksLikeIterationIdentityCheck(file, idx, left, right) {
					return
				}
				// Indexed buffers, singleton companion objects, and cleanup guards
				// intentionally rely on object identity.
				if looksLikeArrayElementIdentityCheck(file, left, right) ||
					looksLikeSingletonTypeIdentityCheck(file, left, right) ||
					looksLikeResourceCleanupIdentityGuard(file, idx, opType) {
					return
				}
				if ctx.Resolver != nil {
					leftType := ctx.Resolver.ResolveFlatNode(left, file)
					rightType := ctx.Resolver.ResolveFlatNode(right, file)
					// When ForbiddenTypePatterns is configured (default
					// `kotlin.String`), gate firing on whether either
					// operand's resolved FQN matches one of the patterns.
					// This is more precise than the IsKnownValueType
					// heuristic.
					if compiled := referentialEqualityCompileFQNPatterns(r.ForbiddenTypePatterns); len(compiled) > 0 {
						leftFQN := referentialEqualityResolvedFQN(leftType)
						rightFQN := referentialEqualityResolvedFQN(rightType)
						leftMatch := referentialEqualityFQNMatchesAny(leftFQN, compiled)
						rightMatch := referentialEqualityFQNMatchesAny(rightFQN, compiled)
						if !leftMatch && !rightMatch {
							// Only suppress when the resolver actually
							// produced an FQN — otherwise fall through
							// to the existing heuristic so users on
							// resolverless or partial-info builds keep
							// catching common cases.
							if leftFQN != "" || rightFQN != "" {
								return
							}
						}
					} else {
						leftKnown := leftType != nil && leftType.Kind != typeinfer.TypeUnknown
						rightKnown := rightType != nil && rightType.Kind != typeinfer.TypeUnknown
						if (leftKnown || rightKnown) && !typeinfer.IsKnownValueType(leftType) && !typeinfer.IsKnownValueType(rightType) {
							return
						}
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
					return
				}
				if !propertyDeclarationIsVar(file, idx) {
					return
				}
				mutableTypes := r.configuredMutableTypes()
				varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
				if varDecl == 0 {
					return
				}
				if !doubleMutabilityHasExplicitMutableType(file, varDecl, mutableTypes) {
					if !doubleMutabilityInitializerLooksLikeMutableFactory(file, idx) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.85, Fix: api.FixSemantic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &CharArrayToStringCallRule{BaseRule: BaseRule{RuleName: "CharArrayToStringCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects charArray.toString() calls that return the array's address instead of its content."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Fix: api.FixIdiomatic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: r.check,
		})
	}
	{
		r := &DontDowncastCollectionTypesRule{BaseRule: BaseRule{RuleName: "DontDowncastCollectionTypes", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects downcasting read-only collection types to mutable variants like 'as MutableList'."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Fix: api.FixSemantic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: r.check,
		})
	}
	{
		r := &ImplicitUnitReturnTypeRule{BaseRule: BaseRule{RuleName: "ImplicitUnitReturnType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects block-body functions that implicitly return Unit without an explicit return type."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || file.FlatHasModifier(idx, "override") {
					return
				}
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
		r := &NoElseInWhenSealedRule{BaseRule: BaseRule{RuleName: "NoElseInWhenSealed", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects when expressions on sealed classes or enums that are missing variants and have no else branch."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: 0.9, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if ctx.Resolver == nil {
					return
				}
				if whenHasElseBranchFlat(file, idx) {
					return
				}
				if _, ok := file.FlatFindChild(idx, "when_subject"); !ok {
					return
				}
				kind, subjectName, variants := whenSubjectExhaustiveKindFlat(file, idx, ctx.Resolver)
				if kind == "" || len(variants) == 0 {
					return
				}
				typeNames, entryNames := collectWhenCoveredVariants(file, idx)
				var missing []string
				switch kind {
				case "sealed":
					missing = missingSealedVariants(typeNames, variants)
				case "enum":
					missing = missingEnumEntries(entryNames, variants)
				}
				if len(missing) == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("'when' on %s '%s' is not exhaustive: missing %s. Add the missing branches or an 'else' clause.",
						kind, subjectName, strings.Join(missing, ", ")))
			},
		})
	}
	{
		r := &ElseCaseInsteadOfExhaustiveWhenRule{BaseRule: BaseRule{RuleName: "ElseCaseInsteadOfExhaustiveWhen", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects when expressions on sealed classes or enums that use an else branch instead of exhaustive matching."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: 0.75, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !whenHasElseBranchFlat(file, idx) {
					return
				}
				if _, ok := file.FlatFindChild(idx, "when_subject"); !ok {
					return
				}
				if whenElseBranchTerminatesFlat(file, idx) {
					return
				}
				if ctx.Resolver == nil {
					return
				}
				subjectTypeName, variants := whenSubjectExhaustiveVariantsFlat(file, idx, ctx.Resolver)
				if len(variants) == 0 {
					return
				}
				coveredTypes := make(map[string]bool)
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
				if len(coveredTypes) == 0 || !whenVariantsCoveredFlat(coveredTypes, variants) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("When expression on sealed type '%s' uses 'else' but all variants are covered. Remove the else branch.", subjectTypeName))
			},
		})
	}
}

func looksLikeViewIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return looksLikeViewIdentityOperand(file.FlatNodeText(left)) ||
		looksLikeViewIdentityOperand(file.FlatNodeText(right))
}

func looksLikeViewIdentityOperand(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	switch text {
	case "view", "itemView":
		return true
	}
	if strings.HasPrefix(text, "binding.") || strings.Contains(text, ".binding.") {
		return true
	}
	if strings.HasPrefix(text, "holder.") &&
		(strings.Contains(text, "View") || strings.Contains(text, ".view") || strings.Contains(text, ".binding.")) {
		return true
	}
	return strings.HasSuffix(text, "View")
}
