package scanner

import (
	"math"
	"path/filepath"
	"sort"
	"sync"
)

const (
	severityInfo uint8 = iota
	severityWarning
	severityError
)

var findingRowOrderPool = sync.Pool{
	New: func() interface{} {
		return make([]int, 0, 256)
	},
}

// FindingColumns stores finding rows in mostly-scalar parallel slices.
// String-heavy fields are interned into side tables so the hot row data stays flat.
type FindingColumns struct {
	Files         []string
	RuleSets      []string
	Rules         []string
	Messages      []string
	FixPool       []Fix
	BinaryFixPool []BinaryFix

	FileIdx        []uint32
	Line           []uint32
	Col            []uint16
	RuleSetIdx     []uint16
	RuleIdx        []uint16
	SeverityID     []uint8
	MessageIdx     []uint32
	Confidence     []uint8
	FixStart       []uint32
	BinaryFixStart []uint32
	N              int

	fileLexRanks    []uint32
	ruleSetLexRanks []uint16
	ruleLexRanks    []uint16
}

// FindingCollector incrementally builds a FindingColumns instance.
type FindingCollector struct {
	columns    FindingColumns
	fileIdx    map[string]uint32
	ruleSetIdx map[string]uint16
	ruleIdx    map[string]uint16
	messageIdx map[string]uint32
	strings    *LocalPool
}

// NewFindingCollector creates a collector sized for an expected number of rows.
func NewFindingCollector(capacity int) *FindingCollector {
	return &FindingCollector{
		columns: FindingColumns{
			FileIdx:        make([]uint32, 0, capacity),
			Line:           make([]uint32, 0, capacity),
			Col:            make([]uint16, 0, capacity),
			RuleSetIdx:     make([]uint16, 0, capacity),
			RuleIdx:        make([]uint16, 0, capacity),
			SeverityID:     make([]uint8, 0, capacity),
			MessageIdx:     make([]uint32, 0, capacity),
			Confidence:     make([]uint8, 0, capacity),
			FixStart:       make([]uint32, 0, capacity),
			BinaryFixStart: make([]uint32, 0, capacity),
		},
		fileIdx:    make(map[string]uint32),
		ruleSetIdx: make(map[string]uint16),
		ruleIdx:    make(map[string]uint16),
		messageIdx: make(map[string]uint32),
		strings:    NewLocalPool(globalStringPool),
	}
}

// CollectFindings materializes findings into columnar storage in one step.
func CollectFindings(findings []Finding) FindingColumns {
	collector := NewFindingCollector(len(findings))
	for _, finding := range findings {
		collector.Append(finding)
	}
	return *collector.Columns()
}

// Append adds a finding row to the collector.
func (c *FindingCollector) Append(f Finding) {
	c.columns.FileIdx = append(c.columns.FileIdx, c.internFile(f.File))
	c.columns.Line = append(c.columns.Line, clampUint32(f.Line))
	c.columns.Col = append(c.columns.Col, clampUint16(f.Col))
	c.columns.RuleSetIdx = append(c.columns.RuleSetIdx, c.internRuleSet(f.RuleSet))
	c.columns.RuleIdx = append(c.columns.RuleIdx, c.internRule(f.Rule))
	c.columns.SeverityID = append(c.columns.SeverityID, severityToID(f.Severity))
	c.columns.MessageIdx = append(c.columns.MessageIdx, c.internMessage(f.Message))
	c.columns.Confidence = append(c.columns.Confidence, confidenceToByte(f.Confidence))
	c.columns.FixStart = append(c.columns.FixStart, c.appendFix(f.Fix))
	c.columns.BinaryFixStart = append(c.columns.BinaryFixStart, c.appendBinaryFix(f.BinaryFix))
	c.columns.N++
}

// AppendAll adds each finding in order.
func (c *FindingCollector) AppendAll(findings []Finding) {
	for _, finding := range findings {
		c.Append(finding)
	}
}

