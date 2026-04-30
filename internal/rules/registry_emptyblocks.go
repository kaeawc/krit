package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"regexp"
	"strings"
)

func registerEmptyblocksRules() {

	// --- from emptyblocks.go ---
	{
		r := &EmptyCatchBlockRule{BaseRule: BaseRule{RuleName: "EmptyCatchBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects catch blocks with an empty body that silently swallow exceptions."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				if r.AllowedExceptionNameRegex != nil {
					caughtVar := extractCaughtVarNameFlat(file, idx)
					if caughtVar != "" && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty catch block detected. Empty catch blocks should be avoided.")
				nodeText := file.FlatNodeText(idx)
				braceStart := strings.Index(nodeText, "{")
				braceEnd := strings.LastIndex(nodeText, "}")
				if braceStart >= 0 && braceEnd > braceStart {
					indent := detectIndent(file.Content, int(file.FlatStartByte(idx)))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)) + braceStart,
						EndByte:     int(file.FlatStartByte(idx)) + braceEnd + 1,
						Replacement: "{\n" + indent + "    // TODO: handle exception\n" + indent + "}",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyClassBlockRule{BaseRule: BaseRule{RuleName: "EmptyClassBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects class declarations with an empty body that can have their braces removed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_body"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "object_literal" {
					return
				}
				text := file.FlatNodeText(idx)
				inner := strings.TrimSpace(text)
				if len(inner) >= 2 && inner[0] == '{' && inner[len(inner)-1] == '}' {
					body := strings.TrimSpace(inner[1 : len(inner)-1])
					cleaned := stripComments(body)
					if strings.TrimSpace(cleaned) == "" {
						f := r.Finding(file, file.FlatRow(idx)+1, 1,
							"Empty class body detected. Consider removing the empty braces.")
						startByte := int(file.FlatStartByte(idx))
						for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
							startByte--
						}
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   startByte,
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: "",
						}
						ctx.Emit(f)
					}
				}
			},
		})
	}
	{
		r := &EmptyDefaultConstructorRule{BaseRule: BaseRule{RuleName: "EmptyDefaultConstructor", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects explicit empty default constructors that are redundant and can be removed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "annotation") {
					return
				}
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				if ctor == 0 {
					return
				}
				ctorText := file.FlatNodeText(ctor)
				emptyCtorRe := regexp.MustCompile(`constructor\s*\(\s*\)`)
				emptyParenRe := regexp.MustCompile(`^\s*\(\s*\)\s*$`)
				if !emptyCtorRe.MatchString(ctorText) && !emptyParenRe.MatchString(ctorText) {
					return
				}
				if file.FlatHasModifier(ctor, "private") ||
					file.FlatHasModifier(ctor, "internal") ||
					file.FlatHasModifier(ctor, "protected") ||
					file.FlatHasModifier(ctor, "public") {
					return
				}
				if mods, ok := file.FlatFindChild(ctor, "modifiers"); ok && file.FlatHasChildOfType(mods, "annotation") {
					return
				}
				if deadCodeDeclarationHasDIAnnotation(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(ctor)+1, 1,
					"Empty default constructor detected. It can be removed.")
				startByte := int(file.FlatStartByte(ctor))
				for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
					startByte--
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     int(file.FlatEndByte(ctor)),
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyDoWhileBlockRule{BaseRule: BaseRule{RuleName: "EmptyDoWhileBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects do-while loops with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"do_while_statement"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty do-while block detected.")
				doS, doE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   doS,
					EndByte:     doE,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyElseBlockRule{BaseRule: BaseRule{RuleName: "EmptyElseBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects else blocks with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Find the `else` token and its companion control_structure_body.
				var elseTok, elseBody uint32
				sawElse := false
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "else" {
						elseTok = child
						sawElse = true
						continue
					}
					if sawElse && file.FlatType(child) == "control_structure_body" {
						elseBody = child
						break
					}
				}
				if elseBody == 0 {
					return
				}
				// Must be a braced body — `else expr` form is never "empty".
				if !controlBodyHasBraces(file, elseBody) {
					return
				}
				// Empty iff no `statements` child — comments and whitespace
				// inside the braces don't produce a statements node.
				if file.FlatHasChildOfType(elseBody, "statements") {
					return
				}
				f := r.Finding(file, file.FlatRow(elseTok)+1, 1,
					"Empty else block detected.")
				elseByteStart := int(file.FlatStartByte(elseTok))
				for elseByteStart > 0 && (file.Content[elseByteStart-1] == ' ' || file.Content[elseByteStart-1] == '\t' || file.Content[elseByteStart-1] == '\n' || file.Content[elseByteStart-1] == '\r') {
					elseByteStart--
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   elseByteStart,
					EndByte:     int(file.FlatEndByte(elseBody)),
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyFinallyBlockRule{BaseRule: BaseRule{RuleName: "EmptyFinallyBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects finally blocks with an empty body that serve no purpose."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"finally_block"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty finally block detected.")
				startByte := int(file.FlatStartByte(idx))
				for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t' || file.Content[startByte-1] == '\n' || file.Content[startByte-1] == '\r') {
					startByte--
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyForBlockRule{BaseRule: BaseRule{RuleName: "EmptyForBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects for loops with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				body, _ := file.FlatFindChild(idx, "control_structure_body")
				if body == 0 {
					return
				}
				bodyText := file.FlatNodeText(body)
				if !strings.Contains(bodyText, "{") {
					return
				}
				if !isBlockEmptyFlat(file, body) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty for block detected.")
				forS, forE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   forS,
					EndByte:     forE,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyFunctionBlockRule{BaseRule: BaseRule{RuleName: "EmptyFunctionBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects function declarations with an empty body."}, IgnoreOverridden: false}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "open") {
					return
				}
				isOverride := file.FlatHasModifier(idx, "override")
				if isOverride && r.IgnoreOverridden {
					return
				}
				if HasIgnoredAnnotation(file.FlatNodeText(idx),
					[]string{"Inject", "Provides", "Binds", "BindsInstance",
						"BindsOptionalOf", "IntoSet", "IntoMap", "ElementsIntoSet",
						"Multibinds", "ContributesBinding", "ContributesMultibinding",
						"ContributesTo", "ContributesSubcomponent"}) {
					return
				}
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "class_declaration" {
						break
					}
					if file.FlatType(p) == "interface" {
						return
					}
				}
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) != "class_declaration" {
						continue
					}
					for i := 0; i < file.FlatChildCount(p); i++ {
						if file.FlatType(file.FlatChild(p, i)) == "interface" {
							return
						}
					}
					break
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				bodyText := file.FlatNodeText(body)
				if !strings.Contains(bodyText, "{") {
					return
				}
				if !isBlockEmptyFlat(file, body) {
					return
				}
				inner := bodyText
				if i := strings.Index(inner, "{"); i >= 0 {
					inner = inner[i+1:]
				}
				if j := strings.LastIndex(inner, "}"); j >= 0 {
					inner = inner[:j]
				}
				trimmedInner := strings.TrimSpace(inner)
				if strings.HasPrefix(trimmedInner, "//") || strings.HasPrefix(trimmedInner, "/*") ||
					strings.Contains(trimmedInner, "TODO") {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty function body detected.")
				braceStart := strings.Index(bodyText, "{")
				braceEnd := strings.LastIndex(bodyText, "}")
				if braceStart >= 0 && braceEnd > braceStart {
					indent := detectIndent(file.Content, int(file.FlatStartByte(body)))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(body)) + braceStart,
						EndByte:     int(file.FlatStartByte(body)) + braceEnd + 1,
						Replacement: "{\n" + indent + "    TODO(\"Not yet implemented\")\n" + indent + "}",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyIfBlockRule{BaseRule: BaseRule{RuleName: "EmptyIfBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects if blocks with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				condEnd := strings.Index(text, ")")
				if condEnd < 0 {
					return
				}
				rest := text[condEnd+1:]
				elseIdx := strings.Index(rest, "else")
				ifPart := rest
				if elseIdx >= 0 {
					ifPart = rest[:elseIdx]
				}
				braceStart := strings.Index(ifPart, "{")
				if braceStart < 0 {
					return
				}
				braceEnd := strings.LastIndex(ifPart, "}")
				if braceEnd <= braceStart {
					return
				}
				body := ifPart[braceStart+1 : braceEnd]
				cleaned := stripComments(body)
				if strings.TrimSpace(cleaned) == "" {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Empty if block detected.")
					ifS, ifE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   ifS,
						EndByte:     ifE,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EmptyInitBlockRule{BaseRule: BaseRule{RuleName: "EmptyInitBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects init blocks with an empty body that can be removed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"anonymous_initializer"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty init block detected.")
				initS, initE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   initS,
					EndByte:     initE,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyKotlinFileRule{BaseRule: BaseRule{RuleName: "EmptyKotlinFile", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects Kotlin files with no meaningful code beyond package and import declarations."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &EmptySecondaryConstructorRule{BaseRule: BaseRule{RuleName: "EmptySecondaryConstructor", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects secondary constructors with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"secondary_constructor"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "{") {
					return
				}
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty secondary constructor detected.")
				braceStart := strings.LastIndex(nodeText, "{")
				if braceStart >= 0 {
					removStart := int(file.FlatStartByte(idx)) + braceStart
					for removStart > int(file.FlatStartByte(idx)) && (file.Content[removStart-1] == ' ' || file.Content[removStart-1] == '\t') {
						removStart--
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   removStart,
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EmptyTryBlockRule{BaseRule: BaseRule{RuleName: "EmptyTryBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects try blocks with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"try_expression"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				tryIdx := strings.Index(text, "try")
				if tryIdx < 0 {
					return
				}
				afterTry := text[tryIdx+3:]
				braceStart := strings.Index(afterTry, "{")
				if braceStart < 0 {
					return
				}
				depth := 0
				braceEnd := -1
				for i := braceStart; i < len(afterTry); i++ {
					if afterTry[i] == '{' {
						depth++
					} else if afterTry[i] == '}' {
						depth--
						if depth == 0 {
							braceEnd = i
							break
						}
					}
				}
				if braceEnd < 0 {
					return
				}
				body := afterTry[braceStart+1 : braceEnd]
				cleaned := stripComments(body)
				if strings.TrimSpace(cleaned) == "" {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Empty try block detected.")
					tryS, tryE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   tryS,
						EndByte:     tryE,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EmptyWhenBlockRule{BaseRule: BaseRule{RuleName: "EmptyWhenBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects when expressions with no entries."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				hasEntries := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "when_entry" {
						hasEntries = true
						break
					}
				}
				if hasEntries {
					return
				}
				if isBlockEmptyFlat(file, idx) {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Empty when block detected.")
					whenS, whenE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   whenS,
						EndByte:     whenE,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EmptyWhileBlockRule{BaseRule: BaseRule{RuleName: "EmptyWhileBlock", RuleSetName: "empty-blocks", Sev: "warning", Desc: "Detects while loops with an empty body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"while_statement"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBlockEmptyFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Empty while block detected.")
				whileS, whileE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   whileS,
					EndByte:     whileE,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
}
