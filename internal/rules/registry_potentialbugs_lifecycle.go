package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerPotentialbugsLifecycleRules() {

	// --- from potentialbugs_lifecycle.go ---
	{
		r := &ExitOutsideMainRule{BaseRule: BaseRule{RuleName: "ExitOutsideMain", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects exitProcess() or System.exit() calls outside of the main function."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if file.Language == scanner.LangJava && strings.HasPrefix(text, "System.exit(") && !javaSourceResolvesSimpleType(file, "System", "java.lang.System") {
					return
				}
				if !strings.HasPrefix(text, "exitProcess(") && !strings.HasPrefix(text, "System.exit(") {
					return
				}
				for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
					if file.FlatType(parent) == "function_declaration" || file.FlatType(parent) == "method_declaration" {
						name := extractIdentifierFlat(file, parent)
						if name == "main" {
							return
						}
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Do not call exitProcess() or System.exit() outside of the main function.")
			},
		})
	}
	{
		r := &ExplicitGarbageCollectionCallRule{BaseRule: BaseRule{RuleName: "ExplicitGarbageCollectionCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects explicit calls to System.gc() or Runtime.getRuntime().gc() which rarely improve performance."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
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
				startByte := int(file.FlatStartByte(idx))
				endByte := int(file.FlatEndByte(idx))
				for startByte > 0 && file.Content[startByte-1] != '\n' {
					startByte--
				}
				if endByte < len(file.Content) && file.Content[endByte] == '\n' {
					endByte++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     endByte,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &InvalidRangeRule{BaseRule: BaseRule{RuleName: "InvalidRange", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects backwards ranges like 10..1 that produce empty ranges instead of using downTo."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"range_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Needs: v2.NeedsResolver, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
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
				// Common detekt usage is to allow `lateinit var ...` in
				// fixture/test base classes named like `*Test` or `*Spec`.
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Fix: v2.FixCosmetic, Implementation: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
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
				if !missingSuperCallHasRequiredSuperEvidence(file, idx, name) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, Implementation: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: closeableConstructorCallees()},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
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
