package rules

// Android Lint Correctness rules ported from AOSP.
// 85 rules total. Rules that can detect issues in Kotlin source have real
// implementations; XML/Manifest-only rules are stubs (Check returns nil).

import (
	"fmt"
	"strconv"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// parseInt parses an integer string, returning 0 on error.
func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// parseFloat parses a float string, returning 0 on error.
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// ---------------------------------------------------------------------------
// helper: quick AndroidRule constructor
// ---------------------------------------------------------------------------

func alcRule(id, brief string, sev AndroidLintSeverity, pri int) AndroidRule {
	return AndroidRule{
		BaseRule:   BaseRule{RuleName: id, RuleSetName: androidRuleSet, Sev: alcSevToSev(sev)},
		IssueID:    id,
		Brief:      brief,
		Category:   ALCCorrectness,
		ALSeverity: sev,
		Priority:   pri,
		Origin:     "AOSP Android Lint",
	}
}

func alcSevToSev(s AndroidLintSeverity) string {
	switch s {
	case ALSFatal, ALSError:
		return "error"
	case ALSWarning:
		return "warning"
	default:
		return "info"
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// Kotlin-detectable rule types
// ---------------------------------------------------------------------------

// DefaultLocaleRule detects String.format() without Locale,
// .toLowerCase()/.toUpperCase() without Locale.
type DefaultLocaleRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DefaultLocaleRule) Confidence() float64 { return 0.75 }

// CommitPrefEditsRule detects SharedPreferences.edit() without .commit() or .apply().
type CommitPrefEditsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CommitPrefEditsRule) Confidence() float64 { return 0.75 }

// CommitTransactionRule detects FragmentTransaction without .commit().
type CommitTransactionRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CommitTransactionRule) Confidence() float64 { return 0.75 }

// AssertRule detects assert statements (disabled on Android).
type AssertRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AssertRule) Confidence() float64 { return 0.75 }

// CheckResultRule detects ignoring return values annotated with @CheckResult.
type CheckResultRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CheckResultRule) Confidence() float64 { return 0.75 }

// ShiftFlagsRule detects flag constants not using shift operators.
type ShiftFlagsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShiftFlagsRule) Confidence() float64 { return 0.75 }

// UniqueConstantsRule detects duplicate annotation constant values.
type UniqueConstantsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UniqueConstantsRule) Confidence() float64 { return 0.75 }

// WrongThreadRule detects UI operations on wrong thread.
type WrongThreadRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongThreadRule) Confidence() float64 { return 0.75 }

// SQLiteStringRule detects SQL string issues (using string instead of TEXT).
type SQLiteStringRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SQLiteStringRule) Confidence() float64 { return 0.75 }

// RegisteredRule detects Activity/Service/BroadcastReceiver/ContentProvider subclasses
// and flags them with a reminder to register in AndroidManifest.xml.
// Skips classes annotated with @AndroidEntryPoint (Hilt auto-registers).
type RegisteredRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RegisteredRule) Confidence() float64 { return 0.75 }

var androidComponentBases = map[string]string{
	"Activity":             "Activity",
	"android.app.Activity": "Activity",
	"AppCompatActivity":    "Activity",
	"androidx.appcompat.app.AppCompatActivity": "Activity",
	"FragmentActivity":                         "Activity",
	"androidx.fragment.app.FragmentActivity":   "Activity",
	"ComponentActivity":                        "Activity",
	"androidx.activity.ComponentActivity":      "Activity",
	"Service":                                  "Service",
	"android.app.Service":                      "Service",
	"IntentService":                            "Service",
	"android.app.IntentService":                "Service",
	"LifecycleService":                         "Service",
	"androidx.lifecycle.LifecycleService":      "Service",
	"JobIntentService":                         "Service",
	"androidx.core.app.JobIntentService":       "Service",
	"BroadcastReceiver":                        "BroadcastReceiver",
	"android.content.BroadcastReceiver":        "BroadcastReceiver",
	"ContentProvider":                          "ContentProvider",
	"android.content.ContentProvider":          "ContentProvider",
}

type androidSupertypeRef struct {
	name      string
	simple    string
	qualified bool
}

type androidClassDecl struct {
	idx   uint32
	owner uint32
}

