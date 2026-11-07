package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerPotentialbugsLifecycleRules() {

	// --- from potentialbugs_lifecycle.go ---
	{
		r := &ExitOutsideMainRule{BaseRule: BaseRule{RuleName: "ExitOutsideMain", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects exitProcess() or System.exit() calls outside of the main function."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !exitOutsideMainCallMatches(file, idx) {
					return
				}
				if exitOutsideMainInsideMain(file, idx) {
					return
				}
				ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
					"Do not call exitProcess() or System.exit() outside of the main function.")
			},
		})
	}
	{
		r := &ExplicitGarbageCollectionCallRule{BaseRule: BaseRule{RuleName: "ExplicitGarbageCollectionCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects explicit garbage collection or finalization calls such as System.gc() and Runtime.getRuntime().gc(), which rarely improve performance."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := strings.TrimPrefix(strings.Join(strings.Fields(file.FlatNodeText(idx)), ""), "java.lang.")
				switch text {
				case "System.gc()", "Runtime.getRuntime().gc()", "System.runFinalization()", "Runtime.getRuntime().runFinalization()":
				default:
					return
				}
				// Tests call gc() to force finalization or measure leaks.
				if scanner.IsTestFile(file.Path) {
					return
				}
				if file.Language == scanner.LangJava {
					switch text {
					case "System.gc()", "System.runFinalization()":
						if !javaSourceResolvesSimpleType(file, "System", "java.lang.System") {
							return
						}
					case "Runtime.getRuntime().gc()", "Runtime.getRuntime().runFinalization()":
						if !javaSourceResolvesSimpleType(file, "Runtime", "java.lang.Runtime") {
							return
						}
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Do not call garbage collector explicitly. It is rarely necessary and can degrade performance.")
				callStart := int(file.FlatStartByte(idx))
				callEnd := int(file.FlatEndByte(idx))
				// Only autofix when the call stands alone as a statement on its
				// own line. Otherwise (e.g. `foo(System.gc())`,
				// `if (cond) System.gc();`) removing the line would delete
				// surrounding code.
				lineStart := callStart
				for lineStart > 0 && file.Content[lineStart-1] != '\n' {
					lineStart--
				}
				prefixOnlyWS := true
				for i := lineStart; i < callStart; i++ {
					if file.Content[i] != ' ' && file.Content[i] != '\t' {
						prefixOnlyWS = false
						break
					}
				}
				cursor := callEnd
				for cursor < len(file.Content) && (file.Content[cursor] == ' ' || file.Content[cursor] == '\t') {
					cursor++
				}
				if cursor < len(file.Content) && file.Content[cursor] == ';' {
					cursor++
				}
				for cursor < len(file.Content) && (file.Content[cursor] == ' ' || file.Content[cursor] == '\t' || file.Content[cursor] == '\r') {
					cursor++
				}
				atLineEnd := cursor >= len(file.Content) || file.Content[cursor] == '\n'
				if prefixOnlyWS && atLineEnd {
					if cursor < len(file.Content) && file.Content[cursor] == '\n' {
						cursor++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   lineStart,
						EndByte:     cursor,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &InvalidRangeRule{BaseRule: BaseRule{RuleName: "InvalidRange", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects empty ranges made from literals, such as 10..1, 1 downTo 10 or 5 until 1."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"range_expression", "infix_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
				if left == 0 || right == 0 {
					return
				}
				lower, okLower := intLiteralValue(file, left)
				upper, okUpper := intLiteralValue(file, right)
				if !okLower || !okUpper {
					return
				}
				startText := file.FlatNodeText(left)
				endText := file.FlatNodeText(right)
				if file.FlatType(idx) == "infix_expression" {
					// Only the infix forms `a downTo b` and `a until b` build a range.
					op := file.FlatNodeText(file.FlatChild(idx, 1))
					switch {
					case op == "downTo" && lower < upper:
						ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Invalid range: %s downTo %s. The range is empty. Use '..' for ascending ranges.", startText, endText)))
					case op == "until" && lower >= upper:
						// detekt flags `a until a` too: the range is empty.
						ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Invalid range: %s until %s. The range is empty. Use 'downTo' for descending ranges.", startText, endText)))
					}
					return
				}
				if strings.Contains(file.FlatNodeText(idx), "..<") {
					// `a..<a` is empty as well, so only a strictly ascending bound is clean.
					if lower < upper {
						return
					}
					ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Invalid range: %s..<%s. The range is empty. Use 'downTo' for descending ranges.", startText, endText)))
					return
				}
				if lower <= upper {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Invalid range: %s..%s. The range is empty. Use 'downTo' for descending ranges.", startText, endText))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: startText + " downTo " + endText,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &IteratorHasNextCallsNextMethodRule{BaseRule: BaseRule{RuleName: "IteratorHasNextCallsNextMethod", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects hasNext() implementations that call next(), which modifies iterator state."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "object_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasSuperNamed(file, idx, "Iterator") {
					return
				}
				if fn := memberFunctionNamed(file, idx, "hasNext"); fn != 0 && hasNextCallsNext(file, fn) {
					emitAtClassName(ctx, idx, "Calling hasNext() on an Iterator should have no side-effects. Calling next() is a side effect.")
				}
			},
		})
	}
	{
		r := &IteratorNotThrowingNoSuchElementExceptionRule{BaseRule: BaseRule{RuleName: "IteratorNotThrowingNoSuchElementException", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Iterator.next() implementations that do not throw NoSuchElementException when exhausted."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "object_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasSuperNamed(file, idx, "Iterator") {
					return
				}
				if fn := memberFunctionNamed(file, idx, "next"); fn != 0 && !throwsNoSuchElement(file, fn) {
					emitAtClassName(ctx, idx, "This implementation of Iterator does not correctly implement the next() method as it doesn't throw a NoSuchElementException when no elements remain in the Iterator.")
				}
			},
		})
	}
	{
		r := &LateinitUsageRule{BaseRule: BaseRule{RuleName: "LateinitUsage", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects lateinit var usage and suggests lazy initialization or nullable types instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
					return
				}
				if !file.FlatHasModifier(idx, "lateinit") {
					return
				}
				if deadCodeDeclarationHasDIAnnotation(file, idx) {
					return
				}
				// IgnoreOnClassesPattern: skip lateinit usages whose
				// enclosing class name matches the configured pattern.
				// Common usage is to allow `lateinit var ...` in fixture/test
				// base classes named like `*Test` or `*Spec`.
				if r.IgnoreOnClassesPattern != nil {
					if owner, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration"); ok {
						clsName := extractIdentifierFlat(file, owner)
						if clsName != "" && r.IgnoreOnClassesPattern.MatchString(clsName) {
							return
						}
					}
				}
				// detekt reports the declaration node, which starts at the comment directly above it.
				at := leadingCommentStart(file, idx)
				ctx.EmitAt(file.FlatRow(at)+1, file.FlatCol(at)+1,
					"'lateinit' usage detected. Consider using lazy initialization or nullable types instead.")
			},
		})
	}
	{
		r := &MissingPackageDeclarationRule{BaseRule: BaseRule{RuleName: "MissingPackageDeclaration", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Kotlin or Java source files that are missing a package declaration."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"source_file", "program"}, Languages: []scanner.Language{scanner.LangKotlin}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingSuperCallRule{
			BaseRule: BaseRule{RuleName: "MissingSuperCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects override functions that do not call the corresponding super method."},
			MustInvokeSuperAnnotations: []string{
				"androidx.annotation.CallSuper",
				"javax.annotation.OverridingMethodsMustInvokeSuper",
			},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "override") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				superFound := false
				file.FlatWalkNodes(body, "call_expression", func(callNode uint32) {
					if superFound {
						return
					}
					callText := file.FlatNodeText(callNode)
					if strings.Contains(callText, "super."+name+"(") || strings.Contains(callText, "super<") {
						superFound = true
					}
				})
				if !superFound {
					bodyText := file.FlatNodeText(body)
					if strings.Contains(bodyText, "super<") {
						superFound = true
					}
				}
				if superFound {
					return
				}
				if !missingSuperCallHasRequiredSuperEvidence(file, idx, name) &&
					!missingSuperCallParentMethodHasAnnotation(file, idx, name, r.MustInvokeSuperAnnotations) {
					return
				}
				f := r.Finding(file, declLine(file, idx), declCol(file, idx),
					fmt.Sprintf("Override function '%s' does not call super.%s().", name, name))
				bodyText := file.FlatNodeText(body)
				if strings.HasPrefix(strings.TrimSpace(bodyText), "{") {
					bracePos := int(file.FlatStartByte(body)) + strings.Index(bodyText, "{") + 1
					funcLine := file.Lines[file.FlatRow(idx)]
					indent := ""
					for _, ch := range funcLine {
						if ch == ' ' || ch == '\t' {
							indent += string(ch)
						} else {
							break
						}
					}
					insertion := "\n" + indent + "    super." + name + "()"
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   bracePos,
						EndByte:     bracePos,
						Replacement: insertion,
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &MissingUseCallRule{BaseRule: BaseRule{RuleName: "MissingUseCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Closeable or AutoCloseable resources opened without a .use {} block."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				callee, ok := missingUseCloseableConstructorConfirmed(ctx, idx)
				if !ok {
					return
				}
				if missingUseHasUseChainFlat(file, idx) {
					return
				}
				if missingUseAssignedWithUseFlat(file, idx) {
					return
				}
				if missingUseIsClassPropertyFlat(file, idx) {
					return
				}
				if missingUseIsArgumentFlat(file, idx) {
					return
				}
				if missingUseIsReturnExpressionFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("%s opened without .use {}. This may lead to resource leaks.", callee))
			},
		})
	}
}
