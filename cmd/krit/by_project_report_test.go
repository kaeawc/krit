package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type projectCounts struct {
	Findings int `json:"findings"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// A whole-build scan with a Gradle model attributes each finding to the project
// whose sources hold it (#1635): JSON summary.byProject and findings[].project,
// SARIF runs[0].properties.byProject and results[].properties.project.
func TestReportsAttributeFindingsToGradleProjects(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := "package demo\n\nfun example() {\n    val x = 1\n    val y = 2\n}\n"
	write("settings.gradle.kts", `include(":app", ":lib")`+"\n")
	write("app/src/main/kotlin/demo/App.kt", source)
	write("lib/src/main/kotlin/demo/Lib.kt", "package demo\n\nfun lib() {\n    val z = 3\n}\n")
	write("scripts/Tool.kt", "package demo\n\nfun tool() {\n    val w = 4\n}\n")
	for _, project := range []string{"app", "lib"} {
		dir := filepath.ToSlash(filepath.Join(root, project))
		model, err := json.Marshal(map[string]any{"schema": 2, "projects": []any{map[string]any{
			"path": ":" + project, "dir": dir,
			"sourceSets": []any{map[string]any{"name": "main", "kind": "main", "sourceDirs": []string{dir + "/src/main/kotlin"}}},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		write(".krit/gradle-model/"+project+".json", string(model))
	}

	out := t.TempDir()
	jsonReport, sarifReport := filepath.Join(out, "krit.json"), filepath.Join(out, "krit.sarif")
	base := []string{"--no-daemon", "--no-cache", "--no-type-inference", "--no-type-oracle", "--no-fir",
		"--enable-rules", "UnusedVariable", "--disable-rules", "InvalidPackageDeclaration",
		"--report", "json=" + jsonReport, "--report", "sarif=" + sarifReport}
	if _, stderr, code := runKrit(t, append(base, root)...); code != 1 {
		t.Fatalf("exit %d, want 1 (findings); stderr:\n%s", code, stderr)
	}
	want := map[string]projectCounts{
		":app": {Findings: 2, Warnings: 2},
		":lib": {Findings: 1, Warnings: 1},
		"":     {Findings: 1, Warnings: 1},
	}

	var report struct {
		Findings []struct {
			File    string `json:"file"`
			Project string `json:"project"`
		} `json:"findings"`
		Summary struct {
			ByProject map[string]projectCounts `json:"byProject"`
		} `json:"summary"`
	}
	readJSON(t, jsonReport, &report)
	if !reflect.DeepEqual(report.Summary.ByProject, want) {
		data, _ := os.ReadFile(jsonReport)
		t.Fatalf("summary.byProject = %v, want %v\n%s", report.Summary.ByProject, want, data)
	}
	byFile := map[string]string{}
	for _, f := range report.Findings {
		byFile[filepath.ToSlash(f.File)] = f.Project
	}
	wantFiles := map[string]string{"app/src/main/kotlin/demo/App.kt": ":app", "lib/src/main/kotlin/demo/Lib.kt": ":lib", "scripts/Tool.kt": ""}
	if !reflect.DeepEqual(byFile, wantFiles) {
		t.Fatalf("finding projects = %v, want %v", byFile, wantFiles)
	}

	var sarif struct {
		Runs []struct {
			Results []struct {
				Properties struct {
					Project string `json:"project"`
				} `json:"properties"`
			} `json:"results"`
			Properties struct {
				ByProject map[string]projectCounts `json:"byProject"`
			} `json:"properties"`
		} `json:"runs"`
	}
	readJSON(t, sarifReport, &sarif)
	if len(sarif.Runs) != 1 || !reflect.DeepEqual(sarif.Runs[0].Properties.ByProject, want) {
		t.Fatalf("runs[0].properties.byProject = %+v, want %v", sarif.Runs, want)
	}
	attributed := 0
	for _, r := range sarif.Runs[0].Results {
		if r.Properties.Project != "" {
			attributed++
		}
	}
	if attributed != 3 {
		t.Fatalf("%d SARIF results carry a project, want 3", attributed)
	}

	// Without a model there is nothing to attribute.
	if _, stderr, code := runKrit(t, append(append([]string{"--no-gradle-model"}, base...), root)...); code != 1 {
		t.Fatalf("--no-gradle-model: exit %d; stderr:\n%s", code, stderr)
	}
	var plain struct {
		Summary map[string]json.RawMessage `json:"summary"`
	}
	readJSON(t, jsonReport, &plain)
	if _, ok := plain.Summary["byProject"]; ok {
		t.Fatalf("summary.byProject present without a model: %s", plain.Summary["byProject"])
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, data)
	}
}