// AppendRow copies a single row from an existing columnar finding set.
func (c *FindingCollector) AppendRow(columns *FindingColumns, row int) {
	if columns == nil || row < 0 || row >= columns.Len() {
		return
	}

	c.columns.FileIdx = append(c.columns.FileIdx, c.internFile(columns.Files[columns.FileIdx[row]]))
	c.columns.Line = append(c.columns.Line, columns.Line[row])
	c.columns.Col = append(c.columns.Col, columns.Col[row])
	c.columns.RuleSetIdx = append(c.columns.RuleSetIdx, c.internRuleSet(columns.RuleSets[columns.RuleSetIdx[row]]))
	c.columns.RuleIdx = append(c.columns.RuleIdx, c.internRule(columns.Rules[columns.RuleIdx[row]]))
	c.columns.SeverityID = append(c.columns.SeverityID, columns.SeverityID[row])
	c.columns.MessageIdx = append(c.columns.MessageIdx, c.internMessage(columns.Messages[columns.MessageIdx[row]]))
	c.columns.Confidence = append(c.columns.Confidence, columns.Confidence[row])
	c.columns.FixStart = append(c.columns.FixStart, c.appendExistingFix(columns, row))
	c.columns.BinaryFixStart = append(c.columns.BinaryFixStart, c.appendExistingBinaryFix(columns, row))
	c.columns.N++
}

// AppendColumns merges an existing columnar finding set into the collector.
func (c *FindingCollector) AppendColumns(columns *FindingColumns) {
	if columns == nil || columns.Len() == 0 {
		return
	}

	fileIdx := make([]uint32, len(columns.Files))
	for i, file := range columns.Files {
		fileIdx[i] = c.internFile(file)
	}
	ruleSetIdx := make([]uint16, len(columns.RuleSets))
	for i, ruleSet := range columns.RuleSets {
		ruleSetIdx[i] = c.internRuleSet(ruleSet)
	}
	ruleIdx := make([]uint16, len(columns.Rules))
	for i, rule := range columns.Rules {
		ruleIdx[i] = c.internRule(rule)
	}
	messageIdx := make([]uint32, len(columns.Messages))
	for i, message := range columns.Messages {
		messageIdx[i] = c.internMessage(message)
	}

	fixOffset := len(c.columns.FixPool)
	if len(columns.FixPool) > 0 {
		c.columns.FixPool = append(c.columns.FixPool, columns.FixPool...)
	}
	binaryFixOffset := len(c.columns.BinaryFixPool)
	if len(columns.BinaryFixPool) > 0 {
		for _, fix := range columns.BinaryFixPool {
			c.columns.BinaryFixPool = append(c.columns.BinaryFixPool, cloneBinaryFix(fix))
		}
	}

	for i := 0; i < columns.Len(); i++ {
		c.columns.FileIdx = append(c.columns.FileIdx, fileIdx[columns.FileIdx[i]])
		c.columns.Line = append(c.columns.Line, columns.Line[i])
		c.columns.Col = append(c.columns.Col, columns.Col[i])
		c.columns.RuleSetIdx = append(c.columns.RuleSetIdx, ruleSetIdx[columns.RuleSetIdx[i]])
		c.columns.RuleIdx = append(c.columns.RuleIdx, ruleIdx[columns.RuleIdx[i]])
		c.columns.SeverityID = append(c.columns.SeverityID, columns.SeverityID[i])
		c.columns.MessageIdx = append(c.columns.MessageIdx, messageIdx[columns.MessageIdx[i]])
		c.columns.Confidence = append(c.columns.Confidence, columns.Confidence[i])

		fixStart := columns.FixStart[i]
		if fixStart > 0 {
			fixStart += uint32(fixOffset)
		}
		c.columns.FixStart = append(c.columns.FixStart, fixStart)

		binaryFixStart := columns.BinaryFixStart[i]
		if binaryFixStart > 0 {
			binaryFixStart += uint32(binaryFixOffset)
		}
		c.columns.BinaryFixStart = append(c.columns.BinaryFixStart, binaryFixStart)
		c.columns.N++
	}
}

// Columns returns the built columns.
func (c *FindingCollector) Columns() *FindingColumns {
	return &c.columns
}

// Len returns the number of stored findings.
func (c *FindingColumns) Len() int {
	return c.N
}

