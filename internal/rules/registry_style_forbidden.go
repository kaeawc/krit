package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleForbiddenRules() {
	registerStyleWildcardImport()
	registerStyleForbiddenComment()
	registerStyleForbiddenVoid()
	registerStyleForbiddenImport()
	registerStyleForbiddenMethodCall()
	registerStyleForbiddenAnnotation()
	registerStyleForbiddenNamedParam()
	registerStyleForbiddenOptIn()
	registerStyleForbiddenSuppress()
	registerStyleMagicNumber()
}

func registerStyleWildcardImport() {
	r := &WildcardImportRule{BaseRule: BaseRule{RuleName: "WildcardImport", RuleSetName: "style", Sev: "warning", Desc: "Detects wildcard import statements that should be replaced with explicit imports."}, ExcludeImports: []string{"java.util.*", "platform.**", "kotlinx.cinterop.*"}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"import_header", "import_declaration"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Skip test files and test-fixture Kotlin sources. Fixtures routinely
			// use wildcard imports of an API surface being tested (e.g. KSP's
			// integration-tests copy Kotlin files that `import foo.bar.*`).
			if scanner.IsTestFile(file.Path) {
				return
			}
			imp := importStatementPath(file, idx)
			if !strings.HasSuffix(imp, ".*") {
				return
			}
			for _, excl := range r.ExcludeImports {
				if wildcardImportExcluded(imp, excl) {
					return
				}
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Wildcard import '%s' should be replaced with explicit imports.", imp))
		},
	})
}

// forbiddenCommentReasons holds detekt's default reasons; other markers get its generic message.
var forbiddenCommentReasons = map[string]string{
	"FIXME:":    "Forbidden FIXME todo marker in comment, please fix the problem.",
	"STOPSHIP:": "Forbidden STOPSHIP todo marker in comment, please address the problem before shipping the code.",
	"TODO:":     "Forbidden TODO todo marker in comment, please do the changes.",
}

func forbiddenCommentMessage(marker string) string {
	if reason, ok := forbiddenCommentReasons[marker]; ok {
		return reason
	}
	return "This comment contains '" + marker + "' that has been defined as forbidden."
}

// kdocDefaultSection returns the byte offset in a KDoc comment where its default section starts and
// that section's text. Like detekt, tag sections (@param and so on) are not checked.
func kdocDefaultSection(text string) (offset int, content string) {
	offset = len("/**")
	rest := text[offset:]
	// A section that starts on the next line begins at its first non-blank character.
	if first, _, hasNL := strings.Cut(rest, "\n"); hasNL && strings.TrimSpace(first) == "" {
		skip := len(first) + 1
		for skip < len(rest) && (rest[skip] == ' ' || rest[skip] == '\t') {
			skip++
		}
		offset += skip
	}
	body := forbiddenCommentBody(text)
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "@") {
			break
		}
		kept = append(kept, line)
	}
	return offset, strings.Join(kept, "\n")
}

func registerStyleForbiddenComment() {
	r := &ForbiddenCommentRule{BaseRule: BaseRule{RuleName: "ForbiddenComment", RuleSetName: "style", Sev: "warning", Desc: "Detects comments containing forbidden markers like TODO, FIXME, or STOPSHIP."}, Comments: defaultForbiddenCommentMarkers}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"line_comment", "multiline_comment", "block_comment"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Justified string match: tree-sitter exposes comments as leaf text
			// nodes, so marker detection has no AST body node to dispatch on.
			raw := file.FlatNodeText(idx)
			text := forbiddenCommentBody(raw)
			row, col := file.FlatRow(idx)+1, file.FlatCol(idx)+1
			// detekt checks only the default section of a KDoc and reports at its start.
			if strings.HasPrefix(raw, "/**") && !strings.HasPrefix(raw, "/**/") {
				var offset int
				offset, text = kdocDefaultSection(raw)
				if n := strings.Count(raw[:offset], "\n"); n > 0 {
					row += n
					col = offset - strings.LastIndex(raw[:offset], "\n")
				} else {
					col += offset
				}
			}
			// If the comment matches the allowed pattern, skip it.
			if r.AllowedPatterns != nil && r.AllowedPatterns.MatchString(text) {
				return
			}
			for _, value := range r.Values {
				if strings.Contains(strings.ToLower(text), strings.ToLower(value)) {
					ctx.EmitAt(row, col, "This comment contains '"+value+"' that has been defined as forbidden.")
				}
			}
			markers := r.Comments
			if len(markers) == 0 {
				markers = defaultForbiddenCommentMarkers
			}
			for _, marker := range markers {
				if strings.Contains(text, marker) {
					if forbiddenCommentAllowedByDefault(text, marker) || markerOnlyInsideQuotedText(text, marker) {
						continue
					}
					ctx.EmitAt(row, col, forbiddenCommentMessage(marker))
				}
			}
		},
	})
}

