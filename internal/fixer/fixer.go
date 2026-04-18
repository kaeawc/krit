package fixer

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type textFixRow struct {
	rule string
	line int
	fix  scanner.Fix
}

func splitTextFixRowsByMode(fixes []textFixRow) (byteFixes, lineFixes []textFixRow) {
	for _, f := range fixes {
		if f.fix.ByteMode {
			byteFixes = append(byteFixes, f)
		} else {
			lineFixes = append(lineFixes, f)
		}
	}
	return
}

// ValidateFixResult checks whether the given content (for a .kt file) still parses
// without additional errors compared to the original. This is opt-in validation.
func ValidateFixResult(path string, original, fixed []byte) error {
	if !strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".kts") {
		return nil // only validate Kotlin files
	}

	origErrors := countParseErrors(original)
	fixedErrors := countParseErrors(fixed)

	if fixedErrors > origErrors {
		return fmt.Errorf("fix produced %d parse errors (original had %d)", fixedErrors, origErrors)
	}
	return nil
}

// countParseErrors parses content as Kotlin and counts ERROR nodes in the flat tree.
func countParseErrors(content []byte) int {
	parser := scanner.GetKotlinParser()
	defer scanner.PutKotlinParser(parser)
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return 1 // treat parse failure as one error
	}
	file := scanner.NewParsedFile("", content, tree)
	if file == nil || file.FlatTree == nil {
		return 0
	}

	count := 0
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if file.FlatType(idx) == "ERROR" {
			count++
		}
	})
	return count
}

// FixResult holds the result of applying fixes to a file.
type FixResult struct {
	Applied      int
	DroppedFixes []DroppedFix
}

// ApplyAllFixesColumns applies text fixes from columnar findings across all files.
// It reconstructs only rows that carry text fixes, preserving the existing file-level
// fixer behavior without materializing the entire finding set.
func ApplyAllFixesColumns(columns *scanner.FindingColumns, suffix string) (totalFixes int, filesModified int, errors []error) {
	if columns == nil || columns.Len() == 0 {
		return 0, 0, nil
	}

	byFile := make(map[string][]int)
	columns.VisitRowsWithTextFixes(func(row int) {
		file := columns.FileAt(row)
		byFile[file] = append(byFile[file], row)
	})

	for path, rows := range byFile {
		res, err := applyFixesDetailedColumns(path, columns, rows, suffix, false)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if res.Applied > 0 {
			totalFixes += res.Applied
			filesModified++
		}
	}
	return
}

