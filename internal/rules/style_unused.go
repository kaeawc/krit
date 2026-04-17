package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// UnusedImportRule detects import statements where the imported name is not used.
type UnusedImportRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedImportRule) Confidence() float64 { return 0.75 }

func (r *UnusedImportRule) NodeTypes() []string { return []string{"import_header"} }

func (r *UnusedImportRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "import ") {
		return nil
	}
	imp := strings.TrimPrefix(trimmed, "import ")
	imp = strings.TrimSpace(imp)
	parts := strings.Split(imp, ".")
	shortName := parts[len(parts)-1]
	if shortName == "*" {
		return nil
	}
	if idx := strings.Index(imp, " as "); idx >= 0 {
		shortName = strings.TrimSpace(imp[idx+4:])
	}
	importLine := file.FlatRow(idx) + 1
	// Search file content for usage of the imported name outside import/package lines
	used := false
	for i, line := range file.Lines {
		if i+1 == importLine {
			continue
		}
		lt := strings.TrimSpace(line)
		if strings.HasPrefix(lt, "import ") || strings.HasPrefix(lt, "package ") {
			continue
		}
		if strings.Contains(line, shortName) {
			used = true
			break
		}
	}
	if used {
		return nil
	}
	f := r.Finding(file, importLine, 1,
		fmt.Sprintf("Unused import '%s'.", shortName))
	// Auto-fix: remove the entire import line
	startByte := int(file.FlatStartByte(idx))
	endByte := int(file.FlatEndByte(idx))
	if endByte < len(file.Content) && file.Content[endByte] == '\n' {
		endByte++
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     endByte,
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// UnusedParameterRule detects function parameters that are never used in the body.
type UnusedParameterRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

func (r *UnusedParameterRule) NodeTypes() []string { return []string{"function_declaration"} }

// Confidence reports a tier-2 (medium) base confidence. The rule
// uses strings.Count on the function body to detect parameter usage,
// which false-positives when the parameter name is a substring of
// another identifier (e.g. `id` matching `guid`) and false-negatives
// when usage is stringified or reflection-based. Even with the
// existing exclusion list (override, operator, actual, composable,
// DSL stubs, overloads) the substring heuristic is the tight
// constraint on accuracy.
func (r *UnusedParameterRule) Confidence() float64 { return 0.75 }

func (r *UnusedParameterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	summary := getFunctionDeclSummaryFlat(file, idx)
	// Skip override/open/abstract functions — parameters are required by the contract
	if summary.hasOverride || summary.hasOpen || summary.hasAbstract || summary.hasOperator {
		return nil
	}
	// Skip Kotlin Multiplatform `actual` implementations — the parameter
	// list is dictated by the corresponding `expect` declaration and
	// removing an unused one would break the cross-platform contract.
	if file.FlatHasModifier(idx, "actual") ||
		file.FlatHasModifier(idx, "expect") {
		return nil
	}
	// Skip functions annotated with framework entry-point markers.
	if summary.hasEntryPoint {
		return nil
	}
	if summary.hasComposable {
		return nil
	}
	if summary.hasSubscribeLike {
		return nil
	}
	// Skip functions inside interface declarations — they define contracts
	// that implementers override.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "class_declaration" {
			for i := 0; i < file.FlatChildCount(p); i++ {
				c := file.FlatChild(p, i)
				if file.FlatType(c) == "interface" || (file.FlatType(c) == "class" && file.FlatNodeTextEquals(c, "interface")) {
					return nil
				}
			}
			break
		}
	}
	if summary.body == 0 {
		return nil
	}
	bodyText := file.FlatNodeText(summary.body)
	// Skip no-op stub functions: `fun foo(...) = Unit` or `fun foo(...) { }`.
	// The parameters are intentionally unused.
	trimmedBody := strings.TrimSpace(bodyText)
	if trimmedBody == "= Unit" || trimmedBody == "{}" || trimmedBody == "{ }" {
		return nil
	}
	// Skip stubs whose only action is to throw — these are
	// compiler-replaced intrinsics or platform-specific unimplemented
	// points (e.g., `throw UnsupportedOperationException("Implemented by the compiler")`).
	if strings.Contains(trimmedBody, "throw ") &&
		(strings.HasPrefix(trimmedBody, "{") && strings.Count(trimmedBody, "\n") <= 3) {
		return nil
	}
	if strings.HasPrefix(trimmedBody, "= throw ") ||
		strings.HasPrefix(trimmedBody, "= TODO(") ||
		strings.HasPrefix(trimmedBody, "= error(") {
		return nil
	}
	if summary.paramsNode == 0 {
		return nil
	}
	// Skip overloaded functions — a sibling function with the same name
	// in the same class typically forwards params via `foo(a, b, c)` which
	// the naive string search may miss because bodyText doesn't include
	// sibling scopes. Avoid false positives by skipping overloads.
	if hasSiblingOverloadFlat(file, idx, summary.name) {
		return nil
	}
	// Include default-value expressions of all parameters in the search,
	// since a parameter can legitimately be used as the default value of
	// a subsequent parameter.
	paramsText := file.FlatNodeText(summary.paramsNode)
	searchText := bodyText + "\n" + paramsText
	var findings []scanner.Finding
	for _, param := range summary.params {
		paramName := param.name
		if paramName == "" {
			continue
		}
		if r.AllowedNames != nil && r.AllowedNames.MatchString(paramName) {
			continue
		}
		// Skip parameters explicitly marked unused via @Suppress("unused")
		// or @Suppress("UNUSED_PARAMETER") — the annotation is the
		// author's acknowledgement that the parameter is intentional.
		paramText := file.FlatNodeText(param.idx)
		if strings.Contains(paramText, "@Suppress") &&
			(strings.Contains(paramText, "\"unused\"") ||
				strings.Contains(paramText, "\"UNUSED_PARAMETER\"")) {
			continue
		}
		// Check occurrences, but subtract the parameter's own declaration.
		// The parameter name appears at least once in paramsText (its own
		// declaration); we need another occurrence.
		count := strings.Count(searchText, paramName)
		if count <= 1 {
			f := r.Finding(file, file.FlatRow(param.idx)+1, 1,
				fmt.Sprintf("Parameter '%s' is unused.", paramName))
			findings = append(findings, f)
		}
	}
	return findings
}

