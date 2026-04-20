package main

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// Test-only slice-taking wrappers over the columnar CLI entry points.
// Production callers always provide FindingColumns directly; these helpers
// exist so the existing columnar_cli_test.go fixtures can keep using
// []scanner.Finding literals.

func runRuleAudit(findings []scanner.Finding, opts ruleAuditOpts) int {
	columns := scanner.CollectFindings(findings)
	return runRuleAuditColumns(&columns, opts)
}

func runDeadCodeRemoval(findings []scanner.Finding, format string, dryRun bool, suffix string) int {
	columns := scanner.CollectFindings(findings)
	return runDeadCodeRemovalColumns(&columns, format, dryRun, suffix)
}

func runBaselineAudit(findings []scanner.Finding, baseline *scanner.Baseline, baselinePath, basePath string, scanPaths []string, format string) int {
	columns := scanner.CollectFindings(findings)
	return runBaselineAuditColumns(&columns, baseline, baselinePath, basePath, scanPaths, format)
}

func runSampleFindings(findings []scanner.Finding, ruleName string, count int, contextLines int, basePath string) int {
	columns := scanner.CollectFindings(findings)
	return runSampleFindingsColumns(&columns, ruleName, count, contextLines, basePath)
}
