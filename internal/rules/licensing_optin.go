package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// wellKnownOptInMarkers lists OptIn marker classes shipped with widely used
// Kotlin and AndroidX libraries. The rule treats any marker outside this set
// as a likely stale or typo'd reference. Custom project markers can be added
// via the `additionalMarkers` config option.
var wellKnownOptInMarkers = map[string]bool{
	// kotlin stdlib
	"ExperimentalStdlibApi":     true,
	"ExperimentalUnsignedTypes": true,
	"ExperimentalMultiplatform": true,
	"ExperimentalTypeInference": true,
	"ExperimentalContracts":     true,
	"ExperimentalTime":          true,
	"ExperimentalSubclassOptIn": true,
	"RequiresOptIn":             true,

	// kotlinx.coroutines
	"ExperimentalCoroutinesApi": true,
	"DelicateCoroutinesApi":     true,
	"InternalCoroutinesApi":     true,
	"FlowPreview":               true,
	"ObsoleteCoroutinesApi":     true,

	// kotlinx.serialization
	"ExperimentalSerializationApi": true,
	"InternalSerializationApi":     true,

	// Compose / AndroidX
	"ExperimentalComposeApi":             true,
	"ExperimentalComposeUiApi":           true,
	"ExperimentalAnimationApi":           true,
	"ExperimentalAnimationGraphicsApi":   true,
	"ExperimentalFoundationApi":          true,
	"ExperimentalLayoutApi":              true,
	"ExperimentalMaterialApi":            true,
	"ExperimentalMaterial3Api":           true,
	"ExperimentalMaterial3ExpressiveApi": true,
	"ExperimentalTextApi":                true,
	"ExperimentalPagingApi":              true,
	"ExperimentalPagerApi":               true,
	"ExperimentalCoilApi":                true,
	"ExperimentalGlideComposeApi":        true,
	"ExperimentalComposableApi":          true,
	"InternalComposeApi":                 true,
	"ExperimentalComposeRuntimeApi":      true,
	"ExperimentalGraphicsApi":            true,

	// kotlinx.atomicfu / kotlinx.io / others
	"ExperimentalAtomicfuApi": true,
	"ExperimentalIoApi":       true,
	"InternalAtomicfuApi":     true,

	// AndroidX miscellaneous
	"ExperimentalCarApi":              true,
	"ExperimentalHorologistApi":       true,
	"ExperimentalLifecycleComposeApi": true,
	"ExperimentalMediaCompatApi":      true,
	"ExperimentalNavigationApi":       true,
	"ExperimentalWindowApi":           true,
	"ExperimentalWindowCoreApi":       true,
}

// optInMarkerName extracts the marker class simple name from the expression
// inside `@OptIn(...)`. tree-sitter parses `Foo::class` as either a
// class_literal or a callable_reference, and `pkg.Foo::class` as a
// navigation_expression — all three shapes appear in real Kotlin sources.
func optInMarkerName(file *scanner.File, expr uint32) string {
	if expr == 0 {
		return ""
	}
	switch file.FlatType(expr) {
	case "class_literal":
		name := ""
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "simple_identifier":
				name = file.FlatNodeText(c)
			case "navigation_expression":
				if n := flatNavigationExpressionLastIdentifier(file, c); n != "" {
					name = n
				}
			}
		}
		return name
	case "callable_reference":
		name := ""
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "simple_identifier", "type_identifier":
				name = file.FlatNodeText(c)
			case "user_type":
				if ident := flatLastChildOfType(file, c, "type_identifier"); ident != 0 {
					name = file.FlatNodeText(ident)
				}
			case "navigation_expression":
				if n := flatNavigationExpressionLastIdentifier(file, c); n != "" {
					name = n
				}
			}
		}
		return name
	case "navigation_expression":
		if n := flatNavigationExpressionLastIdentifier(file, expr); n != "" {
			return n
		}
	}
	return ""
}