func forbiddenCommentBody(text string) string {
	body := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(body, "//"):
		return strings.TrimPrefix(body[2:], " ")
	case strings.HasPrefix(body, "/**"):
		body = body[3:]
	case strings.HasPrefix(body, "/*"):
		body = body[2:]
	}
	body = strings.TrimSuffix(strings.TrimSpace(body), "*/")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		line = strings.TrimLeft(line, " \t")
		if strings.HasPrefix(line, "*") {
			line = strings.TrimPrefix(line[1:], " ")
		}
		lines[i] = line
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func registerStyleForbiddenVoid() {
	r := &ForbiddenVoidRule{BaseRule: BaseRule{RuleName: "ForbiddenVoid", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of the Java Void type that should be replaced with Kotlin Unit."}, IgnoreIdioms: true}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"user_type"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			text := file.FlatNodeText(idx)
			if text != "Void" && text != "java.lang.Void" {
				return
			}
			// Skip when used as a type argument to a Java-interop generic type.
			// Walk up the AST: user_type -> type_arguments -> user_type (the outer generic)
			inFunctionParam := false
			for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
				if file.FlatType(p) == "type_arguments" {
					// IgnoreUsageInGenerics: skip Void anywhere inside a
					// type-argument list, regardless of the outer generic.
					// This is broader than the hardcoded
					// javaInteropGenericTypes allowlist below.
					if r.IgnoreUsageInGenerics {
						return
					}
					if outer, ok := file.FlatParent(p); ok && r.IgnoreIdioms {
						outerText := file.FlatNodeText(outer)
						// Extract the outer type name (up to the first '<')
						if i := strings.Index(outerText, "<"); i >= 0 {
							outerName := strings.TrimSpace(outerText[:i])
							if dotIdx := strings.LastIndex(outerName, "."); dotIdx >= 0 {
								outerName = outerName[dotIdx+1:]
							}
							if javaInteropGenericTypes[outerName] {
								return
							}
						}
					}
				}
				// Detect function parameters — if Void is used as a parameter type in
				// an override function (override fun doInBackground(vararg params: Void?)),
				// it's required by the Java generic contract.
				if file.FlatType(p) == "parameter" || file.FlatType(p) == "value_parameter" || file.FlatType(p) == "function_value_parameter" {
					inFunctionParam = true
				}
				if file.FlatType(p) == "function_declaration" {
					if inFunctionParam && file.FlatHasModifier(p, "override") && (r.IgnoreIdioms || r.IgnoreOverridden) {
						return
					}
					if r.IgnoreOverridden && file.FlatHasModifier(p, "override") {
						return
					}
					break
				}
				if file.FlatType(p) == "class_declaration" {
					break
				}
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"'Void' should be replaced with 'Unit'.")
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: "Unit",
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleForbiddenImport() {
	r := &ForbiddenImportRule{BaseRule: BaseRule{RuleName: "ForbiddenImport", RuleSetName: "style", Sev: "warning", Desc: "Detects import statements matching configured forbidden patterns."}, Patterns: defaultForbiddenImports}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"import_header", "import_declaration"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			imp := importStatementPath(file, idx)
			if i := strings.Index(imp, " as "); i >= 0 {
				imp = strings.TrimSpace(imp[:i])
			}
			// Merge Patterns and ForbiddenImports
			patterns := r.Patterns
			if len(r.ForbiddenImports) > 0 {
				patterns = r.ForbiddenImports
			}
			// detekt matches the imported name, which has no trailing `.*` for a wildcard import.
			fqName := strings.TrimSuffix(imp, ".*")
			matched := false
			for _, p := range r.Imports {
				if importGlobMatches(p, fqName) {
					matched = true
					break
				}
			}
			for _, re := range r.ForbiddenPatterns {
				if re != nil && re.MatchString(fqName) {
					matched = true
					break
				}
			}
			if matched {
				patterns = append(append([]string(nil), patterns...), imp)
			}
			for _, p := range patterns {
				if importMatchesLegacyPattern(imp, p) {
					// Check allowed imports
					allowed := false
					for _, a := range r.AllowedImports {
						if strings.Contains(imp, a) {
							allowed = true
							break
						}
					}
					if allowed {
						continue
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("The import `%s` has been forbidden in the detekt config.", strings.TrimSpace(fqName)))
					// Removing the import leaves its uses unresolved, so this fix is semantic-level only.
					impEnd := int(file.FlatEndByte(idx))
					impEnd = lineBreakEnd(file.Content, impEnd)
					impStart := int(file.FlatStartByte(idx))
					for impStart > 0 && file.Content[impStart-1] != '\n' {
						impStart--
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   impStart,
						EndByte:     impEnd,
						Replacement: "",
					}
					ctx.Emit(f)
					return
				}
			}
		},
	})
}

