package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnusedImportRule detects import statements where the imported name is not used.
type UnusedImportRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule.
// Detection uses a structural per-file reference-name index, but it does not
// fully resolve ambiguous imports.
func (r *UnusedImportRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedImportRule) hasReferenceName(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	names := filefacts.FileFact(fileFactsCache(), file, slotUnusedImportRefs, func() map[string]struct{} {
		return collectReferenceNamesOutsideImports(file)
	})
	_, ok := names[name]
	return ok
}

func collectReferenceNamesOutsideImports(file *scanner.File) map[string]struct{} {
	names := make(map[string]struct{})
	file.FlatWalkAllNodes(0, func(n uint32) {
		t := file.FlatType(n)
		// The grammar can attach a trailing KDoc to the preceding import_header.
		if t == "multiline_comment" {
			// detekt only reads a KDoc that documents a declaration; one inside a body is a plain comment.
			if kdocDocumentsDeclaration(file, n) {
				addKDocReferenceNames(file.FlatNodeText(n), names)
			}
			return
		}
		if nodeHasAncestorTypeFlat(file, n, "import_header", "package_header") {
			return
		}
		// The grammar drops the type arguments of `Type<Arg>::member` and the type of a destructured lambda
		// parameter `{ (a): Type -> }`, so read the words of those spans from the source.
		switch t {
		case "callable_reference":
			addIdentifierWords(file.FlatNodeText(n), names)
			return
		case "lambda_literal":
			text := file.FlatNodeText(n)
			if arrow := strings.Index(text, "->"); arrow >= 0 && strings.Contains(text[:arrow], ")") {
				addIdentifierWords(text[:arrow], names)
			}
			return
		case "interpolated_identifier":
			names[strings.Trim(file.FlatNodeText(n), "`")] = struct{}{}
			return
		}
		if t != "simple_identifier" && t != "type_identifier" && t != "navigation_expression" && t != "user_type" {
			return
		}
		// A declaration's own name is not a reference to an import of the same name.
		if (t == "simple_identifier" || t == "type_identifier") && isDeclarationNameFlat(file, n) {
			return
		}
		if name := semantics.ReferenceName(file, n); name != "" {
			// `foo` and foo resolve to the same symbol; canonicalize for lookup.
			names[strings.Trim(name, "`")] = struct{}{}
		}
	})
	return names
}

// isDeclarationNameFlat reports whether the identifier is the name a declaration introduces.
func isDeclarationNameFlat(file *scanner.File, id uint32) bool {
	parent, ok := file.FlatParent(id)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "function_declaration", "class_declaration", "object_declaration", "type_alias", "parameter",
		"class_parameter", "variable_declaration", "enum_entry", "type_parameter":
		// The name is the first identifier; one after `=` is a default value and does reference imports.
		for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
			if c == id {
				return true
			}
			if t := file.FlatType(c); t == "=" || t == "simple_identifier" || t == "type_identifier" {
				return false
			}
		}
	}
	return false
}

// kdocDocumentsDeclaration reports whether a KDoc comment sits right before a declaration, or is attached to an
// import or package header by the grammar.
func kdocDocumentsDeclaration(file *scanner.File, comment uint32) bool {
	if nodeHasAncestorTypeFlat(file, comment, "import_header", "package_header") {
		return true
	}
	for next := file.FlatNextSib(comment); next != 0; next = file.FlatNextSib(next) {
		if isFlatCommentNode(file, next) {
			continue
		}
		switch file.FlatType(next) {
		case "function_declaration", "class_declaration", "object_declaration", "property_declaration",
			"secondary_constructor", "type_alias", "companion_object", "enum_entry", "class_parameter",
			"anonymous_initializer", "getter", "setter":
			return true
		}
		return false
	}
	return false
}

var identifierWordRe = regexp.MustCompile("[A-Za-z_][A-Za-z0-9_]*")

func addIdentifierWords(text string, names map[string]struct{}) {
	for _, w := range identifierWordRe.FindAllString(text, -1) {
		names[w] = struct{}{}
	}
}

var (
	kdocBracketRefRe = regexp.MustCompile(`\[([^\]\s]+)\]`)
	kdocTagRefRe     = regexp.MustCompile(`(?m)^[ \t]*(?:/\*\*[ \t]*|\*[ \t]*)@(?:see|throws|exception)[ \t]+([^\s{}\[\]]+)`)
)

// addKDocReferenceNames records the leading identifier of each [Name], @see,
// @throws and {@link} reference in a KDoc block, so imports used only in docs
// count as used.
func addKDocReferenceNames(text string, names map[string]struct{}) {
	if !strings.HasPrefix(text, "/**") {
		return
	}
	for _, re := range []*regexp.Regexp{kdocBracketRefRe, kdocTagRefRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			ref := strings.Trim(m[1], "`")
			if dot := strings.IndexByte(ref, '.'); dot >= 0 {
				ref = ref[:dot]
			}
			ref = strings.TrimRight(ref, ".,;:)(")
			if hash := strings.IndexByte(ref, '#'); hash >= 0 {
				ref = ref[:hash]
			}
			ref = strings.Trim(ref, "`")
			if ref != "" {
				names[ref] = struct{}{}
			}
		}
	}
}

// UnusedPrivateClassRule detects private classes that are never referenced.
type UnusedPrivateClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateClassRule) Confidence() float64 { return api.ConfidenceMedium }

