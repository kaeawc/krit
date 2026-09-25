package firchecks

import (
	"slices"

	"github.com/kaeawc/krit/internal/scanner"
)

// Every active catalog ID is sent to the jar. Discovery in the jar decides
// which rules exist; all files are candidates because Go has no checker index.
type FirActiveRules struct{ Names []string }

func ActiveFirRules(enabledRuleIDs []string, _ bool) FirActiveRules {
	ids := slices.Clone(enabledRuleIDs)
	slices.Sort(ids)
	return FirActiveRules{Names: slices.Compact(ids)}
}

type FirFilterSummary struct {
	TotalFiles  int
	MarkedFiles int
	AllFiles    bool
	Paths       []string
}

func CollectFirCheckFiles(files []*scanner.File) FirFilterSummary {
	return FirFilterSummary{TotalFiles: len(files), MarkedFiles: len(files), AllFiles: true}
}
