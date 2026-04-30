package rules

import (
	"fmt"
	"regexp"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// TrailingWhitespaceRule detects lines ending with spaces or tabs.
type TrailingWhitespaceRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — trailing whitespace is a pure lexical condition with no
// known false positives.
func (r *TrailingWhitespaceRule) Confidence() float64 { return 0.95 }

var trailingWhitespaceRe = regexp.MustCompile(`[ \t]+$`)

func (r *TrailingWhitespaceRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if trailingWhitespaceRe.MatchString(line) {
			trimmed := strings.TrimRight(line, " \t")
			f := r.Finding(file, i+1, len(trimmed)+1, "Trailing whitespace detected.")
			lineStart := file.LineOffset(i)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   lineStart,
				EndByte:     lineStart + len(line),
				Replacement: trimmed,
			}
			ctx.Emit(f)
		}
	}
}

// NoTabsRule detects tab characters.
type NoTabsRule struct {
	LineBase
	BaseRule
	// IndentSize is the number of spaces a tab is replaced with by the
	// fix. Configurable via the `indentSize` option (or .editorconfig's
	// `indent_size` / `tab_width`). Defaults to 4.
	IndentSize int
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — presence of a tab character is a pure byte check with no
// room for interpretation.
func (r *NoTabsRule) Confidence() float64 { return 0.95 }

func (r *NoTabsRule) check(ctx *v2.Context) {
	file := ctx.File
	indent := r.IndentSize
	if indent <= 0 {
		indent = 4
	}
	spaces := strings.Repeat(" ", indent)
	for i, line := range file.Lines {
		if strings.Contains(line, "\t") {
			f := r.Finding(file, i+1, strings.Index(line, "\t")+1,
				"Tab character found. Use spaces for indentation.")
			lineStart := file.LineOffset(i)
			replaced := strings.ReplaceAll(line, "\t", spaces)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   lineStart,
				EndByte:     lineStart + len(line),
				Replacement: replaced,
			}
			ctx.Emit(f)
		}
	}
}

// MaxLineLengthRule detects lines exceeding the configured max.
type MaxLineLengthRule struct {
	LineBase
	BaseRule
	Max                      int
	ExcludePackageStatements bool
	ExcludeImportStatements  bool
	ExcludeCommentStatements bool
	ExcludeRawStrings        bool
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — line length is a deterministic measurement, and the rule
// already skips test files and gradle scripts where long lines are
// conventional.
func (r *MaxLineLengthRule) Confidence() float64 { return 0.95 }

func (r *MaxLineLengthRule) check(ctx *v2.Context) {
	file := ctx.File
	// Skip test files — test code commonly uses long descriptive strings
	// and chained builder calls where wrapping hurts readability.
	if isTestFile(file.Path) {
		return
	}
	// Skip gradle build scripts — DSL chains with long type/config names
	// are the norm.
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return
	}
	// Compute set of lines that are inside a triple-quoted raw string literal.
	// Raw strings cannot be wrapped without changing the string contents.
	insideRawString := computeRawStringLines(file.Lines)
	// Compute set of lines that are inside a KDoc / block comment.
	insideBlockComment := computeBlockCommentLines(file.Lines)
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if r.ExcludePackageStatements && strings.HasPrefix(trimmed, "package ") {
			continue
		}
		if r.ExcludeImportStatements && strings.HasPrefix(trimmed, "import ") {
			continue
		}
		if r.ExcludeCommentStatements && scanner.IsCommentLine(line) {
			continue
		}
		// Keep package/import declarations out of line-length wrapping.
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "package ") {
			continue
		}
		// Skip lines inside raw string literals (""").
		if insideRawString[i] {
			continue
		}
		// Skip KDoc / block comment continuation lines (`* ...`).
		if insideBlockComment[i] || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Skip single-line block comments `/** ... */` or `/* ... */`.
		if strings.HasPrefix(trimmed, "/*") && strings.HasSuffix(trimmed, "*/") {
			continue
		}
		if len(line) > r.Max {
			// Skip lines whose overflow is entirely contained in a URL literal.
			if containsURLLiteral(line) {
				continue
			}
			// Skip lines that contain SQL DDL/DML keywords inside string
			// literals — these are typically hard-to-wrap schema definitions.
			if containsSQLLiteral(line) {
				continue
			}
			// Skip lines where the overflow position sits inside a string
			// literal — the long part is string content that cannot be
			// wrapped without changing the string value.
			if overflowInsideStringLiteral(line, r.Max) {
				continue
			}
			// Skip lines containing an Android string resource reference.
			// Resource identifiers are single tokens and cannot be
			// wrapped. Cover the full `R.<kind>.` prefix set — R.string,
			// R.plurals, R.drawable, R.id, R.layout, R.menu, R.color,
			// R.dimen, R.style, R.array, R.bool, R.integer, R.raw,
			// R.mipmap, R.xml, R.anim, R.animator, R.font, R.navigation.
			if containsRResourceReference(line) {
				continue
			}
			// Skip android.util.Log calls — log messages are typically
			// string content that shouldn't be wrapped mid-message.
			if isLogCallLine(trimmed) {
				continue
			}
			// Skip Android Navigation Safe-Args generated Directions
			// calls. The generated `XxxFragmentDirections.actionXxxToYyy()`
			// identifiers are single tokens that can't be wrapped.
			if strings.Contains(line, "Directions.action") {
				continue
			}
			// Skip lines that are a single dotted call-chain expression —
			// these cannot be wrapped without breaking the expression and
			// are a common source of unwrappable overflow in builder DSLs.
			if isDottedCallChainLine(trimmed) {
				continue
			}
			ctx.Emit(r.Finding(file, i+1, r.Max+1,
				fmt.Sprintf("Line exceeds maximum length of %d characters.", r.Max)))
		}
	}
}

