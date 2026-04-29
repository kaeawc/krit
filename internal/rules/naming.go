package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/scanner"
)

// ClassNamingRule checks class names match a pattern.
type ClassNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *ClassNamingRule) Confidence() float64 { return 0.95 }

// FunctionNamingRule checks function names match a pattern.
type FunctionNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern             *regexp.Regexp
	IgnoreAnnotated     []string
	AllowBacktickNames  bool           // if true (default), backtick-quoted names are skipped
	ExcludeClassPattern *regexp.Regexp // classes matching this are excluded
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNamingRule) Confidence() float64 { return 0.95 }

func functionDeclarationHasExplicitReturnTypeFlat(file *scanner.File, idx uint32) bool {
	seenParams := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		c := file.FlatChild(idx, i)
		t := file.FlatType(c)
		if t == "function_value_parameters" {
			seenParams = true
			continue
		}
		if !seenParams {
			continue
		}
		switch t {
		case "user_type", "nullable_type", "function_type", "parenthesized_type", "type_reference", "dynamic_type":
			return true
		case "function_body", "=", "{":
			return false
		}
	}
	text := file.FlatNodeText(idx)
	funIdx := strings.Index(text, "fun ")
	if funIdx < 0 {
		return false
	}
	rest := text[funIdx:]
	paren := strings.Index(rest, ")")
	if paren < 0 {
		return false
	}
	after := strings.TrimLeft(rest[paren+1:], " \t\n\r")
	return strings.HasPrefix(after, ":")
}

func isTopLevelFunctionFlat(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	return ok && file.FlatType(p) == "source_file"
}

func functionHasExpressionBodyReturningCallFlat(file *scanner.File, idx uint32) bool {
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return false
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if !strings.HasPrefix(bodyText, "=") {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(body); i++ {
		c := file.FlatNamedChild(body, i)
		if file.FlatType(c) == "call_expression" {
			return true
		}
	}
	return false
}

// VariableNamingRule checks local variable names.
type VariableNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern                *regexp.Regexp
	PrivateVariablePattern *regexp.Regexp
	ExcludeClassPattern    *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *VariableNamingRule) Confidence() float64 { return 0.95 }

// PackageNamingRule checks package names.
type PackageNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *PackageNamingRule) Confidence() float64 { return 0.95 }

// EnumNamingRule checks enum entry names.
type EnumNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *EnumNamingRule) Confidence() float64 { return 0.95 }

// isTestFile checks if a file is in a test directory.
func isTestFile(path string) bool {
	testDirs := []string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/jvmAndroidTest/",
		"/commonJvmTest/", "/browserCommonTest/", "/jvmCommonTest/",
		"/androidUnitTest/", "/androidInstrumentedTest/", "/jsTest/", "/iosTest/",
		"/nativeTest/", "/nonJvmCommonTest/",
		"/testShared/", "/sharedTest/",
		"/benchmark/", "/canary/",
		"/test-utils/",
		// Google-style Java/Kotlin test roots parallel to `java/`:
		//   <module>/javatests/foo/Bar.kt  (Dagger, Bazel layouts)
		//   <module>/kotlintests/foo/Bar.kt
		"/javatests/", "/kotlintests/", "/javatest/", "/kotlintest/",
		// Functional / gradle-plugin functional tests:
		"/functionalTest/", "/functionaltests/",
		// Kotlin files under any test-resources / fixtures directory are
		// inputs to the test runner, not production code. Examples:
		//   <module>/src/test/resources/...kt
		//   <module>/src/testFixtures/...kt
		//   integration-tests/.../test-processor/src/main/kotlin/...kt
		"/test/resources/", "/testResources/", "/testFixtures/",
		"/integration-tests/", "/integrationTest/",
		"/nonEmulatorCommonTest/", "/nonEmulatorJvmTest/",
		// Kotlin-compiler / KSP / intellij test-data convention. These
		// directories hold deliberately-crafted Kotlin/Java input files
		// used to exercise compiler / analysis features — they have
		// intentional style oddities (empty class bodies, unused
		// parameters, non-standard naming, etc.) and shouldn't be
		// subject to production style rules.
		"/testData/", "/testdata/", "/test-data/",
		// Metro and some other compiler projects use /test/data/ for
		// the same purpose.
		"/test/data/", "/compiler-tests/", "/compilertests/"}
	for _, dir := range testDirs {
		if strings.Contains(path, dir) {
			return true
		}
	}
	return false
}