// standaloneStatementLines returns the byte range of the whole lines (with the final line break) covered by
// a statement node, when the node is directly in a statement list and shares its lines with nothing else.
func standaloneStatementLines(file *scanner.File, idx uint32) (start, end int, ok bool) {
	if parent, found := file.FlatParent(idx); !found || file.FlatType(parent) != "statements" {
		return 0, 0, false
	}
	start, end = int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx))
	for start > 0 && file.Content[start-1] != '\n' {
		start--
		if c := file.Content[start]; c != ' ' && c != '\t' {
			return 0, 0, false
		}
	}
	for end < len(file.Content) && file.Content[end] != '\n' {
		if c := file.Content[end]; c != ' ' && c != '\t' && c != '\r' {
			return 0, 0, false
		}
		end++
	}
	if end < len(file.Content) {
		end++
	}
	return start, end, true
}

func registerStyleForbiddenMethodCall() {
	r := &ForbiddenMethodCallRule{BaseRule: BaseRule{RuleName: "ForbiddenMethodCall", RuleSetName: "style", Sev: "warning", Desc: "Detects calls to methods that are configured as forbidden."}, Methods: defaultForbiddenMethods}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression"},
		Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			methodName, ok := forbiddenMethodCallMatch(ctx, idx, r.Methods)
			if !ok {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("The method `%s` has been forbidden in the detekt config.", methodName))
			// Auto-fix: remove the call only when it is a statement alone on its lines. Deleting a call that
			// is a branch body, an argument or shares a line with other code would change or break the code.
			if start, end, ok := standaloneStatementLines(file, idx); ok {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   start,
					EndByte:     end,
					Replacement: "",
				}
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleForbiddenAnnotation() {
	r := &ForbiddenAnnotationRule{BaseRule: BaseRule{RuleName: "ForbiddenAnnotation", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of annotations that are configured as forbidden."}, Annotations: defaultForbiddenAnnotations}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			for _, ann := range r.Annotations {
				if forbiddenAnnotationMatches(file, idx, ann) {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("The annotation `%s` has been forbidden in the detekt config.", ann))
					annStart, annEnd := annotationRemovalRange(file, idx)
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   annStart,
						EndByte:     annEnd,
						Replacement: "",
					}
					ctx.Emit(f)
					return
				}
			}
		},
	})
}

func forbiddenAnnotationMatches(file *scanner.File, idx uint32, configured string) bool {
	name := annotationFinalName(file, idx)
	if name == "" || configured == "" {
		return false
	}
	simple := configured
	if dot := strings.LastIndex(simple, "."); dot >= 0 {
		simple = simple[dot+1:]
	}
	if name != simple {
		return false
	}
	if !strings.Contains(configured, ".") {
		return true
	}
	if annotationQualifiedName(file, idx) == configured {
		return true
	}
	if fileImportsFQN(file, configured) {
		return true
	}
	// An unqualified annotation from a package every file imports (kotlin.Deprecated, java.lang.SuppressWarnings).
	pkg := strings.TrimSuffix(configured, "."+simple)
	// kotlin.* wins over java.lang.* for the same simple name, so an unqualified @Deprecated is kotlin.Deprecated.
	kotlinShadowsJavaLang := pkg == "java.lang" && name == "Deprecated"
	if annotationQualifiedName(file, idx) == name && implicitlyImportedPackages[pkg] && !kotlinShadowsJavaLang && importedFqName(file, name) == name {
		return true
	}
	return false
}