// rResourceKindPrefixes enumerates the `R.<kind>.` Android resource
// reference prefixes that produce unwrappable identifiers on a line.
var rResourceKindPrefixes = []string{
	"R.string.", "R.plurals.", "R.drawable.", "R.id.", "R.layout.",
	"R.menu.", "R.color.", "R.dimen.", "R.style.", "R.array.",
	"R.bool.", "R.integer.", "R.raw.", "R.mipmap.", "R.xml.",
	"R.anim.", "R.animator.", "R.font.", "R.navigation.", "R.attr.",
	"R.styleable.", "R.interpolator.", "R.transition.",
}

// containsRResourceReference reports whether the line contains any
// `R.<kind>.` resource reference. Any such reference makes the line
// effectively unwrappable because the identifier is a single token.
func containsRResourceReference(line string) bool {
	for _, p := range rResourceKindPrefixes {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

// computeBlockCommentLines returns indices (0-based) that are inside a
// `/* ... */` or `/** ... */` block comment (including opening/closing lines).
func computeBlockCommentLines(lines []string) map[int]bool {
	result := make(map[int]bool)
	inBlock := false
	for i, line := range lines {
		if inBlock {
			result[i] = true
			if strings.Contains(line, "*/") {
				inBlock = false
			}
			continue
		}
		// Check for block comment start
		if idx := strings.Index(line, "/*"); idx >= 0 {
			// Check if it closes on the same line
			if closeIdx := strings.Index(line[idx+2:], "*/"); closeIdx < 0 {
				inBlock = true
				result[i] = true
			}
		}
	}
	return result
}

// computeRawStringLines returns a map of line indices (0-based) that are
// inside a triple-quoted raw string literal. Does not mark the opening and
// closing lines — only the middle lines where the overflow happens.
func computeRawStringLines(lines []string) map[int]bool {
	result := make(map[int]bool)
	inRaw := false
	for i, line := range lines {
		// Count occurrences of """ on this line
		count := strings.Count(line, `"""`)
		if inRaw {
			result[i] = true
		}
		if count%2 == 1 {
			inRaw = !inRaw
		}
	}
	return result
}

// overflowInsideStringLiteral returns true if the character at column max
// (0-indexed byte position) sits inside a "..." string literal. Used to skip
// lines whose overflow is unwrappable string content.
func overflowInsideStringLiteral(line string, max int) bool {
	if max >= len(line) {
		return false
	}
	inStr := false
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			if i >= max {
				return inStr
			}
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			if i >= max {
				return inStr
			}
			continue
		}
		if c == '"' {
			// Don't toggle on triple-quote boundary positions mid-line;
			// raw strings are handled separately by computeRawStringLines.
			inStr = !inStr
		}
		if i >= max {
			return inStr
		}
	}
	return false
}