// OptInMarkerNotRecognisedRule flags `@OptIn(Foo::class)` annotations whose
// marker does not appear in the embedded well-known markers list. A stale
// reference often indicates an experimental API that has graduated or been
// removed.
type OptInMarkerNotRecognisedRule struct {
	FlatDispatchBase
	BaseRule
	AdditionalMarkers []string
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule. The
// embedded marker list cannot enumerate every project-local OptIn marker, so
// callers can extend it via configuration. Classified per roadmap/17.
func (r *OptInMarkerNotRecognisedRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

func (r *OptInMarkerNotRecognisedRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "OptIn" {
		return
	}
	ctor, ok := file.FlatFindChild(idx, "constructor_invocation")
	if !ok {
		return
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		name := optInMarkerName(file, expr)
		if name == "" {
			continue
		}
		if wellKnownOptInMarkers[name] {
			continue
		}
		if r.markerInAdditional(name) {
			continue
		}
		ctx.Emit(r.Finding(file, file.FlatRow(arg)+1, file.FlatCol(arg)+1,
			fmt.Sprintf("@OptIn marker %q is not in the embedded well-known markers list; verify the marker still exists or add it to additionalMarkers.", name)))
	}
}

func (r *OptInMarkerNotRecognisedRule) markerInAdditional(name string) bool {
	for _, m := range r.AdditionalMarkers {
		if i := strings.LastIndex(m, "."); i >= 0 {
			m = m[i+1:]
		}
		if m == name {
			return true
		}
	}
	return false
}

// OptInMarkerExposedPubliclyRule flags public API declarations annotated
// DIRECTLY with an opt-in marker (e.g. `@ExperimentalCoroutinesApi`). A
// `@RequiresOptIn`-style marker propagates to callers, so carrying it on a
// public declaration silently forces every caller to opt in too.
//
// Note: `@OptIn(Foo::class)` is NOT flagged. `@OptIn` CONSUMES the requirement
// locally and is precisely the mechanism for NOT propagating it to callers;
// flagging it would be a false positive.
type OptInMarkerExposedPubliclyRule struct {
	FlatDispatchBase
	BaseRule
	// AdditionalMarkers lets a project register its own propagating opt-in
	// markers (custom `@RequiresOptIn`-annotated classes) so exposing them on
	// public API is flagged too — the embedded well-known set only covers
	// markers shipped by widely used libraries.
	AdditionalMarkers []string
}

// markerInAdditional reports whether the annotation simple name matches one of
// the project-configured additional markers (matched on simple name, so a
// fully-qualified entry still works).
func (r *OptInMarkerExposedPubliclyRule) markerInAdditional(name string) bool {
	for _, m := range r.AdditionalMarkers {
		if i := strings.LastIndex(m, "."); i >= 0 {
			m = m[i+1:]
		}
		if m == name {
			return true
		}
	}
	return false
}

// Confidence reports a tier-1 (high) base confidence. The detection is fully
// AST-driven: we match a propagating opt-in marker annotation whose target
// declaration has no non-public visibility modifier.
func (r *OptInMarkerExposedPubliclyRule) Confidence() float64 { return api.ConfidenceHigher }

// isPropagatingOptInMarkerName reports whether an annotation simple name is a
// `@RequiresOptIn`-style opt-in marker that propagates the opt-in requirement
// to callers when applied directly to a declaration. The opt-in plumbing
// annotations themselves — `@OptIn` (which consumes the requirement), the
// `@RequiresOptIn` meta-annotation (which defines a marker, not a usage), and
// `@Suppress` — never propagate and are excluded.
func isPropagatingOptInMarkerName(name string) bool {
	switch name {
	case "", "OptIn", "RequiresOptIn", "Suppress", "SuppressWarnings":
		return false
	}
	return wellKnownOptInMarkers[name]
}

// optInAnnotationTarget walks up from an `@OptIn` annotation node to the
// declaration it is attached to. Returns the declaration index and true when
// the annotation is anchored to a class, object, function, property, or type
// alias whose visibility we can inspect.
func optInAnnotationTarget(file *scanner.File, annotation uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(annotation); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration",
			"object_declaration",
			"function_declaration",
			"property_declaration",
			"type_alias",
			"secondary_constructor",
			"anonymous_initializer":
			return cur, true
		case "modifiers":
			continue
		default:
			return 0, false
		}
	}
	return 0, false
}