// Clone returns a deep copy of the columns, including pooled fix state.
func (c *FindingColumns) Clone() FindingColumns {
	if c == nil {
		return FindingColumns{}
	}

	clone := FindingColumns{
		Files:          append([]string(nil), c.Files...),
		RuleSets:       append([]string(nil), c.RuleSets...),
		Rules:          append([]string(nil), c.Rules...),
		Messages:       append([]string(nil), c.Messages...),
		FileIdx:        append([]uint32(nil), c.FileIdx...),
		Line:           append([]uint32(nil), c.Line...),
		Col:            append([]uint16(nil), c.Col...),
		RuleSetIdx:     append([]uint16(nil), c.RuleSetIdx...),
		RuleIdx:        append([]uint16(nil), c.RuleIdx...),
		SeverityID:     append([]uint8(nil), c.SeverityID...),
		MessageIdx:     append([]uint32(nil), c.MessageIdx...),
		Confidence:     append([]uint8(nil), c.Confidence...),
		FixStart:       append([]uint32(nil), c.FixStart...),
		BinaryFixStart: append([]uint32(nil), c.BinaryFixStart...),
		N:              c.N,
		fileLexRanks:   append([]uint32(nil), c.fileLexRanks...),
		ruleSetLexRanks: append([]uint16(nil),
			c.ruleSetLexRanks...),
		ruleLexRanks: append([]uint16(nil), c.ruleLexRanks...),
	}
	if len(c.FixPool) > 0 {
		clone.FixPool = append([]Fix(nil), c.FixPool...)
	}
	if len(c.BinaryFixPool) > 0 {
		clone.BinaryFixPool = make([]BinaryFix, len(c.BinaryFixPool))
		for i, fix := range c.BinaryFixPool {
			clone.BinaryFixPool[i] = cloneBinaryFix(fix)
		}
	}
	return clone
}

// FileAt returns the file path for row i.
func (c *FindingColumns) FileAt(i int) string {
	return c.Files[c.FileIdx[i]]
}

// LineAt returns the 1-based line number for row i.
func (c *FindingColumns) LineAt(i int) int {
	return int(c.Line[i])
}

// ColumnAt returns the 1-based column number for row i.
func (c *FindingColumns) ColumnAt(i int) int {
	return int(c.Col[i])
}

// RuleSetAt returns the ruleset name for row i.
func (c *FindingColumns) RuleSetAt(i int) string {
	return c.RuleSets[c.RuleSetIdx[i]]
}

// RuleAt returns the rule name for row i.
func (c *FindingColumns) RuleAt(i int) string {
	return c.Rules[c.RuleIdx[i]]
}

// SeverityAt returns the severity string for row i.
func (c *FindingColumns) SeverityAt(i int) string {
	return severityFromID(c.SeverityID[i])
}

// MessageAt returns the message string for row i.
func (c *FindingColumns) MessageAt(i int) string {
	return c.Messages[c.MessageIdx[i]]
}

// ConfidenceAt returns the confidence value for row i in the 0.0-1.0 range.
func (c *FindingColumns) ConfidenceAt(i int) float64 {
	return float64(c.Confidence[i]) / 100.0
}

// HasFix reports whether row i has a text auto-fix.
func (c *FindingColumns) HasFix(i int) bool {
	return c.FixStart[i] > 0
}

// FixAt returns a copy of the text auto-fix for row i, or nil when absent.
func (c *FindingColumns) FixAt(i int) *Fix {
	if c == nil {
		return nil
	}
	if ref := c.FixStart[i]; ref > 0 {
		fix := c.FixPool[ref-1]
		return &fix
	}
	return nil
}

// BinaryFixAt returns a cloned binary fix for row i, or nil when absent.
func (c *FindingColumns) BinaryFixAt(i int) *BinaryFix {
	if c == nil {
		return nil
	}
	if ref := c.BinaryFixStart[i]; ref > 0 {
		binaryFix := cloneBinaryFix(c.BinaryFixPool[ref-1])
		return &binaryFix
	}
	return nil
}

// CountTextFixes returns the number of rows with a text auto-fix.
func (c *FindingColumns) CountTextFixes() int {
	count := 0
	for _, ref := range c.FixStart {
		if ref > 0 {
			count++
		}
	}
	return count
}