// implicitlyImportedPackages are the packages Kotlin imports into every file on the JVM.
var implicitlyImportedPackages = map[string]bool{
	"kotlin": true, "kotlin.annotation": true, "kotlin.collections": true, "kotlin.comparisons": true,
	"kotlin.io": true, "kotlin.ranges": true, "kotlin.sequences": true, "kotlin.text": true,
	"kotlin.jvm": true, "java.lang": true,
}

func registerStyleForbiddenNamedParam() {
	r := &ForbiddenNamedParamRule{BaseRule: BaseRule{RuleName: "ForbiddenNamedParam", RuleSetName: "style", Sev: "warning", Desc: "Detects named arguments in function calls where they should not be used."}, Methods: []string{"require", "check", "assert"}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			funcName := flatCallExpressionName(file, idx)
			for _, method := range r.Methods {
				// Entries may be fully qualified like the other Forbidden* options, so only the final segment is compared.
				if i := strings.LastIndex(method, "."); i >= 0 {
					method = method[i+1:]
				}
				if funcName == method {
					_, args := flatCallExpressionParts(file, idx)
					if args == 0 {
						return
					}
					for i := 0; i < file.FlatChildCount(args); i++ {
						arg := file.FlatChild(args, i)
						if file.FlatType(arg) == "value_argument" && flatHasValueArgumentLabel(file, arg) {
							ctx.EmitAt(callLine(file, idx), callCol(file, idx),
								fmt.Sprintf("Named arguments should not be used in '%s' calls.", method))
							return
						}
					}
				}
			}
		},
	})
}

