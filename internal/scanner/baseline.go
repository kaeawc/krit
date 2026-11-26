package scanner

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/fsutil"
)

// Baseline represents a set of suppressed finding IDs loaded from a Krit
// baseline file. XML baselines use the SmellBaseline schema for interoperability
// with existing Kotlin analyzer workflows.
type Baseline struct {
	ManuallySuppressed map[string]bool // IDs manually suppressed by user
	CurrentIssues      map[string]bool // IDs from last run (auto-generated)

	// detektBudget counts imported detekt entries per (rule, file name). Each
	// entry silences one finding of the rule in a file with that name.
	detektBudget map[detektKey]int
}

type detektKey struct{ rule, file string }

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
	b.indexDetektEntries()
	return b, nil
}

// parseDetektBaselineID splits a detekt baseline ID, `Rule:File.kt$Scope$sig`,
// into its rule and file name. Krit IDs, `Rule:File.kt:$Rule$msg`, put a colon
// before the first `$` and are rejected.
func parseDetektBaselineID(id string) (rule, file string, ok bool) {
	colon := strings.IndexByte(id, ':')
	if colon <= 0 {
		return "", "", false
	}
	rest := id[colon+1:]
	dollar := strings.IndexByte(rest, '$')
	if dollar <= 0 {
		return "", "", false
	}
	if c := strings.IndexByte(rest, ':'); c >= 0 && c < dollar {
		return "", "", false
	}
	file = rest[:dollar]
	if !strings.Contains(file, ".") || strings.ContainsAny(file, " /\\") {
		return "", "", false
	}
	return id[:colon], file, true
}

