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
func (r *TrailingWhitespaceRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *NoTabsRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
func (r *MaxLineLengthRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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

// logCallLineRe matches a trimmed line that begins with a Log / Timber
// `<level>(` call. An optional dotted package or receiver prefix is
// allowed so fully-qualified forms like `android.util.Log.d(...)` and
// `timber.log.Timber.d(...)` are treated the same as the unqualified
// `Log.d(...)` / `Timber.d(...)`.
var logCallLineRe = regexp.MustCompile(`^(?:[A-Za-z0-9_.]+\.)?(?:Log\.(?:d|i|w|e|v|wtf)|Timber\.(?:d|i|w|e|v))\(`)

// isLogCallLine returns true if the trimmed line is a Log.<level>(...)
// or Timber.<level>(...) call, including fully-qualified variants.
func isLogCallLine(trimmed string) bool {
	return logCallLineRe.MatchString(trimmed)
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