func hasSiblingOverloadFlat(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	parent, ok := file.FlatParent(idx)
	for ok && file.FlatType(parent) != "class_body" && file.FlatType(parent) != "source_file" &&
		file.FlatType(parent) != "class_member_declarations" {
		parent, ok = file.FlatParent(parent)
	}
	if !ok {
		return false
	}
	// Linear sibling walk via FirstChild/NextSib. The previous form used
	// FlatNamedChild(parent, i) in a for-i loop, which is O(k) per call and
	// O(N²) across the iteration. For files with many siblings under one
	// parent (generated code, Dagger modules with lots of @Binds methods)
	// this was a latent quadratic.
	for sib := file.FlatFirstChild(parent); sib != 0; sib = file.FlatNextSib(sib) {
		if !file.FlatIsNamed(sib) || sib == idx {
			continue
		}
		if file.FlatType(sib) != "function_declaration" {
			continue
		}
		if extractIdentifierFlat(file, sib) == name {
			return true
		}
	}
	return false
}

// UnusedVariableRule detects local variables that are never used.
type UnusedVariableRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedVariableRule) Confidence() float64 { return 0.75 }

func (r *UnusedVariableRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *UnusedVariableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	parent, ok := file.FlatParent(idx)
	if !ok {
		return nil
	}
	parentType := file.FlatType(parent)
	if parentType == "source_file" ||
		parentType == "class_body" || parentType == "enum_class_body" ||
		parentType == "companion_object" || parentType == "object_declaration" ||
		parentType == "class_member_declarations" {
		return nil
	}
	// Text-level guard: some tree-sitter shapes nest companion-object
	// properties under intermediate wrapper nodes that our type-based
	// parent check misses. Look at preceding lines — if the nearest
	// enclosing declaration keyword is `companion object` or `object`,
	// treat the property as a class-level member rather than a local.
	propLine := file.FlatRow(idx)
	depth := 0
	for i := propLine - 1; i >= 0 && i >= propLine-200; i-- {
		line := file.Lines[i]
		depth += strings.Count(line, "}") - strings.Count(line, "{")
		if depth < 0 {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "companion object") ||
				strings.HasPrefix(trimmed, "object ") ||
				strings.Contains(trimmed, " object ") {
				return nil
			}
			break
		}
	}
	// Walk the ancestor chain to distinguish "inside a function body
	// (local var)" from "inside a class/object/companion body (class
	// member)". Tree-sitter-kotlin has several AST shapes where the
	// immediate parent isn't one of the obvious class-body types:
	//   - `class Foo : Bar by baz { val x }` misparses `baz { ... }` as
	//     a trailing-lambda call, placing members under a lambda_literal
	//     inside a delegation_specifier.
	//   - `companion object { val x }` sometimes nests the property
	//     under additional wrapper nodes that aren't in the direct
	//     parent check above.
	// If we reach a class/object body ancestor BEFORE hitting a function
	// scope, skip the property — it's a class member, not a local.
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		t := file.FlatType(a)
		if t == "delegation_specifier" || t == "explicit_delegation" {
			return nil
		}
		if t == "class_body" || t == "enum_class_body" ||
			t == "companion_object" || t == "object_declaration" ||
			t == "class_member_declarations" {
			return nil
		}
		if t == "function_body" || t == "function_declaration" ||
			t == "anonymous_function" || t == "source_file" {
			break
		}
	}
	text := file.FlatNodeText(idx)
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "val ") && !strings.HasPrefix(trimmed, "var ") {
		return nil
	}
	varName := propertyDeclarationNameFlat(file, idx)
	if varName == "" {
		return nil
	}
	if r.AllowedNames != nil && r.AllowedNames.MatchString(varName) {
		return nil
	}
	// Walk up to the nearest enclosing scope (statements/lambda/function
	// body) and count textual occurrences of varName. If the variable name
	// appears more than once in that scope (beyond its own declaration),
	// it is used. Tree-sitter can glue a following `(expr)` statement into
	// the initializer's text under semicolon-inference ambiguity, so a
	// naive sibling search can miss uses that actually live on the next
	// line of source code.
	scope := parent
	for {
		t := file.FlatType(scope)
		if t == "statements" || t == "function_body" || t == "lambda_literal" ||
			t == "control_structure_body" || t == "source_file" {
			break
		}
		next, ok := file.FlatParent(scope)
		if !ok {
			scope = parent
			break
		}
		scope = next
	}
	scopeText := file.FlatNodeText(scope)
	// A variable is used if its name appears in more places than just the
	// declaration. Count word-boundary occurrences of varName.
	count := countIdentifierOccurrences(scopeText, varName)
	if count <= 1 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Local variable '%s' is never used.", varName))}
	}
	return nil
}

