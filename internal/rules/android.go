package rules

// Android Lint rules ported from AOSP.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
//
// These rules retain their original AOSP issue IDs, categories, severity levels,
// and descriptions. They are categorized under the "android-lint" rule set
// to distinguish them from Kotlin source-style rules.
//
// Category mapping from AOSP:
//   Category.CORRECTNESS → android-lint (correctness)
//   Category.SECURITY    → android-lint (security)
//   Category.PERFORMANCE → android-lint (performance)
//   Category.USABILITY   → android-lint (usability)
//   Category.A11Y        → android-lint (accessibility)
//   Category.I18N        → android-lint (i18n)
//   Category.ICONS       → android-lint (icons)
//   Category.MESSAGES    → android-lint (messages)
//   Category.RTL         → android-lint (rtl)

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// AndroidLintCategory represents the AOSP lint category.
type AndroidLintCategory string

const (
	ALCCorrectness   AndroidLintCategory = "correctness"
	ALCSecurity      AndroidLintCategory = "security"
	ALCPerformance   AndroidLintCategory = "performance"
	ALCUsability     AndroidLintCategory = "usability"
	ALCAccessibility AndroidLintCategory = "accessibility"
	ALCI18N          AndroidLintCategory = "i18n"
	ALCIcons         AndroidLintCategory = "icons"
	ALCMessages      AndroidLintCategory = "messages"
	ALCUnknown       AndroidLintCategory = "unknown"
)

// AndroidLintSeverity maps to AOSP severity levels.
type AndroidLintSeverity string

const (
	ALSFatal         AndroidLintSeverity = "fatal"
	ALSError         AndroidLintSeverity = "error"
	ALSWarning       AndroidLintSeverity = "warning"
	ALSInformational AndroidLintSeverity = "informational"
)

// AndroidRule is the base for all Android lint rules ported from AOSP.
type AndroidRule struct {
	BaseRule
	IssueID    string              // Original AOSP issue ID (e.g., "ContentDescription")
	Brief      string              // Original brief description
	Category   AndroidLintCategory // AOSP category
	ALSeverity AndroidLintSeverity // AOSP severity
	Priority   int                 // AOSP priority (1-10)
	Origin     string              // Always "AOSP Android Lint"
}

// Description returns the Brief string for Android rules (which all carry
// one from their AOSP origin), falling back to the embedded BaseRule.Desc.
func (r AndroidRule) Description() string {
	if r.Brief != "" {
		return r.Brief
	}
	return r.Desc
}

const androidRuleSet = "android-lint"

// ---- Rule implementations ----

var hardcodedTextLabels = map[string]bool{
	"text": true, "title": true, "label": true, "hint": true, "description": true,
}

var logLevelNames = map[string]bool{
	"v": true, "d": true, "i": true, "w": true, "e": true,
}

// ContentDescriptionRule checks for ImageView/ImageButton without contentDescription.
// Kotlin equivalent: detects missing contentDescription in Compose Image() calls.
type ContentDescriptionRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ContentDescriptionRule) Confidence() float64 { return api.ConfidenceMedium }

// HardcodedTextRule detects hardcoded strings that should use resources.
type HardcodedTextRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *HardcodedTextRule) Confidence() float64 { return api.ConfidenceMedium }

// hardcodedTextAndroidImportPrefixes are the import package prefixes
// that signal a file participates in Android UI / Compose code. The
// HardcodedText rule only fires when at least one such import is
// present, otherwise the named-argument form `text = "…"` is too
// ambiguous (data-class constructors, builders, exception helpers
// all share the spelling).
var hardcodedTextAndroidImportPrefixes = []string{
	"android.widget.",
	"android.view.",
	"android.app.",
	"android.content.",
	"androidx.compose.",
	"androidx.appcompat.",
	"androidx.fragment.",
	"androidx.preference.",
	"com.google.android.material.",
}

// hardcodedTextComposeCallees is a conservative set of well-known
// Compose UI composables whose `text = "…"` argument is virtually
// always user-facing. Used as a fallback when imports are missing
// (e.g. a single-file snippet) so the existing positive fixtures
// keep firing without an explicit import_header.
var hardcodedTextComposeCallees = map[string]bool{
	"Text":                   true,
	"OutlinedText":           true,
	"TextField":              true,
	"OutlinedTextField":      true,
	"BasicText":              true,
	"BasicTextField":         true,
	"Button":                 true,
	"TextButton":             true,
	"OutlinedButton":         true,
	"FilledTonalButton":      true,
	"ElevatedButton":         true,
	"Snackbar":               true,
	"TopAppBar":              true,
	"CenterAlignedTopAppBar": true,
	"AlertDialog":            true,
	"NavigationBarItem":      true,
	"NavigationRailItem":     true,
	"Tab":                    true,
	"LeadingIconTab":         true,
	"DropdownMenuItem":       true,
	"ListItem":               true,
}

