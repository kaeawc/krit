package rules

// Android Lint Correctness rules ported from AOSP.
// 85 rules total. Rules that can detect issues in Kotlin source have real
// implementations; XML/Manifest-only rules are stubs (Check returns nil).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	LineBase
	AndroidRule
}

var (
	defaultLocaleFmtRe   = regexp.MustCompile(`String\.format\s*\(`)
	defaultLocaleToLower = regexp.MustCompile(`\.toLowerCase\s*\(\s*\)`)
	defaultLocaleToUpper = regexp.MustCompile(`\.toUpperCase\s*\(\s*\)`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DefaultLocaleRule) Confidence() float64 { return 0.75 }

func (r *DefaultLocaleRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if defaultLocaleFmtRe.MatchString(line) && !strings.Contains(line, "Locale") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use String.format(Locale, ...) instead."))
		}
		if defaultLocaleToLower.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use lowercase(Locale) instead."))
		}
		if defaultLocaleToUpper.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use uppercase(Locale) instead."))
		}
	}
}


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
	LineBase
	AndroidRule
}

var assertRe = regexp.MustCompile(`\bassert\s*[\({]`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AssertRule) Confidence() float64 { return 0.75 }

func (r *AssertRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if assertRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"assert is not reliable on Android. Use a proper assertion library or throw explicitly."))
		}
	}
}


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
	LineBase
	AndroidRule
}

var shiftFlagRe = regexp.MustCompile(`const\s+val\s+\w+FLAG\w*\s*=\s*\d+`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShiftFlagsRule) Confidence() float64 { return 0.75 }

func (r *ShiftFlagsRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if shiftFlagRe.MatchString(line) && !strings.Contains(line, "shl") && !strings.Contains(line, "<<") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Consider using shift operators (1 shl N) for flag constants for clarity."))
		}
	}
}


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

var wrongThreadRe = regexp.MustCompile(`\b(setText|setImageResource|setVisibility|addView|removeView|invalidate)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongThreadRule) Confidence() float64 { return 0.75 }



// SQLiteStringRule detects SQL string issues (using string instead of TEXT).
type SQLiteStringRule struct {
	LineBase
	AndroidRule
}

var sqliteStringRe = regexp.MustCompile(`(?i)\bSTRING\b.*\bCREATE\s+TABLE\b|\bCREATE\s+TABLE\b.*\bSTRING\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SQLiteStringRule) Confidence() float64 { return 0.75 }

func (r *SQLiteStringRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if sqliteStringRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"SQLite does not support STRING type. Use TEXT instead."))
		}
	}
}


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


var androidComponentBases = []string{
	"Activity", "AppCompatActivity", "FragmentActivity", "ComponentActivity",
	"Service", "IntentService", "LifecycleService", "JobIntentService",
	"BroadcastReceiver",
	"ContentProvider",
}



// androidComponentType returns the Android component type for a class_declaration node, or "".
func androidComponentType(text string) string {
	if strings.Contains(text, "@AndroidEntryPoint") {
		return ""
	}
	if strings.Contains(text, "abstract class") {
		return ""
	}
	for _, base := range androidComponentBases {
		if strings.Contains(text, ": "+base+"(") || strings.Contains(text, ": "+base+" ") ||
			strings.Contains(text, ": "+base+",") || strings.Contains(text, ": "+base+"{") ||
			strings.Contains(text, ": "+base+"\n") || strings.Contains(text, ": "+base+"\r") {
			switch {
			case strings.Contains(base, "Activity"):
				return "Activity"
			case strings.Contains(base, "Service") || base == "IntentService" || base == "LifecycleService" || base == "JobIntentService":
				return "Service"
			case base == "BroadcastReceiver":
				return "BroadcastReceiver"
			case base == "ContentProvider":
				return "ContentProvider"
			}
		}
	}
	return ""
}

// formatRegisteredMsg builds the manifest registration message.
func formatRegisteredMsg(className, componentType string) string {
	return fmt.Sprintf("%s extends %s and should be registered in AndroidManifest.xml.", className, componentType)
}

// NestedScrollingRule detects nested scrolling views.
type NestedScrollingRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NestedScrollingRule) Confidence() float64 { return 0.75 }

func (r *NestedScrollingRule) check(ctx *v2.Context) {
	file := ctx.File
	// Detect nested scroll containers in Compose or layout inflation patterns
	scrollPatterns := []string{"ScrollView", "LazyColumn", "LazyRow", "HorizontalPager", "VerticalPager"}
	nesting := 0
	inScroll := false
	for i, line := range file.Lines {
		for _, p := range scrollPatterns {
			if strings.Contains(line, p+"(") || strings.Contains(line, p+" {") || strings.Contains(line, p+"{") {
				if inScroll {
					ctx.Emit(r.Finding(file, i+1, 1,
						"Nested scrolling detected ("+p+" inside another scroll container). This can cause performance issues."))
				}
				nesting++
				if nesting == 1 {
					inScroll = true
				}
			}
		}
		nesting += strings.Count(line, "{") - strings.Count(line, "}")
		if nesting <= 0 {
			inScroll = false
			nesting = 0
		}
	}
}


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
	LineBase
	AndroidRule
}

var sdfRe = regexp.MustCompile(`SimpleDateFormat\s*\([^)]*\)`)
var sdfLocaleRe = regexp.MustCompile(`SimpleDateFormat\s*\([^,)]+,\s*Locale`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SimpleDateFormatRule) Confidence() float64 { return 0.75 }

func (r *SimpleDateFormatRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if sdfRe.MatchString(line) && !sdfLocaleRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs."))
		}
	}
}


// SetTextI18nRule detects setText() with hardcoded text.
type SetTextI18nRule struct {
	LineBase
	AndroidRule
}

var setTextI18nRe = regexp.MustCompile(`\.setText\s*\(\s*"[^"]+"\s*\)`)

func (r *SetTextI18nRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if setTextI18nRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Do not concatenate text displayed with setText. Use resource strings with placeholders."))
		}
	}
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
	LineBase
	AndroidRule
}

var wrongCallRe = regexp.MustCompile(`\b(onDraw|onMeasure|onLayout)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongCallRule) Confidence() float64 { return 0.75 }

func (r *WrongCallRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		// Only flag direct calls that aren't in super. or override fun
		if wrongCallRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "override") &&
			!strings.Contains(trimmed, "super.") && !strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "fun ") {
			if strings.Contains(trimmed, ".onDraw(") || strings.Contains(trimmed, ".onMeasure(") || strings.Contains(trimmed, ".onLayout(") {
				ctx.Emit(r.Finding(file, i+1, 1,
					"Suspicious method call; should probably call draw/measure/layout instead of onDraw/onMeasure/onLayout."))
			}
		}
	}
}


// Remaining rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// init() -- register first set of correctness rules
// ---------------------------------------------------------------------------

// Remaining correctness rules are in android_correctness_checks.go
