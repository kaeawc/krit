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

func (r *ContentDescriptionRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ContentDescriptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "Image" && name != "Icon" {
		return nil
	}
	_, args := flatCallExpressionParts(file, idx)
	if flatNamedValueArgument(file, args, "contentDescription") != 0 {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		"Image/Icon without contentDescription. Provide a description for accessibility.")}
}

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

func (r *HardcodedTextRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *HardcodedTextRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}
	for i := 0; i < file.FlatChildCount(args); i++ {
		child := file.FlatChild(args, i)
		if file.FlatType(child) != "value_argument" {
			continue
		}
		argText := strings.TrimSpace(file.FlatNodeText(child))
		eqIdx := strings.Index(argText, "=")
		if eqIdx < 0 {
			continue
		}
		label := strings.TrimSpace(argText[:eqIdx])
		if !hardcodedTextLabels[label] {
			continue
		}
		valueText := strings.TrimSpace(argText[eqIdx+1:])
		if valueText == "" || valueText[0] != '"' {
			continue
		}
		if strings.Contains(valueText, "stringResource(") || strings.Contains(valueText, "getString(") {
			continue
		}
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Hardcoded text. Use string resources for localization.")}
	}
	return nil
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
func (r *LogDetectorRule) Confidence() float64 { return 0.75 }

func (r *LogDetectorRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *LogDetectorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	if receiver := flatReceiverNameFromCall(file, idx); receiver != "Log" {
		return nil
	}
	if !logLevelNames[flatCallExpressionName(file, idx)] {
		return nil
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "if_expression" && strings.Contains(file.FlatNodeText(p), "isLoggable") {
			return nil
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" ||
			file.FlatType(p) == "lambda_literal" || file.FlatType(p) == "source_file" {
			break
		}
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		"Unconditional logging call. Wrap in Log.isLoggable() for performance.")}
}

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

func (r *SdCardPathRule) NodeTypes() []string { return []string{"string_literal"} }

func (r *SdCardPathRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "/sdcard") || strings.Contains(text, "/mnt/sdcard") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Hardcoded /sdcard path. Use Environment.getExternalStorageDirectory() instead.")}
	}
	return nil
}

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

func (r *WakelockRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *WakelockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "acquire" {
		return nil
	}
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver == "" {
		return nil
	}
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	foundRelease := false
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		if foundRelease {
			return
		}
		if call == idx {
			return
		}
		if flatCallExpressionName(file, call) != "release" {
			return
		}
		if flatReceiverNameFromCall(file, call) == receiver {
			foundRelease = true
		}
	})
	if foundRelease {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		"WakeLock acquired without release. Ensure WakeLock.release() is called.")}
}

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

func (r *SetJavaScriptEnabledRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SetJavaScriptEnabledRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "setJavaScriptEnabled(true)") || strings.Contains(text, "javaScriptEnabled = true") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Using setJavaScriptEnabled(true). Review for XSS vulnerabilities.")}
	}
	return nil
}

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

func (r *ObsoleteLayoutParamsRule) NodeTypes() []string { return []string{"call_expression"} }

var obsoleteLayoutParamRe = regexp.MustCompile(`\b(preferredWidth|preferredHeight|preferredSize)\b`)

func (r *ObsoleteLayoutParamsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	replacements := map[string]string{
		"preferredWidth":  "width",
		"preferredHeight": "height",
		"preferredSize":   "size",
	}
	matches := obsoleteLayoutParamRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	var findings []scanner.Finding
	for _, m := range matches {
		replacement := replacements[m]
		findings = append(findings, r.Finding(file, file.FlatRow(idx)+1, 1,
			"Obsolete Compose layout modifier '"+m+"' was renamed to '"+replacement+"' in Compose 1.0."))
	}
	return findings
}

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

func (r *ViewHolderRule) NodeTypes() []string { return []string{"class_declaration"} }

var adapterClassRe = regexp.MustCompile(`(?s)class\s+\w+.*?:\s*.*?Adapter`)

func (r *ViewHolderRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// Check if this is an Adapter class — only check the declaration header (first line or two)
	headerEnd := strings.Index(text, "{")
	header := text
	if headerEnd > 0 {
		header = text[:headerEnd]
	}
	if !adapterClassRe.MatchString(header) {
		return nil
	}
	// Check class body for ViewHolder pattern — look for actual ViewHolder
	// class declarations or onCreateViewHolder overrides, not just substrings
	// like onBindViewHolder.
	hasViewHolder := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "class_body" {
			bodyText := file.FlatNodeText(child)
			// Check each line in the body for ViewHolder indicators
			for _, line := range strings.Split(bodyText, "\n") {
				if strings.Contains(line, "onCreateViewHolder") {
					hasViewHolder = true
					break
				}
				if strings.Contains(line, "class") && strings.Contains(line, "ViewHolder") {
					hasViewHolder = true
					break
				}
				if strings.Contains(line, ": RecyclerView.ViewHolder") {
					hasViewHolder = true
					break
				}
			}
		}
	}
	if hasViewHolder {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		"RecyclerView.Adapter subclass should implement the ViewHolder pattern.")}
}
