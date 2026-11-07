package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// arrayLiteralFactories are the array factory functions an annotation argument can replace with `[...]`.
var arrayLiteralFactories = map[string]bool{
	"arrayOf": true, "booleanArrayOf": true, "charArrayOf": true, "byteArrayOf": true, "shortArrayOf": true,
	"intArrayOf": true, "longArrayOf": true, "floatArrayOf": true, "doubleArrayOf": true,
	"ubyteArrayOf": true, "ushortArrayOf": true, "uintArrayOf": true, "ulongArrayOf": true,
}

const explicitCollectionAccessMessage = "Prefer usage of the indexed access operator [] for map element access or insert methods."

func registerStyleIdiomaticDataRules() {

	// --- from style_idiomatic_data.go ---
	{
		r := &UseArrayLiteralsInAnnotationsRule{BaseRule: BaseRule{RuleName: "UseArrayLiteralsInAnnotations", RuleSetName: "style", Sev: "warning", Desc: "Detects arrayOf() calls in annotations that should use array literal [] syntax."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceHigher, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// detekt reports each arrayOf-style call that is directly an argument of the annotation (also after a spread).
				ctor, ok := file.FlatFindChild(idx, "constructor_invocation")
				if !ok {
					return
				}
				args, _ := file.FlatFindChild(ctor, "value_arguments")
				if args == 0 {
					return
				}
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					expr := flatValueArgumentExpression(file, arg)
					spread := false
					if expr != 0 && file.FlatType(expr) == "spread_expression" {
						spread = true
						for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
							if file.FlatType(c) == "call_expression" {
								expr = c
							}
						}
					}
					if expr == 0 || file.FlatType(expr) != "call_expression" || !arrayLiteralFactories[flatCallExpressionName(file, expr)] {
						continue
					}
					f := r.Finding(file, file.FlatRow(expr)+1, file.FlatCol(expr)+1, "Array literals [...] should be preferred as they are more readable than `arrayOf(...)` expressions.")
					// Fix: replace the `arrayOf(args)` span with `[args]`, reconstructed from the AST.
					if inner := flatCallKeyArguments(file, expr); inner != 0 && !spread {
						argsStart := int(file.FlatStartByte(inner))
						argsEnd := int(file.FlatEndByte(inner))
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(expr)),
							EndByte:     int(file.FlatEndByte(expr)),
							Replacement: "[" + string(file.Content[argsStart+1:argsEnd-1]) + "]",
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &UseSumOfInsteadOfFlatMapSizeRule{BaseRule: BaseRule{RuleName: "UseSumOfInsteadOfFlatMapSize", RuleSetName: "style", Sev: "warning", Desc: "Detects flatMap/map followed by size/count/sum chains that should use sumOf instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
				f := r.Finding(file, callLine(file, idx), callCol(file, idx), msg)
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
		r := &UseLetRule{BaseRule: BaseRule{RuleName: "UseLet", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else with a null branch that could use ?.let {}."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if useLetReportable(file, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Use `?.let {}` instead of if/else with a null block when checking for nullable values")
				}
			},
		})
	}
	{
		r := &UseDataClassRule{BaseRule: BaseRule{RuleName: "UseDataClass", RuleSetName: "style", Sev: "warning", Desc: "Detects classes that only hold data and could be data classes."}, AllowVars: false}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UseIfInsteadOfWhenRule{BaseRule: BaseRule{RuleName: "UseIfInsteadOfWhen", RuleSetName: "style", Sev: "warning", Desc: "Detects when expressions with exactly two branches, one of them else, that could be replaced with if."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// detekt: exactly two entries, one of them else, and no entry with several conditions.
				entryCount := 0
				hasElse := false
				multiCondition := false
				subjectDeclaresVariable := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "when_subject":
						subjectDeclaresVariable = file.FlatHasChildOfType(child, "variable_declaration")
					case "when_entry":
						entryCount++
						conditions := 0
						for ec := file.FlatFirstChild(child); ec != 0; ec = file.FlatNextSib(ec) {
							switch file.FlatType(ec) {
							case "else":
								hasElse = true
							case "when_condition":
								conditions++
							}
						}
						if conditions > 1 {
							multiCondition = true
						}
					}
				}
				if r.IgnoreWhenContainingVariableDeclaration && subjectDeclaresVariable {
					return
				}
				if entryCount == 2 && hasElse && !multiCondition {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Binary expressions are better expressed using an `if` expression than a `when` expression.")
					f.Fix = buildUseIfInsteadOfWhenFix(file, idx)
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &UseIfEmptyOrIfBlankRule{BaseRule: BaseRule{RuleName: "UseIfEmptyOrIfBlank", RuleSetName: "style", Sev: "warning", Desc: "Detects manual isEmpty/isBlank checks that could use .ifEmpty {} or .ifBlank {} instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
				f := r.Finding(file, calleeLine(file, idx), calleeCol(file, idx), fmt.Sprintf("Use '.%s {}' instead of manual %s() check.", info.replacement, methodName))
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiver + "." + info.replacement + " { " + defaultText + " }"}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ExplicitCollectionElementAccessMethodRule{BaseRule: BaseRule{RuleName: "ExplicitCollectionElementAccessMethod", RuleSetName: "style", Sev: "warning", Desc: "Detects explicit .get() and .set() calls that should use index operator syntax."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				target, ok := semantics.ResolveCallTarget(ctx, idx)
				if !ok {
					return
				}
				methodName := target.CalleeName
				if methodName != "get" && methodName != "set" && methodName != "put" {
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
				if (methodName == "set" || methodName == "put") && argCount < 2 {
					return
				}
				// set() and put() return a value, so only a statement-level call can become `x[i] = v`.
				if (methodName == "set" || methodName == "put") && !callIsStatement(file, idx) {
					return
				}
				receiver := target.Receiver.Node
				if receiver == 0 || !explicitCollectionAccessReceiverSupported(ctx, receiver, methodName) {
					return
				}
				row := file.FlatRow(idx) + 1
				col := file.FlatCol(idx) + 1
				var argTexts []string
				for i := 0; i < file.FlatChildCount(argsNode); i++ {
					child := file.FlatChild(argsNode, i)
					if file.FlatType(child) == "value_argument" {
						// Index operands cannot be named arguments.
						if expr := flatValueArgumentExpression(file, child); expr != 0 && flatHasValueArgumentLabel(file, child) {
							argTexts = append(argTexts, file.FlatNodeText(expr))
						} else {
							argTexts = append(argTexts, file.FlatNodeText(child))
						}
					}
				}
				receiverText := file.FlatNodeText(receiver)
				if methodName == "get" {
					f := r.Finding(file, row, col, explicitCollectionAccessMessage)
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + strings.Join(argTexts, ", ") + "]"}
					ctx.Emit(f)
					return
				}
				keys := strings.Join(argTexts[:len(argTexts)-1], ", ")
				value := argTexts[len(argTexts)-1]
				f := r.Finding(file, row, col, explicitCollectionAccessMessage)
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + keys + "] = " + value}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &AlsoCouldBeApplyRule{BaseRule: BaseRule{RuleName: "AlsoCouldBeApply", RuleSetName: "style", Sev: "warning", Desc: "Detects .also {} blocks whose statements all start with it. and could use .apply {} instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigher, Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "also" {
					return
				}
				lambda := flatCallTrailingLambda(file, idx)
				if lambda == 0 {
					// detekt also takes a lambda that is the only parenthesized argument.
					if _, args := flatCallExpressionParts(file, idx); args != 0 && file.FlatNamedChildCount(args) == 1 {
						if expr := flatValueArgumentExpression(file, file.FlatNamedChild(args, 0)); expr != 0 && file.FlatType(expr) == "lambda_literal" {
							lambda = expr
						}
					}
				}
				if lambda == 0 {
					return
				}
				receiverStatements, ok := alsoLambdaReceiverStatementCount(file, lambda)
				if ok && receiverStatements >= 1 {
					f := r.Finding(file, callLine(file, idx), callCol(file, idx), "When an `also` block contains only `it`-started expressions, simplify it to the `apply` block.")
					f.Fix = buildAlsoCouldBeApplyFix(file, idx, lambda)
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EqualsNullCallRule{BaseRule: BaseRule{RuleName: "EqualsNullCall", RuleSetName: "style", Sev: "warning", Desc: "Detects .equals(null) calls that should use == null instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				var navNode uint32
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "navigation_expression" && strings.HasSuffix(file.FlatNodeText(child), ".equals") {
						navNode = child
						break
					}
				}
				// A bare `equals(null)` calls this.equals(null); it is reported but has no rewrite.
				bare := navNode == 0 && file.FlatFirstChild(idx) != 0 && file.FlatType(file.FlatFirstChild(idx)) == "simple_identifier" &&
					file.FlatNodeTextEquals(file.FlatFirstChild(idx), "equals")
				if navNode == 0 && !bare {
					return
				}
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if (file.FlatType(child) == "call_suffix" || file.FlatType(child) == "value_arguments") && callArgumentsAreExactlyNull(file, child) {
						f := r.Finding(file, callLine(file, idx), callCol(file, idx), "Equals() method is called with null as parameter. Consider using == to compare to null.")
						if bare {
							ctx.Emit(f)
							return
						}
						navText := file.FlatNodeText(navNode)
						// `a?.equals(null)` yields a nullable Boolean, so no plain `== null` equivalent exists.
						if dotIdx := strings.LastIndex(navText, ".equals"); dotIdx >= 0 && !strings.HasSuffix(navText[:dotIdx], "?") {
							start, end := int(file.FlatStartByte(navNode)), int(file.FlatEndByte(idx))
							receiver := navText[:dotIdx]
							replacement := receiver + " == null"
							if equalsCallNeedsParens(file, idx) {
								replacement = "(" + replacement + ")"
							}
							f.Fix = &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end, Replacement: replacement}
						}
						ctx.Emit(f)
						return
					}
				}
			},
		})
	}
}

// buildAlsoCouldBeApplyFix rewrites `recv.also { it.f(); it.g = 1 }` to
// `recv.apply { f(); g = 1 }`. The fix is conservative: it bails when any
// `it` reference appears outside the leading receiver position of a
// statement (so nested `it` arguments — which would become unbound
// after switching to `apply` — preserve the original code). Returns nil
// when the rewrite cannot be performed safely.
func buildAlsoCouldBeApplyFix(file *scanner.File, call uint32, lambda uint32) *scanner.Fix {
	if file == nil || call == 0 || lambda == 0 {
		return nil
	}
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return nil
	}
	alsoNode := flatNavigationExpressionLastIdentifierNamed(file, nav, "also")
	if alsoNode == 0 {
		return nil
	}

	statements, ok := file.FlatFindChild(lambda, "statements")
	if !ok || statements == 0 {
		return nil
	}

	// Walk every statement and locate the navigation_expression whose
	// receiver is the bare `it`. Collect the byte ranges of "it." so we
	// can strip them in the rewritten text.
	type cut struct{ start, end int }
	var cuts []cut
	statementCount := 0
	for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		statementCount++
		navNode := alsoStatementNavWithItReceiver(file, stmt)
		if navNode == 0 {
			return nil
		}
		itNode := file.FlatNamedChild(navNode, 0)
		if itNode == 0 || file.FlatType(itNode) != "simple_identifier" || !file.FlatNodeTextEquals(itNode, "it") {
			return nil
		}
		// Range to cut: `it` plus the trailing `.` token. The Kotlin
		// tree-sitter grammar puts the `.` either as a direct sibling
		// of `it` (older grammar shape) or as the first child of a
		// `navigation_suffix` node that follows `it` (current grammar).
		itStart := int(file.FlatStartByte(itNode))
		cutEnd := dotEndAfterReceiver(file, navNode, itNode)
		if cutEnd == 0 {
			return nil
		}
		cuts = append(cuts, cut{itStart, cutEnd})
	}
	if statementCount == 0 {
		return nil
	}

	// Conservative guard: every `it` simple_identifier inside the lambda
	// must be one we're stripping. If `it` appears in an argument
	// position (e.g. `it.f(it)` or `g(it)`), the apply rewrite would
	// leave the inner `it` unbound — refuse the fix.
	itTotal := 0
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, "it") {
			itTotal++
		}
	})
	if itTotal != len(cuts) {
		return nil
	}

	// Build the replacement text by editing the original call_expression
	// bytes in place. `also` becomes `apply`; each `it.` is dropped.
	callStart := int(file.FlatStartByte(call))
	callEnd := int(file.FlatEndByte(call))
	edits := make([]byteEdit, 0, 1+len(cuts))
	edits = append(edits, byteEdit{int(file.FlatStartByte(alsoNode)), int(file.FlatEndByte(alsoNode)), "apply"})
	for _, c := range cuts {
		edits = append(edits, byteEdit{c.start, c.end, ""})
	}
	repl, ok2 := applyByteEdits(file.Content, callStart, callEnd, edits)
	if !ok2 {
		return nil
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   callStart,
		EndByte:     callEnd,
		Replacement: repl,
	}
}