// OptInWithoutJustificationRule flags `@OptIn(...)` annotations whose
// enclosing declaration lacks a preceding KDoc comment explaining why the
// opt-in is safe.
type OptInWithoutJustificationRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
func (r *OptInWithoutJustificationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OptInWithoutJustificationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "OptIn" {
		return
	}
	decl, ok := optInAnnotationTarget(file, idx)
	if !ok {
		return
	}
	if _, ok := flatPrecedingKDoc(file, decl); ok {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@OptIn annotation has no KDoc explaining why opting in is safe."))
}

// SuppressedWarningWithoutJustificationRule flags `@Suppress(...)` annotations
// whose enclosing declaration lacks a preceding KDoc comment explaining why
// silencing the warning is safe.
type SuppressedWarningWithoutJustificationRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
func (r *SuppressedWarningWithoutJustificationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SuppressedWarningWithoutJustificationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "Suppress" {
		return
	}
	decl, ok := optInAnnotationTarget(file, idx)
	if !ok {
		return
	}
	if _, ok := flatPrecedingKDoc(file, decl); ok {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@Suppress annotation has no KDoc explaining why silencing the warning is safe."))
}

// RequiresOptInWithoutMessageRule flags `@RequiresOptIn` annotations that omit
// a `message` argument. Without an explanatory message, callers who must opt in
// have no in-source guidance about the experimental contract.
type RequiresOptInWithoutMessageRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 (high) base confidence. The check is a pure AST
// match on `@RequiresOptIn(...)` argument labels.
func (r *RequiresOptInWithoutMessageRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *RequiresOptInWithoutMessageRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "RequiresOptIn" {
		return
	}
	if ctor, ok := file.FlatFindChild(idx, "constructor_invocation"); ok {
		if args, ok := file.FlatFindChild(ctor, "value_arguments"); ok {
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				if flatValueArgumentLabel(file, arg) == "message" {
					return
				}
			}
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@RequiresOptIn is missing a message argument; add message = \"...\" so callers see why opting in is required."))
}

// RequiresOptInWithoutLevelRule flags custom `@RequiresOptIn` annotation
// classes that omit an explicit `level = WARNING|ERROR` argument. Without an
// explicit level callers see only the implicit default and have no signal
// from the source whether the marker is intended as a warning or an error.
type RequiresOptInWithoutLevelRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 (high) base confidence. The detection is
// AST-driven: we match `annotation class` declarations carrying a
// `@RequiresOptIn` annotation and inspect its `level = ...` argument.
func (r *RequiresOptInWithoutLevelRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *RequiresOptInWithoutLevelRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !file.FlatHasModifier(idx, "annotation") {
		return
	}
	ann := requiresOptInAnnotationOnDeclaration(file, idx)
	if ann == 0 {
		return
	}
	if requiresOptInHasLevelArg(file, ann) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ann)+1, file.FlatCol(ann)+1,
		"@RequiresOptIn annotation class omits `level = RequiresOptIn.Level.WARNING` or `ERROR`; declare the intended level explicitly."))
}

func requiresOptInAnnotationOnDeclaration(file *scanner.File, decl uint32) uint32 {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return 0
	}
	var found uint32
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "annotation" {
			continue
		}
		if annotationFinalName(file, child) == "RequiresOptIn" {
			found = child
			break
		}
	}
	return found
}

func requiresOptInHasLevelArg(file *scanner.File, annotation uint32) bool {
	ctor, ok := file.FlatFindChild(annotation, "constructor_invocation")
	if !ok {
		return false
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if flatValueArgumentLabel(file, arg) == "level" {
			return true
		}
	}
	return false
}