func registerStyleForbiddenOptIn() {
	r := &ForbiddenOptInRule{BaseRule: BaseRule{RuleName: "ForbiddenOptIn", RuleSetName: "style", Sev: "warning", Desc: "Detects @OptIn annotations that opt into experimental APIs."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceHigher, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if annotationFinalName(file, idx) != "OptIn" {
				return
			}
			// When marker classes are configured, only flag @OptIn
			// invocations whose argument's ::class receiver matches
			// one of the configured markers by simple name.
			if len(r.MarkerClasses) > 0 {
				want := make(map[string]bool, len(r.MarkerClasses))
				for _, mc := range r.MarkerClasses {
					// Allow fully-qualified names; match on the final
					// segment only.
					if i := strings.LastIndex(mc, "."); i >= 0 {
						mc = mc[i+1:]
					}
					want[mc] = true
				}
				if !annotationHasClassLiteralArgIn(file, idx, want) {
					return
				}
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"@OptIn annotation found. Consider removing or handling the experimental API differently.")
			optInStart, optInEnd := annotationRemovalRange(file, idx)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   optInStart,
				EndByte:     optInEnd,
				Replacement: "",
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleForbiddenSuppress() {
	r := &ForbiddenSuppressRule{BaseRule: BaseRule{RuleName: "ForbiddenSuppress", RuleSetName: "style", Sev: "warning", Desc: "Detects @Suppress annotations that silence a rule the configuration forbids suppressing."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation", "file_annotation"}, Confidence: api.ConfidenceHigher, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if len(r.Rules) == 0 {
				return
			}
			ctor, ok := file.FlatFindChild(idx, "constructor_invocation")
			if !ok {
				return
			}
			if name := annotationConstructorName(file, ctor); name != "Suppress" && name != "SuppressWarnings" {
				return
			}
			forbidden := make(map[string]bool, len(r.Rules))
			for _, rule := range r.Rules {
				forbidden[rule] = true
			}
			var hits []string
			file.FlatWalkNodes(ctor, "string_literal", func(lit uint32) {
				if flatContainsStringInterpolation(file, lit) {
					return
				}
				if content := stringLiteralContent(file, lit); forbidden[content] {
					hits = append(hits, `"`+content+`"`)
				}
			})
			if len(hits) == 0 {
				return
			}
			noun := "rule"
			if len(hits) > 1 {
				noun = "rules"
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"Cannot @Suppress "+noun+" "+strings.Join(hits, ", ")+" due to the current configuration.")
			if file.FlatType(idx) == "annotation" {
				suppressStart, suppressEnd := annotationRemovalRange(file, idx)
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   suppressStart,
					EndByte:     suppressEnd,
					Replacement: "",
				}
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleMagicNumber() {
	r := &MagicNumberRule{
		BaseRule:                  BaseRule{RuleName: "MagicNumber", RuleSetName: "style", Sev: "warning", Desc: "Detects magic number literals in code that should be extracted to named constants."},
		IgnoreAnnotated:           DefaultMagicNumberIgnoreAnnotated,
		CustomPreviewWildcard:     true,
		IgnorePropertyDeclaration: false,
		IgnoreComposeUnits:        true,
		IgnoreColorLiterals:       true,
		IgnoreIdioms:              true,
		IgnoreNumbers: []string{
			"-1", "0", "1", "2",
			"0f", "0.0f", "0.5f", "1f", "1.0f", "-1f",
			"0.5", ".5",
			"90f", "180f", "270f", "360f",
			"100", "100f", "1000", "1000L", "10000", "10000L",
			"255", "255f",
			"60", "60f", "60L", "60000", "60000L",
			"24", "24L",
			"1024", "1024L",
			"16", "16f", "8", "8f", "4", "4f",
		},
		IgnoreHashCodeFunction:                   true,
		IgnoreConstantDeclaration:                true,
		IgnoreAnnotation:                         false,
		IgnoreNamedArgument:                      true,
		IgnoreEnums:                              true,
		IgnoreRanges:                             true,
		IgnoreCompanionObjectPropertyDeclaration: true,
		IgnoreExtensionFunctions:                 true,
		IgnoreLocalVariableDeclaration:           false,
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"integer_literal", "real_literal", "long_literal", "hex_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Skip database migration files — they contain frozen-in-time constants
			// that reflect historical DB schema values and should not be extracted.
			if strings.Contains(file.Path, "/database/helpers/migration/") ||
				strings.Contains(file.Path, "\\database\\helpers\\migration\\") {
				return
			}
			// Skip test source sets (including benchmark/canary macrobenchmark variants).
			// Test-data sizing, timeouts, and iteration counts are legitimate literals.
			if isTestSupportFile(file.Path) {
				return
			}
			// Skip Android debug/dev source sets. Debug-only scaffolding (dropdown
			// defaults, mock-data sizes, dev-menu thresholds) is throwaway tooling,
			// not production constants to extract.
			if strings.Contains(file.Path, "/src/debug/") ||
				strings.Contains(file.Path, "/src/dev/") ||
				strings.Contains(file.Path, "/src/internal/") {
				return
			}
			// Skip if parent is also a literal type we dispatch on — avoids double-counting
			// e.g. 200L produces long_literal containing integer_literal at the same position.
			if p, ok := file.FlatParent(idx); ok && magicNumberLiteralTypes[file.FlatType(p)] {
				return
			}

			text := file.FlatNodeText(idx)
			if r.isIgnoredNumber(file, idx, text) {
				return
			}

			// Skip numbers that are the expression-body of a function: fun maxSize() = 100
			// In tree-sitter, this is integer_literal inside function_body which starts with "="
			if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "function_body" {
				bodyText := file.FlatNodeText(p)
				if strings.HasPrefix(strings.TrimSpace(bodyText), "=") {
					return
				}
			}

			// Skip: fun foo(): Int { return 42 }, where the return is the only statement.
			if magicNumberIsSoleReturn(file, idx) {
				return
			}

			// Skip default parameter values: fun foo(x: Int = 5000), class Foo(val x: Int = 42).
			if magicNumberIsParameterDefault(file, idx) {
				return
			}

			if r.IgnoreIdioms && r.kritIdiom(file, idx, text) {
				return
			}

			// --- Configurable skips ---

			// Skip numbers inside functions with ignored annotations
			if len(r.IgnoreAnnotated) > 0 {
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "function_declaration" {
						fnText := file.FlatNodeText(p)
						if HasIgnoredAnnotation(fnText, r.IgnoreAnnotated) {
							return
						}
						break
					}
				}
			}

			// Skip const declarations, including numbers nested in the initializer.
			if r.IgnoreConstantDeclaration && magicNumberInConstProperty(file, idx) {
				return
			}

			// Skip properties of a companion object (respects config flag)
			if r.IgnoreCompanionObjectPropertyDeclaration && r.inCompanionProperty(file, idx) {
				return
			}

			// Skip numbers inside hashCode functions
			if r.IgnoreHashCodeFunction {
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "function_declaration" {
						if extractIdentifierFlat(file, p) == "hashCode" {
							return
						}
						break
					}
				}
			}

			// Skip numbers inside annotation arguments
			if r.IgnoreAnnotation && file.FlatHasAncestorOfType(idx, "annotation") {
				return
			}

			// Skip named arguments: foo(bar = 42)
			if r.IgnoreNamedArgument && file.FlatHasAncestorOfType(idx, "value_argument") {
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "value_argument" {
						if flatHasValueArgumentLabel(file, p) {
							return
						}
						break
					}
				}
			}

			// Skip numbers inside enum entries
			if r.IgnoreEnums && file.FlatHasAncestorOfType(idx, "enum_entry") {
				return
			}

			// Skip numbers in range expressions (.. operator)
			if r.IgnoreRanges {
				if file.FlatHasAncestorOfType(idx, "range_expression") {
					return
				}
				// Also check for infix range functions: downTo, until, step
				if r.isPartOfInfixRange(file, idx) {
					return
				}
			}

			// Skip property declarations (val x = 42) — non-local only
			if r.IgnorePropertyDeclaration {
				if file.FlatHasAncestorOfType(idx, "property_declaration") &&
					!r.isLocalProperty(file, idx) {
					return
				}
			}

			// Skip local variable declarations (val x = 42 inside a function body)
			if r.IgnoreLocalVariableDeclaration {
				if file.FlatHasAncestorOfType(idx, "property_declaration") &&
					r.isLocalProperty(file, idx) {
					return
				}
			}

			// Skip extension function receivers: 100.toLong(), 24.hours, etc.
			if r.IgnoreExtensionFunctions {
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "navigation_expression" {
					return
				}
				// Also check for call_expression with dot: 100.toString()
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "call_expression" {
					return
				}
			}

			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"This expression contains a magic number. Consider defining it to a well named constant.")
		},
	})
}

