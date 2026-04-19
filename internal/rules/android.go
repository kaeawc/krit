package rules

// Android Lint rules ported from AOSP.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
//
// These rules retain their original AOSP issue IDs, categories, severity levels,
// and descriptions. They are categorized under the "android-lint" rule set
// to distinguish them from detekt-origin rules.
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
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
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
func (r *ContentDescriptionRule) Confidence() float64 { return 0.75 }


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
func (r *HardcodedTextRule) Confidence() float64 { return 0.75 }


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
func (r *LogDetectorRule) Confidence() float64 { return 0.75 }


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
func (r *SdCardPathRule) Confidence() float64 { return 0.75 }


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
func (r *WakelockRule) Confidence() float64 { return 0.75 }


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
func (r *SetJavaScriptEnabledRule) Confidence() float64 { return 0.75 }


// ExportedServiceRule detects exported services/receivers without permission.
type ExportedServiceRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedServiceRule) Confidence() float64 { return 0.75 }

func (r *ExportedServiceRule) CheckLines(file *scanner.File) []scanner.Finding {
	// This is primarily an XML check, but we can detect registration in Kotlin
	return nil
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
func (r *PrivateKeyRule) Confidence() float64 { return 0.75 }

func (r *PrivateKeyRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "BEGIN RSA PRIVATE KEY") || strings.Contains(line, "BEGIN PRIVATE KEY") ||
			strings.Contains(line, "BEGIN EC PRIVATE KEY") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Private key detected in source code. Remove and use secure key storage."))
		}
	}
	return findings
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
func (r *ObsoleteLayoutParamsRule) Confidence() float64 { return 0.75 }

var obsoleteLayoutParamRe = regexp.MustCompile(`\b(preferredWidth|preferredHeight|preferredSize)\b`)

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
func (r *ViewHolderRule) Confidence() float64 { return 0.75 }

var adapterClassRe = regexp.MustCompile(`(?s)class\s+\w+.*?:\s*.*?Adapter`)
