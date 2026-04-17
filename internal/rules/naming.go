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

func (r *ClassNamingRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *ClassNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Test files commonly use `ClassName_suffix` / `ClassName__Subgroup`
	// patterns to group related test classes.
	if isTestFile(file.Path) {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	// Skip backtick-enclosed names (Kotlin test classes: `my test class`)
	if strings.HasPrefix(name, "`") {
		return nil
	}
	if !r.Pattern.MatchString(name) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Class name '%s' does not match pattern: %s", name, r.Pattern.String()))}
	}
	return nil
}

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

func (r *FunctionNamingRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *FunctionNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip test directories (backtick test names are conventional)
	if isTestFile(file.Path) {
		return nil
	}
	text := file.FlatNodeText(idx)

	// Skip `fun interface Foo { ... }` declarations — SAM conversions,
	// parsed as function_declaration by tree-sitter-kotlin. The name
	// follows type naming (PascalCase), not function naming.
	if strings.HasPrefix(strings.TrimLeft(text, " \t"), "fun interface ") ||
		strings.Contains(text, " fun interface ") {
		return nil
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		c := file.FlatChild(idx, i)
		if file.FlatType(c) == "interface" {
			return nil
		}
	}

	// Skip functions with ignored annotations
	if HasIgnoredAnnotation(text, r.IgnoreAnnotated) {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}

	// Skip backtick-quoted names (common in tests, configurable)
	if r.AllowBacktickNames && strings.HasPrefix(name, "`") {
		return nil
	}

	// Skip operator overloads
	operators := map[string]bool{
		"get": true, "set": true, "invoke": true, "plus": true,
		"minus": true, "times": true, "div": true, "rem": true,
		"compareTo": true, "equals": true, "hashCode": true, "toString": true,
	}
	if operators[name] {
		return nil
	}

	if !r.Pattern.MatchString(name) {
		// Factory function convention: Kotlin's coding conventions
		// explicitly allow PascalCase function names when the function
		// creates an instance of a class — the function acts like a
		// constructor proxy. Detect this by: (a) the name starts with
		// an uppercase letter, and (b) the declaration carries an
		// explicit return type. Compose @Composable functions also
		// follow this shape but are already handled via
		// `ignoreAnnotated`. This covers the broader set:
		//   fun Size(w: Int, h: Int): Size = ...
		//   fun MainViewController(): UIViewController = ...
		//   fun OkHttpNetworkFetcherFactory(...): NetworkFetcher = ...
		//   expect fun LruMutableMap<K, V>(initialCapacity: Int): MutableMap<K, V>
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' &&
			functionDeclarationHasExplicitReturnTypeFlat(file, idx) {
			return nil
		}
		// Kotlin convention allows PascalCase for functions that act as
		// constructor proxies / factories. Detekt's `ignoreAnnotated`
		// default covers @Composable but not other factory shapes. Skip
		// top-level PascalCase functions whose body is a single
		// expression returning a call — the classic factory shape:
		//   fun Size(w, h) = Size(Dimension(w), Dimension(h))
		//   fun Dimension(px) = Dimension.Pixels(px)
		//   fun KtorNetworkFetcherFactory(...) = NetworkFetcher.Factory(...)
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' &&
			isTopLevelFunctionFlat(file, idx) &&
			functionHasExpressionBodyReturningCallFlat(file, idx) {
			return nil
		}
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function name '%s' does not match pattern: %s", name, r.Pattern.String()))}
	}
	return nil
}

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
	body := file.FlatFindChild(idx, "function_body")
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

func (r *VariableNamingRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *VariableNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only check variables inside function bodies (local vars)
	if !file.FlatHasAncestorOfType(idx, "function_body") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	// Skip backtick-quoted names (e.g. `in`, `is` — escaped keywords).
	if strings.HasPrefix(name, "`") {
		return nil
	}
	if name != "" && !r.Pattern.MatchString(name) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Variable name '%s' does not match pattern: %s", name, r.Pattern.String()))}
	}
	return nil
}

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

func (r *PackageNamingRule) NodeTypes() []string { return []string{"package_header"} }

