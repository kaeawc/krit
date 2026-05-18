package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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

func (r *TrailingWhitespaceRule) check(ctx *api.Context) {
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

func (r *NoTabsRule) check(ctx *api.Context) {
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

func (r *MaxLineLengthRule) check(ctx *api.Context) {
	file := ctx.File
	// Skip test files — test code commonly uses long descriptive strings
	// and chained builder calls where wrapping hurts readability.
	if scanner.IsTestFile(file.Path) {
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
func overflowInsideStringLiteral(line string, maxLen int) bool {
	if maxLen >= len(line) {
		return false
	}
	inStr := false
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			if i >= maxLen {
				return inStr
			}
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			if i >= maxLen {
				return inStr
			}
			continue
		}
		if c == '"' {
			// Don't toggle on triple-quote boundary positions mid-line;
			// raw strings are handled separately by computeRawStringLines.
			inStr = !inStr
		}
		if i >= maxLen {
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
		stripped = strings.TrimPrefix(stripped, prefix)
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

func (r *NewLineAtEndOfFileRule) check(ctx *api.Context) {
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
	FlatDispatchBase
	BaseRule
}

func (r *SpacingAfterPackageAndImportsRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx

	// Kotlin's import_list span includes trailing blank lines, so fire on
	// the trailing import_header / package_header and climb out of the list
	// to see its successor — each file emits at most once.
	next, hasNext := nextPreludeSibling(file, idx)
	if hasNext {
		switch file.FlatType(next) {
		case "package_header", "import_list", "import_header",
			"package_declaration", "import_declaration":
			return
		}
	}

	endRow := file.RowForByte(int(preludeCodeEndByte(file, idx)))
	if endRow+1 >= len(file.Lines) {
		return
	}
	if strings.TrimSpace(file.Lines[endRow+1]) == "" {
		return
	}

	lineStart := file.LineOffset(endRow + 1)
	f := r.Finding(file, endRow+2, 1,
		"Missing blank line after package/import declarations.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   lineStart,
		EndByte:     lineStart,
		Replacement: "\n",
	}
	ctx.Emit(f)
}

// nextPreludeSibling returns the next file-scope sibling of a package/import
// node. When idx is the last import_header inside a Kotlin import_list, it
// climbs out and returns the list's next sibling instead — the list itself
// is never dispatched.
func nextPreludeSibling(file *scanner.File, idx uint32) (uint32, bool) {
	if next, ok := file.FlatNextSibling(idx); ok {
		return next, true
	}
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "import_list" {
		return 0, false
	}
	return file.FlatNextSibling(parent)
}

// preludeCodeEndByte returns the end byte of the code portion of a
// package/import declaration, ignoring trailing comment children. Kotlin
// tree-sitter attaches trailing block/line comments to the preceding
// import_header, so its raw EndByte sweeps past the real declaration.
func preludeCodeEndByte(file *scanner.File, idx uint32) uint32 {
	var lastCode uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "line_comment", "block_comment", "multiline_comment", "comment":
			continue
		}
		lastCode = c
	}
	if lastCode != 0 {
		return file.FlatEndByte(lastCode)
	}
	return file.FlatEndByte(idx)
}

// MaxChainedCallsOnSameLineRule limits chained method calls on a single line.
type MaxChainedCallsOnSameLineRule struct {
	FlatDispatchBase
	BaseRule
	MaxCalls int
}

// Confidence is 0.95 — chain depth is computed structurally from the
// flat AST, so string-literal, decimal, import, and trailing-comment
// dots no longer count.
func (r *MaxChainedCallsOnSameLineRule) Confidence() float64 { return 0.95 }

func (r *MaxChainedCallsOnSameLineRule) check(ctx *api.Context) {
	file := ctx.File
	if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
		return
	}
	idx := ctx.Idx
	nodeType := file.FlatType(idx)
	// Only the chain head fires — inner chain nodes are skipped so a
	// chain is reported once.
	if parent, ok := file.FlatParent(idx); ok {
		switch nodeType {
		case "call_expression", "navigation_expression":
			pt := file.FlatType(parent)
			if pt == "call_expression" || pt == "navigation_expression" {
				return
			}
		case "method_invocation", "field_access":
			pt := file.FlatType(parent)
			if pt == "method_invocation" || pt == "field_access" {
				return
			}
		}
	}
	count, minRow, maxRow := chainStepCount(file, idx)
	if count <= r.MaxCalls {
		return
	}
	if minRow != maxRow {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Line has %d chained calls, max allowed is %d.", count, r.MaxCalls)))
}

// chainStepCount returns the number of `.foo` (or `?.foo`) steps in the
// chain rooted at head, plus the min/max source row spanned by chain
// steps. For Kotlin it counts navigation_suffix descendants reachable
// through navigation_expression and call_expression skeleton nodes. For
// Java it counts dots implied by the method_invocation / field_access
// shape (1 dot per nested step, plus identifier_count-1 for simple
// receiver chains on a method_invocation).
func chainStepCount(file *scanner.File, head uint32) (count, minRow, maxRow int) {
	if file == nil || head == 0 {
		return 0, 0, 0
	}
	minRow = file.FlatRow(head)
	maxRow = minRow
	switch file.FlatType(head) {
	case "call_expression", "navigation_expression":
		count = walkKotlinChain(file, head, &minRow, &maxRow)
	case "method_invocation", "field_access":
		count = walkJavaChain(file, head, &minRow, &maxRow)
	}
	return count, minRow, maxRow
}

func trackChainRow(minRow, maxRow *int, r int) {
	if r < *minRow {
		*minRow = r
	}
	if r > *maxRow {
		*maxRow = r
	}
}

func walkKotlinChain(file *scanner.File, head uint32, minRow, maxRow *int) int {
	count := 0
	var walk func(uint32)
	walk = func(n uint32) {
		t := file.FlatType(n)
		if t == "navigation_suffix" {
			count++
			trackChainRow(minRow, maxRow, file.FlatRow(n))
			return
		}
		if t != "call_expression" && t != "navigation_expression" {
			return
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			switch file.FlatType(c) {
			case "navigation_expression", "call_expression", "navigation_suffix":
				walk(c)
			}
		}
	}
	walk(head)
	return count
}

func walkJavaChain(file *scanner.File, head uint32, minRow, maxRow *int) int {
	count := 0
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "method_invocation":
			count += javaInvocationStep(file, n, minRow, maxRow, walk)
		case "field_access":
			count += javaFieldAccessStep(file, n, minRow, maxRow, walk)
		}
	}
	walk(head)
	return count
}

func javaInvocationStep(file *scanner.File, n uint32, minRow, maxRow *int, walk func(uint32)) int {
	ids := 0
	var nestedObj uint32
	lastIdentRow := -1
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "argument_list" {
			break
		}
		if !file.FlatIsNamed(c) {
			continue
		}
		switch file.FlatType(c) {
		case "identifier":
			ids++
			lastIdentRow = file.FlatRow(c)
		case "method_invocation", "field_access":
			if nestedObj == 0 {
				nestedObj = c
			}
		}
	}
	if nestedObj != 0 {
		if lastIdentRow >= 0 {
			trackChainRow(minRow, maxRow, lastIdentRow)
		}
		walk(nestedObj)
		return ids
	}
	if ids > 1 {
		if lastIdentRow >= 0 {
			trackChainRow(minRow, maxRow, lastIdentRow)
		}
		return ids - 1
	}
	return 0
}