// isDottedCallChainLine returns true if the trimmed line consists of a
// single dotted call-chain expression — identifiers, dots, parentheses,
// string literals and a few navigation tokens — with no binary operators
// that would provide a natural wrap point, and at least 3 `.` separators.
// Such lines cannot be wrapped without breaking the expression mid-token.
func isDottedCallChainLine(trimmed string) bool {
	// Quick prefix gate: must look like a statement, not a declaration
	// or control flow keyword (those are wrappable at their structure).
	if trimmed == "" {
		return false
	}
	// Strip a leading simple prefix like `val x = `, `return `, `->`.
	stripped := trimmed
	for _, prefix := range []string{"return ", "= ", "+ "} {
		if strings.HasPrefix(stripped, prefix) {
			stripped = stripped[len(prefix):]
		}
	}
	// Walk characters outside string literals, counting dots and detecting
	// any wrap point (comma, binary operator, `?:`, ` = `, assignment).
	dots := 0
	inStr := false
	escaped := false
	for i := 0; i < len(stripped); i++ {
		c := stripped[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '.':
			dots++
		case ',':
			// Argument list comma — natural wrap point.
			return false
		case '+', '*', '/', '%':
			return false
		case '|':
			if i+1 < len(stripped) && stripped[i+1] == '|' {
				return false
			}
		case '&':
			if i+1 < len(stripped) && stripped[i+1] == '&' {
				return false
			}
		case '=':
			// `==`, `!=`, `= `, `>=`, `<=` — comparison/assignment.
			return false
		case '?':
			// `?:` elvis — wrap point.
			if i+1 < len(stripped) && stripped[i+1] == ':' {
				return false
			}
		}
	}
	return dots >= 3
}

// isLogCallLine returns true if the trimmed line is a Log.<level>(...) call.
func isLogCallLine(trimmed string) bool {
	prefixes := []string{"Log.d(", "Log.i(", "Log.w(", "Log.e(", "Log.v(", "Log.wtf(", "Timber.d(", "Timber.i(", "Timber.w(", "Timber.e(", "Timber.v("}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}

// containsURLLiteral returns true if the line contains an http(s) URL
// inside a string literal that extends beyond the line limit.
func containsURLLiteral(line string) bool {
	return strings.Contains(line, `"http://`) || strings.Contains(line, `"https://`)
}

// containsSQLLiteral returns true if the line contains a SQL statement
// inside a string literal (CREATE TABLE, SELECT, INSERT, etc.).
func containsSQLLiteral(line string) bool {
	sqlKeywords := []string{
		`"CREATE TABLE`, `"CREATE INDEX`, `"CREATE UNIQUE`, `"CREATE VIEW`,
		`"CREATE TRIGGER`, `"ALTER TABLE`, `"DROP TABLE`, `"DROP INDEX`,
		`"SELECT `, `"INSERT INTO`, `"UPDATE `, `"DELETE FROM`,
		`"PRAGMA `,
		"\"\"\"CREATE", "\"\"\"SELECT", "\"\"\"INSERT", "\"\"\"UPDATE",
		"\"\"\"DELETE", "\"\"\"ALTER", "\"\"\"DROP",
		// Inside SQL: WHERE/FROM/JOIN/etc. clauses on a continuation line
		` FROM `, ` WHERE `, ` JOIN `, ` LEFT JOIN `, ` INNER JOIN `,
		` GROUP BY `, ` ORDER BY `, ` HAVING `, ` UNION `,
	}
	for _, kw := range sqlKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

// NewLineAtEndOfFileRule detects files not ending with a newline.
type NewLineAtEndOfFileRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — presence/absence of a trailing newline is a single byte check
// with no room for interpretation.
func (r *NewLineAtEndOfFileRule) Confidence() float64 { return 0.95 }

func (r *NewLineAtEndOfFileRule) check(ctx *v2.Context) {
	file := ctx.File
	if len(file.Content) > 0 && file.Content[len(file.Content)-1] != '\n' {
		f := r.Finding(file, len(file.Lines), 1, "File does not end with a newline.")
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   len(file.Content),
			EndByte:     len(file.Content),
			Replacement: "\n",
		}
		ctx.Emit(f)
	}
}

// SpacingAfterPackageAndImportsRule checks for blank line after package/imports.
type SpacingAfterPackageAndImportsRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the detection uses `strings.HasPrefix(trimmed, "package ")`
// and `"import "`, which are unambiguous line starts in Kotlin. No
// heuristic path.
func (r *SpacingAfterPackageAndImportsRule) Confidence() float64 { return 0.95 }

func (r *SpacingAfterPackageAndImportsRule) check(ctx *v2.Context) {
	file := ctx.File
	lastImportLine := -1
	packageLine := -1

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			packageLine = i
		}
		if strings.HasPrefix(trimmed, "import ") {
			lastImportLine = i
		}
	}

	checkLine := lastImportLine
	if checkLine < 0 {
		checkLine = packageLine
	}
	if checkLine >= 0 && checkLine < len(file.Lines)-1 {
		next := strings.TrimSpace(file.Lines[checkLine+1])
		if next != "" {
			f := r.Finding(file, checkLine+2, 1,
				"Missing blank line after package/import declarations.")
			// Insert a blank line after the checkLine
			lineStart := file.LineOffset(checkLine + 1)
			// Insert a newline at the position right after the checkLine's newline
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   lineStart,
				EndByte:     lineStart,
				Replacement: "\n",
			}
			ctx.Emit(f)
		}
	}
}