// countIdentifierOccurrences counts word-boundary occurrences of name in s.
// A match requires that the character before and after the match is not
// part of an identifier.
func countIdentifierOccurrences(s, name string) int {
	if name == "" || len(s) < len(name) {
		return 0
	}
	isIdent := func(b byte) bool {
		return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	}
	count := 0
	for i := 0; i+len(name) <= len(s); {
		j := strings.Index(s[i:], name)
		if j < 0 {
			break
		}
		pos := i + j
		start := pos
		end := pos + len(name)
		beforeOK := start == 0 || !isIdent(s[start-1])
		afterOK := end == len(s) || !isIdent(s[end])
		if beforeOK && afterOK {
			count++
		}
		i = end
	}
	return count
}

// UnusedPrivateClassRule detects private classes that are never referenced.
type UnusedPrivateClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateClassRule) Confidence() float64 { return 0.75 }

func (r *UnusedPrivateClassRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *UnusedPrivateClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasModifier(idx, "private") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	content := string(file.Content)
	count := strings.Count(content, name)
	if count <= 1 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Private class '%s' is never used.", name))}
	}
	return nil
}

// entryPointAnnotationNames lists annotation names that mark a declaration as
// a framework entry point (called via reflection, preview tooling, test
// runners, etc.). Private declarations with these annotations should NOT be
// flagged as unused.
var entryPointAnnotationNames = map[string]bool{
	"Preview":           true, // androidx.compose.ui.tooling.preview.Preview
	"SignalPreview":     true, // Signal-specific preview wrapper
	"ComposePreview":    true,
	"PreviewParameter":  true,
	"PreviewLightDark":  true,
	"DarkPreview":       true,
	"LightPreview":      true,
	"NightPreview":      true,
	"DayPreview":        true,
	"Test":              true, // JUnit @Test
	"ParameterizedTest": true,
	"BeforeEach":        true,
	"AfterEach":         true,
	"BeforeAll":         true,
	"AfterAll":          true,
	"Before":            true,
	"After":             true,
	"BeforeClass":       true,
	"AfterClass":        true,
	"ParameterizedRobolectricTestRunner.Parameters": true,
	"Parameters":    true,
	"Provides":      true, // Dagger
	"Binds":         true,
	"BindsInstance": true,
	"Module":        true,
	"JvmStatic":     true,
	"JvmName":       true,
	"JvmField":      true,
	// Reflection/proguard retention markers.
	"Keep":              true, // androidx.annotation.Keep
	"UsedByReflection":  true,
	"UsedByNative":      true,
	"VisibleForTesting": true, // accessed from test module
	"SerializedName":    true, // Gson/Moshi
	"JsonCreator":       true, // Jackson
	"JsonProperty":      true,
}