// buildUseIfInsteadOfWhenFix rewrites a `when` expression with up to two
// entries into an equivalent `if`/`if-else` expression. Only the
// no-subject form is rewritten — subject forms (`when (x) { 1 -> ... }`)
// would need synthetic `==`/`is`/`in` comparisons whose semantics depend
// on the entry kind, and existing comma-separated multi-condition
// entries are likewise skipped. Returns nil when the rewrite cannot be
// performed safely.
func buildUseIfInsteadOfWhenFix(file *scanner.File, when uint32) *scanner.Fix {
	if file == nil || when == 0 {
		return nil
	}
	type entry struct {
		isElse bool
		// cond is the condition expression node (the when_condition's
		// first named child) when isElse is false. Multi-condition
		// entries (comma-separated) are not supported and force the
		// whole fix to bail.
		cond uint32
		body uint32
	}
	var entries []entry
	for c := file.FlatFirstChild(when); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "when_subject":
			// Subject form (`when (x) { 1 -> ... }`) would need
			// synthetic `==`/`is`/`in` operators whose meaning depends
			// on the entry kind — leave it to the author.
			return nil
		case "when_entry":
			// fall through to entry collection below
		default:
			continue
		}
		var e entry
		condCount := 0
		for ec := file.FlatFirstChild(c); ec != 0; ec = file.FlatNextSib(ec) {
			switch file.FlatType(ec) {
			case "else":
				e.isElse = true
			case "when_condition":
				condCount++
				if condCount > 1 {
					return nil
				}
				if named := file.FlatNamedChild(ec, 0); named != 0 {
					e.cond = named
				}
			case "control_structure_body":
				e.body = ec
			}
		}
		if e.body == 0 {
			return nil
		}
		if !e.isElse && e.cond == 0 {
			return nil
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 || len(entries) > 2 {
		return nil
	}

	var replacement string
	switch len(entries) {
	case 1:
		e := entries[0]
		if e.isElse {
			// `when { else -> X }` collapses to just X.
			replacement = file.FlatNodeText(e.body)
		} else {
			replacement = "if (" + file.FlatNodeText(e.cond) + ") " + file.FlatNodeText(e.body)
		}
	case 2:
		first := entries[0]
		if first.isElse {
			// First entry can't be else if there's a second branch.
			return nil
		}
		condText := file.FlatNodeText(first.cond)
		thenText := file.FlatNodeText(first.body)
		second := entries[1]
		if second.isElse {
			replacement = "if (" + condText + ") " + thenText + " else " + file.FlatNodeText(second.body)
		} else {
			replacement = "if (" + condText + ") " + thenText +
				" else if (" + file.FlatNodeText(second.cond) + ") " + file.FlatNodeText(second.body)
		}
	}

	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(when)),
		EndByte:     int(file.FlatEndByte(when)),
		Replacement: replacement,
	}
}

