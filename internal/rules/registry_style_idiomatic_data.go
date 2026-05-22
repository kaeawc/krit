package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleIdiomaticDataRules() {

	// --- from style_idiomatic_data.go ---
	{
		r := &UseArrayLiteralsInAnnotationsRule{BaseRule: BaseRule{RuleName: "UseArrayLiteralsInAnnotations", RuleSetName: "style", Sev: "warning", Desc: "Detects arrayOf() calls in annotations that should use array literal [] syntax."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceHigher, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if strings.Contains(text, "!= null") && !strings.Contains(text, "else") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Null check could be replaced with ?.let { }.")
				}
			},
		})
	}
	{
		r := &UseDataClassRule{BaseRule: BaseRule{RuleName: "UseDataClass", RuleSetName: "style", Sev: "warning", Desc: "Detects classes with only properties in the constructor that could be data classes."}, AllowVars: false}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "data") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "sealed") || file.FlatHasModifier(idx, "enum") || file.FlatHasModifier(idx, "annotation") {
					return
				}
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				if ctor == 0 {
					return
				}
				paramCount := 0
				sawVar := false
				file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
					trimmed := strings.TrimSpace(file.FlatNodeText(p))
					if strings.HasPrefix(trimmed, "val ") {
						paramCount++
					} else if strings.HasPrefix(trimmed, "var ") {
						paramCount++
						sawVar = true
					}
				})
				if paramCount == 0 {
					return
				}
				// AllowVars: when false, classes whose primary constructor
				// includes any `var` property are excluded from the
				// data-class suggestion. Mutable
				// constructor properties don't fit the immutable-value
				// pattern that data classes capture idiomatically.
				if !r.AllowVars && sawVar {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				entryCount := 0
				containsVarDecl := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "when_entry" {
						entryCount++
						if r.IgnoreWhenContainingVariableDeclaration && !containsVarDecl {
							file.FlatWalkNodes(child, "property_declaration", func(uint32) {
								containsVarDecl = true
							})
						}
					}
				}
				if r.IgnoreWhenContainingVariableDeclaration && containsVarDecl {
					return
				}
				if entryCount <= 2 {
					f := r.Finding(file, file.FlatRow(idx)+1, 1, "When expression with two or fewer branches could be replaced with if.")
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
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Use '.%s {}' instead of manual %s() check.", info.replacement, methodName))
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
				if methodName != "get" && methodName != "set" {
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
						argTexts = append(argTexts, file.FlatNodeText(child))
					}
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
					return
				}
				receiverStatements, ok := alsoLambdaReceiverStatementCount(file, lambda)
				if ok && receiverStatements >= 2 {
					f := r.Finding(file, file.FlatRow(idx)+1, 1, "'also' with multiple 'it.' references could be replaced with 'apply'.")
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
