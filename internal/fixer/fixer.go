package fixer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/iterutil"
	"github.com/kaeawc/krit/internal/logger"
	"github.com/kaeawc/krit/internal/scanner"
)

// pkgLog routes overlap warnings emitted from applyFixes. Package-level
// because the fixer is invoked through stateless top-level functions;
// tests swap via SetLogger to capture records.
var pkgLog logger.Logger = logger.New(logger.Config{Format: logger.FormatText, Level: slog.LevelInfo})

// SetLogger replaces the package-level Logger. Intended for tests.
func SetLogger(l logger.Logger) { pkgLog = l }

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
		if fix := columns.FixAt(row); fix != nil && fix.TargetFile != "" {
			file = fix.TargetFile
		}
		byFile[file] = append(byFile[file], row)
	})

	// Iterate paths in sorted order so emitted errors and log lines have
	// a stable ordering across runs. Bare map iteration here was a source
	// of CI log diff noise — see #27.
	for _, path := range iterutil.SortedKeys(byFile) {
		rows := byFile[path]
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

	fixes := collectTextFixRows(path, columns, rows)
	if len(fixes) == 0 {
		return FixResult{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) || !allFixesExplicitlyTargetPath(path, fixes) {
			return FixResult{}, err
		}
		content = nil
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
		result, droppedFixes = applyByteFixes(result, byteFixes, path)
	}

	if len(lineFixes) > 0 {
		result, droppedFixes = applyLineFixes(result, lineFixes, path)
	}

	for _, d := range droppedFixes {
		pkgLog.Warn("dropped overlapping fix", "rule", d.Rule, "file", d.File, "line", d.Line)
	}
	if len(droppedFixes) > 0 {
		pkgLog.Warn("fixes dropped due to overlapping conflicts", "count", len(droppedFixes), "path", path)
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

func collectTextFixRows(path string, columns *scanner.FindingColumns, rows []int) []textFixRow {
	fixes := make([]textFixRow, 0, len(rows))
	for _, row := range rows {
		if row < 0 || row >= columns.Len() {
			continue
		}
		fix := columns.FixAt(row)
		if fix == nil {
			continue
		}
		targetPath := columns.FileAt(row)
		if fix.TargetFile != "" {
			targetPath = fix.TargetFile
		}
		if targetPath != path {
			continue
		}
		fixes = append(fixes, textFixRow{
			rule: columns.RuleAt(row),
			line: columns.LineAt(row),
			fix:  *fix,
		})
	}
	return fixes
}

func allFixesExplicitlyTargetPath(path string, fixes []textFixRow) bool {
	if len(fixes) == 0 {
		return false
	}
	for _, f := range fixes {
		if f.fix.TargetFile != path {
			return false
		}
	}
	return true
}

func applyByteFixes(result string, byteFixes []textFixRow, path string) (string, []DroppedFix) {
	canonicalSortByteFixes(byteFixes)
	var dropped []DroppedFix
	byteFixes, dropped = deduplicateFixesReverse(byteFixes, textFixRowByteEnd, textFixRowByteStart, textFixRowDroppedFor(path))
	buf := []byte(result)
	for _, f := range byteFixes {
		fix := f.fix
		if fix.StartByte < 0 || fix.EndByte > len(buf) || fix.StartByte > fix.EndByte {
			continue
		}
		buf = append(buf[:fix.StartByte], append([]byte(fix.Replacement), buf[fix.EndByte:]...)...)
	}
	return string(buf), dropped
}

func applyLineFixes(result string, lineFixes []textFixRow, path string) (string, []DroppedFix) {
	canonicalSortLineFixes(lineFixes)
	var dropped []DroppedFix
	lineFixes, dropped = deduplicateFixesReverse(lineFixes, textFixRowLineEnd, textFixRowLineStart, textFixRowDroppedFor(path))
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
	return strings.Join(lines, "\n"), dropped
}

// canonicalSortByteFixes orders byte-mode fixes for reverse-walk
// application. The primary key is descending StartByte so the
// dedup/apply loop walks the buffer back-to-front. Tiebreakers cover
// the cases where two fixes from different rules collide at the same
// position:
//
//   - EndByte desc: a longer span "wins" the slot when starts are
//     equal (the shorter span overlaps the longer's interior, so
//     keeping the longer one drops the inner overlap deterministically).
//   - rule asc: lexicographic rule ID — total order tiebreaker so two
//     unrelated rules touching the same range have a stable winner
//     across runs.
//
// `sort.SliceStable` is used so callers that already supplied an
// upstream secondary order (e.g. a `rows` slice built in column
// order) still see that order respected within equal-key groups.
//
// Regression context: see #26. Previously the bare
// `sort.Slice(StartByte desc)` left ties in undefined order, so when
// two rules emitted overlapping fixes at the same offset
// `deduplicateFixesReverse` dropped a different rule each run.
func canonicalSortByteFixes(fixes []textFixRow) {
	sort.SliceStable(fixes, func(i, j int) bool {
		a, b := fixes[i].fix, fixes[j].fix
		if a.StartByte != b.StartByte {
			return a.StartByte > b.StartByte
		}
		if a.EndByte != b.EndByte {
			return a.EndByte > b.EndByte
		}
		return fixes[i].rule < fixes[j].rule
	})
}

// canonicalSortLineFixes is the line-mode counterpart to
// canonicalSortByteFixes. Same total-order shape on
// (StartLine desc, EndLine desc, rule asc).
func canonicalSortLineFixes(fixes []textFixRow) {
	sort.SliceStable(fixes, func(i, j int) bool {
		a, b := fixes[i].fix, fixes[j].fix
		if a.StartLine != b.StartLine {
			return a.StartLine > b.StartLine
		}
		if a.EndLine != b.EndLine {
			return a.EndLine > b.EndLine
		}
		return fixes[i].rule < fixes[j].rule
	})
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

func textFixRowByteEnd(f textFixRow) int   { return f.fix.EndByte }
func textFixRowByteStart(f textFixRow) int { return f.fix.StartByte }
func textFixRowLineEnd(f textFixRow) int   { return f.fix.EndLine }
func textFixRowLineStart(f textFixRow) int { return f.fix.StartLine }
func textFixRowDroppedFor(path string) func(textFixRow) DroppedFix {
	return func(f textFixRow) DroppedFix {
		return DroppedFix{Rule: f.rule, File: path, Line: f.line}
	}
}