// dotEndAfterReceiver returns the byte offset just past the `.` token
// that follows a leading receiver (e.g. `it`) in a navigation expression
// or directly_assignable_expression. Handles both grammar shapes: a
// direct `.` sibling and a `navigation_suffix` whose first child is the
// `.` token. Returns 0 when no dot is found.
func dotEndAfterReceiver(file *scanner.File, parent, recv uint32) int {
	if file == nil || parent == 0 || recv == 0 {
		return 0
	}
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if c != recv {
			continue
		}
		next := file.FlatNextSib(c)
		if next == 0 {
			return 0
		}
		switch file.FlatType(next) {
		case ".":
			return int(file.FlatEndByte(next))
		case "navigation_suffix":
			dot := file.FlatFirstChild(next)
			if dot != 0 && file.FlatType(dot) == "." {
				return int(file.FlatEndByte(dot))
			}
		}
		return 0
	}
	return 0
}

// alsoStatementNavWithItReceiver returns the navigation_expression whose
// leading receiver is the bare `it` identifier for the given statement.
// Mirrors alsoStatementReceiverIsIt's traversal but returns the
// navigation node rather than a bool, so callers can rewrite the
// `it.<member>` prefix.
func alsoStatementNavWithItReceiver(file *scanner.File, stmt uint32) uint32 {
	if file == nil || stmt == 0 {
		return 0
	}
	switch file.FlatType(stmt) {
	case "call_expression":
		nav, _ := flatCallExpressionParts(file, stmt)
		if alsoNavigationReceiverIsIt(file, nav) {
			return nav
		}
		return 0
	// Assignments such as `it.name = x` are a krit extension: detekt does not count them, though apply fits them best.
	case "assignment", "assignment_expression":
		return alsoStatementNavWithItReceiver(file, file.FlatNamedChild(stmt, 0))
	case "directly_assignable_expression":
		if nav, ok := file.FlatFindChild(stmt, "navigation_expression"); ok {
			if alsoNavigationReceiverIsIt(file, nav) {
				return nav
			}
			return 0
		}
		if alsoNavigationReceiverIsIt(file, stmt) {
			return stmt
		}
		return 0
	case "navigation_expression":
		if alsoNavigationReceiverIsIt(file, stmt) {
			return stmt
		}
		return 0
	case "parenthesized_expression", "annotated_expression":
		return alsoStatementNavWithItReceiver(file, file.FlatNamedChild(stmt, 0))
	}
	if file.FlatNamedChildCount(stmt) == 1 {
		return alsoStatementNavWithItReceiver(file, file.FlatNamedChild(stmt, 0))
	}
	return 0
}

