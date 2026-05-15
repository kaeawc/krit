package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

const (
	sideDaemon   = "daemon"
	sideBaseline = "baseline"
)

// DivergenceLogDir is the per-repo directory under which strict-verify
// writes divergence logs. NextDivergenceLogPath produces sequenced file
// names within this directory.
const DivergenceLogDir = ".krit"

// NextDivergenceLogPath returns the next available
// `${repoDir}/.krit/daemon-divergence-NNNN.log` path. NNNN is a
// zero-padded four-digit sequence; on overflow (NNNN > 9999) the
// timestamp form `daemon-divergence-<unixNanos>.log` is used so we
// never collide silently. The directory is created on demand.
func NextDivergenceLogPath(repoDir string) (string, error) {
	dir := filepath.Join(repoDir, DivergenceLogDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("divergence: prepare log dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("divergence: read log dir: %w", err)
	}
	maxSeq := -1
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n, ok := parseDivergenceSeq(e.Name())
		if !ok {
			continue
		}
		if n > maxSeq {
			maxSeq = n
		}
	}
	next := maxSeq + 1
	if next > 9999 {
		return filepath.Join(dir, fmt.Sprintf("daemon-divergence-%d.log", time.Now().UnixNano())), nil
	}
	return filepath.Join(dir, fmt.Sprintf("daemon-divergence-%04d.log", next)), nil
}

func parseDivergenceSeq(name string) (int, bool) {
	const prefix = "daemon-divergence-"
	const suffix = ".log"
	mid, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return 0, false
	}
	mid, ok = strings.CutSuffix(mid, suffix)
	if !ok || len(mid) != 4 {
		return 0, false
	}
	n, err := strconv.Atoi(mid)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// Diff describes how a daemon analyze result differs from a freshly-computed
// in-process baseline. AddedByDaemon are rows the daemon emitted that the
// baseline did not; DroppedByDaemon are rows the baseline emitted that the
// daemon did not. PathsTouched lists the distinct files implicated by either
// side, sorted for stable logs.
//
// Equality is a multiset comparison on a structural key (file, line, col,
// rule set, rule, severity, message). Two findings sharing that key are
// considered the same row even if their byte offsets, confidence, or fix
// payloads differ — those are downstream rendering concerns and would
// otherwise cause noisy divergence reports across cosmetic refactors.
type Diff struct {
	AddedByDaemon   []scanner.Finding
	DroppedByDaemon []scanner.Finding
	PathsTouched    []string
}

// IsClean reports whether the daemon and baseline produced the same multiset
// of findings.
func (d Diff) IsClean() bool {
	return len(d.AddedByDaemon) == 0 && len(d.DroppedByDaemon) == 0
}

// Compare returns the multiset difference of daemonCols and baselineCols.
// Nil columns are treated as empty.
//
// The algorithm builds a single signed-count map (daemon contributes +1,
// baseline -1), then walks each side in sorted-by-file order, materializing
// a Finding only for rows where the running imbalance proves they are
// excess on that side. The common clean-run case allocates no Finding rows
// at all.
func Compare(daemonCols, baselineCols *scanner.FindingColumns) Diff {
	dN, bN := lenOf(daemonCols), lenOf(baselineCols)
	if dN == 0 && bN == 0 {
		return Diff{}
	}

	counts := make(map[findingKey]int, dN+bN)
	for row := 0; row < dN; row++ {
		counts[keyFromCols(daemonCols, row)]++
	}
	for row := 0; row < bN; row++ {
		counts[keyFromCols(baselineCols, row)]--
	}

	added := collectExcess(daemonCols, counts, +1)
	dropped := collectExcess(baselineCols, counts, -1)

	return Diff{
		AddedByDaemon:   added,
		DroppedByDaemon: dropped,
		PathsTouched:    pathsTouched(added, dropped),
	}
}

// WriteLog renders the diff as newline-delimited JSON: one object per row,
// daemon-added rows first, then baseline-dropped rows. Within each side rows
// are already in file/line order from Compare. The destination directory is
// created on demand. An empty diff produces a zero-byte file so presence-or
// -absence checks stay simple for CI.
func (d Diff) WriteLog(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("divergence: prepare log dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("divergence: create log: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range d.AddedByDaemon {
		if err := enc.Encode(divergenceRow(sideDaemon, row)); err != nil {
			return fmt.Errorf("divergence: write row: %w", err)
		}
	}
	for _, row := range d.DroppedByDaemon {
		if err := enc.Encode(divergenceRow(sideBaseline, row)); err != nil {
			return fmt.Errorf("divergence: write row: %w", err)
		}
	}
	return nil
}

type findingKey struct {
	File     string
	Line     int
	Col      int
	RuleSet  string
	Rule     string
	Severity string
	Message  string
}

func keyFromCols(cols *scanner.FindingColumns, row int) findingKey {
	return findingKey{
		File:     cols.FileAt(row),
		Line:     cols.LineAt(row),
		Col:      cols.ColumnAt(row),
		RuleSet:  cols.RuleSetAt(row),
		Rule:     cols.RuleAt(row),
		Severity: cols.SeverityAt(row),
		Message:  cols.MessageAt(row),
	}
}

func lenOf(cols *scanner.FindingColumns) int {
	if cols == nil {
		return 0
	}
	return cols.N
}

// collectExcess walks cols in sorted order and materializes a Finding for
// each row whose key has remaining imbalance in the direction given by sign
// (+1 means cols is the daemon side, -1 means cols is the baseline side).
// The map is mutated as rows are consumed so duplicate keys yield the right
// number of rows.
func collectExcess(cols *scanner.FindingColumns, counts map[findingKey]int, sign int) []scanner.Finding {
	if cols == nil || cols.N == 0 {
		return nil
	}
	var out []scanner.Finding
	cols.VisitSortedByFileLine(func(row int) {
		k := keyFromCols(cols, row)
		if sign > 0 && counts[k] > 0 {
			out = append(out, cols.Finding(row))
			counts[k]--
		} else if sign < 0 && counts[k] < 0 {
			out = append(out, cols.Finding(row))
			counts[k]++
		}
	})
	return out
}

func pathsTouched(added, dropped []scanner.Finding) []string {
	if len(added) == 0 && len(dropped) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(added)+len(dropped))
	for _, f := range added {
		seen[f.File] = struct{}{}
	}
	for _, f := range dropped {
		seen[f.File] = struct{}{}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// divergenceLogRow is the on-disk shape of a divergence log entry. Field
// names are short and stable so the log stays grep-friendly.
type divergenceLogRow struct {
	Side     string  `json:"side"`
	RuleSet  string  `json:"ruleSet"`
	Rule     string  `json:"rule"`
	File     string  `json:"file"`
	Line     int     `json:"line"`
	Col      int     `json:"col"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	Conf     float64 `json:"confidence,omitempty"`
}

func divergenceRow(side string, f scanner.Finding) divergenceLogRow {
	return divergenceLogRow{
		Side:     side,
		RuleSet:  f.RuleSet,
		Rule:     f.Rule,
		File:     f.File,
		Line:     f.Line,
		Col:      f.Col,
		Severity: f.Severity,
		Message:  f.Message,
		Conf:     f.Confidence,
	}
}
