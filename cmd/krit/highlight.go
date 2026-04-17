package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------- LCS for violation detection ----------

// lcsLength computes the length of the longest common subsequence
// between two string slices.
func lcsLength(a, b []string) ([][]int, int) {
	n, m := len(a), len(b)
	tbl := make([][]int, n+1)
	for i := range tbl {
		tbl[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				tbl[i][j] = tbl[i-1][j-1] + 1
			} else if tbl[i-1][j] >= tbl[i][j-1] {
				tbl[i][j] = tbl[i-1][j]
			} else {
				tbl[i][j] = tbl[i][j-1]
			}
		}
	}
	return tbl, tbl[n][m]
}

// violationStyle renders an entire line with a dark red background.
// It sets both background and foreground so that inner ANSI resets
// from syntax highlighting don't kill the background mid-line.
var violationStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("52")).
	Foreground(lipgloss.Color("255"))

// violationLines computes which 0-indexed line numbers in the positive
// fixture are violations (lines absent from the negative fixture). Uses
// the LCS: any line in the positive that is not part of the common
// subsequence is a violation.
func violationLines(positive, negative string) map[int]bool {
	a := strings.Split(positive, "\n")
	b := strings.Split(negative, "\n")
	tbl, _ := lcsLength(a, b)

	// Backtrack to find which positive lines are in the LCS.
	inLCS := make(map[int]bool, len(a))
	i, j := len(a), len(b)
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			inLCS[i-1] = true
			i--
			j--
		} else if tbl[i-1][j] >= tbl[i][j-1] {
			i--
		} else {
			j--
		}
	}

	violations := make(map[int]bool)
	for idx := range a {
		if !inLCS[idx] {
			violations[idx] = true
		}
	}
	return violations
}

// ---------- Syntax highlighting ----------

// highlightKotlinWithViolations renders Kotlin source with syntax
// highlighting on normal lines and a full-line red background on
// violation lines. Violation lines use raw text (no syntax
// highlighting) to avoid ANSI reset sequences breaking the background.
func highlightKotlinWithViolations(src string, violations map[int]bool) string {
	if src == "" {
		return src
	}
	if len(violations) == 0 {
		return highlightKotlin(src)
	}
	rawLines := strings.Split(src, "\n")
	for i, line := range rawLines {
		if violations[i] {
			rawLines[i] = violationStyle.Render(line)
		} else {
			rawLines[i] = highlightKotlin(line)
		}
	}
	return strings.Join(rawLines, "\n")
}

// highlightKotlin applies a very light keyword highlight to Kotlin
// source. This is not a real tokenizer — it just color-tags a set of
// well-known keywords and string literals so the preview is
// readable at a glance without adding a chroma-sized dependency.
func highlightKotlin(src string) string {
	if src == "" {
		return src
	}
	keywords := []string{
		"package", "import", "fun", "val", "var", "class", "object",
		"interface", "enum", "sealed", "data", "abstract", "open",
		"override", "private", "public", "protected", "internal",
		"return", "if", "else", "when", "for", "while", "do", "in",
		"is", "as", "null", "true", "false", "this", "super", "try",
		"catch", "finally", "throw", "suspend", "actual", "expect",
		"companion", "init", "by", "where",
	}
	kwStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	strStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	commentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	lines := strings.Split(src, "\n")
	for li, line := range lines {
		// Line comment.
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[li] = line[:idx] + commentStyle.Render(line[idx:])
			line = lines[li]
		}
		// String literals (crude — doesn't handle templates or escapes).
		lines[li] = highlightStrings(line, strStyle)
	}
	out := strings.Join(lines, "\n")
	for _, kw := range keywords {
		out = replaceWord(out, kw, kwStyle.Render(kw))
	}
	return out
}

// highlightStrings wraps substrings between double-quote pairs on a
// single line with the given style.
func highlightStrings(line string, style lipgloss.Style) string {
	var b strings.Builder
	inStr := false
	start := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' && (i == 0 || line[i-1] != '\\') {
			if !inStr {
				b.WriteString(line[start:i])
				start = i
				inStr = true
			} else {
				b.WriteString(style.Render(line[start : i+1]))
				start = i + 1
				inStr = false
			}
		}
	}
	b.WriteString(line[start:])
	return b.String()
}

// replaceWord replaces whole-word occurrences of old with new. A
// "word" is bounded by non-identifier characters.
func replaceWord(s, old, new string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		j := strings.Index(s[i:], old)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		absIdx := i + j
		before := byte(' ')
		if absIdx > 0 {
			before = s[absIdx-1]
		}
		after := byte(' ')
		if absIdx+len(old) < len(s) {
			after = s[absIdx+len(old)]
		}
		if isIdentChar(before) || isIdentChar(after) {
			b.WriteString(s[i : absIdx+1])
			i = absIdx + 1
			continue
		}
		b.WriteString(s[i:absIdx])
		b.WriteString(new)
		i = absIdx + len(old)
	}
	return b.String()
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// ---------- Fixture rendering ----------

// renderFixtureContent renders the triggering fixture with violation
// lines highlighted. The viewport handles scrolling externally.
func renderFixtureContent(pair fixturePair, maxCols int) string {
	// Pick the best source for showing what the rule catches.
	// Priority: fixBefore (autofix input), then positive fixture.
	src := pair.positive
	if pair.fixBefore != "" {
		src = pair.fixBefore
	}
	if src == "" {
		return dimStyle.Render("(no fixture)")
	}

	// Compute violation lines: lines in the trigger source that differ
	// from the clean version. Use fixAfter if available, else negative.
	clean := pair.negative
	if pair.fixAfter != "" {
		clean = pair.fixAfter
	}
	var viol map[int]bool
	if clean != "" {
		viol = violationLines(src, clean)
	}

	// Truncate raw source lines before highlighting so byte-based
	// truncation doesn't cut through ANSI escape sequences.
	src = truncateCols(src, maxCols)
	return highlightKotlinWithViolations(src, viol)
}

// truncateCols truncates each line of s to maxCols characters.
func truncateCols(s string, maxCols int) string {
	if maxCols <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > maxCols {
			if len(line) > maxCols-1 {
				lines[i] = line[:maxCols-1] + "…"
			}
		}
	}
	return strings.Join(lines, "\n")
}
