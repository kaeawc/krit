package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

func projectFixture() (scanner.FindingColumns, *ProjectAttribution) {
	cols := scanner.CollectFindings([]scanner.Finding{
		{File: "app/src/main/kotlin/A.kt", Line: 1, Col: 1, RuleSet: "style", Rule: "R1", Severity: "error", Message: "m"},
		{File: "app/src/main/kotlin/A.kt", Line: 2, Col: 1, RuleSet: "style", Rule: "R2", Severity: "warning", Message: "m"},
		{File: "lib/src/main/kotlin/B.kt", Line: 3, Col: 1, RuleSet: "style", Rule: "R1", Severity: "info", Message: "m"},
		{File: "build.gradle.kts", Line: 4, Col: 1, RuleSet: "style", Rule: "R3", Severity: "warning", Message: "m"},
	})
	projects := NewProjectAttribution(map[string]string{
		"app/src/main/kotlin/A.kt": ":app",
		"lib/src/main/kotlin/B.kt": ":lib",
	}, []string{":app", ":empty", ":lib"})
	return cols, projects
}

var wantByProject = map[string]ProjectCounts{
	":app":   {Findings: 2, Errors: 1, Warnings: 1},
	":lib":   {Findings: 1},
	":empty": {},
	"":       {Findings: 1, Warnings: 1},
}

func TestProjectAttributionInJSON(t *testing.T) {
	cols, projects := projectFixture()
	for _, compact := range []bool{false, true} {
		var buf bytes.Buffer
		if err := FormatJSONColumnsWithOptions(&buf, &cols, "v", 0, 0, time.Time{}, nil, nil, nil, nil, nil, nil,
			JSONOptions{Projects: projects, Compact: compact}); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Findings []struct {
				File    string  `json:"file"`
				Project *string `json:"project"`
			} `json:"findings"`
			Summary struct {
				ByProject map[string]ProjectCounts `json:"byProject"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
			t.Fatalf("compact=%v: %v\n%s", compact, err, buf.String())
		}
		if !reflect.DeepEqual(report.Summary.ByProject, wantByProject) {
			t.Fatalf("compact=%v: summary.byProject = %v, want %v", compact, report.Summary.ByProject, wantByProject)
		}
		got := map[string]string{}
		for _, f := range report.Findings {
			if f.Project == nil {
				got[f.File] = "<absent>"
			} else {
				got[f.File] = *f.Project
			}
		}
		want := map[string]string{"app/src/main/kotlin/A.kt": ":app", "lib/src/main/kotlin/B.kt": ":lib", "build.gradle.kts": "<absent>"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("compact=%v: finding projects = %v, want %v", compact, got, want)
		}
	}

	var buf bytes.Buffer
	if err := FormatJSONColumnsWithOptions(&buf, &cols, "v", 0, 0, time.Time{}, nil, nil, nil, nil, nil, nil, JSONOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"byProject"`) || strings.Contains(buf.String(), `"project"`) {
		t.Fatalf("project fields without a model:\n%s", buf.String())
	}
}

func TestProjectAttributionInSARIF(t *testing.T) {
	cols, projects := projectFixture()
	var buf bytes.Buffer
	if err := FormatSARIFColumnsReport(&buf, &cols, "v", SARIFOptions{Projects: projects}); err != nil {
		t.Fatal(err)
	}
	var sarif struct {
		Runs []struct {
			Results []struct {
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				Properties struct {
					Project string `json:"project"`
				} `json:"properties"`
			} `json:"results"`
			Properties struct {
				ByProject map[string]ProjectCounts `json:"byProject"`
			} `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil || len(sarif.Runs) != 1 {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(sarif.Runs[0].Properties.ByProject, wantByProject) {
		t.Fatalf("runs[0].properties.byProject = %v, want %v", sarif.Runs[0].Properties.ByProject, wantByProject)
	}
	got := map[string]string{}
	for _, r := range sarif.Runs[0].Results {
		got[r.Locations[0].PhysicalLocation.ArtifactLocation.URI] = r.Properties.Project
	}
	want := map[string]string{"app/src/main/kotlin/A.kt": ":app", "lib/src/main/kotlin/B.kt": ":lib", "build.gradle.kts": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result projects = %v, want %v", got, want)
	}

	buf.Reset()
	if err := FormatSARIFColumnsReport(&buf, &cols, "v", SARIFOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"byProject"`) || strings.Contains(buf.String(), `"project"`) {
		t.Fatalf("project fields without a model:\n%s", buf.String())
	}
}