func (r *PackageNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Walk named children to find the identifier — the package_header
	// node's text can include trailing KDoc comments which break naive
	// TrimPrefix parsing.
	var pkg string
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		c := file.FlatNamedChild(idx, i)
		t := file.FlatType(c)
		if t == "identifier" || t == "simple_identifier" || t == "navigation_expression" ||
			t == "qualified_identifier" || t == "type_identifier" {
			pkg = strings.TrimSpace(file.FlatNodeText(c))
			break
		}
	}
	if pkg == "" {
		text := file.FlatNodeText(idx)
		if nl := strings.IndexByte(text, '\n'); nl >= 0 {
			text = text[:nl]
		}
		pkg = strings.TrimSpace(strings.TrimPrefix(text, "package "))
	}
	if pkg != "" && !r.Pattern.MatchString(pkg) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Package name '%s' does not match pattern: %s", pkg, r.Pattern.String()))}
	}
	return nil
}

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

func (r *EnumNamingRule) NodeTypes() []string { return []string{"enum_entry"} }

func (r *EnumNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "" && !r.Pattern.MatchString(name) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Enum entry '%s' does not match pattern: %s", name, r.Pattern.String()))}
	}
	return nil
}

// isTestFile checks if a file is in a test directory.
func isTestFile(path string) bool {
	testDirs := []string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/",
		"/commonJvmTest/", "/browserCommonTest/", "/jvmCommonTest/",
		"/androidUnitTest/", "/androidInstrumentedTest/", "/jsTest/", "/iosTest/",
		"/benchmark/", "/canary/",
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

func (r *BooleanPropertyNamingRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *BooleanPropertyNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBooleanPropertyFlat(file, idx) {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	if !strings.HasPrefix(name, "is") && !strings.HasPrefix(name, "has") && !strings.HasPrefix(name, "are") {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Boolean property '%s' should start with 'is', 'has', or 'are'", name))
		// Auto-fix: prepend "is" to the property name via byte-mode replacement on the identifier node
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "simple_identifier" {
				newName := "is" + strings.ToUpper(name[:1]) + name[1:]
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(child)),
					EndByte:     int(file.FlatEndByte(child)),
					Replacement: newName,
				}
				break
			}
		}
		return []scanner.Finding{f}
	}
	return nil
}

func isBooleanPropertyFlat(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	return strings.Contains(text, ": Boolean") || strings.Contains(text, "= true") || strings.Contains(text, "= false")
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

func (r *ConstructorParameterNamingRule) NodeTypes() []string { return []string{"primary_constructor"} }

func (r *ConstructorParameterNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i := 0; i < file.FlatChildCount(idx); i++ {
		paramNode := file.FlatChild(idx, i)
		if file.FlatType(paramNode) != "class_parameter" {
			continue
		}
		// Only check val/var parameters by looking for binding_pattern_kind child
		if !file.FlatHasChildOfType(paramNode, "binding_pattern_kind") {
			continue
		}
		name := extractIdentifierFlat(file, paramNode)
		// Skip backtick-quoted names (e.g. `in`, `is` — escaped Java keywords).
		if strings.HasPrefix(name, "`") {
			continue
		}
		if name != "" && !r.Pattern.MatchString(name) {
			findings = append(findings, r.Finding(file, file.FlatRow(paramNode)+1, 1,
				fmt.Sprintf("Constructor parameter name '%s' does not match pattern: %s", name, r.Pattern.String())))
		}
	}
	return findings
}

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

func (r *ForbiddenClassNameRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *ForbiddenClassNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if len(r.ForbiddenNames) == 0 {
		return nil
	}
	forbidden := make(map[string]bool, len(r.ForbiddenNames))
	for _, n := range r.ForbiddenNames {
		forbidden[n] = true
	}
	name := extractIdentifierFlat(file, idx)
	if name != "" && forbidden[name] {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Class name '%s' is forbidden", name))}
	}
	return nil
}

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

func (r *FunctionNameMaxLengthRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *FunctionNameMaxLengthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "" && len(name) > r.MaxLength {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function name '%s' exceeds maximum length of %d (length: %d)", name, r.MaxLength, len(name)))}
	}
	return nil
}

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

func (r *FunctionNameMinLengthRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *FunctionNameMinLengthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "" && len(name) < r.MinLength {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function name '%s' is below minimum length of %d (length: %d)", name, r.MinLength, len(name)))}
	}
	return nil
}

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

func (r *FunctionParameterNamingRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *FunctionParameterNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	paramsNode := file.FlatFindChild(idx, "function_value_parameters")
	if paramsNode == 0 {
		return nil
	}
	walkFunctionParametersFlat(file, paramsNode, func(paramNode uint32) {
		name := extractIdentifierFlat(file, paramNode)
		// Skip backtick-quoted names (e.g. `in`, `is` — escaped Java keywords).
		if strings.HasPrefix(name, "`") {
			return
		}
		if name != "" && !r.Pattern.MatchString(name) {
			findings = append(findings, r.Finding(file, file.FlatRow(paramNode)+1, 1,
				fmt.Sprintf("Function parameter name '%s' does not match pattern: %s", name, r.Pattern.String())))
		}
	})
	return findings
}

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

func (r *InvalidPackageDeclarationRule) NodeTypes() []string { return []string{"package_header"} }

func (r *InvalidPackageDeclarationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Extract the identifier child node for the package name
	var pkg string
	idNode := file.FlatFindChild(idx, "identifier")
	if idNode != 0 {
		pkg = strings.TrimSpace(file.FlatNodeText(idNode))
	} else {
		text := file.FlatNodeText(idx)
		pkg = strings.TrimSpace(strings.TrimPrefix(text, "package "))
		// Trim to first line only in case the node spans multiple lines
		if idx := strings.Index(pkg, "\n"); idx >= 0 {
			pkg = strings.TrimSpace(pkg[:idx])
		}
	}
	if pkg == "" {
		return nil
	}
	// Convert package to expected path segments
	expectedSuffix := strings.ReplaceAll(pkg, ".", string(filepath.Separator))
	dir := filepath.Dir(file.Path)
	// Normalize to forward slashes for comparison
	dirNorm := filepath.ToSlash(dir)
	expectedNorm := filepath.ToSlash(expectedSuffix)
	if !strings.HasSuffix(dirNorm, expectedNorm) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Package declaration '%s' does not match the file's directory structure", pkg))}
	}
	return nil
}

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

