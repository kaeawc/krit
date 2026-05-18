package fixer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
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
func ValidateFixResult(ctx context.Context, path string, original, fixed []byte) error {
	if !strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".kts") {
		return nil // only validate Kotlin files
	}

	origErrors := countParseErrors(ctx, original)
	fixedErrors := countParseErrors(ctx, fixed)

	if fixedErrors > origErrors {
		return fmt.Errorf("fix produced %d parse errors (original had %d)", fixedErrors, origErrors)
	}
	return nil
}

// countParseErrors parses content as Kotlin and counts ERROR nodes in the
// flat tree. Known grammar gaps in tree-sitter-kotlin are filtered so the
// validation safety net does not reject legal, modern Kotlin output:
//
//   - The rangeUntil operator (`..<`, Kotlin 1.7.20+) is not in the
//     tree-sitter grammar yet and surfaces as an ERROR node containing
//     just `<` immediately after the `..` token. Treating it as a real
//     parse error would block any autofix that emits `..<`, e.g.
//     RangeUntilInsteadOfRangeTo, even though the output is valid
//     Kotlin and compiles cleanly.
func countParseErrors(ctx context.Context, content []byte) int {
	parser := scanner.GetKotlinParser()
	defer scanner.PutKotlinParser(parser)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return 1 // treat parse failure as one error
	}
	file := scanner.NewParsedFile("", content, tree)
	if file == nil || file.FlatTree == nil {
		return 0
	}

	count := 0
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if file.FlatType(idx) != "ERROR" {
			return
		}
		if isRangeUntilGrammarGap(file, content, idx) {
			return
		}
		count++
	})
	return count
}

// isRangeUntilGrammarGap reports whether the ERROR node at idx is the
// `<` portion of a Kotlin rangeUntil operator (`..<`), which the
// tree-sitter grammar models as a parse error.
func isRangeUntilGrammarGap(file *scanner.File, content []byte, idx uint32) bool {
	if file.FlatNodeText(idx) != "<" {
		return false
	}
	start := int(file.FlatStartByte(idx))
	if start < 2 || start > len(content) {
		return false
	}
	return content[start-2] == '.' && content[start-1] == '.'
}

// FixResult holds the result of applying fixes to a file.
type FixResult struct {
	Applied      int
	DroppedFixes []DroppedFix
}

// ApplyAllFixesColumns applies text fixes from columnar findings across all files.
// It reconstructs only rows that carry text fixes, preserving the existing file-level
// fixer behavior without materializing the entire finding set.
func ApplyAllFixesColumns(ctx context.Context, columns *scanner.FindingColumns, suffix string) (totalFixes int, filesModified int, errors []error) {
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
		// validate=true engages the post-fix Kotlin parse safety net for
		// .kt/.kts files. Fixes that introduce new parse errors are
		// dropped (file untouched, entries surface in DroppedFixes)
		// instead of being silently written to disk. Non-Kotlin files
		// short-circuit inside ValidateFixResult so the safety net has
		// no measurable cost for Java/XML/Gradle batches.
		res, err := applyFixesDetailedColumns(ctx, path, columns, rows, suffix, true)
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

func applyFixesDetailedColumns(ctx context.Context, path string, columns *scanner.FindingColumns, rows []int, suffix string, validate bool) (FixResult, error) {
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
		if err := ValidateFixResult(ctx, path, content, []byte(result)); err != nil {
			// Treat validation failure as a soft drop rather than a
			// catastrophic error: the file stays untouched on disk and
			// every fix that contributed to the rejected output is
			// reported via DroppedFixes so callers (CLI, LSP, pipeline
			// output) can surface why nothing was written. Returning an
			// error here would abort the file's fix batch and the
			// dropped fixes would be invisible to the user.
			reason := fmt.Sprintf("fix validation failed: %v", err)
			validationDrops := make([]DroppedFix, 0, len(fixes))
			for _, f := range fixes {
				validationDrops = append(validationDrops, DroppedFix{
					Rule:   f.rule,
					File:   path,
					Line:   f.line,
					Reason: reason,
				})
			}
			pkgLog.Warn("dropped fix batch due to parse-validation failure",
				"file", path, "fixes", len(fixes), "error", err)
			return FixResult{DroppedFixes: append(droppedFixes, validationDrops...)}, nil
		}
	}

	outPath := path
	if suffix != "" {
		outPath = path + suffix
	}
	if err := fsutil.WriteFileAtomic(outPath, []byte(result), filePerm(outPath)); err != nil {
		return FixResult{DroppedFixes: droppedFixes}, err
	}

	return FixResult{Applied: len(fixes), DroppedFixes: droppedFixes}, nil
}