// StripTextFixes removes text auto-fixes from rows matching drop and returns
// the number of stripped fixes. Binary fixes are preserved.
func (c *FindingColumns) StripTextFixes(drop func(row int) bool) int {
	if drop == nil {
		return 0
	}
	stripped := 0
	for row, ref := range c.FixStart {
		if ref == 0 {
			continue
		}
		if drop(row) {
			c.FixStart[row] = 0
			stripped++
		}
	}
	return stripped
}

// VisitRowsWithTextFixes visits row indexes that carry a text auto-fix.
func (c *FindingColumns) VisitRowsWithTextFixes(yield func(row int)) {
	if c == nil {
		return
	}
	for row, ref := range c.FixStart {
		if ref > 0 {
			yield(row)
		}
	}
}

// VisitRowsWithBinaryFixes visits row indexes that carry a binary auto-fix.
func (c *FindingColumns) VisitRowsWithBinaryFixes(yield func(row int)) {
	if c == nil {
		return
	}
	for row, ref := range c.BinaryFixStart {
		if ref > 0 {
			yield(row)
		}
	}
}

// Finding reconstructs the i'th row as a compatibility Finding value.
func (c *FindingColumns) Finding(i int) Finding {
	finding := Finding{
		File:       c.FileAt(i),
		Line:       c.LineAt(i),
		Col:        c.ColumnAt(i),
		RuleSet:    c.RuleSetAt(i),
		Rule:       c.RuleAt(i),
		Severity:   c.SeverityAt(i),
		Message:    c.MessageAt(i),
		Confidence: c.ConfidenceAt(i),
	}
	finding.Fix = c.FixAt(i)
	finding.BinaryFix = c.BinaryFixAt(i)
	return finding
}

// Findings reconstructs all rows as compatibility Finding values.
func (c *FindingColumns) Findings() []Finding {
	findings := make([]Finding, c.N)
	for i := 0; i < c.N; i++ {
		findings[i] = c.Finding(i)
	}
	return findings
}

// FindingsWithFixes reconstructs only rows that carry a text or binary fix.
// Row order is preserved so downstream fix application stays deterministic.
func (c *FindingColumns) FindingsWithFixes() []Finding {
	if c == nil || c.N == 0 {
		return nil
	}

	count := 0
	for i := 0; i < c.N; i++ {
		if c.FixStart[i] > 0 || c.BinaryFixStart[i] > 0 {
			count++
		}
	}
	if count == 0 {
		return nil
	}

	findings := make([]Finding, 0, count)
	for i := 0; i < c.N; i++ {
		if c.FixStart[i] == 0 && c.BinaryFixStart[i] == 0 {
			continue
		}
		findings = append(findings, c.Finding(i))
	}
	return findings
}

// FilterRows keeps rows for which keep returns true while preserving fix pools
// and interned string tables via the collector append path.
func (c *FindingColumns) FilterRows(keep func(row int) bool) FindingColumns {
	if c == nil {
		return FindingColumns{}
	}
	if keep == nil {
		return c.Clone()
	}

	filtered := NewFindingCollector(c.Len())
	for row := 0; row < c.Len(); row++ {
		if keep(row) {
			filtered.AppendRow(c, row)
		}
	}
	return *filtered.Columns()
}

// FilterColumnsByFilePaths keeps only rows whose file path resolves to an
// absolute path present in allowedPaths.
func FilterColumnsByFilePaths(columns *FindingColumns, allowedPaths map[string]bool) FindingColumns {
	if columns == nil || columns.Len() == 0 || len(allowedPaths) == 0 {
		return FindingColumns{}
	}

	allowedFileIdx := make([]bool, len(columns.Files))
	for i, file := range columns.Files {
		abs, err := filepath.Abs(file)
		if err != nil {
			abs = file
		}
		allowedFileIdx[i] = allowedPaths[abs]
	}

	return columns.FilterRows(func(row int) bool {
		return allowedFileIdx[columns.FileIdx[row]]
	})
}

// PromoteWarningsToErrors rewrites warning severities in-place without
// materializing Finding structs.
func (c *FindingColumns) PromoteWarningsToErrors() {
	for i, severity := range c.SeverityID {
		if severity == severityWarning {
			c.SeverityID[i] = severityError
		}
	}
}

