package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Android Lint rules: Usability, Icons, Messages, UNKNOWN categories.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
//
// Stub aliases for XML/resource/manifest-only rules have been removed;
// those checks are handled by specialized manifest, resource, Gradle, and
// icon-check pipelines.

// =====================================================================
// UNKNOWN category rule types — real source-scanning implementations
// =====================================================================

// NewApiRule detects calls to APIs introduced after minSdk using a static
// lookup table of common Android APIs and their introduction levels.
// Lines guarded by @RequiresApi, @TargetApi, or Build.VERSION.SDK_INT checks
// are skipped.
type NewApiRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NewApiRule) Confidence() float64 { return 0.75 }

// newApiTable maps method/class names to their introduction API level.
var newApiTable = map[string]int{
	"setElevation":               21,
	"getSystemService<":          23,
	"NotificationChannel":        26,
	"BiometricPrompt":            28,
	"setDecorFitsSystemWindows":  30,
	"WindowInsetsController":     30,
	"requestPermissions":         23,
	"checkSelfPermission":        23,
	"getColor(":                  23,
	"getDrawable(":               21,
	"setTranslationZ":            21,
	"setClipToOutline":           21,
	"createNotificationChannel":  26,
	"NotificationCompat.Builder": 26,
	"JobScheduler":               21,
	"JobInfo":                    21,
	"WorkManager":                28,
	"MediaBrowserServiceCompat":  21,
}

// InlinedApiRule detects usage of constants inlined from newer APIs using a
// static lookup table. Guarded blocks are skipped.
type InlinedApiRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InlinedApiRule) Confidence() float64 { return 0.75 }

// inlinedApiEntry pairs a constant pattern with its introduction API level.
type inlinedApiEntry struct {
	Pattern string
	Level   int
}

// inlinedApiTable is ordered longest-pattern-first so that e.g.
// SYSTEM_UI_FLAG_IMMERSIVE_STICKY matches before SYSTEM_UI_FLAG_IMMERSIVE.
var inlinedApiTable = []inlinedApiEntry{
	{"SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION", 16},
	{"SYSTEM_UI_FLAG_IMMERSIVE_STICKY", 19},
	{"SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN", 16},
	{"SYSTEM_UI_FLAG_HIDE_NAVIGATION", 14},
	{"Build.VERSION_CODES.UPSIDE_DOWN_CAKE", 34},
	{"SYSTEM_UI_FLAG_LAYOUT_STABLE", 16},
	{"Build.VERSION_CODES.TIRAMISU", 33},
	{"SYSTEM_UI_FLAG_FULLSCREEN", 14},
	{"SYSTEM_UI_FLAG_LOW_PROFILE", 14},
	{"SYSTEM_UI_FLAG_IMMERSIVE", 19},
	{"Build.VERSION_CODES.S_V2", 32},
	{"READ_EXTERNAL_STORAGE", 16},
	{"FEATURE_CAMERA_AUTOFOCUS", 7},
	{"FEATURE_BLUETOOTH_LE", 18},
	{"Build.VERSION_CODES.S", 31},
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

var overrideMethodNames = map[string]bool{
	"onBackPressed":         true,
	"onNavigateUp":          true,
	"onCreateOptionsMenu":   true,
	"onOptionsItemSelected": true,
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OverrideRule) Confidence() float64 { return 0.75 }

func overrideEnclosingAndroidComponentFlat(file *scanner.File, fn uint32) bool {
	classDecl, ok := flatEnclosingAncestor(file, fn, "class_declaration")
	if !ok {
		return false
	}
	if privacyClassExtendsActivity(file, classDecl) {
		return true
	}
	for _, base := range fragmentSuperclasses {
		if privacyClassDirectlyExtendsFlat(file, classDecl, base) {
			return true
		}
	}
	return false
}

// AssertRule is defined in android_correctness.go with CheckLines implementation.

// OldTargetApiRule is implemented in android_gradle.go as GradleOldTargetApiRule.

// UnusedResourcesRule flags references to R.string, R.drawable, R.layout resources
// whose names follow common "temp" or "test" patterns (test_, temp_, unused_, old_).
// Full unused-resource detection requires cross-file analysis; this is a light heuristic.
type UnusedResourcesRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UnusedResourcesRule) Confidence() float64 { return 0.75 }

func unusedResourceReferenceFlat(file *scanner.File, idx uint32) (resType string, resName string, ok bool) {
	path := contactsIdentifierPathFlat(file, idx)
	if len(path) != 3 || path[0] != "R" {
		return "", "", false
	}
	switch path[1] {
	case "string", "drawable", "layout", "color", "dimen", "style":
	default:
		return "", "", false
	}
	for _, prefix := range []string{"test_", "temp_", "unused_", "old_"} {
		if strings.HasPrefix(path[2], prefix) {
			return path[1], path[2], true
		}
	}
	return "", "", false
}

// InconsistentArraysRule → InconsistentArraysResourceRule (android_resource.go)
// DuplicateActivityRule → DuplicateActivityManifestRule (android_manifest.go)
// MissingApplicationIconRule → MissingApplicationIconManifestRule (android_manifest.go)
// GradleCompatibleRule → GradlePluginCompatibilityRule (android_gradle.go)
// GradleDependencyRule → NewerVersionAvailableRule (android_gradle.go)
// GradleOverridesRule → GradleOverridesRule (android_gradle.go)