func isTestSupportFile(path string) bool {
	if isTestFile(path) {
		return true
	}
	lower := strings.ToLower(path)
	return strings.Contains(lower, "-testing/") ||
		strings.Contains(lower, "/testing/") ||
		strings.Contains(lower, "/test-utils/") ||
		strings.Contains(lower, "-test-utils/")
}

// ---------- New naming rules ----------

// BooleanPropertyNamingRule checks that Boolean properties start with is/has/are.
type BooleanPropertyNamingRule struct {
	FlatDispatchBase
	BaseRule
	AllowedPattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *BooleanPropertyNamingRule) Confidence() float64 { return 0.95 }

func isBooleanPropertyFlat(file *scanner.File, idx uint32) bool {
	if extractPropertyTypeFlat(file, idx) == "Boolean" {
		return true
	}
	text := file.FlatNodeText(idx)
	if eq := strings.Index(text, "="); eq >= 0 {
		initializer := strings.TrimSpace(text[eq+1:])
		return initializer == "true" || initializer == "false"
	}
	return false
}

// ConstructorParameterNamingRule checks constructor val/var parameter names.
type ConstructorParameterNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern                 *regexp.Regexp
	PrivateParameterPattern *regexp.Regexp
	ExcludeClassPattern     *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *ConstructorParameterNamingRule) Confidence() float64 { return 0.95 }

// ForbiddenClassNameRule flags disallowed class names.
type ForbiddenClassNameRule struct {
	FlatDispatchBase
	BaseRule
	ForbiddenNames []string
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *ForbiddenClassNameRule) Confidence() float64 { return 0.95 }

// FunctionNameMaxLengthRule flags function names exceeding a max length.
type FunctionNameMaxLengthRule struct {
	FlatDispatchBase
	BaseRule
	MaxLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNameMaxLengthRule) Confidence() float64 { return 0.95 }

// FunctionNameMinLengthRule flags function names below a min length.
type FunctionNameMinLengthRule struct {
	FlatDispatchBase
	BaseRule
	MinLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNameMinLengthRule) Confidence() float64 { return 0.95 }

// FunctionParameterNamingRule checks function parameter names.
type FunctionParameterNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern             *regexp.Regexp
	ExcludeClassPattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionParameterNamingRule) Confidence() float64 { return 0.95 }

// InvalidPackageDeclarationRule checks that the package declaration matches the directory structure.
type InvalidPackageDeclarationRule struct {
	FlatDispatchBase
	BaseRule
	RootPackage              string
	RequireRootInDeclaration bool
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *InvalidPackageDeclarationRule) Confidence() float64 { return 0.95 }

// LambdaParameterNamingRule checks lambda parameter names.
type LambdaParameterNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *LambdaParameterNamingRule) Confidence() float64 { return 0.95 }

// MatchingDeclarationNameRule checks that a file with a single non-private
// top-level class or object has a filename matching that declaration's name.
// Matches detekt behaviour: only KtClassOrObject (class, interface, object, enum)
// are counted; private classes are excluded; top-level functions, properties and
// typealiases do not count toward the "single declaration" check. A typealias
// whose name matches the filename suppresses the finding (allows Foo.kt with
// typealias Foo = FooImpl + class FooImpl).
type MatchingDeclarationNameRule struct {
	FlatDispatchBase
	BaseRule
	MustBeFirst          bool
	MultiplatformTargets []string
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *MatchingDeclarationNameRule) Confidence() float64 { return 0.95 }