// FilterByMinConfidence drops rows whose confidence is strictly below the
// given threshold (0.0-1.0). A row with confidence == 0 is treated as
// "unset" and kept when min is 0, dropped when min > 0. Returns a new
// FindingColumns; the original is unchanged.
func (c *FindingColumns) FilterByMinConfidence(min float64) FindingColumns {
	if min <= 0 {
		return *c
	}
	// Confidence is stored as uint8 percentage (0-100); convert the
	// threshold once and compare as int to avoid float in the hot loop.
	thresh := uint8(min*100 + 0.5)
	return c.FilterRows(func(row int) bool {
		return c.Confidence[row] >= thresh
	})
}

// SortByFileLine reorders rows in-place using file, line, then column ordering.
func (c *FindingColumns) SortByFileLine() {
	if c.N < 2 {
		return
	}

	order := getFindingRowOrderBuffer(c.N)
	defer putFindingRowOrderBuffer(order)
	for i := range order {
		order[i] = i
	}

	inverse := getFindingRowOrderBuffer(c.N)
	defer putFindingRowOrderBuffer(inverse)
	c.sortRowOrderByFileLine(order, inverse)

	// Convert dest->src order into a src->dest permutation so every row column
	// can be reordered in place through the same cycle decomposition.
	for dest, src := range order {
		inverse[src] = dest
	}

	applyIndexedPermutationInPlace(inverse, func(i, j int) {
		swapSlice(c.FileIdx, i, j)
		swapSlice(c.Line, i, j)
		swapSlice(c.Col, i, j)
		swapSlice(c.RuleSetIdx, i, j)
		swapSlice(c.RuleIdx, i, j)
		swapSlice(c.SeverityID, i, j)
		swapSlice(c.MessageIdx, i, j)
		swapSlice(c.Confidence, i, j)
		swapSlice(c.FixStart, i, j)
		swapSlice(c.BinaryFixStart, i, j)
	})
}

// SortedRowOrderByFileLine returns row indexes ordered by file, line, column,
// then lexical ruleset/rule tie-breakers. The column data is not mutated.
func (c *FindingColumns) SortedRowOrderByFileLine() []int {
	if c.N == 0 {
		return nil
	}
	order := make([]int, c.N)
	for i := range order {
		order[i] = i
	}
	if c.N < 2 {
		return order
	}
	scratch := getFindingRowOrderBuffer(c.N)
	defer putFindingRowOrderBuffer(scratch)
	c.sortRowOrderByFileLine(order, scratch)
	return order
}

// VisitSortedByFileLine visits row indexes ordered by file, line, column, then
// lexical ruleset/rule tie-breakers without allocating a result slice.
func (c *FindingColumns) VisitSortedByFileLine(yield func(row int)) {
	if c.N == 0 {
		return
	}
	order := getFindingRowOrderBuffer(c.N)
	defer putFindingRowOrderBuffer(order)
	for i := range order {
		order[i] = i
	}
	if c.N >= 2 {
		scratch := getFindingRowOrderBuffer(c.N)
		defer putFindingRowOrderBuffer(scratch)
		c.sortRowOrderByFileLine(order, scratch)
	}
	for _, row := range order {
		yield(row)
	}
}

func (c *FindingColumns) sortRowOrderByFileLine(order, scratch []int) {
	fileRanks := c.fileSortRanks()
	ruleSetRanks := c.ruleSetSortRanks()
	ruleRanks := c.ruleSortRanks()

	// Stable least-significant-key-first radix passes preserve the original row
	// index as the final tie-breaker while avoiding O(n log n) comparator sorting.
	radixSortOrderByUint16Lookup(order, scratch, c.RuleIdx, ruleRanks)
	radixSortOrderByUint16Lookup(order, scratch, c.RuleSetIdx, ruleSetRanks)
	radixSortOrderByUint16(order, scratch, c.Col)
	radixSortOrderByUint32(order, scratch, c.Line)
	radixSortOrderByUint32Lookup(order, scratch, c.FileIdx, fileRanks)
}