// androidComponentType returns the Android component type for a
// class_declaration node, or "". It only inspects declaration/annotation
// AST nodes and uses resolver hierarchy data when the dispatcher provides it.
func androidComponentType(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) (string, float64) {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return "", 0
	}
	if androidHasAnnotationFlat(file, idx, "AndroidEntryPoint") || file.FlatHasModifier(idx, "abstract") {
		return "", 0
	}

	decls := androidSameFileClassDeclarations(file)
	componentType, confidence := androidComponentTypeForDeclaration(file, idx, resolver, decls, make(map[uint32]bool))
	if componentType == "" {
		return "", 0
	}
	return componentType, confidence
}

func androidComponentTypeForDeclaration(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver, decls map[string][]androidClassDecl, seen map[uint32]bool) (string, float64) {
	if idx == 0 || seen[idx] {
		return "", 0
	}
	seen[idx] = true

	owner := androidEnclosingOwner(file, idx)
	for _, super := range androidDirectSupertypesFlat(file, idx) {
		if super.name == "" {
			continue
		}
		if !super.qualified {
			if localIdx := androidSameOwnerClassDeclaration(decls, super.simple, owner, idx); localIdx != 0 {
				componentType, confidence := androidComponentTypeForDeclaration(file, localIdx, resolver, decls, seen)
				if componentType != "" {
					if confidence == 0 || confidence > 0.85 {
						confidence = 0.85
					}
					return componentType, confidence
				}
				// A same-scope source declaration resolves the reference and
				// shadows framework simple names.
				continue
			}
		}
		if componentType, confidence, resolved := androidComponentTypeForSupertype(file, super, resolver); resolved && componentType != "" {
			return componentType, confidence
		}
	}
	return "", 0
}

func androidComponentTypeForSupertype(file *scanner.File, super androidSupertypeRef, resolver typeinfer.TypeResolver) (string, float64, bool) {
	if componentType := androidKnownComponentType(super.name, true); componentType != "" && super.qualified {
		return componentType, 0.95, true
	}

	if resolver != nil {
		if !super.qualified {
			if fqn := resolver.ResolveImport(super.simple, file); fqn != "" {
				if componentType := androidKnownComponentType(fqn, false); componentType != "" {
					return componentType, 0.95, true
				}
				if componentType, resolved := androidComponentTypeFromHierarchy(fqn, resolver, make(map[string]bool)); resolved {
					return componentType, 0.90, true
				}
				return "", 0, true
			}
		}
		if componentType, resolved := androidComponentTypeFromHierarchy(super.name, resolver, make(map[string]bool)); resolved {
			return componentType, 0.90, true
		}
	}

	if componentType := androidKnownComponentType(super.name, false); componentType != "" && super.qualified {
		return componentType, 0.95, true
	}
	if componentType := androidKnownComponentType(super.simple, true); componentType != "" && !super.qualified {
		return componentType, 0.65, true
	}
	return "", 0, false
}

func androidComponentTypeFromHierarchy(typeName string, resolver typeinfer.TypeResolver, seen map[string]bool) (string, bool) {
	if typeName == "" || resolver == nil || seen[typeName] {
		return "", false
	}
	seen[typeName] = true

	info := resolver.ClassHierarchy(typeName)
	if info == nil {
		return "", false
	}
	if componentType := androidKnownComponentType(info.FQN, false); componentType != "" {
		return componentType, true
	}
	for _, super := range info.Supertypes {
		if componentType := androidKnownComponentType(super, false); componentType != "" {
			return componentType, true
		}
		if componentType, resolved := androidComponentTypeFromHierarchy(super, resolver, seen); resolved && componentType != "" {
			return componentType, true
		}
	}
	return "", true
}

func androidKnownComponentType(name string, allowSimple bool) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if componentType := androidComponentBases[name]; componentType != "" {
		if allowSimple || strings.Contains(name, ".") {
			return componentType
		}
	}
	return ""
}

func androidDirectSupertypesFlat(file *scanner.File, idx uint32) []androidSupertypeRef {
	var supertypes []androidSupertypeRef
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			if ref := androidSupertypeRefFromDelegationFlat(file, child); ref.name != "" {
				supertypes = append(supertypes, ref)
			}
		case "delegation_specifiers":
			for spec := file.FlatFirstChild(child); spec != 0; spec = file.FlatNextSib(spec) {
				if file.FlatType(spec) != "delegation_specifier" {
					continue
				}
				if ref := androidSupertypeRefFromDelegationFlat(file, spec); ref.name != "" {
					supertypes = append(supertypes, ref)
				}
			}
		}
	}
	return supertypes
}

