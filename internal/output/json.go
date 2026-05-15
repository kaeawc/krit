package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// JSONReport is the top-level JSON output.
type JSONReport struct {
	Success     bool                        `json:"success"`
	Version     string                      `json:"version"`
	DurationMs  int64                       `json:"durationMs"`
	Files       int                         `json:"files"`
	Rules       int                         `json:"rules"`
	Experiments []string                    `json:"experiments,omitempty"`
	Findings    []JSONFinding               `json:"findings"`
	Summary     JSONSummary                 `json:"summary"`
	Cache       *cache.Stats                `json:"cache,omitempty"`
	Caches      []cacheutil.NamedCacheStats `json:"caches,omitempty"`
	CacheBudget *cacheutil.BudgetReport     `json:"cacheBudget,omitempty"`
	PerfTiming  []perf.TimingEntry          `json:"perfTiming,omitempty"`
	PerfRules   []rules.RuleExecutionStat   `json:"perfRuleStats,omitempty"`
}

// JSONFinding is a single finding in JSON output.
type JSONFinding struct {
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Column     int     `json:"column"`
	StartByte  *int    `json:"startByte,omitempty"`
	EndByte    *int    `json:"endByte,omitempty"`
	RuleSet    string  `json:"ruleSet"`
	Rule       string  `json:"rule"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Fixable    bool    `json:"fixable"`
	FixLevel   string  `json:"fixLevel,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Effort     string  `json:"effort,omitempty"`
}

// JSONSummary summarizes findings.
type JSONSummary struct {
	Total     int            `json:"total"`
	ByRuleSet map[string]int `json:"byRuleSet"`
	ByRule    map[string]int `json:"byRule"`
	Fixable   int            `json:"fixable"`
}

// FormatJSONColumns writes columnar findings as JSON with two-space
// indentation. CLI consumers see the indented form.
func FormatJSONColumns(w io.Writer, columns *scanner.FindingColumns, version string,
	fileCount, ruleCount int, start time.Time,
	perfTimings []perf.TimingEntry, activeRules []*api.Rule,
	experiments []string,
	cacheStats *cache.Stats,
	caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport,
	perfRuleStats ...[]rules.RuleExecutionStat) error {
	return formatJSONColumnsImpl(w, columns, version, fileCount, ruleCount, start,
		perfTimings, activeRules, experiments, cacheStats, caches, cacheBudget, true,
		perfRuleStats...)
}

// FormatJSONColumnsCompact writes columnar findings as JSON with no
// indentation or internal newlines. The daemon's streaming response
// path uses this so the wire-level JSON sits cleanly inside the
// line-delimited daemon protocol (a single embedded '\n' would let
// the client's bufio.Reader.ReadBytes('\n') return a truncated body).
func FormatJSONColumnsCompact(w io.Writer, columns *scanner.FindingColumns, version string,
	fileCount, ruleCount int, start time.Time,
	perfTimings []perf.TimingEntry, activeRules []*api.Rule,
	experiments []string,
	cacheStats *cache.Stats,
	caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport,
	perfRuleStats ...[]rules.RuleExecutionStat) error {
	return formatJSONColumnsImpl(w, columns, version, fileCount, ruleCount, start,
		perfTimings, activeRules, experiments, cacheStats, caches, cacheBudget, false,
		perfRuleStats...)
}

