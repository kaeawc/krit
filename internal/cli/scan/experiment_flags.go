package scan

import (
	"fmt"
	"io"

	"github.com/kaeawc/krit/internal/experiment"
)

// partitionDeprecatedExperiments splits names into those still active and
// those marked deprecated, in input order. Pure; callers inject
// experiment.IsDeprecated so the partitioning logic is unit-testable
// without depending on the live experiment catalog.
func partitionDeprecatedExperiments(names []string, isDeprecated func(string) bool) (kept, deprecated []string) {
	for _, name := range names {
		if isDeprecated(name) {
			deprecated = append(deprecated, name)
			continue
		}
		kept = append(kept, name)
	}
	return kept, deprecated
}

// applyExperimentFlags resolves the active experiment set for this run.
// User-enabled deprecated experiments produce a stderr-style warning
// (written to w) and are stripped before merging with defaults and
// user-disabled experiments. Calls experiment.SetCurrent on success.
func applyExperimentFlags(experimentCSV, experimentOffCSV string, w io.Writer) {
	userEnabled, deprecated := partitionDeprecatedExperiments(
		experiment.ParseCSV(experimentCSV),
		experiment.IsDeprecated,
	)
	for _, name := range deprecated {
		fmt.Fprintf(w, "warning: experiment %q is deprecated and will be ignored\n", name)
	}
	enabled := experiment.MergeEnabled(
		experiment.DefaultEnabled(),
		userEnabled,
		experiment.ParseCSV(experimentOffCSV),
	)
	experiment.SetCurrent(enabled)
}