// fileNameWithoutSuffix strips multiplatform and .kt/.kts suffixes from a path.
// For example "Foo.android.kt" with target "android" yields "Foo".
func fileNameWithoutSuffix(path string, multiplatformTargets []string) string {
	base := filepath.Base(path)
	for _, target := range multiplatformTargets {
		suffix := "." + target + ".kt"
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// MemberNameEqualsClassNameRule flags members whose name equals the containing class name.
type MemberNameEqualsClassNameRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOverridden bool
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *MemberNameEqualsClassNameRule) Confidence() float64 { return 0.95 }

// NoNameShadowingRule flags inner declarations that shadow outer ones.
// Matches detekt's NoNameShadowing behavior:
//   - Skips underscore "_" names
//   - Non-inner class bodies reset scope (outer names not inherited)
//   - Object/companion object declarations reset scope
//   - Class member function params do NOT shadow class constructor params
//     (accessible via this.name)
type NoNameShadowingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule is highly accurate per-shadow but extremely noisy on real
// codebases (~1,785 findings on pocket-android-app per roadmap/17).
// Many shadows are intentional (scoping functions, kotlinx.coroutines
// flow collectors, builder DSLs). Medium confidence keeps it in
// --min-confidence=medium pipelines but lets strict pipelines filter
// it out of their default gate.
func (r *NoNameShadowingRule) Confidence() float64 { return 0.75 }

type noNameShadowFindingKey struct {
	line int
	name string
}

// shadowScanCtx carries the per-scan state threaded through walkScopeFlat and
// reportIfShadowedFlat. `visible` and `blocked` vary per scope, so they stay
// as walkScopeFlat parameters; `file`, `findings`, and `seen` are constant for
// the whole scan.
type shadowScanCtx struct {
	file     *scanner.File
	findings *[]scanner.Finding
	seen     map[noNameShadowFindingKey]bool
}

func (r *NoNameShadowingRule) walkScopeFlat(node uint32, ctx *shadowScanCtx, visible map[string]int, blocked map[string]bool) {
	file := ctx.file
	if visible == nil {
		visible = make(map[string]int, 16)
	}
	var localNames map[string]bool
	addLocalName := func(name string) {
		if localNames == nil {
			localNames = make(map[string]bool, 8)
		}
		if localNames[name] {
			return
		}
		localNames[name] = true
		visible[name]++
	}
	defer func() {
		for name := range localNames {
			if visible[name] <= 1 {
				delete(visible, name)
				continue
			}
			visible[name]--
		}
	}()

	var collectNames func(n uint32)
	collectNames = func(n uint32) {
		// When tree-sitter-kotlin mis-parses a multi-line class header
		// (`class X\ninternal constructor(...)`) it emits a fake
		// property_declaration (type=constructor) followed by a
		// lambda_literal containing the real class body. Treat the lambda
		// body as a scope barrier in that case.
		skipNextLambdaAsClassBody := false
		for i := 0; i < file.FlatNamedChildCount(n); i++ {
			child := file.FlatNamedChild(n, i)
			childType := file.FlatType(child)

			// Typealias parameter labels are not real declarations; skip entirely.
			if childType == "type_alias" {
				continue
			}

			// Class/object declarations create scope barriers: non-inner classes,
			// companion objects, and object declarations reset the inherited scope.
			if noNameShadowIsScopeBarrierFlat(file, child) {
				r.walkScopeFlat(child, ctx, nil, nil)
				continue
			}
			if skipNextLambdaAsClassBody && childType == "lambda_literal" {
				skipNextLambdaAsClassBody = false
				// Walk with a fresh scope, but pre-populate blocked with
				// property names so function params of the same name inside
				// aren't flagged as shadowing class properties.
				lambdaBlocked := make(map[string]bool, 16)
				var pushProps func(x uint32)
				pushProps = func(x uint32) {
					for j := 0; j < file.FlatNamedChildCount(x); j++ {
						c := file.FlatNamedChild(x, j)
						if file.FlatType(c) == "property_declaration" {
							if name := extractIdentifierFlat(file, c); name != "" {
								lambdaBlocked[name] = true
							}
						}
						if file.FlatType(c) == "statements" {
							pushProps(c)
						}
					}
				}
				pushProps(child)
				r.walkScopeFlat(child, ctx, nil, lambdaBlocked)
				continue
			}

			// Collect property/variable declarations in this scope.
			// Handle destructuring: `val (a, b) = expr` has multiple
			// variable_declaration children inside a multi_variable_declaration.
			if childType == "property_declaration" || childType == "variable_declaration" {
				if parent, ok := file.FlatParent(child); ok && file.FlatType(parent) == "source_file" {
					continue
				}
				// Tree-sitter-kotlin mis-parses `class X\ninternal constructor(...)`
				// as a property_declaration with user_type "constructor" and a
				// multi_variable_declaration holding the constructor params.
				// Skip these entirely — the real parser would produce a
				// primary_constructor, not an outer-scope declaration.
				if childType == "property_declaration" {
					if ut, ok := file.FlatFindChild(child, "user_type"); ok {
						if file.FlatNodeTextEquals(ut, "constructor") {
							skipNextLambdaAsClassBody = true
							continue
						}
					}
				}
				// Check for destructuring (multi_variable_declaration child)
				multi, _ := file.FlatFindChild(child, "multi_variable_declaration")
				if multi != 0 {
					// Destructured: extract all component names, skip initializer scan.
					for j := 0; j < file.FlatChildCount(multi); j++ {
						compDecl := file.FlatChild(multi, j)
						if file.FlatType(compDecl) != "variable_declaration" {
							continue
						}
						compName := extractIdentifierFlat(file, compDecl)
						if compName != "" && compName != "_" {
							r.reportIfShadowedFlat(ctx, compName, compDecl, visible, localNames, blocked)
							addLocalName(compName)
						}
					}
					// Skip further processing to avoid picking up the initializer's name
					continue
				}
				name := extractIdentifierFlat(file, child)
				if name != "" && name != "_" {
					if file.FlatHasAncestorOfType(child, "catch_block") {
						addLocalName(name)
						continue
					}
					if noNameShadowHasLocalSuppression(file, child) {
						addLocalName(name)
						continue
					}
					// Skip the self-shadowing null-narrowing idiom:
					//     val foo = foo ?: default
					//     val foo = foo ?: return
					// This is the canonical Kotlin pattern for smart-casting
					// a nullable parameter to non-null inside a function body.
					// The new binding replaces the nullable one in-scope with
					// an identical name — no reader confusion possible.
					if isNullNarrowingSelfShadowFlat(file, child, name) {
						addLocalName(name)
						continue
					}
					if isSimpleSelfAliasShadowFlat(file, child, name) {
						addLocalName(name)
						continue
					}
					r.reportIfShadowedFlat(ctx, name, child, visible, localNames, blocked)
					addLocalName(name)
				}
				// Don't recurse into property/variable declarations for further
				// name collection: lambda/function types in the type annotation
				// (e.g. `val f: (screen: Screen) -> Unit`) contain `parameter`
				// nodes that are purely documentation, not real bindings, and
				// would otherwise be picked up as declarations shadowing
				// method parameters of the same name.
				continue
			}

			// Collect parameters — but skip class_parameter nodes from shadow
			// checks since constructor params are handled at scope boundaries.
			// Also skip override function parameters — their names are
			// dictated by the superclass/interface contract.
			if childType == "parameter" {
				name := extractIdentifierFlat(file, child)
				if name != "" && name != "_" {
					// Check if we're inside an override function.
					enclosingFn := n
					if file.FlatType(enclosingFn) != "function_declaration" {
						for p, ok := file.FlatParent(enclosingFn); ok; p, ok = file.FlatParent(p) {
							enclosingFn = p
							if file.FlatType(enclosingFn) == "function_declaration" {
								break
							}
						}
					}
					if file.FlatType(enclosingFn) == "function_declaration" &&
						file.FlatHasModifier(enclosingFn, "override") {
						// Skip shadow check for override param; still add to scope
						addLocalName(name)
					} else {
						r.reportIfShadowedFlat(ctx, name, child, visible, localNames, blocked)
						addLocalName(name)
					}
				}
				// Don't recurse into the parameter's type — function/lambda
				// types contain `parameter` nodes that are just labels in the
				// type signature, not real bindings.
				continue
			}

			// class_parameter (constructor param): add to scope for init blocks
			// but don't check against outer scope (we reset at class boundaries).
			if childType == "class_parameter" {
				name := extractIdentifierFlat(file, child)
				if name != "" && name != "_" {
					addLocalName(name)
				}
				// Don't recurse into the class parameter's type annotation.
				// Function-type labels inside constructor/property types
				// (e.g. `(wrapped: Drawable, canvas: Canvas) -> Unit`) are
				// documentation only, not real bindings.
				continue
			}

			// Recurse into scope-creating blocks
			if noNameShadowIsScopeNodeFlat(file, child) {
				switch childType {
				case "function_declaration", "secondary_constructor":
					r.walkScopeFlat(child, ctx, visible, blocked)
				case "class_body":
					if blocked == nil {
						blocked = make(map[string]bool, 8)
					}
					addedBlocked := noNameShadowPushBlockedNamesFlat(file, child, blocked)
					r.walkScopeFlat(child, ctx, visible, blocked)
					for _, name := range addedBlocked {
						delete(blocked, name)
					}
				default:
					r.walkScopeFlat(child, ctx, visible, blocked)
				}
			} else {
				if experiment.Enabled("no-name-shadowing-prune") && !noNameShadowMayContainDeclarationsFlat(file, child) {
					continue
				}
				collectNames(child)
			}
		}
	}
	collectNames(node)
}

// reportIfShadowed checks if name shadows an outer declaration and appends a
// finding if so, with deduplication.
func (r *NoNameShadowingRule) reportIfShadowedFlat(ctx *shadowScanCtx, name string, child uint32, visible map[string]int, localNames map[string]bool, blocked map[string]bool) {
	file := ctx.file
	findings := ctx.findings
	seen := ctx.seen
	if blocked != nil && blocked[name] {
		return
	}
	// Skip destructuring components. `val (a, b) = pair` and lambda
	// destructuring `{ (view, day) -> ... }` bind names that refer to a
	// specific element of the source expression — the names are not
	// freely choosable without changing semantics. Detekt's NoNameShadowing
	// also excludes destructuring declarations.
	if isInsideDestructuringFlat(file, child) {
		return
	}
	if localNames == nil || !localNames[name] {
		if visible[name] == 0 {
			return
		}
		line := file.FlatRow(child) + 1
		// Use the declaration's actual column so multiple shadowing
		// declarations on the same line (e.g. two function parameters)
		// produce distinct finding keys.
		col := file.FlatCol(child) + 1
		key := noNameShadowFindingKey{line: line, name: name}
		if !seen[key] {
			seen[key] = true
			*findings = append(*findings, r.Finding(file, line, col,
				fmt.Sprintf("Name '%s' shadows an outer declaration", name)))
		}
	}
}

func isNullNarrowingSelfShadowFlat(file *scanner.File, decl uint32, name string) bool {
	text := file.FlatNodeText(decl)
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	rhs := strings.TrimSpace(text[eq+1:])
	if !strings.HasPrefix(rhs, name) {
		return false
	}
	after := strings.TrimSpace(rhs[len(name):])
	return strings.HasPrefix(after, "?:") || strings.HasPrefix(after, "?.")
}

func noNameShadowHasLocalSuppression(file *scanner.File, decl uint32) bool {
	text := file.FlatNodeText(decl)
	return strings.Contains(text, "NAME_SHADOWING") ||
		strings.Contains(text, "NoNameShadowing") ||
		strings.Contains(text, "detekt:NoNameShadowing") ||
		strings.Contains(text, "detekt.NoNameShadowing")
}

func isSimpleSelfAliasShadowFlat(file *scanner.File, decl uint32, name string) bool {
	text := file.FlatNodeText(decl)
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	rhs := strings.TrimSpace(text[eq+1:])
	rhs = strings.TrimPrefix(rhs, "this.")
	return rhs == name
}

func isExtensionFunctionDeclFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "function_declaration" {
		return false
	}
	var sawUserType bool
	for i := 0; i < file.FlatChildCount(idx); i++ {
		c := file.FlatChild(idx, i)
		switch file.FlatType(c) {
		case "user_type":
			sawUserType = true
		case "simple_identifier":
			if sawUserType {
				return true
			}
			return false
		}
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if !strings.HasPrefix(text, "fun ") {
		return false
	}
	rest := strings.TrimPrefix(text, "fun ")
	if parenIdx := strings.Index(rest, "("); parenIdx > 0 {
		head := rest[:parenIdx]
		return strings.Contains(head, ".")
	}
	return false
}

func isInsideDestructuringFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "multi_variable_declaration":
			return true
		case "function_declaration", "lambda_literal", "class_body", "function_body", "statements", "source_file":
			return false
		}
	}
	return false
}