func markerOnlyInsideQuotedText(text, marker string) bool {
	found := false
	for searchStart := 0; searchStart < len(text); {
		idx := strings.Index(text[searchStart:], marker)
		if idx < 0 {
			return found
		}
		pos := searchStart + idx
		found = true
		inQuote := false
		escaped := false
		for i := 0; i < pos; i++ {
			switch text[i] {
			case '\\':
				escaped = !escaped
				continue
			case '"':
				if !escaped {
					inQuote = !inQuote
				}
			}
			escaped = false
		}
		if inQuote {
			searchStart = pos + len(marker)
			continue
		}
		return false
	}
	return found
}

// annotationRemovalRange returns the bytes to delete to drop one annotation:
// its whole line when it stands alone on the line, otherwise only the
// annotation and the blanks after it.
func annotationRemovalRange(file *scanner.File, idx uint32) (int, int) {
	start, end := int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx))
	lineStart := start
	for lineStart > 0 && (file.Content[lineStart-1] == ' ' || file.Content[lineStart-1] == '\t') {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(file.Content) && (file.Content[lineEnd] == ' ' || file.Content[lineEnd] == '\t' || file.Content[lineEnd] == '\r') {
		lineEnd++
	}
	aloneBefore := lineStart == 0 || file.Content[lineStart-1] == '\n'
	aloneAfter := lineEnd >= len(file.Content) || file.Content[lineEnd] == '\n'
	if aloneBefore && aloneAfter {
		if lineEnd < len(file.Content) {
			lineEnd++
		}
		return lineStart, lineEnd
	}
	blankEnd := end
	for blankEnd < len(file.Content) && (file.Content[blankEnd] == ' ' || file.Content[blankEnd] == '\t') {
		blankEnd++
	}
	return start, blankEnd
}