// filePerm returns the existing file's permission bits so atomic overwrite
// preserves them (e.g. 0o600 source files stay 0o600). os.WriteFile preserves
// the mode of an existing file implicitly; WriteFileAtomic creates a fresh
// inode via tempfile+rename, so the caller must pass the desired perms.
func filePerm(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
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
	sep := detectLineEnding(result)
	lines := splitLinesForJoin(result, sep)
	for _, f := range lineFixes {
		fix := f.fix
		start := fix.StartLine - 1
		end := fix.EndLine
		if start < 0 || end > len(lines) || start > end {
			continue
		}
		// Trim one trailing newline before splitting: `lines` was built
		// with strings.Split on "\n", so each element is a line without
		// a trailing newline. A Replacement like "// noop\n" would split
		// into ["// noop", ""], appending a blank line on every pass and
		// breaking idempotency. Treat the trailing "\n" as the line
		// terminator the lines model already implies.
		var replacement []string
		if fix.Replacement != "" {
			replacement = splitLinesForJoin(strings.TrimSuffix(fix.Replacement, "\n"), sep)
		}
		newLines := make([]string, 0, len(lines)-end+start+len(replacement))
		newLines = append(newLines, lines[:start]...)
		newLines = append(newLines, replacement...)
		newLines = append(newLines, lines[end:]...)
		lines = newLines
	}
	return strings.Join(lines, sep), dropped
}

// detectLineEnding returns "\r\n" if the first newline in s is preceded by
// a carriage return, otherwise "\n". Rules emit replacement text with bare
// "\n", so applyLineFixes needs the source's ending to avoid producing a
// file with mixed CRLF/LF lines (which corrupts Windows checkouts).
func detectLineEnding(s string) string {
	i := strings.IndexByte(s, '\n')
	if i > 0 && s[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// splitLinesForJoin splits s on "\n" and, when sep is "\r\n", strips the
// trailing "\r" each line carries so that the subsequent Join(sep) produces
// uniform CRLF output instead of leaving stray "\r" embedded mid-line.
func splitLinesForJoin(s, sep string) []string {
	lines := strings.Split(s, "\n")
	if sep == "\r\n" {
		for i, ln := range lines {
			lines[i] = strings.TrimSuffix(ln, "\r")
		}
	}
	return lines
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

// DroppedFix records a fix that was dropped before being written to disk.
// Reason describes why the fix was dropped (overlap conflict, parse-error
// safety net, etc.) and is empty for overlap drops, which are the historic
// default.
type DroppedFix struct {
	Rule   string
	File   string
	Line   int
	Reason string
}

// deduplicateFixesReverse removes overlapping fixes from a reverse-sorted
// slice. The caller supplies key extractors that return the start and the
// half-open end of each fix (byte or line depending on the fix mode) and
// a meta extractor that shapes the DroppedFix entry for conflicts.
//
// The endKey extractor MUST return an exclusive end offset so the overlap
// test (`endKey(f) > lastStart`) correctly rejects fixes whose range
// touches the previously kept fix's start. Byte ranges are already
// half-open in scanner.Fix; line ranges are inclusive on both ends, so
// the line-mode extractor returns `EndLine + 1` to normalize.
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

// textFixRowLineEnd returns an exclusive end-line key for the dedup pass.
// applyLineFixes treats Fix.EndLine as inclusive (slicing lines[start:end]
// where end == fix.EndLine), so two adjacent fixes like [5..7] and [7..7]
// both touch line 7. The dedup uses `endKey > lastStart` with half-open
// semantics, so we add 1 here to make the inclusive line range register
// as overlapping with a later fix that starts on the same line. Without
// this conversion the dedup would keep both fixes and applyLineFixes
// would garble line 7 by editing it twice.
func textFixRowLineEnd(f textFixRow) int   { return f.fix.EndLine + 1 }
func textFixRowLineStart(f textFixRow) int { return f.fix.StartLine }
func textFixRowDroppedFor(path string) func(textFixRow) DroppedFix {
	return func(f textFixRow) DroppedFix {
		return DroppedFix{Rule: f.rule, File: path, Line: f.line}
	}
}
