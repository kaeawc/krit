package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"regexp"
	"strings"
)

func registerExceptionsRules() {

	// --- from exceptions.go ---
	{
		r := &ExceptionRaisedInUnexpectedLocationRule{BaseRule: BaseRule{RuleName: "ExceptionRaisedInUnexpectedLocation", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside equals, hashCode, toString, or finalize methods."}, MethodNames: []string{"equals", "hashCode", "toString", "finalize"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "method_declaration"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if !r.includesMethod(name) {
					return
				}
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					ctx.EmitAt(file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						fmt.Sprintf("Exception thrown inside '%s()'. This method should not throw exceptions.", name))
				})
			},
		})
	}
	{
		r := &InstanceOfCheckForExceptionRule{BaseRule: BaseRule{RuleName: "InstanceOfCheckForException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects instanceof/is checks for exception types inside catch blocks instead of using specific catch clauses."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				catchVarName := extractCaughtVarNameFlat(file, idx)
				if catchVarName == "" {
					return
				}
				skipWhenDispatch := experiment.Enabled("instance-of-check-skip-when-dispatch")
				for _, nodeType := range []string{"is_expression", "type_check", "check_expression"} {
					file.FlatWalkNodes(idx, nodeType, func(isNode uint32) {
						text := file.FlatNodeText(isNode)
						if !isExceptionRe.MatchString(text) {
							return
						}
						if file.FlatChildCount(isNode) < 1 {
							return
						}
						lhs := file.FlatNodeText(file.FlatChild(isNode, 0))
						if strings.TrimSpace(lhs) != catchVarName {
							return
						}
						if skipWhenDispatch && isInsideWhenDispatchOnCatchVarFlat(file, isNode, catchVarName) {
							return
						}
						ctx.EmitAt(file.FlatRow(isNode)+1, file.FlatCol(isNode)+1,
							"Instance-of check for exception type inside catch block. Use specific catch clauses instead.")
					})
				}
				if file.Language == scanner.LangJava {
					file.FlatWalkNodes(idx, "instanceof_expression", func(instanceOfNode uint32) {
						text := file.FlatNodeText(instanceOfNode)
						if !javaInstanceOfExceptionRe.MatchString(text) {
							return
						}
						lhs := strings.TrimSpace(strings.Split(text, "instanceof")[0])
						if lhs != catchVarName {
							return
						}
						ctx.EmitAt(file.FlatRow(instanceOfNode)+1, file.FlatCol(instanceOfNode)+1,
							"Instance-of check for exception type inside catch block. Use specific catch clauses instead.")
					})
				}
			},
		})
	}
	{
		r := &NotImplementedDeclarationRule{BaseRule: BaseRule{RuleName: "NotImplementedDeclaration", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects TODO() calls that throw NotImplementedError at runtime."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "TODO" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"TODO() call found. Replace with an actual implementation.")
			},
		})
	}
	{
		r := &RethrowCaughtExceptionRule{BaseRule: BaseRule{RuleName: "RethrowCaughtException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks whose only statement is rethrowing the caught exception, making the catch block useless."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				caughtVar := extractCaughtVarNameFlat(file, idx)
				if caughtVar == "" {
					return
				}
				if file.Language == scanner.LangJava {
					if javaCatchOnlyRethrowsVar(file, idx, caughtVar) {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Caught exception '%s' is immediately rethrown. Remove the catch block or add handling logic.", caughtVar))
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
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Caught exception '%s' is immediately rethrown. Remove the catch block or add handling logic.", caughtVar))
					}
				}
			},
		})
	}
	{
		r := &ReturnFromFinallyRule{BaseRule: BaseRule{RuleName: "ReturnFromFinally", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects return statements inside finally blocks that can swallow exceptions from the try/catch."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"finally_block", "finally_clause"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
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
						f := r.Finding(file, file.FlatRow(jumpNode)+1, file.FlatCol(jumpNode)+1,
							"Return from finally block. This can swallow exceptions from try/catch.")
						lineIdx := file.FlatRow(jumpNode)
						lineStart := file.LineOffset(lineIdx)
						lineEnd := lineStart + len(file.Lines[lineIdx]) + 1
						if lineEnd > len(file.Content) {
							lineEnd = len(file.Content)
						}
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   lineStart,
							EndByte:     lineEnd,
							Replacement: "",
						}
						ctx.Emit(f)
					})
				}
			},
		})
	}
	{
		r := &SwallowedExceptionRule{BaseRule: BaseRule{RuleName: "SwallowedException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks that silently swallow the caught exception without logging, handling, or rethrowing."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes:      []string{"catch_block"},
			Confidence:     0.75,
			Fix:            v2.FixSemantic,
			Implementation: r,
			Check:          r.checkSwallowedException,
		})
	}
	{
		r := &ThrowingExceptionFromFinallyRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionFromFinally", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside finally blocks that can mask exceptions from the try/catch."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"finally_block", "finally_clause"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					f := r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						"Exception thrown inside finally block. This can swallow exceptions from try/catch.")
					lineIdx := file.FlatRow(throwNode)
					lineStart := file.LineOffset(lineIdx)
					lineEnd := lineStart + len(file.Lines[lineIdx]) + 1
					if lineEnd > len(file.Content) {
						lineEnd = len(file.Content)
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   lineStart,
						EndByte:     lineEnd,
						Replacement: "",
					}
					ctx.Emit(f)
				})
			},
		})
	}
	{
		r := &ThrowingExceptionsWithoutMessageOrCauseRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionsWithoutMessageOrCause", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects common exception types thrown without a descriptive message or cause argument."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				isThrow := false
				parent, ok := file.FlatParent(idx)
				if ok {
					if file.FlatType(parent) == "jump_expression" {
						text := file.FlatNodeText(parent)
						isThrow = strings.HasPrefix(strings.TrimSpace(text), "throw")
					}
					if file.FlatType(parent) == "statements" {
						for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
							if file.FlatType(prev) != "jump_expression" {
								continue
							}
							text := file.FlatNodeText(prev)
							if strings.HasPrefix(strings.TrimSpace(text), "throw") {
								isThrow = true
								break
							}
						}
					}
				}
				if !isThrow {
					return
				}
				exName := ""
				if file.FlatChildCount(idx) > 0 {
					first := file.FlatChild(idx, 0)
					if file.FlatType(first) == "simple_identifier" {
						exName = file.FlatNodeText(first)
					}
				}
				if exName == "" {
					return
				}
				exceptionSet := r.exceptionAllowlist()
				if !experiment.Enabled("exceptions-allowlist-cache") && len(r.Exceptions) > 0 {
					exceptionSet = make(map[string]bool, len(r.Exceptions))
					for _, e := range r.Exceptions {
						exceptionSet[e] = true
					}
				}
				if !exceptionSet[exName] {
					return
				}
				argCount := throwingExceptionArgCountFlat(file, idx)
				if argCount < 0 {
					return
				}
				if argCount > 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Exception '%s' thrown without a message or cause. Provide a descriptive message.", exName))
			},
		})
	}
	{
		r := &ThrowingNewInstanceOfSameExceptionRule{BaseRule: BaseRule{RuleName: "ThrowingNewInstanceOfSameException", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catch blocks that throw a new instance of the same exception type instead of rethrowing the original."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block", "catch_clause"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				caughtType := extractCaughtTypeNameFlat(file, idx)
				if caughtType == "" {
					return
				}
				caughtVar := extractCaughtVarNameFlat(file, idx)
				walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
					throwText := file.FlatNodeText(throwNode)
					pattern := fmt.Sprintf(`throw\s+(?:new\s+)?%s\s*\(`, regexp.QuoteMeta(caughtType))
					matched, err := regexp.MatchString(pattern, throwText)
					if err != nil {
						matched = strings.Contains(throwText, "throw "+caughtType+"(")
					}
					if !matched {
						return
					}
					if caughtVar != "" {
						parenIdx := strings.Index(throwText, "(")
						if parenIdx >= 0 {
							argsText := strings.TrimSpace(throwText[parenIdx+1:])
							closeIdx := strings.LastIndex(argsText, ")")
							if closeIdx >= 0 {
								argsText = strings.TrimSpace(argsText[:closeIdx])
								if strings.Contains(argsText, ",") {
									argPattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(caughtVar))
									if m, _ := regexp.MatchString(argPattern, argsText); m {
										return
									}
								}
							}
						}
					}
					ctx.EmitAt(file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
						fmt.Sprintf("New instance of '%s' thrown inside catch block that already catches it. Simply rethrow the original.", caughtType))
				})
			},
		})
	}
	{
		r := &ThrowingExceptionInMainRule{BaseRule: BaseRule{RuleName: "ThrowingExceptionInMain", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throw statements inside the main() function instead of graceful error handling."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "method_declaration"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "main" {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if !strings.HasPrefix(text, "error(") {
					return
				}
				argText := text[6 : len(text)-1]
				argText = strings.TrimSpace(argText)
				lower := strings.ToLower(argText)
				if strings.Contains(lower, "exception") || strings.Contains(lower, "throwable") ||
					strings.Contains(lower, "error") || lower == "e" || lower == "ex" || lower == "err" || lower == "t" {
					if !strings.HasPrefix(argText, "\"") {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("error(%s) passes a Throwable. Use throw instead, or pass the message string.", argText))
					}
				}
			},
		})
	}
	{
		r := &ObjectExtendsThrowableRule{BaseRule: BaseRule{RuleName: "ObjectExtendsThrowable", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects Kotlin object declarations that extend Throwable, which are singletons that lose stack trace information."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"object_declaration"}, Confidence: 0.75, Implementation: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
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
								ctx.EmitAt(file.FlatRow(idx)+1, 1,
									fmt.Sprintf("Object '%s' extends '%s'. Objects that extend Throwable are singletons and lose stack trace information.", name, simpleName))
								return
							}
						}
						return
					}
				}
				text := file.FlatNodeText(idx)
				for _, t := range throwableBaseTypes {
					if strings.Contains(text, ": "+t) || strings.Contains(text, ":"+t+"(") || strings.Contains(text, ": "+t+"(") {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Object '%s' extends '%s'. Objects that extend Throwable are singletons and lose stack trace information.", name, t))
						return
					}
				}
			},
		})
	}
}
