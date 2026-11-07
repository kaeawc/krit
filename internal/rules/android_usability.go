package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Android Lint rules: Usability, Icons, Messages, UNKNOWN categories.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
//
// Source aliases for XML/resource/manifest-only rules have been removed;
// those checks are handled by specialized manifest, resource, Gradle, and
// icon-check pipelines.

// =====================================================================
// UNKNOWN category rule types — real source-scanning implementations
// =====================================================================

// NewAPIRule detects calls to APIs introduced after minSdk using a static
// lookup table of common Android APIs and their introduction levels.
// Lines guarded by @RequiresApi, @TargetApi, or Build.VERSION.SDK_INT checks
// are skipped.
type NewAPIRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NewAPIRule) Confidence() float64 { return api.ConfidenceMedium }

// newAPITable maps framework method/class names to their introduction API level.
// Androidx APIs (WorkManager, NotificationCompat, BiometricPrompt) and the reified
// getSystemService<T>() extension run on every API level, so they are not listed.
var newAPITable = map[string]int{
	"setElevation":              21,
	"NotificationChannel":       26,
	"setDecorFitsSystemWindows": 30,
	"WindowInsetsController":    30,
	"requestPermissions":        23,
	"checkSelfPermission":       23,
	"getColor(":                 23,
	"getDrawable(":              21,
	"setTranslationZ":           21,
	"setClipToOutline":          21,
	"createNotificationChannel": 26,
	"JobScheduler":              21,
	"JobInfo":                   21,
}

// newAPIOwners lists the framework classes that declare each ambiguous method name. The same name also exists on
// older or compat APIs (Resources.getColor(int), TypedArray.getColor, androidx Fragment.requestPermissions), so
// when the oracle resolves the call its target must belong to one of these classes.
var newAPIOwners = map[string][]string{
	"setElevation":        {"android.view.View"},
	"setTranslationZ":     {"android.view.View"},
	"setClipToOutline":    {"android.view.View"},
	"requestPermissions":  {"android.app.Activity", "android.app.Fragment"},
	"checkSelfPermission": {"android.content.Context"},
	"getColor":            {"android.content.Context"},
	"getDrawable":         {"android.content.Context"},
}

// newAPICallTargetMatches reports whether a call named name may be the framework method. Without an oracle every
// call matches. With one, the call must resolve to a declaring class listed in newAPIOwners; names with no owner
// list always match.
func newAPICallTargetMatches(ctx *api.Context, name string) bool {
	owners, ok := newAPIOwners[name]
	if !ok || ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return true
	}
	cr, isComposite := ctx.Resolver.(*oracle.CompositeResolver)
	if !isComposite || cr.Oracle() == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(cr.Oracle(), ctx.File, ctx.Idx)
	if target == "" {
		return false
	}
	for _, owner := range owners {
		if target == owner+"."+name {
			return true
		}
	}
	return false
}

// InlinedAPIRule detects usage of constants inlined from newer APIs using a
// static lookup table. Guarded blocks are skipped.
type InlinedAPIRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InlinedAPIRule) Confidence() float64 { return api.ConfidenceMedium }

// inlinedAPIEntry pairs a constant pattern with its introduction API level.
type inlinedAPIEntry struct {
	Pattern string
	Level   int
}

// inlinedAPITable omits Build.VERSION_CODES: those constants are how code checks the SDK level.
// It is ordered longest-pattern-first so that e.g.
// SYSTEM_UI_FLAG_IMMERSIVE_STICKY matches before SYSTEM_UI_FLAG_IMMERSIVE.
var inlinedAPITable = []inlinedAPIEntry{
	{"SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION", 16},
	{"SYSTEM_UI_FLAG_IMMERSIVE_STICKY", 19},
	{"SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN", 16},
	{"SYSTEM_UI_FLAG_HIDE_NAVIGATION", 14},
	{"SYSTEM_UI_FLAG_LAYOUT_STABLE", 16},
	{"SYSTEM_UI_FLAG_FULLSCREEN", 14},
	{"SYSTEM_UI_FLAG_LOW_PROFILE", 14},
	{"SYSTEM_UI_FLAG_IMMERSIVE", 19},
	{"READ_EXTERNAL_STORAGE", 16},
	{"FEATURE_CAMERA_AUTOFOCUS", 7},
	{"FEATURE_BLUETOOTH_LE", 18},
	{"FEATURE_LEANBACK", 21},
	{"FEATURE_NFC", 9},
}

// OverrideRule detects methods that need `override` for correct behavior
// across API levels. Currently checks for `fun onBackPressed()` without
// `override` in Activity/Fragment subclasses.
type OverrideRule struct {
	FlatDispatchBase
	AndroidRule
}

// overrideMethodNames maps each overridable method to its parameter count, so an unrelated helper with the
// same name but another signature is not mistaken for an override.
var overrideMethodNames = map[string][]int{
	"onBackPressed":         {0},
	"onNavigateUp":          {0},
	"onCreateOptionsMenu":   {1, 2}, // Fragment also takes the MenuInflater
	"onOptionsItemSelected": {1},
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OverrideRule) Confidence() float64 { return api.ConfidenceMedium }

// overrideActivityOnlyMethods are declared by Activity but not by Fragment, so a Fragment's own helper of that
// name is not an override.
var overrideActivityOnlyMethods = map[string]bool{"onBackPressed": true, "onNavigateUp": true}

func overrideEnclosingAndroidComponentFlat(file *scanner.File, fn uint32, name string) bool {
	classDecl, ok := flatEnclosingAncestor(file, fn, "class_declaration")
	if !ok {
		return false
	}
	if privacyClassExtendsActivity(file, classDecl) {
		return true
	}
	if overrideActivityOnlyMethods[name] {
		return false
	}
	for _, base := range fragmentSuperclasses {
		if privacyClassDirectlyExtendsFlat(file, classDecl, base) {
			return true
		}
	}
	return false
}

// AssertRule is defined in android_correctness.go with CheckLines implementation.

// OldTargetApiRule is implemented in android_gradle.go as GradleOldTargetAPIRule.

// InconsistentArraysRule → InconsistentArraysResourceRule (android_resource.go)
// DuplicateActivityRule → DuplicateActivityManifestRule (android_manifest.go)
// MissingApplicationIconRule → MissingApplicationIconManifestRule (android_manifest.go)
// GradleCompatibleRule → GradlePluginCompatibilityRule (android_gradle.go)
// GradleDependencyRule → NewerVersionAvailableRule (android_gradle.go)
// GradleOverridesRule → GradleOverridesRule (android_gradle.go)
