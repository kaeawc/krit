package rules

// Android Lint Correctness rules ported from AOSP.
// 85 rules total. Rules that can detect issues in Kotlin source have real
// implementations; XML/Manifest-only rules are stubs (Check returns nil).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
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

func (r *DefaultLocaleRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if defaultLocaleFmtRe.MatchString(line) && !strings.Contains(line, "Locale") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use String.format(Locale, ...) instead."))
		}
		if defaultLocaleToLower.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use lowercase(Locale) instead."))
		}
		if defaultLocaleToUpper.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Implicitly using the default locale. Use uppercase(Locale) instead."))
		}
	}
	return findings
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

func (r *CommitPrefEditsRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *CommitPrefEditsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, ".edit()") {
		return nil
	}
	// Walk up to the enclosing function/block
	parent, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
	if !ok {
		return nil
	}
	funcText := file.FlatNodeText(parent)
	if strings.Contains(funcText, ".edit()") &&
		!strings.Contains(funcText, ".commit()") &&
		!strings.Contains(funcText, ".apply()") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"SharedPreferences.edit() without commit() or apply().")}
	}
	return nil
}


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

func (r *CommitTransactionRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *CommitTransactionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "beginTransaction") && !strings.Contains(text, "FragmentTransaction") {
		return nil
	}
	parent, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
	if !ok {
		return nil
	}
	funcText := file.FlatNodeText(parent)
	if strings.Contains(funcText, "beginTransaction") &&
		!strings.Contains(funcText, ".commit()") &&
		!strings.Contains(funcText, ".commitNow()") &&
		!strings.Contains(funcText, ".commitAllowingStateLoss()") &&
		!strings.Contains(funcText, ".commitNowAllowingStateLoss()") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"FragmentTransaction without commit(). Call commit() or commitAllowingStateLoss().")}
	}
	return nil
}


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

func (r *AssertRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if assertRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"assert is not reliable on Android. Use a proper assertion library or throw explicitly."))
		}
	}
	return findings
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

func (r *CheckResultRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *CheckResultRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if this call expression is an expression_statement (return value discarded)
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "expression_statement" {
		return nil
	}
	text := file.FlatNodeText(idx)
	// Common Android methods whose return must not be ignored
	checkResultMethods := []string{
		".animate(", ".buildUpon(", ".edit(",
		"String.format(", ".format(",
		".trim(", ".replace(",
	}
	for _, m := range checkResultMethods {
		if strings.Contains(text, m) {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				"The result of this call is not used. Check if the return value should be consumed.")}
		}
	}
	return nil
}


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

func (r *ShiftFlagsRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if shiftFlagRe.MatchString(line) && !strings.Contains(line, "shl") && !strings.Contains(line, "<<") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Consider using shift operators (1 shl N) for flag constants for clarity."))
		}
	}
	return findings
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

func (r *UniqueConstantsRule) NodeTypes() []string { return []string{"annotation"} }

func (r *UniqueConstantsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "IntDef") && !strings.Contains(text, "StringDef") {
		return nil
	}
	// Simple duplicate value check: extract numeric or string constants
	parts := strings.Split(text, ",")
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Find numeric constants
		for _, tok := range strings.Fields(p) {
			tok = strings.Trim(tok, "()[]{}\"")
			if len(tok) > 0 && tok[0] >= '0' && tok[0] <= '9' {
				if seen[tok] {
					return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
						"Duplicate constant value "+tok+" in annotation definition.")}
				}
				seen[tok] = true
			}
		}
	}
	return nil
}


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

func (r *WrongThreadRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *WrongThreadRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// Check if the function is annotated with @WorkerThread
	prev, ok := file.FlatPrevSibling(idx)
	if !ok {
		return nil
	}
	prevText := file.FlatNodeText(prev)
	if !strings.Contains(prevText, "WorkerThread") {
		return nil
	}
	var findings []scanner.Finding
	lines := strings.Split(text, "\n")
	startLine := file.FlatRow(idx)
	for j, line := range lines {
		if wrongThreadRe.MatchString(line) && !strings.Contains(line, "runOnUiThread") && !strings.Contains(line, "post(") {
			findings = append(findings, r.Finding(file, startLine+j+1, 1,
				"UI operation in @WorkerThread context. Use runOnUiThread or Handler.post()."))
		}
	}
	return findings
}


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

