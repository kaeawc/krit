package scanner

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Baseline represents a set of suppressed finding IDs loaded from a Krit
// baseline file. XML baselines use the SmellBaseline schema for interoperability
// with existing Kotlin analyzer workflows.
type Baseline struct {
	ManuallySuppressed map[string]bool // IDs manually suppressed by user
	CurrentIssues      map[string]bool // IDs from last run (auto-generated)
}

// BaselineEntry is a parsed baseline ID from one of the baseline sections.
type BaselineEntry struct {
	ID        string
	Section   string
	Rule      string
	Path      string
	Signature string
}

// BaselineXML is the XML structure used by Krit baseline files.
type BaselineXML struct {
	XMLName            xml.Name       `xml:"SmellBaseline"`
	ManuallySuppressed BaselineIDList `xml:"ManuallySuppressedIssues"`
	CurrentIssues      BaselineIDList `xml:"CurrentIssues"`
}

type BaselineIDList struct {
	IDs []string `xml:"ID"`
}

// LoadBaseline reads a baseline file.  Detects format automatically:
// files ending in .json are read as JSON; all others are parsed as baseline XML.
func LoadBaseline(path string) (*Baseline, error) {
	if strings.HasSuffix(path, ".json") {
		return LoadBaselineJSON(path)
	}
	return LoadBaselineXML(path)
}

// LoadBaselineXML reads a baseline XML file.
func LoadBaselineXML(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var db BaselineXML
	if err := xml.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("invalid baseline XML: %w", err)
	}
	b := &Baseline{
		ManuallySuppressed: make(map[string]bool),
		CurrentIssues:      make(map[string]bool),
	}
	for _, id := range db.ManuallySuppressed.IDs {
		b.ManuallySuppressed[id] = true
	}
	for _, id := range db.CurrentIssues.IDs {
		b.CurrentIssues[id] = true
	}
	return b, nil
}

// baselineJSON is the JSON encoding of a Baseline used for krit-native files.
type baselineJSON struct {
	ManuallySuppressed []string `json:"manuallySuppressed,omitempty"`
	CurrentIssues      []string `json:"currentIssues,omitempty"`
}

// LoadBaselineJSON reads a krit-native JSON baseline file.
func LoadBaselineJSON(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bj baselineJSON
	if err := json.Unmarshal(data, &bj); err != nil {
		return nil, fmt.Errorf("invalid baseline JSON: %w", err)
	}
	b := &Baseline{
		ManuallySuppressed: make(map[string]bool, len(bj.ManuallySuppressed)),
		CurrentIssues:      make(map[string]bool, len(bj.CurrentIssues)),
	}
	for _, id := range bj.ManuallySuppressed {
		b.ManuallySuppressed[id] = true
	}
	for _, id := range bj.CurrentIssues {
		b.CurrentIssues[id] = true
	}
	return b, nil
}

// WriteBaselineJSON writes a Baseline to a JSON file.  The output is sorted
// for stable diffs.  Atomically replaces any existing file.
func WriteBaselineJSON(path string, b *Baseline) error {
	ms := make([]string, 0, len(b.ManuallySuppressed))
	ci := make([]string, 0, len(b.CurrentIssues))
	for id := range b.ManuallySuppressed {
		ms = append(ms, id)
	}
	for id := range b.CurrentIssues {
		ci = append(ci, id)
	}
	sort.Strings(ms)
	sort.Strings(ci)
	bj := baselineJSON{
		ManuallySuppressed: ms,
		CurrentIssues:      ci,
	}
	data, err := json.MarshalIndent(bj, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(data, '\n'), 0o644)
}

// Contains checks if a finding ID is in the baseline (either section).
func (b *Baseline) Contains(id string) bool {
	return b.ManuallySuppressed[id] || b.CurrentIssues[id]
}

// Entries returns parsed, sorted entries from both baseline sections.
func (b *Baseline) Entries() []BaselineEntry {
	entries := make([]BaselineEntry, 0, len(b.ManuallySuppressed)+len(b.CurrentIssues))
	for id := range b.ManuallySuppressed {
		entries = append(entries, ParseBaselineEntry("ManuallySuppressedIssues", id))
	}
	for id := range b.CurrentIssues {
		entries = append(entries, ParseBaselineEntry("CurrentIssues", id))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Section != entries[j].Section {
			return entries[i].Section < entries[j].Section
		}
		return entries[i].ID < entries[j].ID
	})
	return entries
}

// ParseBaselineEntry splits a baseline ID into its parts.
func ParseBaselineEntry(section, id string) BaselineEntry {
	entry := BaselineEntry{
		ID:      id,
		Section: section,
	}
	parts := strings.SplitN(id, ":", 3)
	if len(parts) > 0 {
		entry.Rule = parts[0]
	}
	if len(parts) > 1 {
		entry.Path = parts[1]
	}
	if len(parts) > 2 {
		entry.Signature = parts[2]
	}
	return entry
}

// BaselineID generates a baseline ID for a finding.
// Format: "RuleName:filename:signature"
// The signature is typically the entity name (function, class, etc.)
//
// When basePath is set, uses relative path instead of just filename.
// This avoids collisions between same-named files in different modules.
func BaselineID(f Finding, signature string, basePath string) string {
	return baselineIDParts(f.File, f.Rule, f.Message, signature, basePath)
}

