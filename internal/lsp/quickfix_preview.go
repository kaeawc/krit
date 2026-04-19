package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

const quickFixPreviewContextLines = 3

func buildQuickFixPreview(uri, content string, finding scanner.Finding) *CodeActionPreview {
	if finding.Fix == nil {
		return nil
	}

	fixLevel, ok := lookupCodeActionFixLevel(finding.Rule)
	if !ok || fixLevel < rules.FixIdiomatic {
		return nil
	}

	updated, ok := applyPreviewFix(content, finding.Fix)
	if !ok || updated == content {
		return nil
	}

	label := filepath.Base(uriToPath(uri))
	if label == "" || label == "." || label == string(filepath.Separator) {
		label = "document.kt"
	}

	diff := buildUnifiedPreviewDiff(label, content, updated)
	if diff == "" {
		return nil
	}

	return &CodeActionPreview{
		FixLevel: fixLevel.String(),
		Diff:     diff,
	}
}

func lookupCodeActionFixLevel(ruleName string) (rules.FixLevel, bool) {
	for _, r := range v2.Registry {
		if r.ID != ruleName {
			continue
		}
		lvl, ok := rules.GetV2FixLevel(r)
		if !ok {
			return 0, false
		}
		return lvl, true
	}
	return 0, false
}

func applyPreviewFix(content string, fix *scanner.Fix) (string, bool) {
	if fix == nil {
		return "", false
	}

	if fix.ByteMode {
		if fix.StartByte < 0 || fix.EndByte < fix.StartByte || fix.EndByte > len(content) {
			return "", false
		}
		return content[:fix.StartByte] + fix.Replacement + content[fix.EndByte:], true
	}

	if fix.StartLine <= 0 || fix.EndLine <= 0 {
		return "", false
	}

	lines := strings.Split(content, "\n")
	start := fix.StartLine - 1
	end := fix.EndLine
	if start < 0 || start > end || end > len(lines) {
		return "", false
	}

	replacement := []string(nil)
	if fix.Replacement != "" {
		replacement = strings.Split(fix.Replacement, "\n")
	}

	newLines := make([]string, 0, len(lines)-end+start+len(replacement))
	newLines = append(newLines, lines[:start]...)
	newLines = append(newLines, replacement...)
	newLines = append(newLines, lines[end:]...)
	return strings.Join(newLines, "\n"), true
}

func buildUnifiedPreviewDiff(label, before, after string) string {
	beforeLines := diffLines(before)
	afterLines := diffLines(after)

	prefix := commonPrefixLen(beforeLines, afterLines)
	suffix := commonSuffixLen(beforeLines[prefix:], afterLines[prefix:])

	if prefix == len(beforeLines) && prefix == len(afterLines) {
		return ""
	}

	oldStart := maxInt(prefix-quickFixPreviewContextLines, 0)
	newStart := maxInt(prefix-quickFixPreviewContextLines, 0)
	oldEnd := minInt(len(beforeLines)-suffix+quickFixPreviewContextLines, len(beforeLines))
	newEnd := minInt(len(afterLines)-suffix+quickFixPreviewContextLines, len(afterLines))

	oldChangedStart := prefix
	oldChangedEnd := len(beforeLines) - suffix
	newChangedStart := prefix
	newChangedEnd := len(afterLines) - suffix

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n", label)
	fmt.Fprintf(&sb, "+++ b/%s\n", label)
	fmt.Fprintf(
		&sb,
		"@@ -%s +%s @@\n",
		formatUnifiedRange(oldStart+1, oldEnd-oldStart),
		formatUnifiedRange(newStart+1, newEnd-newStart),
	)

	for _, line := range beforeLines[oldStart:minInt(oldChangedStart, oldEnd)] {
		writeUnifiedDiffLine(&sb, ' ', line)
	}
	for _, line := range beforeLines[maxInt(oldChangedStart, oldStart):minInt(oldChangedEnd, oldEnd)] {
		writeUnifiedDiffLine(&sb, '-', line)
	}
	for _, line := range afterLines[maxInt(newChangedStart, newStart):minInt(newChangedEnd, newEnd)] {
		writeUnifiedDiffLine(&sb, '+', line)
	}
	for _, line := range beforeLines[maxInt(oldChangedEnd, oldStart):oldEnd] {
		writeUnifiedDiffLine(&sb, ' ', line)
	}

	return sb.String()
}

func diffLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func commonPrefixLen(a, b []string) int {
	limit := minInt(len(a), len(b))
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return limit
}

func commonSuffixLen(a, b []string) int {
	limit := minInt(len(a), len(b))
	for i := 0; i < limit; i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			return i
		}
	}
	return limit
}

func formatUnifiedRange(startLine, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", startLine)
	}
	return fmt.Sprintf("%d,%d", startLine, count)
}

func writeUnifiedDiffLine(sb *strings.Builder, prefix byte, line string) {
	sb.WriteByte(prefix)
	sb.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		sb.WriteByte('\n')
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