func (c *FindingCollector) internFile(value string) uint32 {
	value = c.internTableString(value)
	if idx, ok := c.fileIdx[value]; ok {
		return idx
	}
	idx := uint32(len(c.columns.Files))
	c.columns.Files = append(c.columns.Files, value)
	c.columns.fileLexRanks = nil
	c.fileIdx[value] = idx
	return idx
}

func (c *FindingCollector) internRuleSet(value string) uint16 {
	value = c.internTableString(value)
	if idx, ok := c.ruleSetIdx[value]; ok {
		return idx
	}
	idx := len(c.columns.RuleSets)
	if idx > math.MaxUint16 {
		panic("too many unique rule sets for uint16 index")
	}
	c.columns.RuleSets = append(c.columns.RuleSets, value)
	c.columns.ruleSetLexRanks = nil
	out := uint16(idx)
	c.ruleSetIdx[value] = out
	return out
}

func (c *FindingCollector) internRule(value string) uint16 {
	value = c.internTableString(value)
	if idx, ok := c.ruleIdx[value]; ok {
		return idx
	}
	idx := len(c.columns.Rules)
	if idx > math.MaxUint16 {
		panic("too many unique rules for uint16 index")
	}
	c.columns.Rules = append(c.columns.Rules, value)
	c.columns.ruleLexRanks = nil
	out := uint16(idx)
	c.ruleIdx[value] = out
	return out
}

func (c *FindingCollector) internMessage(value string) uint32 {
	value = c.internTableString(value)
	if idx, ok := c.messageIdx[value]; ok {
		return idx
	}
	idx := uint32(len(c.columns.Messages))
	c.columns.Messages = append(c.columns.Messages, value)
	c.messageIdx[value] = idx
	return idx
}

func (c *FindingCollector) internTableString(value string) string {
	if value == "" {
		return ""
	}
	if c.strings == nil {
		c.strings = NewLocalPool(globalStringPool)
	}
	return c.strings.Intern(value)
}

func (c *FindingCollector) appendFix(fix *Fix) uint32 {
	if fix == nil {
		return 0
	}
	c.columns.FixPool = append(c.columns.FixPool, *fix)
	return uint32(len(c.columns.FixPool))
}

func (c *FindingCollector) appendBinaryFix(fix *BinaryFix) uint32 {
	if fix == nil {
		return 0
	}
	c.columns.BinaryFixPool = append(c.columns.BinaryFixPool, cloneBinaryFix(*fix))
	return uint32(len(c.columns.BinaryFixPool))
}

func (c *FindingCollector) appendExistingFix(columns *FindingColumns, row int) uint32 {
	ref := columns.FixStart[row]
	if ref == 0 {
		return 0
	}
	c.columns.FixPool = append(c.columns.FixPool, columns.FixPool[ref-1])
	return uint32(len(c.columns.FixPool))
}

func (c *FindingCollector) appendExistingBinaryFix(columns *FindingColumns, row int) uint32 {
	ref := columns.BinaryFixStart[row]
	if ref == 0 {
		return 0
	}
	c.columns.BinaryFixPool = append(c.columns.BinaryFixPool, cloneBinaryFix(columns.BinaryFixPool[ref-1]))
	return uint32(len(c.columns.BinaryFixPool))
}

func severityToID(severity string) uint8 {
	switch severity {
	case "error":
		return severityError
	case "warning":
		return severityWarning
	default:
		return severityInfo
	}
}

func severityFromID(id uint8) string {
	switch id {
	case severityError:
		return "error"
	case severityWarning:
		return "warning"
	default:
		return "info"
	}
}

func confidenceToByte(confidence float64) uint8 {
	if confidence <= 0 {
		return 0
	}
	if confidence >= 1 {
		return 100
	}
	return uint8(math.Round(confidence * 100))
}

func clampUint32(v int) uint32 {
	if v <= 0 {
		return 0
	}
	if v >= math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}

func clampUint16(v int) uint16 {
	if v <= 0 {
		return 0
	}
	if v >= math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(v)
}

func cloneBinaryFix(fix BinaryFix) BinaryFix {
	fix.Content = append([]byte(nil), fix.Content...)
	return fix
}