// entryPointAnnotationNames lists annotation names that mark a declaration as
// a framework entry point (called via reflection, preview tooling, test
// runners, etc.). Private declarations with these annotations should NOT be
// flagged as unused.
var entryPointAnnotationNames = map[string]bool{
	"Preview":           true, // androidx.compose.ui.tooling.preview.Preview
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
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
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
	if flatChildrenHaveEntryPointAnnotation(file, idx) {
		return true
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if flatChildrenHaveEntryPointAnnotation(file, mods) {
			return true
		}
	}
	return flatLeadingTextHasEntryPointAnnotation(file.FlatNodeText(idx))
}

func flatChildrenHaveEntryPointAnnotation(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if isEntryPointAnnotationName(flatAnnotationSimpleName(file.FlatNodeText(child))) {
			return true
		}
	}
	return false
}

func flatAnnotationSimpleName(text string) string {
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
	return text
}

func flatLeadingTextHasEntryPointAnnotation(text string) bool {
	if funIdx := strings.Index(text, "fun "); funIdx >= 0 {
		text = text[:funIdx]
	}
	for {
		at := strings.Index(text, "@")
		if at < 0 {
			return false
		}
		text = text[at+1:]
		end := 0
		for end < len(text) {
			ch := text[end]
			if (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == '_' || ch == '.' || ch == ':' {
				end++
				continue
			}
			break
		}
		if end > 0 && isEntryPointAnnotationName(flatAnnotationSimpleName(text[:end])) {
			return true
		}
		text = text[end:]
	}
}

func isEntryPointAnnotationName(name string) bool {
	if entryPointAnnotationNames[name] {
		return true
	}
	// Compose projects commonly wrap @Preview in local annotations such as
	// @ConstellationPreviews, @PreviewAll, or @PreviewLightDarkDevices. Those
	// functions are discovered by tooling and should not be removed as unused.
	return strings.Contains(name, "Preview")
}

func privateOperatorFunctionHasSymbolUse(file *scanner.File, idx uint32, name string) bool {
	if file == nil || !file.FlatHasModifier(idx, "operator") {
		return false
	}
	// invoke calls `x()` have no symbol that identifies the receiver type.
	if name == "invoke" {
		return true
	}
	start, end := int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx))
	if start < 0 || end < start || end > len(file.Content) {
		return false
	}
	outside := string(file.Content[:start]) + "\n" + string(file.Content[end:])
	if operatorComponentName.MatchString(name) {
		return destructuringUse.MatchString(outside)
	}
	for _, symbol := range kotlinOperatorSymbols(name) {
		if strings.Contains(outside, symbol) {
			return true
		}
	}
	return false
}

var (
	operatorComponentName = regexp.MustCompile(`^component\d+$`)
	destructuringUse      = regexp.MustCompile(`\b(?:val|var)\s*\(|\bfor\s*\(\s*\(|[{,(]\s*\(\s*\w+\s*,\s*\w+[^)]*\)\s*->`)
)

// kotlinOperatorSymbols returns the source tokens that call an operator function.
func kotlinOperatorSymbols(name string) []string {
	switch name {
	case "plus", "unaryPlus":
		return []string{"+"}
	case "minus", "unaryMinus":
		return []string{"-"}
	case "times":
		return []string{"*"}
	case "div":
		return []string{"/"}
	case "rem", "mod":
		return []string{"%"}
	case "not":
		return []string{"!"}
	case "inc":
		return []string{"++"}
	case "dec":
		return []string{"--"}
	case "get", "set":
		return []string{"["}
	case "contains":
		return []string{" in ", "!in"}
	case "iterator":
		return []string{" in "}
	case "compareTo":
		return []string{"<", ">"}
	case "rangeTo":
		return []string{".."}
	case "rangeUntil":
		return []string{"..<"}
	case "equals":
		return []string{"=="}
	case "getValue", "setValue", "provideDelegate":
		return []string{" by "}
	case "plusAssign":
		return []string{"+="}
	case "minusAssign":
		return []string{"-="}
	case "timesAssign":
		return []string{"*="}
	case "divAssign":
		return []string{"/="}
	case "remAssign", "modAssign":
		return []string{"%="}
	}
	return nil
}

func flatHasFrameworkAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
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
func (r *UnusedPrivateFunctionRule) Confidence() float64 { return api.ConfidenceMedium }

// UnusedPrivatePropertyRule detects private properties that are never referenced.
type UnusedPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivatePropertyRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UnusedPrivateMemberRule) Confidence() float64 { return api.ConfidenceMedium }

// propertyDeclarationHasValVar reports whether a property_declaration was introduced by val or var.
func propertyDeclarationHasValVar(file *scanner.File, idx uint32) bool {
	kind, ok := file.FlatFindChild(idx, "binding_pattern_kind")
	if !ok {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(kind))
	return text == "val" || text == "var"
}

// unusedPropertySideEffectCall matches calls that register, start or subscribe something when evaluated.
var unusedPropertySideEffectCall = regexp.MustCompile(`^(subscribe\w*|launch\w*|async|register\w*|observe\w*|attach\w*|listen\w*|enqueue|schedule\w*|collect|connect|bindService|start|startService|add\w*(Listener|Observer|Callback))$`)

// unusedPropertyInitializerHasSideEffects reports a direct initializer that calls something with an
// observable effect. A delegate such as `by lazy` does not run until read, so it is not checked.
func unusedPropertyInitializerHasSideEffects(file *scanner.File, prop uint32) bool {
	init := propertyInitializerExpression(file, prop)
	if init == 0 {
		return false
	}
	found := false
	check := func(call uint32) {
		if unusedPropertySideEffectCall.MatchString(flatCallExpressionName(file, call)) {
			found = true
		}
	}
	if file.FlatType(init) == "call_expression" {
		check(init)
	}
	file.FlatWalkNodes(init, "call_expression", check)
	return found
}