func applyFixesDetailedColumns(path string, columns *scanner.FindingColumns, rows []int, suffix string, validate bool) (FixResult, error) {
	if columns == nil || len(rows) == 0 {
		return FixResult{}, nil
	}

	fixes := make([]textFixRow, 0, len(rows))
	for _, row := range rows {
		if row < 0 || row >= columns.Len() || columns.FileAt(row) != path {
			continue
		}
		fix := columns.FixAt(row)
		if fix == nil {
			continue
		}
		fixes = append(fixes, textFixRow{
			rule: columns.RuleAt(row),
			line: columns.LineAt(row),
			fix:  *fix,
		})
	}
	if len(fixes) == 0 {
		return FixResult{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return FixResult{}, err
	}

	byteFixes, lineFixes := splitTextFixRowsByMode(fixes)

	if len(byteFixes) > 0 && len(lineFixes) > 0 {
		return FixResult{}, fmt.Errorf(
			"mixed-mode fixes in %s: %d byte-mode fix(es) and %d line-mode fix(es) cannot be combined; rules %s",
			path, len(byteFixes), len(lineFixes), mixedModeRulesRows(byteFixes, lineFixes))
	}

	result := string(content)
	var droppedFixes []DroppedFix

	if len(byteFixes) > 0 {
		sort.Slice(byteFixes, func(i, j int) bool {
			return byteFixes[i].fix.StartByte > byteFixes[j].fix.StartByte
		})
		var byteDropped []DroppedFix
		byteFixes, byteDropped = deduplicateFixesReverse(byteFixes, textFixRowByteEnd, textFixRowByteStart, textFixRowDroppedFor(path))
		droppedFixes = append(droppedFixes, byteDropped...)
		buf := []byte(result)
		for _, f := range byteFixes {
			fix := f.fix
			if fix.StartByte < 0 || fix.EndByte > len(buf) || fix.StartByte > fix.EndByte {
				continue
			}
			buf = append(buf[:fix.StartByte], append([]byte(fix.Replacement), buf[fix.EndByte:]...)...)
		}
		result = string(buf)
	}

	if len(lineFixes) > 0 {
		sort.Slice(lineFixes, func(i, j int) bool {
			return lineFixes[i].fix.StartLine > lineFixes[j].fix.StartLine
		})
		var lineDropped []DroppedFix
		lineFixes, lineDropped = deduplicateFixesReverse(lineFixes, textFixRowLineEnd, textFixRowLineStart, textFixRowDroppedFor(path))
		droppedFixes = append(droppedFixes, lineDropped...)
		lines := strings.Split(result, "\n")
		for _, f := range lineFixes {
			fix := f.fix
			start := fix.StartLine - 1
			end := fix.EndLine
			if start < 0 || end > len(lines) || start > end {
				continue
			}
			replacement := strings.Split(fix.Replacement, "\n")
			if fix.Replacement == "" {
				replacement = nil
			}
			newLines := make([]string, 0, len(lines)-end+start+len(replacement))
			newLines = append(newLines, lines[:start]...)
			newLines = append(newLines, replacement...)
			newLines = append(newLines, lines[end:]...)
			lines = newLines
		}
		result = strings.Join(lines, "\n")
	}

	for _, d := range droppedFixes {
		log.Printf("warning: dropped overlapping fix for rule %s in %s at line %d", d.Rule, d.File, d.Line)
	}
	if len(droppedFixes) > 0 {
		log.Printf("warning: %d fix(es) dropped due to overlapping conflicts in %s", len(droppedFixes), path)
	}

	if result == string(content) {
		return FixResult{DroppedFixes: droppedFixes}, nil
	}

	if validate {
		if err := ValidateFixResult(path, content, []byte(result)); err != nil {
			return FixResult{DroppedFixes: droppedFixes}, fmt.Errorf("fix validation failed for %s: %w", path, err)
		}
	}

	outPath := path
	if suffix != "" {
		outPath = path + suffix
	}
	if err := os.WriteFile(outPath, []byte(result), 0644); err != nil {
		return FixResult{DroppedFixes: droppedFixes}, err
	}

	return FixResult{Applied: len(fixes), DroppedFixes: droppedFixes}, nil
}

// mixedModeRulesRows returns a compact "[byte: A,B | line: C,D]" description
// of which rules emitted which mode when a mixed-mode conflict occurs.
func mixedModeRulesRows(byteFixes, lineFixes []textFixRow) string {
	byteNames := uniqueNames(len(byteFixes), func(i int) string { return byteFixes[i].rule })
	lineNames := uniqueNames(len(lineFixes), func(i int) string { return lineFixes[i].rule })
	return fmt.Sprintf("[byte: %s | line: %s]", strings.Join(byteNames, ","), strings.Join(lineNames, ","))
}

func uniqueNames(n int, get func(int) string) []string {
	seen := make(map[string]struct{}, n)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		name := get(i)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// DroppedFix records a fix that was dropped due to overlap conflict.
type DroppedFix struct {
	Rule string
	File string
	Line int
}

// deduplicateFixesReverse removes overlapping fixes from a reverse-sorted
// slice. The caller supplies key extractors that return the start and end
// offsets (byte or line depending on the fix mode) and a meta extractor
// that shapes the DroppedFix entry for conflicts.
func deduplicateFixesReverse[T any](fixes []T, endKey, startKey func(T) int, meta func(T) DroppedFix) ([]T, []DroppedFix) {
	if len(fixes) <= 1 {
		return fixes, nil
	}
	var result []T
	var dropped []DroppedFix
	lastStart := -1
	for _, f := range fixes {
		if lastStart >= 0 && endKey(f) > lastStart {
			dropped = append(dropped, meta(f))
			continue // overlaps with previous fix
		}
		result = append(result, f)
		lastStart = startKey(f)
	}
	return result, dropped
}

func findingByteEnd(f scanner.Finding) int    { return f.Fix.EndByte }
func findingByteStart(f scanner.Finding) int  { return f.Fix.StartByte }
func findingLineEnd(f scanner.Finding) int    { return f.Fix.EndLine }
func findingLineStart(f scanner.Finding) int  { return f.Fix.StartLine }
func findingDropped(f scanner.Finding) DroppedFix {
	return DroppedFix{Rule: f.Rule, File: f.File, Line: f.Line}
}

func textFixRowByteEnd(f textFixRow) int   { return f.fix.EndByte }
func textFixRowByteStart(f textFixRow) int { return f.fix.StartByte }
func textFixRowLineEnd(f textFixRow) int   { return f.fix.EndLine }
func textFixRowLineStart(f textFixRow) int { return f.fix.StartLine }
func textFixRowDroppedFor(path string) func(textFixRow) DroppedFix {
	return func(f textFixRow) DroppedFix {
		return DroppedFix{Rule: f.rule, File: path, Line: f.line}
	}
}