// indexDetektEntries builds the per-(rule, file) budget from detekt-format
// IDs, also crediting the krit rules that replaced a renamed detekt rule.
// Krit-format IDs of a removed or renamed krit rule (config.LegacyRuleIDs)
// credit the rule that replaced it the same way, because the new rule's ID
// and message no longer match the old entry.
func (b *Baseline) indexDetektEntries() {
	renames := config.DetektRuleRenames()
	legacy := config.LegacyRuleIDs()
	credit := func(rule, file string) {
		if b.detektBudget == nil {
			b.detektBudget = map[detektKey]int{}
		}
		b.detektBudget[detektKey{rule, file}]++
	}
	count := func(id string) {
		rule, file, ok := parseDetektBaselineID(id)
		if !ok {
			if entry := ParseBaselineEntry("", id); entry.Path != "" {
				if current, renamed := legacy[entry.Rule]; renamed {
					credit(current, filepath.Base(entry.Path))
				}
			}
			return
		}
		credit(rule, file)
		for _, target := range renames[rule] {
			credit(target, file)
		}
	}
	for id := range b.ManuallySuppressed {
		count(id)
	}
	for id := range b.CurrentIssues {
		count(id)
	}
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
	namer := newBaselineIDs(basePath)
	for i := 0; i < columns.Len(); i++ {
		id := namer.id(columns.FileAt(i), columns.RuleAt(i), columns.MessageAt(i), "")
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
	return fsutil.WriteFileAtomicKeepMode(path, append(header, data...), 0o644)
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
	var budget map[detektKey]int
	if len(baseline.detektBudget) > 0 {
		budget = make(map[detektKey]int, len(baseline.detektBudget))
		for k, v := range baseline.detektBudget {
			budget[k] = v
		}
	}
	namer, byName := newBaselineIDs(basePath), newBaselineIDs("")
	return columns.FilterRows(func(row int) bool {
		file, rule, message := columns.FileAt(row), columns.RuleAt(row), columns.MessageAt(row)
		if baseline.Contains(namer.id(file, rule, message, "")) {
			return false
		}
		if basePath != "" && baseline.Contains(byName.id(file, rule, message, "")) {
			return false
		}
		if baselineMatchesRuleAlias(columns, row, baseline, namer, byName) {
			return false
		}
		if budget != nil {
			key := detektKey{columns.RuleAt(row), filepath.Base(columns.FileAt(row))}
			if budget[key] > 0 {
				budget[key]--
				return false
			}
		}
		return true
	})
}

// BaselineRuleAliases, when set, returns the former IDs of a rule. A baseline entry recorded under a former ID keeps
// silencing the finding after the rule is renamed or the finding moves to another rule. The rules package installs
// it.
var BaselineRuleAliases func(rule string) []string

// baselineMatchesRuleAlias reports whether the baseline holds the row's finding under one of its rule's former IDs.
func baselineMatchesRuleAlias(columns *FindingColumns, row int, baseline *Baseline, namer, byName *baselineIDs) bool {
	if BaselineRuleAliases == nil {
		return false
	}
	file, message := columns.FileAt(row), columns.MessageAt(row)
	for _, alias := range BaselineRuleAliases(columns.RuleAt(row)) {
		if baseline.Contains(namer.id(file, alias, message, "")) {
			return true
		}
		if namer.base != "" && baseline.Contains(byName.id(file, alias, message, "")) {
			return true
		}
	}
	return false
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
	return newBaselineIDs(basePath).id(file, rule, message, signature)
}

// baselineIDs builds baseline IDs for one base path. A finding's file is
// named relative to the base, with both made absolute first: finding paths
// are absolute on some code paths (the daemon, cached findings, FIR results,
// manifests) and relative on others, and filepath.Rel fails when exactly one
// side is absolute. Without that, the same finding got a repository-path ID
// in one run mode and a base-name ID in another (#1600).
type baselineIDs struct {
	base     string // absolute, cleaned; "" names files by base name
	realBase string // base with symlinks resolved, computed on first need
	realOnce sync.Once
	cwd      string
	cwdOnce  sync.Once
}

func newBaselineIDs(basePath string) *baselineIDs {
	b := &baselineIDs{}
	if basePath != "" {
		if abs, err := filepath.Abs(basePath); err == nil {
			b.base = abs
		} else {
			b.base = filepath.Clean(basePath)
		}
	}
	return b
}

func (b *baselineIDs) id(file, rule, message, signature string) string {
	if signature == "" {
		signature = fmt.Sprintf("$%s$%s", rule, message)
	}
	return fmt.Sprintf("%s:%s:%s", rule, b.fileName(file), signature)
}

// fileName is file's path relative to the base, or its base name when no
// base is set or the two cannot be related (different Windows volumes).
func (b *baselineIDs) fileName(file string) string {
	if b.base == "" {
		return filepath.Base(file)
	}
	abs := file
	if !filepath.IsAbs(abs) {
		b.cwdOnce.Do(func() { b.cwd, _ = os.Getwd() })
		abs = filepath.Join(b.cwd, file)
	}
	rel, err := filepath.Rel(b.base, abs)
	if err == nil && escapesBase(rel) {
		// One side may be spelled through a symlink (/var vs /private/var
		// on macOS); compare the resolved paths before calling the file
		// outside the base.
		b.realOnce.Do(func() { b.realBase, _ = filepath.EvalSymlinks(b.base) })
		if realFile, ferr := filepath.EvalSymlinks(abs); ferr == nil && b.realBase != "" {
			if realRel, rerr := filepath.Rel(b.realBase, realFile); rerr == nil && !escapesBase(realRel) {
				rel = realRel
			}
		}
	}
	if err != nil {
		warnBaselineBaseName(file, b.base, err)
		return filepath.Base(file)
	}
	return rel
}

func escapesBase(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

var baselineWarnOnce sync.Once

// warnBaselineBaseName reports, once per process, a finding whose baseline
// ID falls back to its base name because it cannot be named relative to the
// base path.
func warnBaselineBaseName(file, base string, err error) {
	baselineWarnOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "warning: baseline IDs: %s is not relative to --base-path %s (%v); naming it by file name\n", file, base, err)
	})
}
