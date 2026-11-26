package output

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// ProjectAttribution names the Gradle project of each finding's file, for a
// scan that covers several projects (the Gradle plugin's kritCheckAll, #1635).
// It is reported as each finding's `project` and as `byProject` counts: in
// JSON under summary.byProject, in SARIF as each result's properties.project
// and runs[0].properties.byProject. A finding whose file is in no project of
// the model counts under "".
type ProjectAttribution struct {
	byFile   map[string]string
	projects []string
}

// NewProjectAttribution maps report file paths (as findings spell them in the
// report) to project paths. Every one of projects appears in byProject, with
// zero counts when it has no findings.
func NewProjectAttribution(byFile map[string]string, projects []string) *ProjectAttribution {
	return &ProjectAttribution{byFile: byFile, projects: projects}
}

// ProjectOf returns the project of a report file path, or "".
func (a *ProjectAttribution) ProjectOf(file string) string {
	if a == nil {
		return ""
	}
	return a.byFile[file]
}

// ProjectCounts is one project's entry in byProject.
type ProjectCounts struct {
	Findings int `json:"findings"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// ByProject counts the findings of columns per project, or returns nil
// without an attribution.
func (a *ProjectAttribution) ByProject(columns *scanner.FindingColumns) map[string]ProjectCounts {
	if a == nil {
		return nil
	}
	out := make(map[string]ProjectCounts, len(a.projects))
	for _, project := range a.projects {
		out[project] = ProjectCounts{}
	}
	cols := normalizedFindingColumns(columns)
	for row := 0; row < cols.Len(); row++ {
		project := a.byFile[cols.FileAt(row)]
		counts := out[project]
		counts.Findings++
		switch cols.SeverityAt(row) {
		case "error":
			counts.Errors++
		case "warning":
			counts.Warnings++
		}
		out[project] = counts
	}
	return out
}
