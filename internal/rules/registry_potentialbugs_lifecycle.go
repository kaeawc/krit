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
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !exitOutsideMainCallMatches(file, idx) {
					return
				}
				if exitOutsideMainInsideMain(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Do not call exitProcess() or System.exit() outside of the main function.")
			},
		})
	}
	{
		r := &ExplicitGarbageCollectionCallRule{BaseRule: BaseRule{RuleName: "ExplicitGarbageCollectionCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects explicit calls to System.gc() or Runtime.getRuntime().gc() which rarely improve performance."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if text != "System.gc()" && text != "Runtime.getRuntime().gc()" {
					return
				}
				if file.Language == scanner.LangJava {
					switch text {
					case "System.gc()":
						if !javaSourceResolvesSimpleType(file, "System", "java.lang.System") {
							return
						}
					case "Runtime.getRuntime().gc()":
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
		r := &InvalidRangeRule{BaseRule: BaseRule{RuleName: "InvalidRange", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects backwards ranges like 10..1 that produce empty ranges instead of using downTo."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"range_expression"}, Confidence: 0.75, Fix: api.FixIdiomatic, Implementation: r,
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
				if file.FlatType(left) != "integer_literal" || file.FlatType(right) != "integer_literal" {
					return
				}
				startText := file.FlatNodeText(left)
				endText := file.FlatNodeText(right)
				if len(startText) > len(endText) || (len(startText) == len(endText) && startText > endText) {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Invalid range: %s..%s. The range is empty. Use 'downTo' for descending ranges.", startText, endText))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: startText + " downTo " + endText,
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &IteratorHasNextCallsNextMethodRule{BaseRule: BaseRule{RuleName: "IteratorHasNextCallsNextMethod", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects hasNext() implementations that call next(), which modifies iterator state."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Needs: api.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "hasNext" {
					return
				}
				if !enclosingImplementsIteratorFlat(ctx, idx) {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				found := false
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					if found {
						return
					}
					if flatCallExpressionName(file, callIdx) == "next" {
						found = true
					}
				})
				if found {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"hasNext() should not call next(). This modifies the iterator state.")
				}
			},
		})
	}
	{
		r := &IteratorNotThrowingNoSuchElementExceptionRule{BaseRule: BaseRule{RuleName: "IteratorNotThrowingNoSuchElementException", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Iterator.next() implementations that do not throw NoSuchElementException when exhausted."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Needs: api.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "next" {
					return
				}
				if !enclosingImplementsIteratorFlat(ctx, idx) {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				if !functionThrowsNoSuchElementExceptionFlat(ctx, body) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Iterator's next() method should throw NoSuchElementException when there are no more elements.")
				}
			},
		})
	}
	{
		r := &LateinitUsageRule{BaseRule: BaseRule{RuleName: "LateinitUsage", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects lateinit var usage and suggests lazy initialization or nullable types instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Implementation: r,
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"'lateinit' usage detected. Consider using lazy initialization or nullable types instead.")
			},
		})
	}
	{
		r := &MissingPackageDeclarationRule{BaseRule: BaseRule{RuleName: "MissingPackageDeclaration", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Kotlin or Java source files that are missing a package declaration."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Fix: api.FixCosmetic, Implementation: r,
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
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
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
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
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
			Confidence: 0.75, Implementation: r,
			OracleCallTargets:      &api.OracleCallTargetFilter{CalleeNames: closeableConstructorCallees()},
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
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
