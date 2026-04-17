package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// regressionCase names a playground project and the baseline file to
// diff its findings against. Baselines live in cmd/krit/testdata/regression/
// and are checked in as the source of truth for CI regression gating
// (roadmap/17 Phase 7).
type regressionCase struct {
	name         string // human-readable test name
	projectPath  string // path passed to krit, relative to cmd/krit
	baselinePath string // checked-in baseline JSON, relative to cmd/krit
}

// regressionCases enumerates the codebases gated by the regression
// suite. Keep the list small — every case runs the full scanner,
// which adds wall time to `go test`. Add more projects only when
// they cover rule categories that the existing ones don't.
var regressionCases = []regressionCase{
	{
		name:         "playground-android-app",
		projectPath:  "../../playground/android-app/",
		baselinePath: "testdata/regression/playground-android-app.json",
	},
	{
		name:         "playground-kotlin-webservice",
		projectPath:  "../../playground/kotlin-webservice/",
		baselinePath: "testdata/regression/playground-kotlin-webservice.json",
	},
}

// normalizedFinding is the subset of a finding that the regression
// gate cares about. Message text is intentionally included — changes
// to rule messages must update the baseline, keeping rule copy
// reviewable alongside rule logic.
type normalizedFinding struct {
	Rule    string `json:"rule"`
	RuleSet string `json:"ruleSet"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

// baseline is the JSON schema written to disk. Only the findings
// array is compared; the other fields are metadata to help humans
// understand why a baseline changed when diffing commits.
type baseline struct {
	Project     string              `json:"project"`
	TotalRules  int                 `json:"totalRules,omitempty"`
	FindingsLen int                 `json:"findings_count"`
	Findings    []normalizedFinding `json:"findings"`
}

// TestRegression_Baselines runs krit on each playground project and
// diffs the output against a checked-in baseline. Set
// UPDATE_REGRESSION_BASELINES=1 to regenerate baselines after an
// intentional rule change.
//
// Findings are normalized: absolute paths are trimmed to paths
// relative to the project root, and findings are sorted by
// (file, line, col, rule) so scan order cannot cause spurious diffs.
// Only the fields in normalizedFinding are compared — confidence,
// severity, and fix metadata are intentionally excluded to keep
// baseline churn focused on what users see in reports.
//
// This closes Phase 7 of roadmap/17: "Real-codebase regression suite."
func TestRegression_Baselines(t *testing.T) {
	for _, tc := range regressionCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := runRegressionScan(t, tc.projectPath)

			if os.Getenv("UPDATE_REGRESSION_BASELINES") == "1" {
				writeBaseline(t, tc, got)
				t.Logf("regenerated baseline: %s (%d findings)", tc.baselinePath, len(got))
				return
			}

			want := readBaseline(t, tc.baselinePath)
			diffBaseline(t, tc.name, want, got)
		})
	}
}

func runRegressionScan(t *testing.T, projectPath string) []normalizedFinding {
	t.Helper()

	// Resolve the project path against the test's working directory
	// (cmd/krit) so we can trim it from each finding's absolute path.
	absProject, err := filepath.Abs(projectPath)
	if err != nil {
		t.Fatalf("abs %q: %v", projectPath, err)
	}

	out, err := exec.Command(binPath,
		"-f", "json",
		"-no-cache",
		"-no-type-inference",
		"-no-type-oracle",
		"-q",
		projectPath,
	).CombinedOutput()
	if err != nil {
		// Exit code 1 means "findings present" — that's expected on
		// the playground projects. Any other error is a real failure.
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("krit run failed: %v\n%s", err, out)
		}
	}

	var raw struct {
		Findings []struct {
			Rule    string `json:"rule"`
			RuleSet string `json:"ruleSet"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Col     int    `json:"col"`
			Message string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}

	// Normalize file paths so baselines are portable across checkout
	// locations. Krit emits paths in several shapes depending on which
	// rule family produced the finding and whether the scan argument
	// was absolute or relative:
	//
	//   - An absolute path matching the project prefix — strip to
	//     project-relative.
	//   - A relative path matching the project-path argument — strip
	//     the argument prefix to project-relative.
	//   - A bare project-relative path like "res/values/strings.xml"
	//     emitted by the Android resource pipeline — keep as-is.
	//
	// After normalization, every finding's File should be a path
	// relative to the project root so baselines are independent of
	// where the repo is checked out.
	projectPrefix := strings.TrimSuffix(filepath.ToSlash(projectPath), "/") + "/"
	absPrefix := strings.TrimSuffix(filepath.ToSlash(absProject), "/") + "/"

	findings := make([]normalizedFinding, 0, len(raw.Findings))
	for _, f := range raw.Findings {
		slashed := filepath.ToSlash(f.File)
		relFile := slashed
		switch {
		case strings.HasPrefix(slashed, absPrefix):
			relFile = strings.TrimPrefix(slashed, absPrefix)
		case strings.HasPrefix(slashed, projectPrefix):
			relFile = strings.TrimPrefix(slashed, projectPrefix)
		}
		findings = append(findings, normalizedFinding{
			Rule:    f.Rule,
			RuleSet: f.RuleSet,
			File:    relFile,
			Line:    f.Line,
			Col:     f.Col,
			Message: f.Message,
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
	return findings
}

func readBaseline(t *testing.T, path string) []normalizedFinding {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline %q: %v\nTo generate: UPDATE_REGRESSION_BASELINES=1 go test ./cmd/krit/ -run TestRegression_Baselines", path, err)
	}
	var b baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("parse baseline %q: %v", path, err)
	}
	return b.Findings
}