func (r *LambdaParameterNamingRule) NodeTypes() []string { return []string{"lambda_literal"} }

func (r *LambdaParameterNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	paramsNode := file.FlatFindChild(idx, "lambda_parameters")
	if paramsNode == 0 {
		return nil
	}
	// Check variable_declaration children (lambda params)
	file.FlatForEachChild(paramsNode, func(child uint32) {
		if file.FlatType(child) != "variable_declaration" && file.FlatType(child) != "simple_identifier" {
			return
		}
		name := ""
		if file.FlatType(child) == "simple_identifier" {
			name = file.FlatNodeText(child)
		} else {
			name = extractIdentifierFlat(file, child)
		}
		// Skip backtick-quoted names (e.g. `in`, `is` — escaped keywords).
		if strings.HasPrefix(name, "`") {
			return
		}
		if name != "" && !r.Pattern.MatchString(name) {
			findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
				fmt.Sprintf("Lambda parameter name '%s' does not match pattern: %s", name, r.Pattern.String())))
		}
	})
	return findings
}

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

func (r *MatchingDeclarationNameRule) NodeTypes() []string { return []string{"source_file"} }

func (r *MatchingDeclarationNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip .kts script files — script class names don't match filenames
	if strings.HasSuffix(file.Path, ".kts") {
		return nil
	}

	type classDecl struct {
		name string
		idx  uint32
	}

	var nonPrivateClasses []classDecl
	var typeAliasNames []string
	var firstDeclNode uint32 // first top-level declaration of any kind
	hasComposableFunc := false
	hasExtensionFunc := false

	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration":
			if firstDeclNode == 0 {
				firstDeclNode = child
			}
			// Skip private classes/objects (detekt: filterNot { it.isPrivate() })
			if file.FlatHasModifier(child, "private") {
				continue
			}
			name := extractIdentifierFlat(file, child)
			if name != "" {
				nonPrivateClasses = append(nonPrivateClasses, classDecl{name, child})
			}
		case "type_alias":
			if firstDeclNode == 0 {
				firstDeclNode = child
			}
			name := extractIdentifierFlat(file, child)
			if name != "" {
				typeAliasNames = append(typeAliasNames, name)
			}
		case "function_declaration":
			if firstDeclNode == 0 {
				firstDeclNode = child
			}
			// @Composable functions at top-level alongside a class indicate
			// a Compose UI file where the filename is a collective topic.
			if flatHasAnnotationNamed(file, child, "Composable") {
				hasComposableFunc = true
			}
			// Extension functions (`fun Foo.bar()`) alongside a class
			// indicate a "type + extensions" grouping pattern where the
			// file name reflects a theme, not any single declaration.
			if isExtensionFunctionDeclFlat(file, child) {
				hasExtensionFunc = true
			}
		case "property_declaration":
			if firstDeclNode == 0 {
				firstDeclNode = child
			}
		}
	}

	// Only flag when there is exactly one non-private class/object.
	if len(nonPrivateClasses) != 1 {
		return nil
	}
	// Skip when the file contains top-level @Composable functions — this
	// is the common Compose UI pattern where class + composables coexist.
	if hasComposableFunc {
		return nil
	}
	// Skip when the file contains top-level extension functions — the
	// "type + extension helpers" pattern is idiomatic Kotlin and the
	// file name typically reflects the group, not the single type.
	if experiment.Enabled("matching-declaration-name-skip-ext-fun-files") && hasExtensionFunc {
		return nil
	}

	decl := nonPrivateClasses[0]

	// mustBeFirst: when true (default), only flag if the class/object is the
	// first top-level declaration in the file.
	if r.MustBeFirst && firstDeclNode != 0 && decl.idx != firstDeclNode {
		return nil
	}

	fileName := fileNameWithoutSuffix(file.Path, r.MultiplatformTargets)

	// A typealias with the same name as the filename suppresses the finding
	// (e.g. Foo.kt: typealias Foo = FooImpl; class FooImpl)
	for _, ta := range typeAliasNames {
		if ta == fileName {
			return nil
		}
	}

	// Strip any remaining dot-qualifiers (`SvgImage.nonAndroid` →
	// `SvgImage`). Arbitrary variant suffixes appear in KMP source sets
	// whose names are not in the MultiplatformTargets list (nonAndroid,
	// appleMain, desktopMain, etc.). Only used for variant matching —
	// we still flag plain `.kt` files whose name doesn't match.
	bareFileName := fileName
	hadDotQualifier := false
	if dot := strings.Index(bareFileName, "."); dot > 0 {
		bareFileName = bareFileName[:dot]
		hadDotQualifier = true
	}
	// `main.kt` / `Main.kt` is the conventional Kotlin entry-point file
	// and rarely shares its name with the enclosing class.
	if strings.EqualFold(fileName, "main") {
		return nil
	}
	// For files with a dot-qualifier suffix, accept when the declaration
	// starts with the bare file name: `SvgImage.nonAndroid.kt` →
	// `class SvgImage` or `class SvgImageNonAndroid`. This is specific
	// to multiplatform variants so it doesn't leak into plain files
	// where `FooImpl` in `Foo.kt` should still be flagged.
	if hadDotQualifier && len(bareFileName) >= 3 &&
		strings.HasPrefix(decl.name, bareFileName) {
		return nil
	}

	if fileName != decl.name {
		return []scanner.Finding{
			r.Finding(file, file.FlatRow(decl.idx)+1, 1,
				fmt.Sprintf("File name '%s' does not match the single top-level declaration '%s'", fileName, decl.name)),
		}
	}
	return nil
}

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

func (r *MemberNameEqualsClassNameRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *MemberNameEqualsClassNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	className := extractIdentifierFlat(file, idx)
	if className == "" {
		return nil
	}
	classBody := file.FlatFindChild(idx, "class_body")
	if classBody == 0 {
		return nil
	}
	var findings []scanner.Finding
	// Check direct children of class_body for functions and properties
	for i := 0; i < file.FlatChildCount(classBody); i++ {
		child := file.FlatChild(classBody, i)
		switch file.FlatType(child) {
		case "function_declaration", "property_declaration":
			memberName := extractIdentifierFlat(file, child)
			if memberName == className {
				findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
					fmt.Sprintf("Member '%s' has the same name as the containing class", memberName)))
			}
		}
	}
	return findings
}

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

func (r *NoNameShadowingRule) NodeTypes() []string { return []string{"source_file"} }

// Confidence reports a tier-2 (medium) base confidence because this
// rule is highly accurate per-shadow but extremely noisy on real
// codebases (~1,785 findings on pocket-android-app per roadmap/17).
// Many shadows are intentional (scoping functions, kotlinx.coroutines
// flow collectors, builder DSLs). Medium confidence keeps it in
// --min-confidence=medium pipelines but lets strict pipelines filter
// it out of their default gate.
func (r *NoNameShadowingRule) Confidence() float64 { return 0.75 }

