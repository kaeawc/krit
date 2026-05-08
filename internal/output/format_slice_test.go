package output

import (
	"io"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Test-only slice-taking wrappers over the columnar formatters. Production
// code always provides FindingColumns directly; these helpers exist so the
// existing test corpus keeps exercising the formatters with literal
// []scanner.Finding fixtures.

func FormatPlain(w io.Writer, findings []scanner.Finding) {
	columns := scanner.CollectFindings(findings)
	FormatPlainColumns(w, &columns)
}

func FormatSARIF(w io.Writer, findings []scanner.Finding, version string) error {
	columns := scanner.CollectFindings(findings)
	return FormatSARIFColumns(w, &columns, version)
}

func FormatCheckstyle(w io.Writer, findings []scanner.Finding) {
	columns := scanner.CollectFindings(findings)
	FormatCheckstyleColumns(w, &columns)
}

func FormatJSON(w io.Writer, findings []scanner.Finding, version string,
	fileCount, ruleCount int, start time.Time,
	perfTimings []perf.TimingEntry, activeRules []*api.Rule,
	experiments []string,
	cacheStats *cache.Stats) error {
	columns := scanner.CollectFindings(findings)
	return FormatJSONColumns(w, &columns, version, fileCount, ruleCount, start, perfTimings, activeRules, experiments, cacheStats, nil, nil)
}