// BaselineIDAt generates a baseline ID directly from a
// columnar finding row without reconstructing a Finding value first.
func BaselineIDAt(columns *FindingColumns, row int, signature string, basePath string) string {
	if columns == nil || row < 0 || row >= columns.Len() {
		return ""
	}
	return baselineIDParts(columns.FileAt(row), columns.RuleAt(row), columns.MessageAt(row), signature, basePath)
}

// BaselineIDFilenameOnly generates a baseline ID using only the file basename.
// Use this when comparing against baseline files that do not include module paths.
func BaselineIDFilenameOnly(f Finding, signature string) string {
	return BaselineID(f, signature, "")
}

// BaselineIDFilenameOnlyAt generates a filename-only baseline ID directly from
// a columnar finding row.
func BaselineIDFilenameOnlyAt(columns *FindingColumns, row int, signature string) string {
	return BaselineIDAt(columns, row, signature, "")
}

// WriteBaseline writes findings as a baseline XML file.
// If basePath is set, uses relative paths for multi-module disambiguation.
// If basePath is empty, uses filename-only IDs for legacy compatibility.
func WriteBaseline(path string, findings []Finding, basePath string) error {
	columns := CollectFindings(findings)
	return WriteBaselineColumns(path, &columns, basePath)
}

// WriteBaselineColumns writes columnar findings as a baseline XML file.
func WriteBaselineColumns(path string, columns *FindingColumns, basePath string) error {
	return WriteBaselineIDsXML(path, CollectBaselineIDs(columns, basePath))
}

// CollectBaselineIDs returns the sorted, deduplicated baseline IDs for
// columns under basePath. The order and dedup semantics match
// WriteBaselineColumns so that calling WriteBaselineIDsXML with the
// returned slice produces a byte-identical baseline file. Useful for
// daemon callers that compute IDs on a remote process and ship them to
// the CLI for the actual file write — preserves the "daemon never
// touches user files" invariant without forcing a second
// FindingColumns serialization on the wire.
func CollectBaselineIDs(columns *FindingColumns, basePath string) []string {
	if columns == nil {
		return nil
	}
	ids := make([]string, 0, columns.Len())
	seen := make(map[string]bool, columns.Len())
	for i := 0; i < columns.Len(); i++ {
		id := BaselineIDAt(columns, i, "", basePath)
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// WriteBaselineIDsXML serialises a pre-computed baseline-ID slice into
// the same XML schema WriteBaselineColumns emits and writes it to
// path. Caller is responsible for ensuring ids is sorted and
// deduplicated (CollectBaselineIDs returns the canonical shape). Used
// by the CLI when receiving baseline IDs from the daemon's
// analyze-project verb.
func WriteBaselineIDsXML(path string, ids []string) error {
	db := BaselineXML{
		ManuallySuppressed: BaselineIDList{},
		CurrentIssues:      BaselineIDList{IDs: ids},
	}
	data, err := xml.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	header := []byte(xml.Header)
	return os.WriteFile(path, append(header, data...), 0644)
}

// FilterByBaseline removes findings that are in the baseline.
// Tries both relative-path IDs and filename-only IDs for compatibility with
// baseline files generated with or without module-relative paths.
func FilterByBaseline(findings []Finding, baseline *Baseline, basePath string) []Finding {
	if baseline == nil {
		return findings
	}

	columns := CollectFindings(findings)
	filtered := FilterColumnsByBaseline(&columns, baseline, basePath)
	return filtered.Findings()
}

// FilterColumnsByBaseline removes findings that are in the baseline without
// materializing intermediate []Finding slices.
func FilterColumnsByBaseline(columns *FindingColumns, baseline *Baseline, basePath string) FindingColumns {
	if columns == nil {
		return FindingColumns{}
	}
	if baseline == nil {
		return columns.Clone()
	}
	return columns.FilterRows(func(row int) bool {
		id := BaselineIDAt(columns, row, "", basePath)
		if baseline.Contains(id) {
			return false
		}
		if basePath != "" {
			idFilenameOnly := BaselineIDFilenameOnlyAt(columns, row, "")
			if baseline.Contains(idFilenameOnly) {
				return false
			}
		}
		return true
	})
}

// FindingSignature extracts a signature for a finding based on the source line.
func FindingSignature(f Finding, lines []string) string {
	if f.Line <= 0 || f.Line > len(lines) {
		return ""
	}
	line := strings.TrimSpace(lines[f.Line-1])
	// Extract the declaration signature (function name, class name, etc.)
	// Truncate at { or = for cleaner signatures
	if idx := strings.Index(line, "{"); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if idx := strings.Index(line, "="); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line
}

func baselineIDParts(file, rule, message, signature, basePath string) string {
	filename := filepath.Base(file)
	if basePath != "" {
		// Use path relative to basePath for multi-module disambiguation
		if rel, err := filepath.Rel(basePath, file); err == nil {
			filename = rel
		}
	}
	if signature == "" {
		signature = fmt.Sprintf("$%s$%s", rule, message)
	}
	return fmt.Sprintf("%s:%s:%s", rule, filename, signature)
}
