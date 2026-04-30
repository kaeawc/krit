package main

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaSupportFixtures_JavaOnlyAndroidApp(t *testing.T) {
	project := "../../tests/fixtures/java-android-support/java-only-app"
	inventory := javaSupportFixtureInventory(t, project)
	if inventory.Java == 0 || inventory.Kotlin != 0 || inventory.Gradle == 0 || inventory.XML == 0 {
		t.Fatalf("unexpected java-only fixture inventory: %+v", inventory)
	}

	report := runJavaSupportFixture(t, project)
	for _, rule := range []string{
		"AddJavascriptInterface",
		"SetJavaScriptEnabled",
		"CommitPrefEdits",
		"CommitTransaction",
		"CheckResult",
		"BufferedReadWithoutBuffer",
		"CursorLoopWithColumnIndexInLoop",
	} {
		if report.Summary.ByRule[rule] == 0 {
			t.Fatalf("expected %s finding in Java-only support fixture; byRule=%v", rule, report.Summary.ByRule)
		}
		if report.javaFindingsByRule()[rule] == 0 {
			t.Fatalf("expected %s to target a Java source file", rule)
		}
	}
	if report.Summary.ByRule["HardcodedValuesResource"] == 0 {
		t.Fatalf("expected resource finding in Java-only support fixture; byRule=%v", report.Summary.ByRule)
	}
	if !report.hasPerfPath("crossFileAnalysis/javaIndexing") {
		t.Fatalf("expected Java indexing perf path, got %v", report.perfPaths())
	}
}

func TestJavaSupportFixtures_MixedAndroidApp(t *testing.T) {
	project := "../../tests/fixtures/java-android-support/mixed-app"
	inventory := javaSupportFixtureInventory(t, project)
	if inventory.Java == 0 || inventory.Kotlin == 0 || inventory.Gradle == 0 || inventory.XML == 0 {
		t.Fatalf("unexpected mixed fixture inventory: %+v", inventory)
	}

	report := runJavaSupportFixture(t, project)
	if report.Files < inventory.Java+inventory.Kotlin {
		t.Fatalf("report files=%d, want at least Java+Kotlin count from fixture inventory %+v", report.Files, inventory)
	}
	if report.javaFindingsByRule()["AddJavascriptInterface"] == 0 {
		t.Fatalf("expected AddJavascriptInterface to target Java in mixed fixture; byRule=%v", report.Summary.ByRule)
	}
	if report.kotlinFindingsByRule()["UnusedVariable"] == 0 {
		t.Fatalf("expected UnusedVariable to target Kotlin in mixed fixture; byRule=%v", report.Summary.ByRule)
	}
	if !report.hasPerfPath("ruleExecution/topDispatchRules/AddJavascriptInterface") {
		t.Fatalf("expected AddJavascriptInterface perf rule path, got %v", report.perfPaths())
	}
}

type javaSupportReport struct {
	Files    int `json:"files"`
	Findings []struct {
		File string `json:"file"`
		Rule string `json:"rule"`
	} `json:"findings"`
	Summary struct {
		Total  int            `json:"total"`
		ByRule map[string]int `json:"byRule"`
	} `json:"summary"`
	PerfTiming []javaSupportPerfEntry `json:"perfTiming"`
}

type javaSupportPerfEntry struct {
	Name     string                 `json:"name"`
	Children []javaSupportPerfEntry `json:"children"`
}

type javaSupportInventory struct {
	Java   int
	Kotlin int
	XML    int
	Gradle int
}

func runJavaSupportFixture(t *testing.T, project string) javaSupportReport {
	t.Helper()
	stdout, stderr, code := runKrit(t,
		"--no-cache",
		"--no-type-inference",
		"--no-type-oracle",
		"--all-rules",
		"--perf",
		"--perf-rules",
		"-q",
		"-f", "json",
		project,
	)
	if code != 1 {
		t.Fatalf("expected fixture findings exit code 1, got %d\nstderr: %s\nstdout: %s", code, stderr, stdout)
	}
	var report javaSupportReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid fixture JSON: %v\nstdout: %s", err, stdout)
	}
	if report.Summary.Total == 0 {
		t.Fatal("expected Java support fixture findings")
	}
	return report
}

func javaSupportFixtureInventory(t *testing.T, project string) javaSupportInventory {
	t.Helper()
	var inventory javaSupportInventory
	if err := filepath.WalkDir(project, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".gradle", "build":
				return filepath.SkipDir
			}
			return nil
		}
		slashed := filepath.ToSlash(path)
		name := d.Name()
		switch {
		case strings.HasSuffix(name, ".java"):
			inventory.Java++
		case strings.HasSuffix(name, ".kt"):
			inventory.Kotlin++
		case strings.HasSuffix(name, ".gradle") || strings.HasSuffix(name, ".gradle.kts") || name == "settings.gradle.kts":
			inventory.Gradle++
		case name == "AndroidManifest.xml" || (strings.Contains(slashed, "/res/") && strings.HasSuffix(name, ".xml")):
			inventory.XML++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return inventory
}

func (r javaSupportReport) javaFindingsByRule() map[string]int {
	out := make(map[string]int)
	for _, finding := range r.Findings {
		if strings.HasSuffix(finding.File, ".java") {
			out[finding.Rule]++
		}
	}
	return out
}

func (r javaSupportReport) kotlinFindingsByRule() map[string]int {
	out := make(map[string]int)
	for _, finding := range r.Findings {
		if strings.HasSuffix(finding.File, ".kt") {
			out[finding.Rule]++
		}
	}
	return out
}

func (r javaSupportReport) hasPerfPath(want string) bool {
	for _, path := range r.perfPaths() {
		if path == want {
			return true
		}
	}
	return false
}

func (r javaSupportReport) perfPaths() []string {
	var out []string
	var walk func(prefix string, entries []javaSupportPerfEntry)
	walk = func(prefix string, entries []javaSupportPerfEntry) {
		for _, entry := range entries {
			path := entry.Name
			if prefix != "" {
				path = prefix + "/" + entry.Name
			}
			out = append(out, path)
			walk(path, entry.Children)
		}
	}
	walk("", r.PerfTiming)
	return out
}
