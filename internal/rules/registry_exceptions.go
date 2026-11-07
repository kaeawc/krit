package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerExceptionsRules() {

	// --- from exceptions.go ---
	{
		r := &ExceptionRaisedInUnexpectedLocationRule{BaseRule: BaseRule{RuleName: "ExceptionRaisedInUnexpectedLocation", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside equals, hashCode, toString, or finalize methods."}, MethodNames: []string{"equals", "hashCode", "toString", "finalize"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "method_declaration"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !r.includesMethod(extractIdentifierFlat(file, idx)) {
					return
				}
				// detekt reports the function once if any throw sits in its body, wherever it is.
				thrown := false
				walkThrowExpressionsFlat(file, idx, func(uint32) { thrown = true })
				if thrown {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						"This method is not expected to throw exceptions. This can cause weird behavior.")
				}
			},
		})
	}
	{
		r := &InstanceOfCheckForExceptionRule{BaseRule: BaseRule{RuleName: "InstanceOfCheckForException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects instanceof/is checks for exception types inside catch blocks instead of using specific catch clauses."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				catchVarName := extractCaughtVarNameFlat(file, idx)
				if catchVarName == "" {
					return
				}
				// detekt flags `is`, `!is` and unsafe `as` on the caught parameter itself, since a second
				// catch clause would do the same without the check. A `when (e) { is X -> }` is not flagged.
				const message = "Instead of catching for a general exception type and checking for a specific exception type, use multiple catch blocks."
				check := func(node uint32) {
					first := file.FlatFirstChild(node)
					if (file.FlatType(first) == "simple_identifier" || file.FlatType(first) == "identifier") && file.FlatNodeText(first) == catchVarName {
						ctx.EmitAt(file.FlatRow(node)+1, file.FlatCol(node)+1, message)
					}
				}
				file.FlatWalkNodes(idx, "check_expression", func(n uint32) {
					for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
						if t := file.FlatType(c); t == "is" || t == "!is" {
							check(n)
							return
						}
					}
				})
				file.FlatWalkNodes(idx, "as_expression", func(n uint32) {
					for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
						if file.FlatType(c) == "as" {
							check(n)
							return
						}
					}
				})
				file.FlatWalkNodes(idx, "instanceof_expression", check)
			},
		})
	}
	{
		r := &NotImplementedDeclarationRule{BaseRule: BaseRule{RuleName: "NotImplementedDeclaration", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects TODO() calls and throws of NotImplementedError, which fail at runtime."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "jump_expression"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// detekt's visitor does not descend into the arguments or lambda of a call, so a TODO() or throw
				// nested in one is not reported.
				if hasAncestorOfType(file, idx, "call_suffix") {
					return
				}
				if file.FlatType(idx) == "jump_expression" {
					if isThrowOfNotImplementedError(file, idx) {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							niMessage)
					}
					return
				}
				if !isKotlinTODOCall(file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					niMessage)
			},
		})
	}
	{
		r := &RethrowCaughtExceptionRule{BaseRule: BaseRule{RuleName: "RethrowCaughtException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks whose only statement is rethrowing the caught exception, making the catch block useless."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				caughtVar := extractCaughtVarNameFlat(file, idx)
				if caughtVar == "" {
					return
				}
				// A rethrow-only catch followed by another catch clause of the
				// same try is an intentional "let this type escape" guard: it
				// keeps the later (typically broader) handler from swallowing
				// it, so removing it would change behavior.
				if hasLaterCatchSibling(file, idx) {
					return
				}
				if file.Language == scanner.LangJava {
					if javaCatchOnlyRethrowsVar(file, idx, caughtVar) {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							"Do not rethrow a caught exception of the same type.")
					}
					return
				}
				body, _ := file.FlatFindChild(idx, "statements")
				if body == 0 {
					return
				}
				stmtCount := 0
				var onlyThrow uint32
				for i := 0; i < file.FlatChildCount(body); i++ {
					child := file.FlatChild(body, i)
					if file.FlatType(child) == "jump_expression" && strings.HasPrefix(file.FlatNodeText(child), "throw") {
						onlyThrow = child
						stmtCount++
					} else if t := file.FlatType(child); t != "line_comment" && t != "multiline_comment" && t != "{" && t != "}" {
						stmtCount++
					}
				}
				if stmtCount == 1 && onlyThrow != 0 {
					throwText := strings.TrimSpace(file.FlatNodeText(onlyThrow))
					if throwText == "throw "+caughtVar || throwText == "throw "+caughtVar+";" {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							"Do not rethrow a caught exception of the same type.")
					}
				}
			},
		})
	}
	{
		r := &ReturnFromFinallyRule{BaseRule: BaseRule{RuleName: "ReturnFromFinally", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects return statements inside finally blocks that can swallow exceptions from the try/catch."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"finally_block", "finally_clause"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				returnTypes := []string{"jump_expression"}
				if file.Language == scanner.LangJava {
					returnTypes = append(returnTypes, "return_statement")
				}
				for _, returnType := range returnTypes {
					file.FlatWalkNodes(idx, returnType, func(jumpNode uint32) {
						text := file.FlatNodeText(jumpNode)
						if !strings.HasPrefix(text, "return") {
							return
						}
						// IgnoreLabeled: skip `return@something` jumps. These
						// typically return from an enclosing lambda rather
						// than the surrounding function and don't swallow
						// the try/catch exception in the same way.
						if r.IgnoreLabeled && strings.HasPrefix(text, "return@") {
							return
						}
						if jumpLeavesOnlyNestedScope(file, jumpNode, idx) {
							return
						}
						f := r.Finding(file, file.FlatRow(jumpNode)+1, file.FlatCol(jumpNode)+1,
							"Do not return within a finally statement. This can discard exceptions.")
						ctx.Emit(f)
					})
				}
			},
		})
	}
	{
		r := &SwallowedExceptionRule{
			BaseRule:                  BaseRule{RuleName: "SwallowedException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks that silently swallow the caught exception without logging, handling, or rethrowing."},
			AllowedExceptionNameRegex: regexp.MustCompile(`^(?:_|(ignore|expected).*)$`),
			IgnoredExceptionTypes:     []string{"InterruptedException", "MalformedURLException", "NumberFormatException", "ParseException"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:      []string{"catch_block"},
			Confidence:     0.75,
			Fix:            api.FixSemantic,
			Implementation: r,
			Check:          r.checkSwallowedException,
		})
	}
	{
		r := &ThrowingExceptionFromFinallyRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionFromFinally", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside finally blocks that can mask exceptions from the try/catch."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"finally_block", "finally_clause"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					if jumpLeavesOnlyNestedScope(file, throwNode, idx) {
						return
					}
					f := r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						"Do not throw an exception within a finally statement. This can discard exceptions and is confusing.")
					ctx.Emit(f)
				})
			},
		})
	}
	{
		r := &ThrowingExceptionsWithoutMessageOrCauseRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionsWithoutMessageOrCause", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects common exception types thrown without a descriptive message or cause argument."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// A call that wraps another call (`f(x) { }`) is one PSI call; handle it at the outermost node.
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "call_expression" && file.FlatFirstChild(p) == idx {
					return
				}
				_, _, name := splitPsiCall(file, idx)
				if name == "" || !r.exceptionAllowlist()[name] {
					return
				}
				if parens, lambdas := callValueArgumentCount(file, idx); parens+lambdas != 0 {
					return
				}
				anchor := calleeNameNode(file, idx)
				ctx.EmitAt(file.FlatRow(anchor)+1, file.FlatCol(anchor)+1,
					"A call to the default constructor of an exception was detected. Instead one of the constructor overloads should be called. This allows to provide more meaningful exceptions.")
			},
		})
	}
	{
		r := &ThrowingNewInstanceOfSameExceptionRule{BaseRule: BaseRule{RuleName: "ThrowingNewInstanceOfSameException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks that throw a new instance of the same exception type instead of rethrowing the original."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				caughtType := extractCaughtTypeNameFlat(file, idx)
				if caughtType == "" {
					return
				}
				caughtVar := extractCaughtVarNameFlat(file, idx)
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					target := throwExpressionTargetFlat(file, throwNode)
					if target == 0 {
						return
					}
					if throwTargetTypeNameFlat(file, target) != caughtType {
						return
					}
					// A new exception that does not wrap the caught one replaces it on purpose.
					if caughtVar == "" || caughtVar == "_" {
						return
					}
					argCount, hasVar := throwTargetArgUsageFlat(file, target, caughtVar)
					if !hasVar || argCount > 1 {
						return
					}
					// A new message or other argument adds context, so the rethrow is not redundant.
					if throwTargetHasExtraArgumentFlat(file, target, caughtVar) {
						return
					}
					ctx.EmitAt(file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						"Avoid catch blocks that rethrow a caught exception wrapped inside a new instance of the same exception.")
				})
			},
		})
	}
	{
		r := &ThrowingExceptionInMainRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionInMain", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside the main() function instead of graceful error handling."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "method_declaration"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "main" || !isProgramEntryMain(file, idx) {
					return
				}
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					ctx.EmitAt(file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						"Exception thrown in main(). Handle exceptions gracefully instead.")
				})
			},
		})
	}
	{
		r := &ErrorUsageWithThrowableRule{BaseRule: BaseRule{RuleName: "ErrorUsageWithThrowable", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects error() calls that pass a Throwable argument instead of using throw directly."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				argText, ok := errorUsageWithThrowableArgument(ctx, idx)
				if !ok {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					fmt.Sprintf("error(%s) passes a Throwable. Use throw instead, or pass the message string.", argText))
			},
		})
	}
	{
		r := &ObjectExtendsThrowableRule{BaseRule: BaseRule{RuleName: "ObjectExtendsThrowable", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects Kotlin object declarations that extend Throwable, which are singletons that lose stack trace information."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"object_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if ctx.Resolver != nil {
					info := ctx.Resolver.ClassHierarchy(name)
					if info != nil {
						throwableSet := map[string]bool{
							"Throwable": true, "Exception": true, "Error": true, "RuntimeException": true,
							"kotlin.Throwable": true, "java.lang.Throwable": true,
							"java.lang.Exception": true, "java.lang.Error": true,
							"java.lang.RuntimeException": true,
						}
						for _, st := range info.Supertypes {
							simpleParts := strings.Split(st, ".")
							simpleName := simpleParts[len(simpleParts)-1]
							if throwableSet[st] || throwableSet[simpleName] {
								ctx.EmitAt(declLine(file, idx), declCol(file, idx),
									objectExtendsThrowableMessage)
								return
							}
						}
						return
					}
				}
				text := file.FlatNodeText(idx)
				for _, t := range throwableBaseTypes {
					if strings.Contains(text, ": "+t) || strings.Contains(text, ":"+t+"(") || strings.Contains(text, ": "+t+"(") {
						ctx.EmitAt(declLine(file, idx), declCol(file, idx),
							objectExtendsThrowableMessage)
						return
					}
				}
			},
		})
	}
}

const niMessage = "The NotImplementedDeclaration should only be used when a method stub is necessary. This defers the development of the functionality of this function. Hence, the `NotImplementedDeclaration` should only serve as a temporary declaration. Before releasing, this type of declaration should be removed."

const objectExtendsThrowableMessage = "An `object` should not extend and type of Throwable. Throwables are stateful and should be instantiated only when needed for when a specific error occurs. An `object`, being a singleton, that extends any type of Throwable consequently introduces a global singleton exception whose instance may be inadvertently reused from multiple places, thus introducing shared mutable state."