func writeBaseline(t *testing.T, tc regressionCase, findings []normalizedFinding) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(tc.baselinePath), 0o755); err != nil {
		t.Fatalf("mkdir baseline dir: %v", err)
	}
	b := baseline{
		Project:     tc.projectPath,
		FindingsLen: len(findings),
		Findings:    findings,
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(tc.baselinePath, raw, 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
}

func diffBaseline(t *testing.T, name string, want, got []normalizedFinding) {
	t.Helper()

	key := func(f normalizedFinding) string {
		return f.File + "|" + f.Rule + "|" + itoaDecimal(f.Line) + "|" + itoaDecimal(f.Col) + "|" + f.Message
	}
	wantKeys := make(map[string]normalizedFinding, len(want))
	for _, f := range want {
		wantKeys[key(f)] = f
	}
	gotKeys := make(map[string]normalizedFinding, len(got))
	for _, f := range got {
		gotKeys[key(f)] = f
	}

	var added, removed []normalizedFinding
	for k, f := range gotKeys {
		if _, ok := wantKeys[k]; !ok {
			added = append(added, f)
		}
	}
	for k, f := range wantKeys {
		if _, ok := gotKeys[k]; !ok {
			removed = append(removed, f)
		}
	}

	if len(added) == 0 && len(removed) == 0 {
		return
	}

	sort.Slice(added, func(i, j int) bool { return key(added[i]) < key(added[j]) })
	sort.Slice(removed, func(i, j int) bool { return key(removed[i]) < key(removed[j]) })

	var msg strings.Builder
	msg.WriteString("regression against baseline for ")
	msg.WriteString(name)
	msg.WriteString("\n")
	if len(added) > 0 {
		msg.WriteString("  added findings (new positives — may be FPs or an over-broadened rule):\n")
		for _, f := range added {
			msg.WriteString("    + ")
			msg.WriteString(f.Rule)
			msg.WriteString(" ")
			msg.WriteString(f.File)
			msg.WriteString(":")
			msg.WriteString(itoaDecimal(f.Line))
			msg.WriteString(": ")
			msg.WriteString(f.Message)
			msg.WriteString("\n")
		}
	}
	if len(removed) > 0 {
		msg.WriteString("  removed findings (previously reported — may be a narrowing or a silent regression):\n")
		for _, f := range removed {
			msg.WriteString("    - ")
			msg.WriteString(f.Rule)
			msg.WriteString(" ")
			msg.WriteString(f.File)
			msg.WriteString(":")
			msg.WriteString(itoaDecimal(f.Line))
			msg.WriteString(": ")
			msg.WriteString(f.Message)
			msg.WriteString("\n")
		}
	}
	msg.WriteString("\nIf this change is intentional, regenerate the baseline:\n")
	msg.WriteString("  UPDATE_REGRESSION_BASELINES=1 go test ./cmd/krit/ -run TestRegression_Baselines\n")
	t.Fatal(msg.String())
}

// itoaDecimal is a tiny strconv.Itoa inlined to avoid pulling in
// another import on a test-only code path.
func itoaDecimal(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
