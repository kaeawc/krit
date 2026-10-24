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
		if nodeHasAncestorTypeFlat(file, n, "import_header", "package_header") {
			return
		}
		t := file.FlatType(n)
		if t != "simple_identifier" && t != "type_identifier" && t != "navigation_expression" && t != "user_type" {
			return
		}
		if name := semantics.ReferenceName(file, n); name != "" {
			// `foo` and foo resolve to the same symbol; canonicalize for lookup.
			names[strings.Trim(name, "`")] = struct{}{}
		}
	})
	return names
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
	symbol := kotlinOperatorSymbol(name)
	if symbol == "" {
		return false
	}
	start, end := int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx))
	if start < 0 || end < start || end > len(file.Content) {
		return false
	}
	before := string(file.Content[:start])
	after := string(file.Content[end:])
	return strings.Contains(before, symbol) || strings.Contains(after, symbol)
}

func kotlinOperatorSymbol(name string) string {
	switch name {
	case "times":
		return "*"
	default:
		return ""
	}
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