// MaxChainedCallsOnSameLineRule limits chained method calls on a single line.
type MaxChainedCallsOnSameLineRule struct {
	LineBase
	BaseRule
	MaxCalls int
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *MaxChainedCallsOnSameLineRule) Confidence() float64 { return 0.75 }

func (r *MaxChainedCallsOnSameLineRule) check(ctx *v2.Context) {
	file := ctx.File
	if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
		return
	}
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		dotCount := strings.Count(line, ".")
		if dotCount > r.MaxCalls {
			ctx.Emit(r.Finding(file, i+1, 1,
				fmt.Sprintf("Line has %d chained calls, max allowed is %d.", dotCount, r.MaxCalls)))
		}
	}
}

// CascadingCallWrappingRule checks that chained calls are wrapped consistently.
type CascadingCallWrappingRule struct {
	LineBase
	BaseRule
	IncludeElvis bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *CascadingCallWrappingRule) Confidence() float64 { return 0.75 }

func (r *CascadingCallWrappingRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".") && i > 0 {
			prevTrimmed := strings.TrimSpace(file.Lines[i-1])
			if strings.HasPrefix(prevTrimmed, ".") {
				continue // already wrapped
			}
			if strings.Contains(prevTrimmed, ".") && !strings.HasSuffix(prevTrimmed, "{") {
				indent := len(line) - len(strings.TrimLeft(line, " \t"))
				prevIndent := len(file.Lines[i-1]) - len(strings.TrimLeft(file.Lines[i-1], " \t"))
				if indent <= prevIndent {
					ctx.Emit(r.Finding(file, i+1, 1,
						"Chained call should be indented from the previous line."))
				}
			}
		}
	}
}

// UnderscoresInNumericLiteralsRule detects large numbers without underscores.
type UnderscoresInNumericLiteralsRule struct {
	FlatDispatchBase
	BaseRule
	AcceptableLength         int // maximum consecutive digits allowed without underscores
	AllowNonStandardGrouping bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *UnderscoresInNumericLiteralsRule) Confidence() float64 { return 0.75 }

// formatWithUnderscores inserts underscores every 3 digits from the right for readability.
func formatWithUnderscores(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	var result []byte
	for i, c := range digits {
		if i > 0 && (n-i)%3 == 0 {
			result = append(result, '_')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func maxConsecutiveDigits(text string) int {
	maxRun := 0
	current := 0
	for _, c := range text {
		if c >= '0' && c <= '9' {
			current++
			if current > maxRun {
				maxRun = current
			}
			continue
		}
		current = 0
	}
	return maxRun
}

func stripNumericLiteralUnderscores(text string) string {
	if !strings.Contains(text, "_") {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, c := range text {
		if c != '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func hasStandardNumericGrouping(text string) bool {
	groups := strings.Split(text, "_")
	if len(groups) == 1 {
		return true
	}
	if len(groups[0]) == 0 || len(groups[0]) > 3 {
		return false
	}
	for _, group := range groups[1:] {
		if len(group) != 3 {
			return false
		}
	}
	return true
}

// EqualsOnSignatureLineRule detects `=` on the next line of a function signature.
type EqualsOnSignatureLineRule struct {
	LineBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *EqualsOnSignatureLineRule) Confidence() float64 { return 0.75 }

func (r *EqualsOnSignatureLineRule) check(ctx *v2.Context) {
	file := ctx.File
	for i := 1; i < len(file.Lines); i++ {
		trimmed := strings.TrimSpace(file.Lines[i])
		if trimmed == "=" || strings.HasPrefix(trimmed, "= ") {
			prevTrimmed := strings.TrimSpace(file.Lines[i-1])
			if strings.HasPrefix(prevTrimmed, "fun ") || strings.Contains(prevTrimmed, " fun ") {
				f := r.Finding(file, i+1, 1,
					"'=' should be on the same line as the function signature.")
				// Compute byte offsets: merge prev line with current line
				merged := strings.TrimRight(file.Lines[i-1], " \t") + " " + trimmed
				prevLineIdx := i - 1
				byteStart := 0
				for j := 0; j < prevLineIdx; j++ {
					byteStart += len(file.Lines[j]) + 1
				}
				byteEnd := byteStart + len(file.Lines[i-1]) + 1 + len(file.Lines[i])
				if i+1 < len(file.Lines) {
					byteEnd++ // include the newline after current line
				}
				if byteEnd > len(file.Content) {
					byteEnd = len(file.Content)
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   byteStart,
					EndByte:     byteEnd,
					Replacement: merged,
				}
				ctx.Emit(f)
			}
		}
	}
}