func (r *SQLiteStringRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if sqliteStringRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"SQLite does not support STRING type. Use TEXT instead."))
		}
	}
	return findings
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

func (r *RegisteredRule) NodeTypes() []string { return []string{"class_declaration"} }

var androidComponentBases = []string{
	"Activity", "AppCompatActivity", "FragmentActivity", "ComponentActivity",
	"Service", "IntentService", "LifecycleService", "JobIntentService",
	"BroadcastReceiver",
	"ContentProvider",
}

func (r *RegisteredRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)

	// Skip if annotated with @AndroidEntryPoint (Hilt auto-registers)
	if strings.Contains(text, "@AndroidEntryPoint") {
		return nil
	}

	// Skip abstract classes
	if strings.Contains(text, "abstract class") {
		return nil
	}

	// Check if class extends an Android component
	var componentType string
	for _, base := range androidComponentBases {
		if strings.Contains(text, ": "+base+"(") || strings.Contains(text, ": "+base+" ") ||
			strings.Contains(text, ": "+base+",") || strings.Contains(text, ": "+base+"{") ||
			strings.Contains(text, ": "+base+"\n") || strings.Contains(text, ": "+base+"\r") {
			switch {
			case strings.Contains(base, "Activity"):
				componentType = "Activity"
			case strings.Contains(base, "Service") || base == "IntentService" || base == "LifecycleService" || base == "JobIntentService":
				componentType = "Service"
			case base == "BroadcastReceiver":
				componentType = "BroadcastReceiver"
			case base == "ContentProvider":
				componentType = "ContentProvider"
			}
			break
		}
	}
	if componentType == "" {
		return nil
	}

	// Extract class name for the message
	className := extractIdentifierFlat(file, idx)
	if className == "" {
		className = "This class"
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("%s extends %s and should be registered in AndroidManifest.xml.", className, componentType))}
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

func (r *NestedScrollingRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	// Detect nested scroll containers in Compose or layout inflation patterns
	scrollPatterns := []string{"ScrollView", "LazyColumn", "LazyRow", "HorizontalPager", "VerticalPager"}
	nesting := 0
	inScroll := false
	for i, line := range file.Lines {
		for _, p := range scrollPatterns {
			if strings.Contains(line, p+"(") || strings.Contains(line, p+" {") || strings.Contains(line, p+"{") {
				if inScroll {
					findings = append(findings, r.Finding(file, i+1, 1,
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
	return findings
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

func (r *SimpleDateFormatRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if sdfRe.MatchString(line) && !sdfLocaleRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs."))
		}
	}
	return findings
}


// SetTextI18nRule detects setText() with hardcoded text.
type SetTextI18nRule struct {
	LineBase
	AndroidRule
}

var setTextI18nRe = regexp.MustCompile(`\.setText\s*\(\s*"[^"]+"\s*\)`)

func (r *SetTextI18nRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if setTextI18nRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Do not concatenate text displayed with setText. Use resource strings with placeholders."))
		}
	}
	return findings
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

func (r *StopShipRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "STOPSHIP") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"STOPSHIP comment found. This must be resolved before shipping."))
		}
	}
	return findings
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

func (r *WrongCallRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		// Only flag direct calls that aren't in super. or override fun
		if wrongCallRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "override") &&
			!strings.Contains(trimmed, "super.") && !strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "fun ") {
			if strings.Contains(trimmed, ".onDraw(") || strings.Contains(trimmed, ".onMeasure(") || strings.Contains(trimmed, ".onLayout(") {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Suspicious method call; should probably call draw/measure/layout instead of onDraw/onMeasure/onLayout."))
			}
		}
	}
	return findings
}


// Remaining rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// init() -- register first set of correctness rules
// ---------------------------------------------------------------------------

// Remaining correctness rules are in android_correctness_checks.go
