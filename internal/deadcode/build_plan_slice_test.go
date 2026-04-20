package deadcode

import "github.com/kaeawc/krit/internal/scanner"

// BuildPlan is a test-only slice-taking wrapper over BuildPlanColumns.
// Production code constructs the plan directly from FindingColumns via
// cmd/krit/remove_dead_code.go.
func BuildPlan(findings []scanner.Finding) Plan {
	columns := scanner.CollectFindings(findings)
	return BuildPlanColumns(&columns)
}