func alsoLambdaReceiverStatementCount(file *scanner.File, lambda uint32) (int, bool) {
	if file == nil || lambda == 0 {
		return 0, false
	}
	statements, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return 0, false
	}
	receiverStatements := 0
	for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		if !alsoStatementReceiverIsIt(file, stmt) {
			return 0, false
		}
		receiverStatements++
	}
	return receiverStatements, receiverStatements > 0
}

func alsoStatementReceiverIsIt(file *scanner.File, stmt uint32) bool {
	return alsoStatementNavWithItReceiver(file, stmt) != 0
}

func alsoNavigationReceiverIsIt(file *scanner.File, nav uint32) bool {
	if file == nil || nav == 0 {
		return false
	}
	first := file.FlatNamedChild(nav, 0)
	return first != 0 && file.FlatType(first) == "simple_identifier" && file.FlatNodeText(first) == "it"
}

// callArgumentsAreExactlyNull reports whether a call suffix holds a single argument that is the literal null.
func callArgumentsAreExactlyNull(file *scanner.File, suffix uint32) bool {
	args := suffix
	if file.FlatType(suffix) == "call_suffix" {
		args, _ = file.FlatFindChild(suffix, "value_arguments")
	}
	if args == 0 {
		return false
	}
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		count++
		if strings.TrimSpace(file.FlatNodeText(arg)) != "null" {
			return false
		}
	}
	return count == 1
}
