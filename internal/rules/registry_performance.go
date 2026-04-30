package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	"strings"
)

func registerPerformanceRules() {

	// --- from performance.go ---
	{
		r := &ArrayPrimitiveRule{BaseRule: BaseRule{RuleName: "ArrayPrimitive", RuleSetName: "performance", Sev: "warning", Desc: "Detects Array<Int> and similar boxed primitive arrays that should use IntArray, LongArray, etc."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"user_type", "call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "call_expression" {
					name := flatCallExpressionName(file, idx)
					if name != "arrayOf" && name != "emptyArray" {
						return
					}
					typeArgs := callExpressionTypeArgumentsFlat(file, idx)
					if typeArgs == 0 {
						return
					}
					primitive, replacement, ok := primitiveArrayReplacementForTypeRef(file.FlatNodeText(typeArgs))
					if !ok {
						return
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Use '%s' instead of '%s<%s>()' for better performance.", replacement, name, primitive))
					ctx.Emit(f)
					return
				}
				ident, _ := file.FlatFindChild(idx, "type_identifier")
				typeArgs, _ := file.FlatFindChild(idx, "type_arguments")
				if ident == 0 || typeArgs == 0 {
					return
				}
				if !file.FlatNodeTextEquals(ident, "Array") {
					return
				}
				text := file.FlatNodeText(typeArgs)
				if ctx.Resolver != nil {
					argName := simpleTypeReferenceName(text)
					fqn := ctx.Resolver.ResolveImport(argName, file)
					if fqn != "" {
						if replacement, ok := primitiveFQNToReplacement[fqn]; ok {
							f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								fmt.Sprintf("Use '%s' instead of 'Array<%s>' for better performance.", replacement, argName))
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(idx)),
								EndByte:     int(file.FlatEndByte(idx)),
								Replacement: replacement,
							}
							ctx.Emit(f)
						}
						return
					}
				}
				primitive, replacement, ok := primitiveArrayReplacementForTypeRef(text)
				if !ok {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Use '%s' instead of 'Array<%s>' for better performance.", replacement, primitive))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &BitmapDecodeWithoutOptionsRule{BaseRule: BaseRule{RuleName: "BitmapDecodeWithoutOptions", RuleSetName: "performance", Sev: "warning", Desc: "Detects BitmapFactory.decode* calls without BitmapFactory.Options, which may decode full-size bitmaps."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}
				methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
				if !bitmapDecodeMethods[methodName] {
					return
				}
				if file.FlatNamedChildCount(navExpr) == 0 {
					return
				}
				receiver := file.FlatNamedChild(navExpr, 0)
				receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
				if i := strings.LastIndex(receiverText, "."); i >= 0 {
					receiverText = receiverText[i+1:]
				}
				if receiverText != "BitmapFactory" {
					return
				}
				argCount := 0
				for i := 0; i < file.FlatChildCount(args); i++ {
					if file.FlatType(file.FlatChild(args, i)) == "value_argument" {
						argCount++
					}
				}
				if argCount != 1 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("BitmapFactory.%s without BitmapFactory.Options may decode a full-size bitmap. Pass BitmapFactory.Options to control memory usage.", methodName))
			},
		})
	}
	{
		r := &CouldBeSequenceRule{BaseRule: BaseRule{RuleName: "CouldBeSequence", RuleSetName: "performance", Sev: "warning", Desc: "Detects collection operation chains that could use sequences to avoid intermediate allocations."}, AllowedOperations: 2}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Needs:                  v2.NeedsTypeInfo,
			Oracle:                 &v2.OracleFilter{Identifiers: sequenceCollectionOperationNames()},
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: sequenceCollectionOperationNames()},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if collectionChainShouldSkipStartFlat(file, idx) {
					return
				}
				calls := collectCollectionChainCallsFlat(file, idx)
				count := len(calls)
				if count <= r.AllowedOperations {
					return
				}
				rootReceiver := collectionChainRootFlat(file, idx)
				if rootReceiver == 0 {
					return
				}
				if ctx.Resolver != nil {
					if decided, report := sequenceOracleDecisionFlat(file, ctx.Resolver, calls); decided {
						if report {
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
						}
						return
					}
					if decided, report := sequenceResolverDecisionFlat(file, ctx.Resolver, rootReceiver, calls); decided {
						if report {
							ctx.EmitAt(file.FlatRow(idx)+1, 1,
								fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
						}
						return
					}
				}
				name := flatCallExpressionName(file, rootReceiver)
				if obviousSequenceSourceCalls[name] {
					return
				}
				if !obviousCollectionSourceCalls[name] {
					if file.FlatType(rootReceiver) == "simple_identifier" {
						if fn := enclosingFunctionDeclarationFlat(file, idx); fn != 0 {
							if hasCollectionTypeAnnotation(file.FlatNodeText(fn), file.FlatNodeText(rootReceiver)) {
								ctx.EmitAt(file.FlatRow(idx)+1, 1,
									fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
							}
						}
					}
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
			},
		})
	}
	{
		r := &ForEachOnRangeRule{BaseRule: BaseRule{RuleName: "ForEachOnRange", RuleSetName: "performance", Sev: "warning", Desc: "Detects (range).forEach patterns that should use a simple for loop to avoid lambda overhead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "forEach" {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				if file.FlatNamedChildCount(navExpr) == 0 {
					return
				}
				receiver := file.FlatNamedChild(navExpr, 0)
				if !containsRangeExpressionFlat(file, receiver) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Use a regular 'for' loop instead of '(range).forEach' for better performance.")
				f.Fix = forEachOnRangeFixFlat(file, idx, receiver)
				ctx.Emit(f)
			},
		})
	}
	{
		r := &SpreadOperatorRule{BaseRule: BaseRule{RuleName: "SpreadOperator", RuleSetName: "performance", Sev: "warning", Desc: "Detects the spread operator (*array) in function calls which creates an array copy."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"spread_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !spreadOperatorShouldReportFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Spread operator used. This creates a copy of the array.")
			},
		})
	}
	{
		r := &UnnecessaryInitOnArrayRule{BaseRule: BaseRule{RuleName: "UnnecessaryInitOnArray", RuleSetName: "performance", Sev: "warning", Desc: "Detects IntArray(n) { 0 } and similar array initializations where the init value is already the default."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				_, lambda, ok := unnecessaryInitArrayDefaultLambdaFlat(file, idx)
				if !ok {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Unnecessary initialization. The default value is already the array's default.")
				f.Fix = unnecessaryInitArrayFixFlat(file, lambda)
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryPartOfBinaryExpressionRule{BaseRule: BaseRule{RuleName: "UnnecessaryPartOfBinaryExpression", RuleSetName: "performance", Sev: "warning", Desc: "Detects redundant parts of binary expressions like x && true or x || false."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"conjunction_expression", "disjunction_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatNamedChildCount(idx) < 2 {
					return
				}
				left := file.FlatNamedChild(idx, 0)
				right := file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
				leftText := file.FlatNodeText(left)
				rightText := file.FlatNodeText(right)
				isConjunction := file.FlatType(idx) == "conjunction_expression"
				var redundant bool
				var keepNode uint32
				if isConjunction {
					if rightText == "true" {
						redundant = true
						keepNode = left
					} else if leftText == "true" {
						redundant = true
						keepNode = right
					}
				} else {
					if rightText == "false" {
						redundant = true
						keepNode = left
					} else if leftText == "false" {
						redundant = true
						keepNode = right
					}
				}
				if !redundant {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Unnecessary part of binary expression. 'true' or 'false' literal in logical expression is redundant.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: file.FlatNodeText(keepNode),
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryTemporaryInstantiationRule{BaseRule: BaseRule{RuleName: "UnnecessaryTemporaryInstantiation", RuleSetName: "performance", Sev: "warning", Desc: "Detects temporary wrapper instantiations like Integer.valueOf(x).toString() that can be simplified."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				src := file.FlatNodeBytes(idx)
				if !looksLikeTempInstantiation(src) {
					return
				}
				if flatCallExpressionName(file, idx) != "toString" {
					return
				}
				nav, _ := file.FlatFindChild(idx, "navigation_expression")
				if nav == 0 {
					return
				}
				innerCall := tempInstantiationReceiverFlat(file, nav)
				if innerCall == 0 || file.FlatType(innerCall) != "call_expression" {
					return
				}
				method := flatCallExpressionName(file, innerCall)
				if !tempInstantiationMethods[method] {
					return
				}
				innerNav, _ := file.FlatFindChild(innerCall, "navigation_expression")
				if innerNav == 0 {
					return
				}
				typeName := tempInstantiationTypeNameFlat(file, innerNav)
				if !tempInstantiationTypeNames[typeName] {
					return
				}
				arg := tempInstantiationFirstArgumentFlat(file, innerCall)
				if arg == "" {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Unnecessary temporary instantiation. Use the type's toString() or conversion method directly.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: arg + ".toString()",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryTypeCastingRule{BaseRule: BaseRule{RuleName: "UnnecessaryTypeCasting", RuleSetName: "performance", Sev: "warning", Desc: "Detects safe casts immediately compared with null and redundant casts to an already-known type."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"equality_expression", "as_expression"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "equality_expression" {
					expr, target, ok := safeCastComparedNotNullParts(file, idx)
					if !ok {
						return
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Unnecessary safe cast to '%s' before null check. Use a type check instead.", target))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: expr + " is " + target,
					}
					ctx.Emit(f)
					return
				}
				text := file.FlatNodeText(idx)
				parts := strings.Split(text, " as ")
				if len(parts) != 2 {
					return
				}
				target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "?"))
				expr := strings.TrimSpace(parts[0])
				if ctx.Resolver != nil && file.FlatChildCount(idx) >= 2 {
					exprType := flatResolveByName(file, ctx.Resolver, file.FlatChild(idx, 0))
					targetType := flatResolveByName(file, ctx.Resolver, file.FlatChild(idx, file.FlatChildCount(idx)-1))
					if exprType != nil && targetType != nil &&
						exprType.Kind != typeinfer.TypeUnknown && targetType.Kind != typeinfer.TypeUnknown {
						if exprType.FQN == targetType.FQN {
							f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								fmt.Sprintf("Unnecessary cast to '%s'. The expression is already of this type.", target))
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(idx)),
								EndByte:     int(file.FlatEndByte(idx)),
								Replacement: expr,
							}
							ctx.Emit(f)
						}
						return
					}
				}
				matched := false
				if strings.HasSuffix(expr, ": "+target) || strings.HasSuffix(expr, ":"+target) {
					matched = true
				}
				if !matched {
					for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
						if file.FlatType(parent) == "property_declaration" || file.FlatType(parent) == "function_declaration" {
							parentText := file.FlatNodeText(parent)
							declType := ""
							if i := strings.Index(parentText, ":"); i >= 0 {
								rest := parentText[i+1:]
								if eqIdx := strings.Index(rest, "="); eqIdx >= 0 {
									declType = strings.TrimSpace(rest[:eqIdx])
								}
							}
							if declType == target {
								matched = true
							}
							break
						}
					}
				}
				if matched {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Unnecessary cast to '%s'. The expression is already of this type.", target))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: expr,
					}
					ctx.Emit(f)
				}
			},
		})
	}
}