func androidSupertypeRefFromDelegationFlat(file *scanner.File, idx uint32) androidSupertypeRef {
	ut, _ := file.FlatFindChild(idx, "user_type")
	if ut == 0 {
		if call, ok := file.FlatFindChild(idx, "constructor_invocation"); ok {
			ut, _ = file.FlatFindChild(call, "user_type")
		}
	}
	if ut == 0 {
		return androidSupertypeRef{}
	}
	name := androidUserTypeNameFlat(file, ut)
	return androidSupertypeRef{
		name:      name,
		simple:    androidSimpleName(name),
		qualified: strings.Contains(name, "."),
	}
}

func androidUserTypeNameFlat(file *scanner.File, idx uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if cut := strings.Index(text, "<"); cut >= 0 {
		text = text[:cut]
	}
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "?")
	return strings.ReplaceAll(text, " ", "")
}

func androidSimpleName(name string) string {
	name = strings.TrimSpace(name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

func androidSameFileClassDeclarations(file *scanner.File) map[string][]androidClassDecl {
	decls := make(map[string][]androidClassDecl)
	if file == nil {
		return decls
	}
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		name := extractIdentifierFlat(file, candidate)
		if name == "" {
			return
		}
		decls[name] = append(decls[name], androidClassDecl{
			idx:   candidate,
			owner: androidEnclosingOwner(file, candidate),
		})
	})
	return decls
}

func androidSameOwnerClassDeclaration(decls map[string][]androidClassDecl, name string, owner uint32, exclude uint32) uint32 {
	for _, decl := range decls[name] {
		if decl.idx != exclude && decl.owner == owner {
			return decl.idx
		}
	}
	return 0
}

func androidEnclosingOwner(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			return p
		}
	}
	return 0
}

func androidHasAnnotationFlat(file *scanner.File, idx uint32, annotationName string) bool {
	mods, ok := file.FlatFindChild(idx, "modifiers")
	if !ok {
		return false
	}
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "annotation" {
			continue
		}
		if androidAnnotationSimpleName(file.FlatNodeText(child)) == annotationName {
			return true
		}
	}
	return false
}

func androidAnnotationSimpleName(text string) string {
	text = strings.TrimSpace(strings.TrimPrefix(text, "@"))
	if colon := strings.Index(text, ":"); colon >= 0 {
		text = text[colon+1:]
	}
	if paren := strings.Index(text, "("); paren >= 0 {
		text = text[:paren]
	}
	return androidSimpleName(strings.TrimSpace(text))
}

// formatRegisteredMsg builds the manifest registration message.
func formatRegisteredMsg(className, componentType string) string {
	return fmt.Sprintf("%s extends %s and should be registered in AndroidManifest.xml.", className, componentType)
}

// NestedScrollingRule detects nested scrolling views.
type NestedScrollingRule struct {
	FlatDispatchBase
	AndroidRule
}

var nestedScrollNames = map[string]bool{
	"ScrollView": true, "LazyColumn": true, "LazyRow": true,
	"HorizontalPager": true, "VerticalPager": true,
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NestedScrollingRule) Confidence() float64 { return 0.75 }

// ScrollViewCountRule detects ScrollView with multiple children.
// Primarily XML; stub.
type ScrollViewCountRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ScrollViewCountRule) Confidence() float64 { return 0.75 }

// SimpleDateFormatRule detects SimpleDateFormat without Locale.
type SimpleDateFormatRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SimpleDateFormatRule) Confidence() float64 { return 0.75 }

// SetTextI18nRule detects setText() with hardcoded text.
type SetTextI18nRule struct {
	FlatDispatchBase
	AndroidRule
}

// StopShipRule detects STOPSHIP comments.
type StopShipRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *StopShipRule) Confidence() float64 { return 0.75 }

func (r *StopShipRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "STOPSHIP") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"STOPSHIP comment found. This must be resolved before shipping."))
		}
	}
}

// WrongCallRule detects calling the wrong View draw/layout methods.
type WrongCallRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongCallRule) Confidence() float64 { return 0.75 }

// Remaining rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// init() -- register first set of correctness rules
// ---------------------------------------------------------------------------

// Remaining correctness rules are in android_correctness_checks.go
