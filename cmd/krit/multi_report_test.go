package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countReportFindings parses each report format and returns its finding count.
func countReportFindings(t *testing.T, format, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s report: %v", format, err)
	}
	switch format {
	case "json":
		var out struct {
			Findings []json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("json report: %v\n%s", err, data)
		}
		return len(out.Findings)
	case "sarif":
		var out struct {
			Runs []struct {
				Results []json.RawMessage `json:"results"`
			} `json:"runs"`
		}
		if err := json.Unmarshal(data, &out); err != nil || len(out.Runs) != 1 {
			t.Fatalf("sarif report: %v\n%s", err, data)
		}
		return len(out.Runs[0].Results)
	case "checkstyle":
		var out struct {
			Files []struct {
				Errors []struct{} `xml:"error"`
			} `xml:"file"`
		}
		if err := xml.Unmarshal(data, &out); err != nil {
			t.Fatalf("checkstyle report: %v\n%s", err, data)
		}
		n := 0
		for _, f := range out.Files {
			n += len(f.Errors)
		}
		return n
	case "plain":
		return strings.Count(string(data), "UnusedVariable")
	}
	t.Fatalf("unknown format %s", format)
	return 0
}

// One run writes every --report FORMAT=PATH file from the same findings
// (#1578); with no -f/-o, stdout carries no report.
func TestMultipleReportsFromOneRun(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n    val y = 2\n}\n")
	out := t.TempDir()
	reports := map[string]string{
		"json":       filepath.Join(out, "krit.json"),
		"sarif":      filepath.Join(out, "krit.sarif"),
		"checkstyle": filepath.Join(out, "krit.xml"),
		"plain":      filepath.Join(out, "krit.txt"),
	}
	args := []string{"--no-daemon", "--no-cache", "--no-type-inference", "--no-type-oracle",
		"--enable-rules", "UnusedVariable", "--disable-rules", "InvalidPackageDeclaration"}
	for _, format := range []string{"json", "sarif", "checkstyle", "plain"} {
		args = append(args, "--report", format+"="+reports[format])
	}
	stdout, stderr, code := runKrit(t, append(args, dir)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (findings); stderr:\n%s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no report when only --report FORMAT=PATH is given", stdout)
	}
	if n := strings.Count(stderr, "Found "); n != 1 {
		t.Fatalf("stderr reports %d scans, want one:\n%s", n, stderr)
	}
	jsonCount := countReportFindings(t, "json", reports["json"])
	if jsonCount == 0 {
		t.Fatal("json report has no findings")
	}
	for _, format := range []string{"sarif", "checkstyle"} {
		if got := countReportFindings(t, format, reports[format]); got != jsonCount {
			t.Errorf("%s report has %d findings, json has %d", format, got, jsonCount)
		}
	}
	if got := countReportFindings(t, "plain", reports["plain"]); got < 2 {
		t.Errorf("plain report names UnusedVariable %d times, want both findings", got)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(reports) {
		t.Errorf("report directory has %d entries (staging files left?), want %d", len(entries), len(reports))
	}

	// An explicit -f keeps the stdout report next to the files.
	stdout, stderr, code = runKrit(t, append(append([]string{"-f", "json"}, args...), dir)...)
	if code != 1 {
		t.Fatalf("with -f json: exit %d; stderr:\n%s", code, stderr)
	}
	var primary struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &primary); err != nil || len(primary.Findings) != jsonCount {
		t.Fatalf("with -f json: stdout has %d findings (%v), want %d", len(primary.Findings), err, jsonCount)
	}
}

func TestReportFlagRejectsBadTargets(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n")
	out := filepath.Join(t.TempDir(), "r.json")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown format", []string{"--report", "yaml=" + out}, `unknown report format "yaml"`},
		{"empty path", []string{"--report", "json="}, "has no path"},
		{"duplicate path", []string{"--report", "json=" + out, "--report", "sarif=" + out}, "is given twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runKrit(t, append(append([]string{"--no-daemon", "--no-cache"}, tc.args...), dir)...)
			if code != 2 || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit %d, stderr %q; want exit 2 mentioning %q", code, stderr, tc.want)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("a rejected run wrote %s", out)
			}
		})
	}
}
