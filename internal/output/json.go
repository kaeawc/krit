package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

	// Build fix-level lookup
	fixLevels := make(map[string]string)
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			fixLevels[r.ID] = lvl.String()
		}
	}

	byRuleSet := make(map[string]int)
	byRule := make(map[string]int)
	fixableCount := 0
	findingsJSON, err := buildJSONFindings(columns, fixLevels, byRuleSet, byRule, &fixableCount, indent)
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
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
		Summary: JSONSummary{
			Total:     cols.Len(),
			ByRuleSet: byRuleSet,
			ByRule:    byRule,
			Fixable:   fixableCount,
		},
		Cache:       cacheStats,
		Caches:      caches,
		CacheBudget: cacheBudget,
		PerfTiming:  perfTimings,
	}
	if len(perfRuleStats) > 0 {
		report.PerfRules = perfRuleStats[0]
	}

	enc := json.NewEncoder(w)
	if indent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(report)
}

func buildJSONFindings(columns *scanner.FindingColumns, fixLevels map[string]string, byRuleSet, byRule map[string]int, fixableCount *int, indent bool) (json.RawMessage, error) {
	cols := normalizedFindingColumns(columns)
	if cols.Len() == 0 {
		return json.RawMessage("[]"), nil
	}

	var buf bytes.Buffer
	var marshalErr error
	buf.WriteByte('[')
	if indent {
		buf.WriteByte('\n')
	}
	first := true
	cols.VisitSortedByFileLine(func(row int) {
		if marshalErr != nil {
			return
		}
		if !first {
			buf.WriteByte(',')
			if indent {
				buf.WriteByte('\n')
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
		byRuleSet[ruleSet]++
		byRule[rule]++

		encoded, err := json.Marshal(finding)
		if err != nil {
			marshalErr = err
			return
		}
		if indent {
			buf.WriteString("    ")
		}
		buf.Write(encoded)
	})
	if marshalErr != nil {
		return nil, marshalErr
	}
	if indent {
		buf.WriteString("\n  ")
	}
	buf.WriteByte(']')
	return json.RawMessage(buf.Bytes()), nil
}
