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

// splitByMode separates findings into byte-mode and line-mode fixes.
func splitByMode(fixes []scanner.Finding) (byteFixes, lineFixes []scanner.Finding) {
	for _, f := range fixes {
		if f.Fix.ByteMode {
			byteFixes = append(byteFixes, f)
		} else {
			lineFixes = append(lineFixes, f)
		}
	}
	return
}

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

// ApplyFixes applies all fixes for a single file.
// If suffix is empty, edits the file in place. Otherwise writes to path+suffix.
// Fixes are applied in reverse order (bottom-up) to preserve line/byte offsets.
// If validate is true, the result is parsed to check for new syntax errors.
// Returns the number of fixes applied.
func ApplyFixes(path string, findings []scanner.Finding, suffix string) (int, error) {
	return ApplyFixesWithValidation(path, findings, suffix, false)
}

// ApplyFixesWithValidation is like ApplyFixes but with an opt-in validation flag.
func ApplyFixesWithValidation(path string, findings []scanner.Finding, suffix string, validate bool) (int, error) {
	res, err := ApplyFixesDetailed(path, findings, suffix, validate)
	return res.Applied, err
}

// ApplyFixesDetailed is like ApplyFixes but returns a FixResult with dropped fix info.
func ApplyFixesDetailed(path string, findings []scanner.Finding, suffix string, validate bool) (FixResult, error) {
	// Filter to findings with fixes for this file
	var fixes []scanner.Finding
	for _, f := range findings {
		if f.File == path && f.Fix != nil {
			fixes = append(fixes, f)
		}
	}
	if len(fixes) == 0 {
		return FixResult{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return FixResult{}, err
	}

	// Separate byte-mode and line-mode fixes
	byteFixes, lineFixes := splitByMode(fixes)

	// Mixed-mode rejection: if both modes present, prefer byte-mode and drop line-mode
	if len(byteFixes) > 0 && len(lineFixes) > 0 {
		log.Printf("warning: %s has both byte-mode (%d) and line-mode (%d) fixes; dropping line-mode fixes",
			path, len(byteFixes), len(lineFixes))
		lineFixes = nil
	}

	result := string(content)

	var droppedFixes []DroppedFix

	// Apply byte-mode fixes first (more precise), in reverse order
	if len(byteFixes) > 0 {
		sort.Slice(byteFixes, func(i, j int) bool {
			return byteFixes[i].Fix.StartByte > byteFixes[j].Fix.StartByte
		})
		// Deduplicate overlapping byte ranges
		var byteDropped []DroppedFix
		byteFixes, byteDropped = deduplicateByteFixesReverse(byteFixes)
		droppedFixes = append(droppedFixes, byteDropped...)
		buf := []byte(result)
		for _, f := range byteFixes {
			fix := f.Fix
			if fix.StartByte < 0 || fix.EndByte > len(buf) || fix.StartByte > fix.EndByte {
				continue
			}
			buf = append(buf[:fix.StartByte], append([]byte(fix.Replacement), buf[fix.EndByte:]...)...)
		}
		result = string(buf)
	}

	// Apply line-mode fixes in reverse order
	if len(lineFixes) > 0 {
		sort.Slice(lineFixes, func(i, j int) bool {
			return lineFixes[i].Fix.StartLine > lineFixes[j].Fix.StartLine
		})
		var lineDropped []DroppedFix
		lineFixes, lineDropped = deduplicateLineFixesReverse(lineFixes)
		droppedFixes = append(droppedFixes, lineDropped...)
		lines := strings.Split(result, "\n")
		for _, f := range lineFixes {
			fix := f.Fix
			start := fix.StartLine - 1 // 1-indexed to 0-indexed
			end := fix.EndLine
			if start < 0 || end > len(lines) || start > end {
				continue
			}
			replacement := strings.Split(fix.Replacement, "\n")
			// If replacement is empty string, we're deleting lines
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

	// Log dropped fixes
	for _, d := range droppedFixes {
		log.Printf("warning: dropped overlapping fix for rule %s in %s at line %d", d.Rule, d.File, d.Line)
	}
	if len(droppedFixes) > 0 {
		log.Printf("warning: %d fix(es) dropped due to overlapping conflicts in %s", len(droppedFixes), path)
	}

	// Only write if content changed
	if result == string(content) {
		return FixResult{DroppedFixes: droppedFixes}, nil
	}

	// Post-fix validation (opt-in)
	if validate {
		if err := ValidateFixResult(path, content, []byte(result)); err != nil {
			return FixResult{DroppedFixes: droppedFixes}, fmt.Errorf("fix validation failed for %s: %w", path, err)
		}
	}

	outPath := path
	if suffix != "" {
		outPath = path + suffix
	}
	err = os.WriteFile(outPath, []byte(result), 0644)
	if err != nil {
		return FixResult{DroppedFixes: droppedFixes}, err
	}

	return FixResult{Applied: len(fixes), DroppedFixes: droppedFixes}, nil
}

// ApplyAllFixes applies fixes across all files.
// If suffix is empty, edits files in place. Otherwise writes to path+suffix.
// Returns total fixes applied and files modified.
func ApplyAllFixes(findings []scanner.Finding, suffix string) (totalFixes int, filesModified int, errors []error) {
	// Group findings by file
	byFile := make(map[string][]scanner.Finding)
	for _, f := range findings {
		if f.Fix != nil {
			byFile[f.File] = append(byFile[f.File], f)
		}
	}

	for path, fileFindings := range byFile {
		n, err := ApplyFixes(path, fileFindings, suffix)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if n > 0 {
			totalFixes += n
			filesModified++
		}
	}
	return
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
		log.Printf("warning: %s has both byte-mode (%d) and line-mode (%d) fixes; dropping line-mode fixes",
			path, len(byteFixes), len(lineFixes))
		lineFixes = nil
	}

	result := string(content)
	var droppedFixes []DroppedFix

	if len(byteFixes) > 0 {
		sort.Slice(byteFixes, func(i, j int) bool {
			return byteFixes[i].fix.StartByte > byteFixes[j].fix.StartByte
		})
		var byteDropped []DroppedFix
		byteFixes, byteDropped = deduplicateByteTextFixRowsReverse(path, byteFixes)
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
		lineFixes, lineDropped = deduplicateLineTextFixRowsReverse(path, lineFixes)
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

// DroppedFix records a fix that was dropped due to overlap conflict.
type DroppedFix struct {
	Rule string
	File string
	Line int
}

// deduplicateByteFixesReverse removes overlapping byte fixes (already sorted descending).
// Returns the kept fixes and any dropped fixes.
func deduplicateByteFixesReverse(fixes []scanner.Finding) ([]scanner.Finding, []DroppedFix) {
	if len(fixes) <= 1 {
		return fixes, nil
	}
	var result []scanner.Finding
	var dropped []DroppedFix
	lastStart := -1
	for _, f := range fixes {
		if lastStart >= 0 && f.Fix.EndByte > lastStart {
			dropped = append(dropped, DroppedFix{
				Rule: f.Rule,
				File: f.File,
				Line: f.Line,
			})
			continue // overlaps with previous fix
		}
		result = append(result, f)
		lastStart = f.Fix.StartByte
	}
	return result, dropped
}

// deduplicateLineFixesReverse removes overlapping line fixes (already sorted descending).
// Returns the kept fixes and any dropped fixes.
func deduplicateLineFixesReverse(fixes []scanner.Finding) ([]scanner.Finding, []DroppedFix) {
	if len(fixes) <= 1 {
		return fixes, nil
	}
	var result []scanner.Finding
	var dropped []DroppedFix
	lastStart := -1
	for _, f := range fixes {
		if lastStart >= 0 && f.Fix.EndLine > lastStart {
			dropped = append(dropped, DroppedFix{
				Rule: f.Rule,
				File: f.File,
				Line: f.Line,
			})
			continue
		}
		result = append(result, f)
		lastStart = f.Fix.StartLine
	}
	return result, dropped
}

func deduplicateByteTextFixRowsReverse(path string, fixes []textFixRow) ([]textFixRow, []DroppedFix) {
	if len(fixes) <= 1 {
		return fixes, nil
	}
	var result []textFixRow
	var dropped []DroppedFix
	lastStart := -1
	for _, f := range fixes {
		if lastStart >= 0 && f.fix.EndByte > lastStart {
			dropped = append(dropped, DroppedFix{
				Rule: f.rule,
				File: path,
				Line: f.line,
			})
			continue
		}
		result = append(result, f)
		lastStart = f.fix.StartByte
	}
	return result, dropped
}

func deduplicateLineTextFixRowsReverse(path string, fixes []textFixRow) ([]textFixRow, []DroppedFix) {
	if len(fixes) <= 1 {
		return fixes, nil
	}
	var result []textFixRow
	var dropped []DroppedFix
	lastStart := -1
	for _, f := range fixes {
		if lastStart >= 0 && f.fix.EndLine > lastStart {
			dropped = append(dropped, DroppedFix{
				Rule: f.rule,
				File: path,
				Line: f.line,
			})
			continue
		}
		result = append(result, f)
		lastStart = f.fix.StartLine
	}
	return result, dropped
}