func javaFieldAccessStep(file *scanner.File, n uint32, minRow, maxRow *int, walk func(uint32)) int {
	var nestedObj uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			continue
		}
		t := file.FlatType(c)
		if t == "identifier" {
			trackChainRow(minRow, maxRow, file.FlatRow(c))
			continue
		}
		if t == "method_invocation" || t == "field_access" {
			if nestedObj == 0 {
				nestedObj = c
			}
		}
	}
	if nestedObj != 0 {
		walk(nestedObj)
	}
	return 1
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

func (r *CascadingCallWrappingRule) check(ctx *api.Context) {
	file := ctx.File
	insideRawString := computeRawStringLines(file.Lines)
	insideBlockComment := computeBlockCommentLines(file.Lines)
	for i, line := range file.Lines {
		if insideRawString[i] || insideBlockComment[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		isDot := strings.HasPrefix(trimmed, ".")
		isElvis := r.IncludeElvis && strings.HasPrefix(trimmed, "?:")
		if (!isDot && !isElvis) || i == 0 {
			continue
		}
		if insideRawString[i-1] || insideBlockComment[i-1] {
			continue
		}
		prevTrimmed := strings.TrimSpace(file.Lines[i-1])
		if strings.HasPrefix(prevTrimmed, ".") || (r.IncludeElvis && strings.HasPrefix(prevTrimmed, "?:")) {
			continue // already wrapped
		}
		// `.` chain requires the previous line to have a `.`; the `?:`
		// continuation only requires that the previous line is part of
		// an expression (not a block opener).
		if isDot && !strings.Contains(prevTrimmed, ".") {
			continue
		}
		if strings.HasSuffix(prevTrimmed, "{") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		prevIndent := len(file.Lines[i-1]) - len(strings.TrimLeft(file.Lines[i-1], " \t"))
		if indent <= prevIndent {
			msg := "Chained call should be indented from the previous line."
			if isElvis {
				msg = "Elvis-operator continuation should be indented from the previous line."
			}
			ctx.Emit(r.Finding(file, i+1, 1, msg))
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

func (r *EqualsOnSignatureLineRule) check(ctx *api.Context) {
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