// hardcodedTextStringLiteralIsPureInterpolation returns true when the
// string at valueText is a Kotlin string literal whose entire content
// is interpolation — e.g. `"$x"`, `"${a + b}"`, `"$a$b"`. Such templates
// carry no localizable literal text and must not fire HardcodedText.
//
// Raw triple-quoted strings (`"""…"""`) are NOT treated as pure
// interpolation because the bracketing is different and the interior
// is still a hardcoded string from a localization point of view.
func hardcodedTextStringLiteralIsPureInterpolation(valueText string) bool {
	if len(valueText) < 2 || valueText[0] != '"' {
		return false
	}
	// Triple-quoted raw strings: not interpolation-only.
	if strings.HasPrefix(valueText, `"""`) {
		return false
	}
	// Strip surrounding quotes — the lexer guarantees a matching close
	// quote for parser-accepted source. If we can't find one, bail.
	end := strings.LastIndexByte(valueText, '"')
	if end <= 0 {
		return false
	}
	inner := valueText[1:end]
	if inner == "" {
		return false
	}
	for i := 0; i < len(inner); {
		c := inner[i]
		if c != '$' {
			return false
		}
		i++
		if i >= len(inner) {
			return false
		}
		if inner[i] == '{' {
			depth := 1
			i++
			for i < len(inner) && depth > 0 {
				switch inner[i] {
				case '{':
					depth++
				case '}':
					depth--
				}
				i++
			}
			continue
		}
		// Bare `$ident` — consume an identifier run.
		for i < len(inner) {
			ch := inner[i]
			if ch == '_' || ch == '.' ||
				(ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') {
				i++
				continue
			}
			break
		}
	}
	return true
}

// hardcodedTextReceiverProven returns true when the call_expression at
// idx targets a known Compose UI composable by name. The guard is
// deliberately conservative — Kotlin's `Foo(text = "…")` is also the
// shape of a data-class constructor, exception helper, or arbitrary
// builder, so absent receiver-type evidence we restrict the rule to
// the well-known Compose composables whose `text`/`title`/etc named
// argument is virtually always user-facing copy.
//
// We additionally require the file to import at least one
// Android/Compose package — without that signal there is no reason
// to believe the call is on Compose at all (the callee name could be
// from any framework).
func hardcodedTextReceiverProven(ctx *api.Context, callExpr uint32) bool {
	file := ctx.File
	name := flatCallExpressionName(file, callExpr)
	if !hardcodedTextComposeCallees[name] {
		return false
	}
	return fileFactsCache().Imports(file).HasAnyPrefix(hardcodedTextAndroidImportPrefixes...)
}

func (r *HardcodedTextRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return
	}
	if !hardcodedTextReceiverProven(ctx, idx) {
		return
	}
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, child)
		if !hardcodedTextLabels[label] {
			continue
		}
		exprIdx := flatValueArgumentExpression(file, child)
		if exprIdx == 0 {
			continue
		}
		valueText := strings.TrimSpace(file.FlatNodeText(exprIdx))
		if valueText == "" || valueText[0] != '"' {
			continue
		}
		// Skip strings already wrapped in a resource lookup. The
		// `Contains` form catches concatenations like
		// `"Hello " + getString(R.string.world)` as well as
		// `stringResource(R.string.hello)` / `context.getString(...)`.
		if strings.Contains(valueText, "stringResource(") || strings.Contains(valueText, "getString(") {
			continue
		}
		// Skip pure-interpolation templates — they carry no static
		// literal text to localize.
		if hardcodedTextStringLiteralIsPureInterpolation(valueText) {
			continue
		}
		ctx.EmitAt(file.FlatRow(idx)+1, 1,
			"Hardcoded text. Use string resources for localization.")
		return
	}
}

// LogDetectorRule detects unconditional logging calls.
type LogDetectorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LogDetectorRule) Confidence() float64 { return api.ConfidenceMedium }

// SdCardPathRule detects hardcoded /sdcard paths.
type SdCardPathRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SdCardPathRule) Confidence() float64 { return api.ConfidenceMedium }

// WakelockRule detects WakeLock usage without proper release.
type WakelockRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WakelockRule) Confidence() float64 { return api.ConfidenceMedium }

// SetJavaScriptEnabledRule detects setJavaScriptEnabled(true) calls.
type SetJavaScriptEnabledRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SetJavaScriptEnabledRule) Confidence() float64 { return api.ConfidenceMedium }

// ExportedServiceRule detects exported services without permission.
type ExportedServiceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedServiceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExportedServiceRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !exportedClassExtendsAndroid(file, idx, "Service", "android.app.Service") {
		return
	}
	if exportedPermissionEnforcedInClass(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Service subclass may be exported without permission. Ensure permissions are enforced.")
}

// PrivateKeyRule detects private key content in source.
type PrivateKeyRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PrivateKeyRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *PrivateKeyRule) check(ctx *api.Context) {
	file := ctx.File
	var st lineScanState
	for i, line := range file.Lines {
		scrubbed := stripCommentsKeepStrings(line, &st)
		if strings.Contains(scrubbed, "BEGIN RSA PRIVATE KEY") ||
			strings.Contains(scrubbed, "BEGIN PRIVATE KEY") ||
			strings.Contains(scrubbed, "BEGIN EC PRIVATE KEY") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Private key detected in source code. Remove and use secure key storage."))
		}
	}
}

// ObsoleteLayoutParamsRule detects deprecated Compose layout modifier APIs
// that were renamed in Compose 1.0 (e.g. preferredWidth -> width).
type ObsoleteLayoutParamsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ObsoleteLayoutParamsRule) Confidence() float64 { return api.ConfidenceMedium }

// ViewHolderRule detects RecyclerView.Adapter subclasses that do not
// implement the ViewHolder pattern (missing ViewHolder or onCreateViewHolder).
type ViewHolderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ViewHolderRule) Confidence() float64 { return api.ConfidenceMedium }