func flatAnnotationListContains(parentText string, childText string, name string) bool {
	text := childText
	text = strings.TrimPrefix(text, "@")
	if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
		text = text[:parenIdx]
	}
	if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
		text = text[colonIdx+1:]
	}
	text = strings.TrimSpace(text)
	if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
		text = text[dotIdx+1:]
	}
	return text == name || (parentText == text && text == name)
}

func flatHasAnnotationNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods := file.FlatFindChild(idx, "modifiers"); mods != 0 {
		for i := 0; i < file.FlatChildCount(mods); i++ {
			child := file.FlatChild(mods, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			if flatAnnotationListContains("", file.FlatNodeText(child), name) {
				return true
			}
		}
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if flatAnnotationListContains("", file.FlatNodeText(child), name) {
			return true
		}
	}
	return false
}

func flatHasEntryPointAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		text = strings.TrimPrefix(text, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if entryPointAnnotationNames[text] {
			return true
		}
	}
	return false
}

func flatHasFrameworkAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		raw := file.FlatNodeText(child)
		text := strings.TrimPrefix(raw, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if frameworkAnnotationNames[text] {
			return true
		}
		if text == "Suppress" || text == "SuppressWarnings" {
			if strings.Contains(raw, `"unused"`) ||
				strings.Contains(raw, `"UNUSED_PARAMETER"`) ||
				strings.Contains(raw, `"UNUSED_VARIABLE"`) ||
				strings.Contains(raw, `"UnusedPrivateProperty"`) ||
				strings.Contains(raw, `"UnusedPrivateMember"`) ||
				strings.Contains(raw, `"UnusedPrivateFunction"`) ||
				strings.Contains(raw, `"UnusedVariable"`) {
				return true
			}
		}
	}
	return false
}

// UnusedPrivateFunctionRule detects private functions that are never called.
type UnusedPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateFunctionRule) Confidence() float64 { return 0.75 }

func (r *UnusedPrivateFunctionRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *UnusedPrivateFunctionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	if !file.FlatHasModifier(idx, "private") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
		return nil
	}
	// Skip functions annotated with @Preview, @SignalPreview, @ComposePreview,
	// @Test, @JvmName, etc. — these are entry points consumed by frameworks.
	if flatHasEntryPointAnnotation(file, idx) {
		return nil
	}
	content := string(file.Content)
	count := strings.Count(content, name)
	if count <= 1 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Private function '%s' is never called.", name))}
	}
	return nil
}

// UnusedPrivatePropertyRule detects private properties that are never referenced.
type UnusedPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivatePropertyRule) Confidence() float64 { return 0.75 }

func (r *UnusedPrivatePropertyRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *UnusedPrivatePropertyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasModifier(idx, "private") {
		return nil
	}
	// Skip properties with framework annotations indicating external use
	// (DI, test mocks, view binding, Compose preview, etc.).
	if flatHasFrameworkAnnotation(file, idx) {
		return nil
	}
	name := propertyDeclarationNameFlat(file, idx)
	if name == "" {
		return nil
	}
	if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
		return nil
	}
	// Skip TAG = Log.tag(...) — the Log.tag() call has side effects and
	// the TAG is a convention that may be referenced later.
	if name == "TAG" {
		nodeText := file.FlatNodeText(idx)
		if strings.Contains(nodeText, "Log.tag(") {
			return nil
		}
	}
	content := string(file.Content)
	count := strings.Count(content, name)
	if count <= 1 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Private property '%s' is never used.", name))}
	}
	return nil
}

// UnusedPrivateMemberRule is a combined check for unused private members.
type UnusedPrivateMemberRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames    *regexp.Regexp
	IgnoreAnnotated []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateMemberRule) Confidence() float64 { return 0.75 }

func (r *UnusedPrivateMemberRule) NodeTypes() []string {
	return []string{"class_declaration", "function_declaration", "property_declaration"}
}

func (r *UnusedPrivateMemberRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasModifier(idx, "private") {
		return nil
	}
	mods := file.FlatFindChild(idx, "modifiers")
	modsText := ""
	if mods != 0 {
		modsText = file.FlatNodeText(mods)
	}
	for _, ann := range r.IgnoreAnnotated {
		if strings.Contains(modsText, ann) {
			return nil
		}
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" && file.FlatType(idx) == "property_declaration" {
		name = propertyDeclarationNameFlat(file, idx)
	}
	if name == "" {
		return nil
	}
	if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
		return nil
	}
	content := string(file.Content)
	count := strings.Count(content, name)
	if count <= 1 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Private member '%s' is never used.", name))}
	}
	return nil
}
