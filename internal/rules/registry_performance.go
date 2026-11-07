package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// minCastExpressionChildren is the minimum shape for resolving an as-expression:
// a source expression and a target type.
const minCastExpressionChildren = 2

func registerPerformanceRules() {

	// --- from performance.go ---
	{
		r := &ArrayPrimitiveRule{BaseRule: BaseRule{RuleName: "ArrayPrimitive", RuleSetName: "performance", Sev: "warning", Desc: "Detects Array<Int> and similar boxed primitive arrays that should use IntArray, LongArray, etc."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"user_type", "call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if arrayPrimitiveInParameterizedContext(file, idx) {
					return
				}
				if file.FlatType(idx) == "call_expression" {
					name := flatCallExpressionName(file, idx)
					if name != "arrayOf" && name != "emptyArray" {
						return
					}
					typeArgs := callExpressionTypeArgumentsFlat(file, idx)
					if typeArgs == 0 || arrayTypeArgIsNullable(file.FlatNodeText(typeArgs)) {
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
				// Array<Int?> holds nulls, which IntArray cannot.
				if arrayTypeArgIsNullable(text) {
					return
				}
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
				// decodeResource(res, id) takes two arguments before the optional Options.
				wantArgs := 1
				if methodName == "decodeResource" {
					wantArgs = 2
				}
				if argCount != wantArgs || bitmapDecodeInSmallImageContext(file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					fmt.Sprintf("BitmapFactory.%s without BitmapFactory.Options may decode a full-size bitmap. Pass BitmapFactory.Options to control memory usage.", methodName))
			},
		})
	}
	{
		r := &CouldBeSequenceRule{BaseRule: BaseRule{RuleName: "CouldBeSequence", RuleSetName: "performance", Sev: "warning", Desc: "Detects collection operation chains that could use sequences to avoid intermediate allocations."}, AllowedOperations: 2}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs:  api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{Identifiers: sequenceCollectionOperationNames()},
			Check: func(ctx *api.Context) {
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
							ctx.EmitAt(callLine(file, idx), callCol(file, idx),
								fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
						}
						return
					}
					if decided, report := sequenceResolverDecisionFlat(file, ctx.Resolver, rootReceiver, calls); decided {
						if report {
							ctx.EmitAt(callLine(file, idx), callCol(file, idx),
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
								ctx.EmitAt(callLine(file, idx), callCol(file, idx),
									fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
							}
						}
					}
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))
			},
		})
	}
	{
		r := &ForEachOnRangeRule{BaseRule: BaseRule{RuleName: "ForEachOnRange", RuleSetName: "performance", Sev: "warning", Desc: "Detects (range).forEach patterns that should use a simple for loop to avoid lambda overhead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"Using the forEach method on ranges has a heavy performance cost. Prefer using simple for loops.")
				f.Fix = forEachOnRangeFixFlat(file, idx, receiver)
				ctx.Emit(f)
			},
		})
	}
	{
		r := &SpreadOperatorRule{BaseRule: BaseRule{RuleName: "SpreadOperator", RuleSetName: "performance", Sev: "warning", Desc: "Detects the spread operator (*array) in function calls which creates an array copy."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"spread_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !spreadOperatorShouldReportFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"In most cases using a spread operator causes a full copy of the array to be created before calling a method. This may result in a performance penalty.")
			},
		})
	}
	{
		r := &UnnecessaryInitOnArrayRule{BaseRule: BaseRule{RuleName: "UnnecessaryInitOnArray", RuleSetName: "performance", Sev: "warning", Desc: "Detects IntArray(n) { 0 } and similar array initializations where the init value is already the default."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				_, lambda, ok := unnecessaryInitArrayDefaultLambdaFlat(file, idx)
				if !ok {
					return
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"Unnecessary initialization. The default value is already the array's default.")
				f.Fix = unnecessaryInitArrayFixFlat(file, lambda)
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryPartOfBinaryExpressionRule{BaseRule: BaseRule{RuleName: "UnnecessaryPartOfBinaryExpression", RuleSetName: "performance", Sev: "warning", Desc: "Detects redundant parts of binary expressions like x && true or x || false."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"conjunction_expression", "disjunction_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
		r := &UnnecessaryTemporaryInstantiationRule{BaseRule: BaseRule{RuleName: "UnnecessaryTemporaryInstantiation", RuleSetName: "performance", Sev: "warning", Desc: "Detects wrapper instantiations like Integer(1).toString() that create a temporary object only to convert a primitive to a String."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// detekt: <Boolean|Byte|Short|Integer|Long|Float|Double>(...).toString(), the constructor
				// call used as the receiver of toString().
				first := file.FlatFirstChild(idx)
				if file.FlatType(first) != "simple_identifier" || !temporaryInstantiationWrappers[file.FlatNodeText(first)] {
					return
				}
				nav, ok := file.FlatParent(idx)
				if !ok || file.FlatType(nav) != "navigation_expression" || file.FlatFirstChild(nav) != idx {
					return
				}
				suffix, ok := file.FlatFindChild(nav, "navigation_suffix")
				if !ok || !file.FlatNodeTextEquals(suffix, ".toString") {
					return
				}
				outer, ok := file.FlatParent(nav)
				if !ok || file.FlatType(outer) != "call_expression" || file.FlatFirstChild(outer) != nav {
					return
				}
				if parens, lambdas := callValueArgumentCount(file, outer); parens+lambdas != 0 {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Avoid temporary objects when converting primitive types to `String`.")
				if args, ok := callParenArguments(file, idx); ok && len(args) == 1 && !strings.HasPrefix(args[0], "\"") {
					operand := args[0]
					if !tempInstantiationSimpleOperand(operand) {
						operand = "(" + operand + ")"
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(outer)),
						EndByte:     int(file.FlatEndByte(outer)),
						Replacement: operand + ".toString()",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryTypeCastingRule{BaseRule: BaseRule{RuleName: "UnnecessaryTypeCasting", RuleSetName: "performance", Sev: "warning", Desc: "Detects safe casts immediately compared with null and redundant casts to an already-known type."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression", "as_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
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
				if ctx.Resolver == nil || file.FlatChildCount(idx) < minCastExpressionChildren {
					return
				}
				// `as?` is handled above; only a plain `as` can be a no-op here.
				for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) == "as?" {
						return
					}
				}
				exprNode := file.FlatChild(idx, 0)
				targetNode := file.FlatChild(idx, file.FlatChildCount(idx)-1)
				targetText := strings.TrimSpace(file.FlatNodeText(targetNode))
				// Casts to parameterized types change the type arguments.
				if strings.Contains(targetText, "<") {
					return
				}
				exprType := flatResolveByName(file, ctx.Resolver, exprNode)
				targetType := flatResolveByName(file, ctx.Resolver, targetNode)
				if exprType == nil || targetType == nil || !exprType.IsResolved() || !targetType.IsResolved() ||
					exprType.Kind == typeinfer.TypeUnknown || targetType.Kind == typeinfer.TypeUnknown ||
					exprType.FQN == "" || exprType.FQN != targetType.FQN {
					return
				}
				targetNullable := file.FlatType(targetNode) == "nullable_type" || targetType.IsNullable()
				// Casting T? to T asserts non-null, so it is not a no-op.
				if exprType.IsNullable() && !targetNullable {
					return
				}
				expr := strings.TrimSpace(file.FlatNodeText(exprNode))
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary cast to '%s'. The expression is already of this type.", targetText))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: expr,
				}
				ctx.Emit(f)
			},
		})
	}
}

var temporaryInstantiationWrappers = map[string]bool{
	"Boolean": true, "Byte": true, "Short": true, "Integer": true, "Long": true, "Float": true, "Double": true,
}

// callParenArguments returns the source text of each argument in parentheses of a call_expression.
func callParenArguments(file *scanner.File, call uint32) ([]string, bool) {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return nil, false
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return nil, false
	}
	var out []string
	for a := file.FlatFirstChild(args); a != 0; a = file.FlatNextSib(a) {
		if file.FlatType(a) == "value_argument" {
			out = append(out, strings.TrimSpace(file.FlatNodeText(a)))
		}
	}
	return out, true
}
