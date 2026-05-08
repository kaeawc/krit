package mcp

import "github.com/kaeawc/krit/internal/scanner"

// findingsToResult is a test-only slice-taking wrapper used by
// tools_columns_test.go to assert the columnar formatter matches the
// slice-materialized baseline. Production code calls findingsToResultColumns
// directly.
func findingsToResult(findings []scanner.Finding) ToolResult {
	columns := scanner.CollectFindings(findings)
	return findingsToResultColumns(&columns)
}
