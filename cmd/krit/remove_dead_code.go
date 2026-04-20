package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kaeawc/krit/internal/deadcode"
	"github.com/kaeawc/krit/internal/scanner"
)

type deadCodeRemovalReport struct {
	Action  string           `json:"action"`
	Summary deadcode.Summary `json:"summary"`
	Errors  []string         `json:"errors,omitempty"`
}

func runDeadCodeRemovalColumns(columns *scanner.FindingColumns, format string, dryRun bool, suffix string) int {
	plan := deadcode.BuildPlanColumns(columns)
	summary := plan.Summary()

	if dryRun {
		return emitDeadCodeRemovalReport(format, deadCodeRemovalReport{
			Action:  "dry-run",
			Summary: summary,
		})
	}

	result := plan.Apply(suffix)
	appliedSummary := summary
	appliedSummary.Declarations = result.Declarations
	appliedSummary.Files = result.Files

	var errors []string
	for _, err := range result.Errors {
		errors = append(errors, err.Error())
	}

	exitCode := 0
	if len(errors) > 0 {
		exitCode = 2
	}

	report := deadCodeRemovalReport{
		Action:  "apply",
		Summary: appliedSummary,
		Errors:  errors,
	}
	if emitCode := emitDeadCodeRemovalReport(format, report); emitCode != 0 {
		return emitCode
	}
	return exitCode
}

func emitDeadCodeRemovalReport(format string, report deadCodeRemovalReport) int {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode dead-code removal JSON: %v\n", err)
			return 2
		}
		return 0
	case "plain":
		printDeadCodeRemovalPlain(report)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: --remove-dead-code supports only plain or json output, got %s\n", format)
		return 2
	}
}

func printDeadCodeRemovalPlain(report deadCodeRemovalReport) {
	switch report.Action {
	case "dry-run":
		fmt.Println("Dead code removal dry-run")
	default:
		fmt.Println("Dead code removal")
	}

	switch {
	case report.Summary.Declarations == 0 && report.Action == "dry-run":
		fmt.Println("No directly removable dead code findings.")
	case report.Summary.Declarations == 0:
		fmt.Println("No dead code was removed.")
	case report.Action == "dry-run":
		fmt.Printf("Would remove %s across %s", plural(report.Summary.Declarations, "declaration", "declarations"), plural(report.Summary.Files, "file", "files"))
		if breakdown := formatKindBreakdown(report.Summary.Kinds); breakdown != "" {
			fmt.Printf(" (%s)", breakdown)
		}
		fmt.Println(".")
	default:
		fmt.Printf("Removed %s across %s", plural(report.Summary.Declarations, "declaration", "declarations"), plural(report.Summary.Files, "file", "files"))
		if breakdown := formatKindBreakdown(report.Summary.Kinds); breakdown != "" {
			fmt.Printf(" (%s)", breakdown)
		}
		fmt.Println(".")
	}

	if report.Summary.Blocked > 0 {
		fmt.Printf("Skipped %s not yet safely removable", plural(report.Summary.Blocked, "dead-code finding", "dead-code findings"))
		if reasons := formatReasonBreakdown(report.Summary.Reasons); reasons != "" {
			fmt.Printf(" (%s)", reasons)
		}
		fmt.Println(".")
	}

	for _, err := range report.Errors {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
}

func formatKindBreakdown(kinds []deadcode.KindCount) string {
	if len(kinds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, plural(kind.Count, kind.Kind, kind.Kind+"s"))
	}
	return strings.Join(parts, ", ")
}

func formatReasonBreakdown(reasons []deadcode.ReasonCount) string {
	if len(reasons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, fmt.Sprintf("%d %s", reason.Count, reason.Reason))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
