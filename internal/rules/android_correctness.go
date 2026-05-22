package rules

// Android Lint Correctness rules ported from AOSP.
// XML and manifest-only checks live in the Android project-data registries.

import (
	"fmt"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// parseInt parses an integer string, returning 0 on error.
func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
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
func (r *DefaultLocaleRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *CommitPrefEditsRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *CommitTransactionRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *AssertRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *CheckResultRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ShiftFlagsRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UniqueConstantsRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *WrongThreadRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *SQLiteStringRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *RegisteredRule) Confidence() float64 { return api.ConfidenceMedium }

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

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NestedScrollingRule) Confidence() float64 { return api.ConfidenceMedium }

// ScrollViewCountRule detects ScrollView with multiple children.
// Primarily XML (see ScrollViewCountResourceRule); the Kotlin source
// heuristic flags `ScrollView(...).apply { addView; addView }` patterns.
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
func (r *ScrollViewCountRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *SimpleDateFormatRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *StopShipRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StopShipRule) check(ctx *api.Context) {
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
func (r *WrongCallRule) Confidence() float64 { return api.ConfidenceMedium }

// Remaining rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// init() -- register first set of correctness rules
// ---------------------------------------------------------------------------

// Remaining correctness rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// View hierarchy receiver-proof helpers (used by SetTextI18n, WrongCall, ...)
// ---------------------------------------------------------------------------

// androidNonViewReceiverRoots names top-level symbols whose builder/widget
// chains expose setText / onDraw / onMeasure / onLayout methods but are
// NOT android.view.View subtypes. A receiver chain starting with one of
// these names must NOT be flagged by View-targeted rules. Toolbar,
// TabLayout, Snackbar, etc. are deliberately omitted — they ARE View
// subclasses and `view.setText(...)` should still fire.
var androidNonViewReceiverRoots = map[string]struct{}{
	"NotificationCompat":     {},
	"Notification":           {},
	"RemoteViews":            {},
	"AccessibilityNodeInfo":  {},
	"AccessibilityRecord":    {},
	"AccessibilityEvent":     {},
	"SpannableStringBuilder": {},
	"Spannable":              {},
	"PrintAttributes":        {},
	"MediaSessionCompat":     {},
	"MenuItem":               {},
	"MenuItemCompat":         {},
	"ActionBar":              {},
	"AlertDialog":            {},
	"AlertDialogCompat":      {},
	"Preference":             {},
	"PreferenceFragment":     {},
	"Tab":                    {},
}

// androidReceiverChainRoot returns the leftmost identifier of the receiver
// chain reaching `nav`. For `NotificationCompat.Builder(ctx).setText(...)`
// the navigation receiver is the call `NotificationCompat.Builder(ctx)`
// and we want to return "NotificationCompat". Returns "" if the root is
// not a simple identifier (e.g. a function-call result or `this`).
func androidReceiverChainRoot(file *scanner.File, recv uint32) string {
	if file == nil || recv == 0 {
		return ""
	}
	for depth := 0; depth < 32 && recv != 0; depth++ {
		switch file.FlatType(recv) {
		case "simple_identifier", "type_identifier":
			return file.FlatNodeText(recv)
		case "navigation_expression":
			// receiver is leftmost named child of navigation_expression
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "call_expression":
			// recurse into the callee
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "parenthesized_expression":
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "this_expression":
			return "this"
		case "super_expression":
			return "super"
		default:
			return ""
		}
	}
	return ""
}

// androidReceiverIsKnownNonView reports whether the leftmost identifier of
// the receiver chain is a documented non-View symbol whose methods include
// setText / onDraw / onMeasure / onLayout but are NOT View members. Used
// to suppress View-targeted rules on builder / RemoteViews / accessibility
// chains.
func androidReceiverIsKnownNonView(file *scanner.File, recv uint32) bool {
	root := androidReceiverChainRoot(file, recv)
	if root == "" {
		return false
	}
	_, present := androidNonViewReceiverRoots[root]
	return present
}

// androidTypeIsViewSubtype reports whether the resolved type is android.view.View
// or a known subtype. Walks supertypes via the resolver to handle source-declared
// View subclasses too. The resolver may be nil — in that case only the type's
// own simple/FQN names are checked against the View name.
func androidTypeIsViewSubtype(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "View" || name == "android.view.View" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "View" || info.FQN == "android.view.View" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// androidTypeIsTextViewSubtype reports whether the resolved type is
// android.widget.TextView or a known subtype (Button, EditText, ...).
// Walks supertypes via the resolver to handle source-declared TextView
// subclasses too.
func androidTypeIsTextViewSubtype(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "TextView" || name == "android.widget.TextView" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "TextView" || info.FQN == "android.widget.TextView" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// androidEnclosingClassExtendsView reports whether the class_declaration
// enclosing `idx` extends android.view.View or a known View subtype. Walks
// declared supertypes and asks the resolver to follow ancestors.
func androidEnclosingClassExtendsView(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	classIdx, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok || classIdx == 0 {
		return false
	}
	for _, super := range androidDirectSupertypesFlat(file, classIdx) {
		if super.name == "" {
			continue
		}
		if androidSupertypeIsView(super, file, resolver) {
			return true
		}
	}
	return false
}

func androidSupertypeIsView(super androidSupertypeRef, file *scanner.File, resolver typeinfer.TypeResolver) bool {
	// Fast path: the supertype is literally View / qualified android.view.View.
	if super.simple == "View" || super.name == "android.view.View" {
		return true
	}
	if resolver == nil {
		// Without a resolver we can still recognise the common simple names.
		if _, ok := androidKnownViewSimpleNames[super.simple]; ok {
			return true
		}
		return false
	}
	// Resolve the imported FQN first, then walk class hierarchy.
	fqn := super.name
	if !super.qualified {
		if resolved := resolver.ResolveImport(super.simple, file); resolved != "" {
			fqn = resolved
		}
	}
	candidate := &typeinfer.ResolvedType{Name: super.simple, FQN: fqn, Kind: typeinfer.TypeClass}
	if androidTypeIsViewSubtype(resolver, candidate) {
		return true
	}
	// Last resort: the simple-name allowlist (covers cases where the
	// resolver lacks hierarchy info for a framework type).
	if _, ok := androidKnownViewSimpleNames[super.simple]; ok {
		return true
	}
	return false
}

// androidKnownViewSimpleNames is the simple-name fallback for cases where
// the resolver cannot follow the hierarchy. Conservative — only includes
// the framework / AndroidX View types most likely to be subclassed by
// custom views overriding onDraw / onMeasure / onLayout.
var androidKnownViewSimpleNames = map[string]struct{}{
	"View":                 {},
	"ViewGroup":            {},
	"TextView":             {},
	"AppCompatTextView":    {},
	"EditText":             {},
	"AppCompatEditText":    {},
	"Button":               {},
	"AppCompatButton":      {},
	"MaterialButton":       {},
	"ImageView":            {},
	"AppCompatImageView":   {},
	"ShapeableImageView":   {},
	"FrameLayout":          {},
	"LinearLayout":         {},
	"RelativeLayout":       {},
	"ConstraintLayout":     {},
	"MotionLayout":         {},
	"CoordinatorLayout":    {},
	"SurfaceView":          {},
	"TextureView":          {},
	"RecyclerView":         {},
	"ListView":             {},
	"GridView":             {},
	"ScrollView":           {},
	"NestedScrollView":     {},
	"HorizontalScrollView": {},
	"WebView":              {},
	"CardView":             {},
	"AbstractComposeView":  {},
	"ComposeView":          {},
	"FloatingActionButton": {},
	"MaterialCardView":     {},
	"AppCompatCheckBox":    {},
	"AppCompatRadioButton": {},
	"SwitchCompat":         {},
	"TextInputEditText":    {},
}