func noNameShadowPushBlockedNamesFlat(file *scanner.File, classBody uint32, blocked map[string]bool) []string {
	var added []string
	for i := 0; i < file.FlatNamedChildCount(classBody); i++ {
		child := file.FlatNamedChild(classBody, i)
		if file.FlatType(child) == "property_declaration" {
			name := extractIdentifierFlat(file, child)
			if name != "" && !blocked[name] {
				blocked[name] = true
				added = append(added, name)
			}
		}
	}

	classDecl, ok := file.FlatParent(classBody)
	if ok {
		for i := 0; i < file.FlatNamedChildCount(classDecl); i++ {
			child := file.FlatNamedChild(classDecl, i)
			switch file.FlatType(child) {
			case "primary_constructor", "class_parameter":
				noNameShadowCollectParamNamesFlat(file, child, func(name string) {
					if name == "" || blocked[name] {
						return
					}
					blocked[name] = true
					added = append(added, name)
				})
			}
		}
	}
	return added
}

func noNameShadowMayContainDeclarationsFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "source_file", "statements", "block", "function_body", "class_body", "lambda_literal",
		"for_statement", "while_statement", "do_while_statement", "if_expression",
		"when_expression", "when_entry", "try_expression", "catch_block", "finally_block",
		"property_declaration", "variable_declaration", "multi_variable_declaration",
		"parameter", "class_parameter", "function_declaration", "secondary_constructor",
		"class_declaration", "object_declaration", "companion_object":
		return true
	}
	return file.FlatNamedChildCount(idx) > 0 && file.FlatHasAncestorOfType(idx, "source_file")
}

func noNameShadowIsScopeBarrierFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "class_declaration":
		return !file.FlatHasModifier(idx, "inner")
	case "object_declaration", "companion_object":
		return true
	}
	return false
}

func noNameShadowIsScopeNodeFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "secondary_constructor", "function_body", "lambda_literal",
		"for_statement", "while_statement", "do_while_statement", "if_expression", "when_expression",
		"try_expression", "catch_block", "finally_block", "class_body", "control_structure_body",
		"anonymous_initializer":
		return true
	}
	return false
}

func noNameShadowCollectParamNamesFlat(file *scanner.File, idx uint32, visit func(string)) {
	switch file.FlatType(idx) {
	case "class_parameter", "parameter":
		if name := extractIdentifierFlat(file, idx); name != "" {
			visit(name)
		}
		return
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		noNameShadowCollectParamNamesFlat(file, file.FlatNamedChild(idx, i), visit)
	}
}

// NonBooleanPropertyPrefixedWithIsRule flags non-Boolean properties that start with "is".
type NonBooleanPropertyPrefixedWithIsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *NonBooleanPropertyPrefixedWithIsRule) Confidence() float64 { return 0.95 }

// ObjectPropertyNamingRule checks property names inside object declarations.
type ObjectPropertyNamingRule struct {
	FlatDispatchBase
	BaseRule
	ConstPattern           *regexp.Regexp
	PropertyPattern        *regexp.Regexp
	PrivatePropertyPattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *ObjectPropertyNamingRule) Confidence() float64 { return 0.95 }

// TopLevelPropertyNamingRule checks top-level property names.
type TopLevelPropertyNamingRule struct {
	FlatDispatchBase
	BaseRule
	ConstPattern           *regexp.Regexp
	PropertyPattern        *regexp.Regexp
	PrivatePropertyPattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *TopLevelPropertyNamingRule) Confidence() float64 { return 0.95 }

// VariableMaxLengthRule flags variable names that are too long.
type VariableMaxLengthRule struct {
	FlatDispatchBase
	BaseRule
	MaxLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *VariableMaxLengthRule) Confidence() float64 { return 0.95 }

// VariableMinLengthRule flags variable names that are too short.
type VariableMinLengthRule struct {
	FlatDispatchBase
	BaseRule
	MinLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *VariableMinLengthRule) Confidence() float64 { return 0.95 }

func walkFunctionParametersFlat(file *scanner.File, idx uint32, visit func(uint32)) {
	if file == nil || idx == 0 {
		return
	}
	if file.FlatType(idx) == "lambda_literal" {
		return
	}
	if file.FlatType(idx) == "parameter" {
		visit(idx)
	}
	file.FlatForEachChild(idx, func(child uint32) {
		walkFunctionParametersFlat(file, child, visit)
	})
}