func formatJSONColumnsImpl(w io.Writer, columns *scanner.FindingColumns, version string,
	fileCount, ruleCount int, start time.Time,
	perfTimings []perf.TimingEntry, activeRules []*api.Rule,
	experiments []string,
	cacheStats *cache.Stats,
	caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport,
	indent bool,
	perfRuleStats ...[]rules.RuleExecutionStat) error {

	cols := normalizedFindingColumns(columns)

	// Build fix-level and effort lookups
	fixLevels := make(map[string]string)
	efforts := make(map[string]string)
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			fixLevels[r.ID] = lvl.String()
		}
		if e := rules.V2RuleEffort(r); e != api.EffortUnset {
			efforts[r.ID] = e.String()
		}
	}

	byRuleSet := make(map[string]int)
	byRule := make(map[string]int)
	fixableCount := 0
	findingsJSON := buildJSONFindings(columns, fixLevels, efforts, byRuleSet, byRule, &fixableCount, indent)

	summary := JSONSummary{
		Total:     cols.Len(),
		ByRuleSet: byRuleSet,
		ByRule:    byRule,
		Fixable:   fixableCount,
	}
	var perfRuleStatsArg []rules.RuleExecutionStat
	if len(perfRuleStats) > 0 {
		perfRuleStatsArg = perfRuleStats[0]
	}

	// Compact mode (the daemon's wire format) bypasses json.Encoder so
	// the 30 MB findings RawMessage doesn't get re-scanned through
	// encoding/json.appendCompact — which dominated CPU on the warm
	// baseline before this fast path. Indented mode keeps the
	// Encoder route since CLI consumers see that form and the
	// human-facing extra escapes / indentation tradeoff isn't worth
	// hand-rolling a second time.
	if !indent {
		return writeCompactReport(w,
			cols.Len() == 0,
			version,
			time.Since(start).Milliseconds(),
			fileCount,
			ruleCount,
			experiments,
			findingsJSON,
			summary,
			cacheStats,
			caches,
			cacheBudget,
			perfTimings,
			perfRuleStatsArg,
		)
	}

	report := struct {
		Success     bool                        `json:"success"`
		Version     string                      `json:"version"`
		DurationMs  int64                       `json:"durationMs"`
		Files       int                         `json:"files"`
		Rules       int                         `json:"rules"`
		Experiments []string                    `json:"experiments,omitempty"`
		Findings    json.RawMessage             `json:"findings"`
		Summary     JSONSummary                 `json:"summary"`
		Cache       *cache.Stats                `json:"cache,omitempty"`
		Caches      []cacheutil.NamedCacheStats `json:"caches,omitempty"`
		CacheBudget *cacheutil.BudgetReport     `json:"cacheBudget,omitempty"`
		PerfTiming  []perf.TimingEntry          `json:"perfTiming,omitempty"`
		PerfRules   []rules.RuleExecutionStat   `json:"perfRuleStats,omitempty"`
	}{
		Success:     cols.Len() == 0,
		Version:     version,
		DurationMs:  time.Since(start).Milliseconds(),
		Files:       fileCount,
		Rules:       ruleCount,
		Experiments: append([]string(nil), experiments...),
		Findings:    findingsJSON,
		Summary:     summary,
		Cache:       cacheStats,
		Caches:      caches,
		CacheBudget: cacheBudget,
		PerfTiming:  perfTimings,
		PerfRules:   perfRuleStatsArg,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// writeCompactReport assembles the report envelope directly into a
// byte buffer without routing through json.Encoder, so the 30 MB
// findings RawMessage doesn't get re-scanned through
// encoding/json.appendCompact. Small sub-objects (summary, caches,
// perfTiming) still go through json.Marshal — they're small enough
// that the reflection overhead doesn't matter, and avoiding hand-
// rolled encoders for every nested type keeps the schema in sync
// with the struct tags.
func writeCompactReport(w io.Writer, success bool, version string, durationMs int64,
	fileCount, ruleCount int, experiments []string,
	findingsJSON json.RawMessage, summary JSONSummary,
	cacheStats *cache.Stats, caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport, perfTimings []perf.TimingEntry,
	perfRuleStatsArg []rules.RuleExecutionStat) error {
	buf := make([]byte, 0, len(findingsJSON)+512)

	buf = append(buf, `{"success":`...)
	if success {
		buf = append(buf, "true"...)
	} else {
		buf = append(buf, "false"...)
	}

	buf = append(buf, `,"version":`...)
	buf = appendJSONString(buf, version)

	buf = append(buf, `,"durationMs":`...)
	buf = strconv.AppendInt(buf, durationMs, 10)

	buf = append(buf, `,"files":`...)
	buf = strconv.AppendInt(buf, int64(fileCount), 10)

	buf = append(buf, `,"rules":`...)
	buf = strconv.AppendInt(buf, int64(ruleCount), 10)

	if len(experiments) > 0 {
		expBytes, err := json.Marshal(experiments)
		if err != nil {
			return fmt.Errorf("marshal experiments: %w", err)
		}
		buf = append(buf, `,"experiments":`...)
		buf = append(buf, expBytes...)
	}

	buf = append(buf, `,"findings":`...)
	buf = append(buf, findingsJSON...)

	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	buf = append(buf, `,"summary":`...)
	buf = append(buf, summaryBytes...)

	if cacheStats != nil {
		csBytes, err := json.Marshal(cacheStats)
		if err != nil {
			return fmt.Errorf("marshal cache: %w", err)
		}
		buf = append(buf, `,"cache":`...)
		buf = append(buf, csBytes...)
	}
	if len(caches) > 0 {
		cBytes, err := json.Marshal(caches)
		if err != nil {
			return fmt.Errorf("marshal caches: %w", err)
		}
		buf = append(buf, `,"caches":`...)
		buf = append(buf, cBytes...)
	}
	if cacheBudget != nil {
		cbBytes, err := json.Marshal(cacheBudget)
		if err != nil {
			return fmt.Errorf("marshal cacheBudget: %w", err)
		}
		buf = append(buf, `,"cacheBudget":`...)
		buf = append(buf, cbBytes...)
	}
	if len(perfTimings) > 0 {
		ptBytes, err := json.Marshal(perfTimings)
		if err != nil {
			return fmt.Errorf("marshal perfTiming: %w", err)
		}
		buf = append(buf, `,"perfTiming":`...)
		buf = append(buf, ptBytes...)
	}
	if len(perfRuleStatsArg) > 0 {
		prBytes, err := json.Marshal(perfRuleStatsArg)
		if err != nil {
			return fmt.Errorf("marshal perfRuleStats: %w", err)
		}
		buf = append(buf, `,"perfRuleStats":`...)
		buf = append(buf, prBytes...)
	}
	buf = append(buf, '}', '\n')
	_, werr := w.Write(buf)
	return werr
}

// CompactReport is the public input for WriteCompactReport — a
// struct wrapper so callers don't have to track positional order
// across 13 args. The daemon's bundle-hit fast path is the only
// external consumer today.
type CompactReport struct {
	Success      bool
	Version      string
	DurationMs   int64
	FileCount    int
	RuleCount    int
	Experiments  []string
	FindingsJSON []byte
	Summary      JSONSummary
	Caches       []cacheutil.NamedCacheStats
	CacheBudget  *cacheutil.BudgetReport
	PerfTimings  []perf.TimingEntry
}

// WriteCompactReport is the package-public entry to the compact
// envelope writer. Used by the daemon's bundle-hit fast path so a
// freshly-formatted (or cached) findings byte slice can be wrapped
// in the same JSON envelope FormatJSONColumnsCompact produces —
// keeps the wire format identical regardless of which route the
// daemon takes.
func WriteCompactReport(w io.Writer, r CompactReport) error {
	return writeCompactReport(w, r.Success, r.Version, r.DurationMs,
		r.FileCount, r.RuleCount, r.Experiments,
		json.RawMessage(r.FindingsJSON), r.Summary,
		nil, r.Caches, r.CacheBudget, r.PerfTimings, nil)
}

// BuildFindingsArrayCompact returns the formatted findings array
// bytes (a JSON array, no surrounding envelope) for the daemon's
// bundle-hit cache. Byte-identical to the "findings" payload
// FormatJSONColumnsCompact emits — the caller can pair these with
// a fresh envelope on every reuse, paying the ~25 ms format cost
// just once per bundle key.
func BuildFindingsArrayCompact(columns *scanner.FindingColumns,
	fixLevels, efforts map[string]string,
	byRuleSet, byRule map[string]int,
	fixableCount *int,
) []byte {
	return []byte(buildJSONFindings(columns, fixLevels, efforts, byRuleSet, byRule, fixableCount, false))
}

func buildJSONFindings(columns *scanner.FindingColumns, fixLevels, efforts map[string]string, byRuleSet, byRule map[string]int, fixableCount *int, indent bool) json.RawMessage {
	cols := normalizedFindingColumns(columns)
	if cols.Len() == 0 {
		return json.RawMessage("[]")
	}

	// Pre-size for ~150 B / finding (the kotlin-corpus warm baseline
	// averages ~140 B; a small over-estimate avoids the geometric
	// re-grow chain a bytes.Buffer would otherwise pay on 87 k
	// AppendByte calls).
	buf := make([]byte, 0, cols.Len()*150)
	buf = append(buf, '[')
	if indent {
		buf = append(buf, '\n')
	}
	first := true
	cols.VisitSortedByFileLine(func(row int) {
		if !first {
			buf = append(buf, ',')
			if indent {
				buf = append(buf, '\n')
			}
		}
		first = false

		ruleSet := cols.RuleSetAt(row)
		rule := cols.RuleAt(row)
		isFixable := cols.HasFix(row)
		var startByte, endByte *int
		start, end := cols.StartByteAt(row), cols.EndByteAt(row)
		if end > start {
			startByte = &start
			endByte = &end
		}
		finding := JSONFinding{
			File:       cols.FileAt(row),
			Line:       cols.LineAt(row),
			Column:     cols.ColumnAt(row),
			StartByte:  startByte,
			EndByte:    endByte,
			RuleSet:    ruleSet,
			Rule:       rule,
			Severity:   cols.SeverityAt(row),
			Message:    cols.MessageAt(row),
			Fixable:    isFixable,
			Confidence: cols.ConfidenceAt(row),
		}
		if isFixable {
			finding.FixLevel = fixLevels[rule]
			*fixableCount = *fixableCount + 1
		}
		finding.Effort = efforts[rule]
		byRuleSet[ruleSet]++
		byRule[rule]++

		if indent {
			buf = append(buf, ' ', ' ', ' ', ' ')
		}
		// Hand-rolled per-finding encoder: byte-identical to
		// json.Marshal(finding) but ~10x faster (no reflection,
		// no intermediate buffer allocations). Pinned by
		// TestAppendFindingJSON_MatchesJSONMarshal.
		buf = appendFindingJSON(buf, finding)
	})
	if indent {
		buf = append(buf, '\n', ' ', ' ')
	}
	buf = append(buf, ']')
	return json.RawMessage(buf)
}
