package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerStyleForbiddenRules() {

	// --- from style_forbidden.go ---
	{
		r := &WildcardImportRule{BaseRule: BaseRule{RuleName: "WildcardImport", RuleSetName: "style", Sev: "warning", Desc: "Detects wildcard import statements that should be replaced with explicit imports."}, ExcludeImports: []string{"java.util.*", "platform.**", "kotlinx.cinterop.*"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Skip test files and test-fixture Kotlin sources. Fixtures routinely
				// use wildcard imports of an API surface being tested (e.g. KSP's
				// integration-tests copy Kotlin files that `import foo.bar.*`).
				if isTestFile(file.Path) {
					return
				}
				// Wildcard imports carry a `wildcard_import` child node in
				// tree-sitter-kotlin — an unambiguous structural signal.
				if !file.FlatHasChildOfType(idx, "wildcard_import") {
					return
				}
				ident, _ := file.FlatFindChild(idx, "identifier")
				if ident == 0 {
					return
				}
				fqn := file.FlatNodeText(ident)
				imp := fqn + ".*"
				for _, excl := range r.ExcludeImports {
					if wildcardImportExcluded(imp, excl) {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Wildcard import '%s' should be replaced with explicit imports.", imp))
			},
		})
	}
	{
		r := &ForbiddenCommentRule{BaseRule: BaseRule{RuleName: "ForbiddenComment", RuleSetName: "style", Sev: "warning", Desc: "Detects comments containing forbidden markers like TODO, FIXME, or STOPSHIP."}, Comments: defaultForbiddenCommentMarkers}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"line_comment", "multiline_comment"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				markers := r.Comments
				if len(markers) == 0 {
					markers = defaultForbiddenCommentMarkers
				}
				for _, marker := range markers {
					if strings.Contains(text, marker) {
						// If the comment matches the allowed pattern, skip it
						if r.AllowedPatterns != nil && r.AllowedPatterns.MatchString(text) {
							continue
						}
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Forbidden comment marker '%s' found.", marker))
						return
					}
				}
			},
		})
	}
	{
		r := &ForbiddenVoidRule{BaseRule: BaseRule{RuleName: "ForbiddenVoid", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of the Java Void type that should be replaced with Kotlin Unit."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"user_type"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if text != "Void" {
					return
				}
				// Skip when used as a type argument to a Java-interop generic type.
				// Walk up the AST: user_type -> type_arguments -> user_type (the outer generic)
				inFunctionParam := false
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "type_arguments" {
						if outer, ok := file.FlatParent(p); ok {
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
						if inFunctionParam && file.FlatHasModifier(p, "override") {
							return
						}
						break
					}
					if file.FlatType(p) == "class_declaration" {
						break
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Use 'Unit' instead of 'Void' in Kotlin.")
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
	{
		r := &ForbiddenImportRule{BaseRule: BaseRule{RuleName: "ForbiddenImport", RuleSetName: "style", Sev: "warning", Desc: "Detects import statements matching configured forbidden patterns."}, Patterns: defaultForbiddenImports}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				imp := strings.TrimPrefix(strings.TrimSpace(text), "import ")
				// Merge Patterns and ForbiddenImports
				patterns := r.Patterns
				if len(r.ForbiddenImports) > 0 {
					patterns = r.ForbiddenImports
				}
				for _, p := range patterns {
					if strings.Contains(imp, p) {
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
						f := r.Finding(file, file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Forbidden import '%s'.", strings.TrimSpace(imp)))
						// Compute byte range to remove import line including trailing newline
						impEnd := int(file.FlatEndByte(idx))
						if impEnd < len(file.Content) && file.Content[impEnd] == '\n' {
							impEnd++
						}
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
	{
		r := &ForbiddenMethodCallRule{BaseRule: BaseRule{RuleName: "ForbiddenMethodCall", RuleSetName: "style", Sev: "warning", Desc: "Detects calls to methods that are configured as forbidden."}, Methods: defaultForbiddenMethods}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"print", "println"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				methodName, ok := forbiddenMethodCallMatch(ctx, idx, r.Methods)
				if !ok {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Forbidden method call '%s'.", methodName))
				// Auto-fix: remove the call statement using AST node byte offsets
				startByte := int(file.FlatStartByte(idx))
				endByte := int(file.FlatEndByte(idx))
				// Expand to cover the full line (leading whitespace + trailing newline)
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
		r := &ForbiddenAnnotationRule{BaseRule: BaseRule{RuleName: "ForbiddenAnnotation", RuleSetName: "style", Sev: "warning", Desc: "Detects usage of annotations that are configured as forbidden."}, Annotations: defaultForbiddenAnnotations}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				for _, ann := range r.Annotations {
					if strings.Contains(text, ann) {
						f := r.Finding(file, file.FlatRow(idx)+1, 1,
							fmt.Sprintf("Forbidden annotation '%s'.", ann))
						// Compute byte range to remove annotation line including trailing newline
						annStart := int(file.FlatStartByte(idx))
						for annStart > 0 && file.Content[annStart-1] != '\n' {
							annStart--
						}
						annEnd := int(file.FlatEndByte(idx))
						if annEnd < len(file.Content) && file.Content[annEnd] == '\n' {
							annEnd++
						}
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
	{
		r := &ForbiddenNamedParamRule{BaseRule: BaseRule{RuleName: "ForbiddenNamedParam", RuleSetName: "style", Sev: "warning", Desc: "Detects named arguments in function calls where they should not be used."}, Methods: []string{"require", "check", "assert"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				funcName := flatCallExpressionName(file, idx)
				for _, method := range r.Methods {
					if funcName == method {
						_, args := flatCallExpressionParts(file, idx)
						if args == 0 {
							return
						}
						for i := 0; i < file.FlatChildCount(args); i++ {
							arg := file.FlatChild(args, i)
							if file.FlatType(arg) == "value_argument" && flatHasValueArgumentLabel(file, arg) {
								ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
									fmt.Sprintf("Named arguments should not be used in '%s' calls.", method))
								return
							}
						}
					}
				}
			},
		})
	}
	{
		r := &ForbiddenOptInRule{BaseRule: BaseRule{RuleName: "ForbiddenOptIn", RuleSetName: "style", Sev: "warning", Desc: "Detects @OptIn annotations that opt into experimental APIs."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: 0.9, Fix: v2.FixSemantic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"@OptIn annotation found. Consider removing or handling the experimental API differently.")
				optInStart := int(file.FlatStartByte(idx))
				for optInStart > 0 && file.Content[optInStart-1] != '\n' {
					optInStart--
				}
				optInEnd := int(file.FlatEndByte(idx))
				if optInEnd < len(file.Content) && file.Content[optInEnd] == '\n' {
					optInEnd++
				}
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
	{
		r := &ForbiddenSuppressRule{BaseRule: BaseRule{RuleName: "ForbiddenSuppress", RuleSetName: "style", Sev: "warning", Desc: "Detects @Suppress annotations that silence warnings instead of fixing the underlying issue."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: 0.9, Fix: v2.FixSemantic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if annotationFinalName(file, idx) != "Suppress" {
					return
				}
				// When specific rule names are configured, require the
				// annotation to carry one of them as a string literal arg.
				if len(r.Rules) > 0 {
					want := make(map[string]bool, len(r.Rules))
					for _, rule := range r.Rules {
						want[rule] = true
					}
					if !annotationHasStringArgIn(file, idx, want) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"@Suppress annotation found. Consider fixing the underlying issue.")
				suppressStart := int(file.FlatStartByte(idx))
				for suppressStart > 0 && file.Content[suppressStart-1] != '\n' {
					suppressStart--
				}
				suppressEnd := int(file.FlatEndByte(idx))
				if suppressEnd < len(file.Content) && file.Content[suppressEnd] == '\n' {
					suppressEnd++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   suppressStart,
					EndByte:     suppressEnd,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &MagicNumberRule{
			BaseRule:                  BaseRule{RuleName: "MagicNumber", RuleSetName: "style", Sev: "warning", Desc: "Detects magic number literals in code that should be extracted to named constants."},
			IgnoreAnnotated:           DefaultMagicNumberIgnoreAnnotated,
			IgnorePropertyDeclaration: false,
			IgnoreComposeUnits:        true,
			IgnoreColorLiterals:       true,
			IgnoreNumbers: []string{
				"-1", "0", "1", "2",
				"0f", "0.0f", "0.5f", "1f", "1.0f", "-1f",
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"integer_literal", "real_literal", "long_literal", "hex_literal"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Skip database migration files — they contain frozen-in-time constants
				// that reflect historical DB schema values and should not be extracted.
				if strings.Contains(file.Path, "/database/helpers/migration/") ||
					strings.Contains(file.Path, "\\database\\helpers\\migration\\") {
					return
				}
				// Skip test source sets (including benchmark/canary macrobenchmark variants).
				// Test-data sizing, timeouts, and iteration counts are legitimate literals.
				if isTestFile(file.Path) {
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
				// Numeric literals written with underscore separators
				// (e.g., 25_000L, 1_000_000) have explicit author-intended grouping;
				// these aren't magic numbers, the author already made the value legible.
				if strings.Contains(text, "_") {
					return
				}
				// Strip suffixes for comparison against ignore list
				clean := strings.TrimRight(text, "fFdDlLuU")
				clean = strings.ReplaceAll(clean, "_", "")
				if r.ignoredNumberSet()[clean] {
					return
				}

				var ancestorCtx *magicNumberAncestorContext
				if experiment.Enabled("magic-number-ancestor-scan") {
					ancestorCtx = buildMagicNumberAncestorContext(file, idx)
				}

				// --- Unconditional skips (detekt behaviour) ---

				// Skip numbers that are the expression-body of a function: fun maxSize() = 100
				// In tree-sitter, this is integer_literal inside function_body which starts with "="
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "function_body" {
					bodyText := file.FlatNodeText(p)
					if strings.HasPrefix(strings.TrimSpace(bodyText), "=") {
						return
					}
				}

				// Skip: fun foo(): Int { return 42 } — number inside a jump_expression (return)
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "jump_expression" {
					pText := file.FlatNodeText(p)
					if strings.HasPrefix(strings.TrimSpace(pText), "return") {
						return
					}
				}

				// Skip default parameter values in functions: fun foo(x: Int = 5000)
				// Tree-sitter puts the literal as a sibling of `parameter` inside `function_value_parameters`.
				if file.FlatHasAncestorOfType(idx, "function_value_parameters") {
					return
				}
				// Skip default parameter values in constructors: class Foo(val x: Int = 42)
				if file.FlatHasAncestorOfType(idx, "class_parameter") {
					return
				}

				// Skip literals inside enum entry constructor arguments:
				// `enum class Foo(val id: Int) { A(1), B(2) }` — the literals are the
				// enum constant definitions, equivalent to named constants.
				if file.FlatHasAncestorOfType(idx, "enum_entry") {
					return
				}
				// Skip literals that are the direct value of a when branch mapping an
				// enum/constant to discrete numeric values — this is a lookup-table
				// idiom, not a magic number. Example: `SIZE.LARGE -> 0.8f`.
				if isWhenBranchValue(file, idx) {
					return
				}
				// Skip literals inside array/list index access: `parts[1]`, `bytes[3]`.
				// These are positional indices, not magic numbers.
				if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "indexing_suffix" {
					return
				}
				// Skip literals that are the RHS of a `.size` / `.length` / `.count`
				// equality/comparison. These are intrinsic collection-cardinality
				// checks (e.g. `parts.size == 7`, `daysEnabled.size == 7`), not
				// extractable constants.
				if isSizeCardinalityComparison(file, idx) {
					return
				}
				// Skip regex-group accessor arguments: `matcher.group(3)`,
				// `match.groupValues[1]`, etc. Regex capture indices are intrinsic
				// to the pattern.
				if experiment.Enabled("magic-number-skip-regex-group-indices") &&
					isInsideRegexGroupAccessor(file, idx) {
					return
				}
				// Skip SDK_INT comparisons: `Build.VERSION.SDK_INT < 24`, etc.
				// API-level literals are semantic constants.
				if isNearSdkIntComparison(file, idx) {
					return
				}
				// Skip literals inside SDK-version annotations: @RequiresApi(26),
				// @TargetApi(31), @ChecksSdkIntAtLeast(N), @RequiresExtension(N).
				if isInsideSdkAnnotation(file, idx) {
					return
				}
				// Skip literals that are the RHS of an ALL_CAPS named constant
				// declaration (e.g., `private val MAX_SIZE = 1024`). Flagging these
				// defeats the "extract to named constant" guidance.
				if isInsideAllCapsConstantDecl(file, idx) {
					return
				}
				// Skip literals inside database migration methods (`onUpgrade`,
				// `onDowngrade`, `onCreate`) where version comparisons reference
				// historical schema versions.
				if isInsideDbMigrationMethod(file, idx) {
					return
				}
				// Skip literals inside crypto/KDF calls — output lengths, key sizes
				// are dictated by the algorithm, not arbitrary.
				if magicNumberInsideNamedMethodCall(file, idx, cryptoMethods, ancestorCtx) {
					return
				}
				// Skip hex literals (0x...) — hex notation is already self-documenting
				// as a color/mask/byte pattern.
				if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
					return
				}
				// Skip Bitmap/collection/cache builder constructors.
				if magicNumberInsideNamedMethodCall(file, idx, bitmapBuilderMethods, ancestorCtx) {
					return
				}
				// Skip HTTP status comparisons: `x.status == 200`, `code >= 500`.
				if isHttpStatusComparison(file, idx) {
					return
				}
				// Skip literals inside Compose `Color(...)` calls — component values
				// 0f-1f are semantic color channels, not magic numbers.
				if magicNumberInsideComposeCall(file, idx, "Color", ancestorCtx) {
					return
				}
				// Skip literals inside Compose/Canvas/Path/animator DSL methods where
				// raw coordinates are inherent to the API.
				if magicNumberInsideGeometryDslCall(file, idx, ancestorCtx) {
					return
				}
				// Skip literals inside coordinate/size constructors like `PointF(x, y)`
				// or setters like `point.set(x, y)`.
				if magicNumberInsideNamedMethodCall(file, idx, coordinateConstructors, ancestorCtx) {
					return
				}
				// Skip literals inside dimension/animation DSL calls.
				if magicNumberInsideNamedMethodCall(file, idx, dimensionConversionMethods, ancestorCtx) ||
					magicNumberInsideNamedMethodCall(file, idx, animationMethods, ancestorCtx) {
					return
				}
				// Skip literals inside range expressions / IntRange constants
				// (e.g., `0x0000..0x024F` Unicode ranges).
				if file.FlatHasAncestorOfType(idx, "range_expression") {
					return
				}
				// Skip literals that are part of a `to` infix expression used in a
				// collection builder (mapOf lookup tables, version maps).
				if isInsideToInfixMap(file, idx) {
					return
				}
				// Skip literals inside preview/sample/fake/mock/stub functions — these
				// are scaffolding for UI tooling, not production constants.
				if isInsidePreviewOrSampleFunctionFlat(file, idx) {
					return
				}
				// Skip literals that are the duration argument of a call whose sibling
				// is a `TimeUnit.X` reference (e.g., `Single.timer(200, TimeUnit.MILLISECONDS)`).
				if magicNumberDurationLiteralWithTimeUnit(file, idx, ancestorCtx) {
					return
				}
				// Skip literals that are arguments to java.math / java.time builder
				// calls where the literal is documentational (BigDecimal.valueOf(3),
				// Instant.ofEpochMilli(0), Duration.ofSeconds(30), etc.).
				if magicNumberInsideNamedMethodCall(file, idx, jvmBuilderMethods, ancestorCtx) {
					return
				}
				// Skip literals inside primitive-array builders `byteArrayOf(1, 2, 3)` —
				// these are data payloads (test fixtures, magic bytes, handshake
				// sequences), never meaningful constants to extract.
				if magicNumberInsideNamedMethodCall(file, idx, primitiveArrayBuilders, ancestorCtx) {
					return
				}
				// Skip HTTP status code literals inside exception constructor calls
				// (e.g., `NonSuccessfulResponseCodeException(404)`). The exception
				// type name together with the known HTTP status range is
				// self-documenting.
				if isHttpStatusExceptionArg(file, idx) {
					return
				}
				// Skip literals that are the RHS of an assignment to a semantic UI /
				// animation / layout property — the property name itself documents
				// the value (`duration = 250`, `textSize = 14f`, `elevation = 4f`).
				if isSemanticPropertyAssignment(file, idx) {
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

				line := file.Lines[file.FlatRow(idx)]
				trimmed := strings.TrimSpace(line)

				// Skip const declarations
				if r.IgnoreConstantDeclaration && strings.Contains(trimmed, "const val") {
					return
				}

				// Skip companion object properties (respects config flag)
				if r.IgnoreCompanionObjectPropertyDeclaration && file.FlatHasAncestorOfType(idx, "companion_object") {
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
							argText := file.FlatNodeText(p)
							if strings.Contains(argText, "=") {
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

				// Skip color literals: hex numbers inside Color() calls or in color-named properties
				if r.IgnoreColorLiterals && strings.HasPrefix(text, "0x") {
					if strings.Contains(line, "Color(") {
						return
					}
					lowerLine := strings.ToLower(trimmed)
					if strings.Contains(lowerLine, "color") || strings.Contains(lowerLine, "background") ||
						strings.Contains(lowerLine, "tint") || strings.Contains(lowerLine, "palette") {
						return
					}
				}

				// Skip Compose dimension units: N.dp, N.sp, N.px, N.em
				if r.IgnoreComposeUnits {
					endByte := int(file.FlatEndByte(idx))
					if endByte+3 <= len(file.Content) {
						after := string(file.Content[endByte:min(endByte+4, len(file.Content))])
						if strings.HasPrefix(after, ".dp") || strings.HasPrefix(after, ".sp") ||
							strings.HasPrefix(after, ".px") || strings.HasPrefix(after, ".em") {
							return
						}
					}
				}

				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Magic number '%s'. Consider extracting it to a named constant.", text))
			},
		})
	}
}