func (r *NoNameShadowingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	ctx := &shadowScanCtx{
		file:     file,
		findings: &findings,
		seen:     make(map[noNameShadowFindingKey]bool),
	}
	r.walkScopeFlat(idx, ctx, nil, nil)
	return findings
}

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
				// Tree-sitter-kotlin mis-parses `class X\ninternal constructor(...)`
				// as a property_declaration with user_type "constructor" and a
				// multi_variable_declaration holding the constructor params.
				// Skip these entirely — the real parser would produce a
				// primary_constructor, not an outer-scope declaration.
				if childType == "property_declaration" {
					if ut := file.FlatFindChild(child, "user_type"); ut != 0 {
						if file.FlatNodeTextEquals(ut, "constructor") {
							skipNextLambdaAsClassBody = true
							continue
						}
					}
				}
				// Check for destructuring (multi_variable_declaration child)
				multi := file.FlatFindChild(child, "multi_variable_declaration")
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

func (r *NonBooleanPropertyPrefixedWithIsRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *NonBooleanPropertyPrefixedWithIsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name == "" || !strings.HasPrefix(name, "is") {
		return nil
	}
	// If it is a Boolean property, this is fine
	if isBooleanPropertyFlat(file, idx) {
		return nil
	}
	// Check that there's an explicit non-Boolean type
	text := file.FlatNodeText(idx)
	if strings.Contains(text, ": ") {
		// Has a type annotation that is not Boolean
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Non-Boolean property '%s' should not be prefixed with 'is'", name))}
	}
	return nil
}

func (r *NonBooleanPropertyPrefixedWithIsRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

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

func (r *ObjectPropertyNamingRule) NodeTypes() []string {
	return []string{"object_declaration", "companion_object"}
}

func (r *ObjectPropertyNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	classBody := file.FlatFindChild(idx, "class_body")
	if classBody == 0 {
		return nil
	}
	allowBacking := experiment.Enabled("naming-allow-backing-properties")
	file.FlatForEachChild(classBody, func(propNode uint32) {
		if file.FlatType(propNode) != "property_declaration" {
			return
		}
		name := extractIdentifierFlat(file, propNode)
		if name == "" {
			return
		}
		// Idiomatic Kotlin backing property: private val _foo = ... with
		// a companion public val foo that exposes it. The leading
		// underscore deliberately violates the camelCase pattern.
		if allowBacking && strings.HasPrefix(name, "_") &&
			file.FlatHasModifier(propNode, "private") {
			return
		}
		if file.FlatHasModifier(propNode, "const") {
			if !r.ConstPattern.MatchString(name) {
				findings = append(findings, r.Finding(file, file.FlatRow(propNode)+1, 1,
					fmt.Sprintf("Object const property '%s' does not match pattern: %s", name, r.ConstPattern.String())))
			}
		} else {
			if !r.PropertyPattern.MatchString(name) {
				findings = append(findings, r.Finding(file, file.FlatRow(propNode)+1, 1,
					fmt.Sprintf("Object property '%s' does not match pattern: %s", name, r.PropertyPattern.String())))
			}
		}
	})
	return findings
}

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

func (r *TopLevelPropertyNamingRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *TopLevelPropertyNamingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only check top-level properties (parent is source_file)
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "source_file" {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	// Idiomatic Kotlin backing property on top-level declarations.
	if experiment.Enabled("naming-allow-backing-properties") &&
		strings.HasPrefix(name, "_") &&
		file.FlatHasModifier(idx, "private") {
		return nil
	}
	if file.FlatHasModifier(idx, "const") {
		if !r.ConstPattern.MatchString(name) {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Top-level const property '%s' does not match pattern: %s", name, r.ConstPattern.String()))}
		}
	} else {
		if !r.PropertyPattern.MatchString(name) {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Top-level property '%s' does not match pattern: %s", name, r.PropertyPattern.String()))}
		}
	}
	return nil
}

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

func (r *VariableMaxLengthRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *VariableMaxLengthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasAncestorOfType(idx, "function_body") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name != "" && len(name) > r.MaxLength {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Variable name '%s' exceeds maximum length of %d (length: %d)", name, r.MaxLength, len(name)))}
	}
	return nil
}

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

func (r *VariableMinLengthRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *VariableMinLengthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasAncestorOfType(idx, "function_body") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	// Skip underscore (unused variable convention)
	if name == "_" {
		return nil
	}
	if len(name) < r.MinLength {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Variable name '%s' is below minimum length of %d (length: %d)", name, r.MinLength, len(name)))}
	}
	return nil
}

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
