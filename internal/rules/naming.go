package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
func (r *ClassNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// FunctionNamingRule checks function names match a pattern.
type FunctionNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern               *regexp.Regexp
	IgnoreAnnotated       []string
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
	AllowBacktickNames    bool           // if true (default), backtick-quoted names are skipped
	ExcludeClassPattern   *regexp.Regexp // classes matching this are excluded
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *VariableNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func variableNamingIsFunctionLocalPropertyFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_body":
			return true
		case "class_body", "class_declaration", "object_declaration", "companion_object":
			return false
		}
	}
	return false
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
func (r *PackageNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EnumNamingRule checks enum entry names.
type EnumNamingRule struct {
	FlatDispatchBase
	BaseRule
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *EnumNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func isTestSupportFile(path string) bool {
	if scanner.IsTestFile(path) {
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
func (r *BooleanPropertyNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func isBooleanPropertyFlat(file *scanner.File, idx uint32) bool {
	declaredType := extractPropertyTypeFlat(file, idx)
	if declaredType == "Boolean" || declaredType == "Boolean?" {
		return true
	}
	if declaredType != "" {
		return false
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
func (r *ConstructorParameterNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// ForbiddenClassNameRule flags disallowed class names.
type ForbiddenClassNameRule struct {
	FlatDispatchBase
	BaseRule
	ForbiddenNames []string
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *ForbiddenClassNameRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// FunctionNameMaxLengthRule flags function names exceeding a max length.
type FunctionNameMaxLengthRule struct {
	FlatDispatchBase
	BaseRule
	MaxLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNameMaxLengthRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// FunctionNameMinLengthRule flags function names below a min length.
type FunctionNameMinLengthRule struct {
	FlatDispatchBase
	BaseRule
	MinLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *FunctionNameMinLengthRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *FunctionParameterNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *InvalidPackageDeclarationRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func invalidPackageDeclarationIgnoredPath(path string) bool {
	normalized := filepath.ToSlash(path)
	for _, marker := range []string{
		"/.claude/skills/",
		"/.github/",
		"/.gitlab/",
		"/.circleci/",
		"/.buildkite/",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
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
func (r *LambdaParameterNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// MatchingDeclarationNameRule checks that a file with a single non-private
// top-level class or object has a filename matching that declaration's name.
// Only KtClassOrObject declarations (class, interface, object, enum) are
// counted; private classes are excluded; top-level functions, properties and
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
func (r *MatchingDeclarationNameRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *MemberNameEqualsClassNameRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// declInitializerFlat returns the named initializer expression of a property
// or variable declaration (the node following the `=` token), or 0 if none.
func declInitializerFlat(file *scanner.File, decl uint32) uint32 {
	seenEq := false
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "=" {
			seenEq = true
			continue
		}
		if seenEq && file.FlatIsNamed(c) {
			t := file.FlatType(c)
			if t == "line_comment" || t == "multiline_comment" {
				continue
			}
			return c
		}
	}
	return 0
}

func containsIdentifierToken(text, name string) bool {
	if name == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], name)
		if idx < 0 {
			return false
		}
		pos := start + idx
		beforeOK := pos == 0 || !isIdentByte(text[pos-1])
		after := pos + len(name)
		afterOK := after == len(text) || !isIdentByte(text[after])
		if beforeOK && afterOK {
			return true
		}
		start = after
		if start >= len(text) {
			return false
		}
	}
}

// containsIdentifierStart reports whether text contains an occurrence of
// prefix where the preceding byte is not part of a Kotlin identifier,
// so the match begins a new identifier token. Unlike
// containsIdentifierToken this does not require a trailing boundary,
// which lets `Java` match `JavaSerializer` while still excluding
// `parseJavadoc`.
func containsIdentifierStart(text, prefix string) bool {
	if prefix == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], prefix)
		if idx < 0 {
			return false
		}
		pos := start + idx
		if pos == 0 || !isIdentByte(text[pos-1]) {
			return true
		}
		start = pos + 1
		if start >= len(text) {
			return false
		}
	}
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

// NonBooleanPropertyPrefixedWithIsRule flags non-Boolean properties that start with "is".
type NonBooleanPropertyPrefixedWithIsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *NonBooleanPropertyPrefixedWithIsRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *ObjectPropertyNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *TopLevelPropertyNamingRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// VariableMaxLengthRule flags variable names that are too long.
type VariableMaxLengthRule struct {
	FlatDispatchBase
	BaseRule
	MaxLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *VariableMaxLengthRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// VariableMinLengthRule flags variable names that are too short.
type VariableMinLengthRule struct {
	FlatDispatchBase
	BaseRule
	MinLength int
}

// Confidence holds the 0.95 dispatch default. Naming rule. Detection regex-matches the declared identifier, which is
// deterministic — the identifier is what it is. No heuristic path.
// Classified per roadmap/17.
func (r *VariableMinLengthRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