func getFindingRowOrderBuffer(n int) []int {
	buf := findingRowOrderPool.Get().([]int)
	if cap(buf) < n {
		return make([]int, n)
	}
	return buf[:n]
}

func putFindingRowOrderBuffer(buf []int) {
	if cap(buf) == 0 {
		return
	}
	findingRowOrderPool.Put(buf[:0])
}

func (c *FindingColumns) fileSortRanks() []uint32 {
	if c.fileLexRanks == nil && len(c.Files) > 0 {
		c.fileLexRanks = lexicographicRanksUint32(c.Files)
	}
	return c.fileLexRanks
}

func (c *FindingColumns) ruleSetSortRanks() []uint16 {
	if c.ruleSetLexRanks == nil && len(c.RuleSets) > 0 {
		c.ruleSetLexRanks = lexicographicRanksUint16(c.RuleSets)
	}
	return c.ruleSetLexRanks
}

func (c *FindingColumns) ruleSortRanks() []uint16 {
	if c.ruleLexRanks == nil && len(c.Rules) > 0 {
		c.ruleLexRanks = lexicographicRanksUint16(c.Rules)
	}
	return c.ruleLexRanks
}

func (c *FindingColumns) prepareSortRanks() {
	if c == nil {
		return
	}
	c.fileSortRanks()
	c.ruleSetSortRanks()
	c.ruleSortRanks()
}

func permuteByOrder[T any](values []T, order []int) []T {
	if len(values) == 0 {
		return values
	}
	permuted := make([]T, len(values))
	for i, src := range order {
		permuted[i] = values[src]
	}
	return permuted
}

func applyIndexedPermutationInPlace(destBySrc []int, swap func(i, j int)) {
	for i := 0; i < len(destBySrc); i++ {
		for destBySrc[i] != i {
			j := destBySrc[i]
			swap(i, j)
			destBySrc[i], destBySrc[j] = destBySrc[j], destBySrc[i]
		}
	}
}

func swapSlice[T any](values []T, i, j int) {
	values[i], values[j] = values[j], values[i]
}

func lexicographicRanksUint32(values []string) []uint32 {
	if len(values) == 0 {
		return nil
	}
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return values[order[i]] < values[order[j]]
	})
	ranks := make([]uint32, len(values))
	for rank, idx := range order {
		ranks[idx] = uint32(rank)
	}
	return ranks
}

func lexicographicRanksUint16(values []string) []uint16 {
	if len(values) == 0 {
		return nil
	}
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return values[order[i]] < values[order[j]]
	})
	ranks := make([]uint16, len(values))
	for rank, idx := range order {
		ranks[idx] = uint16(rank)
	}
	return ranks
}

func radixSortOrderByUint32(order, scratch []int, values []uint32) {
	radixSortOrder(order, scratch, 4, func(row int) uint32 {
		return values[row]
	})
}

func radixSortOrderByUint16(order, scratch []int, values []uint16) {
	radixSortOrder(order, scratch, 2, func(row int) uint32 {
		return uint32(values[row])
	})
}

func radixSortOrderByUint32Lookup(order, scratch []int, indexes []uint32, values []uint32) {
	radixSortOrder(order, scratch, 4, func(row int) uint32 {
		return values[indexes[row]]
	})
}

func radixSortOrderByUint16Lookup(order, scratch []int, indexes []uint16, values []uint16) {
	radixSortOrder(order, scratch, 2, func(row int) uint32 {
		return uint32(values[indexes[row]])
	})
}

func radixSortOrder(order, scratch []int, passes int, key func(int) uint32) {
	src := order
	dst := scratch
	inOrder := true
	for pass := 0; pass < passes; pass++ {
		shift := uint(pass * 8)
		var counts [256]int
		for _, row := range src {
			counts[byte(key(row)>>shift)]++
		}
		offset := 0
		for i, count := range counts {
			counts[i] = offset
			offset += count
		}
		for _, row := range src {
			bucket := byte(key(row) >> shift)
			dst[counts[bucket]] = row
			counts[bucket]++
		}
		src, dst = dst, src
		inOrder = !inOrder
	}
	if !inOrder {
		copy(order, src)
	}
}
