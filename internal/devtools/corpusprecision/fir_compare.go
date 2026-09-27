package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
)

type firSummary struct {
	Authoritative  int `json:"authoritative"`
	Gated          int `json:"gated"`
	Excluded       int `json:"excluded"`
	RuleErrors     int `json:"ruleErrors"`
	GeneratedGated int `json:"generatedGated"`
}

type firRuleRow struct {
	Rule              string              `json:"rule"`
	Confirmed         int                 `json:"confirmed"`
	GoDropped         int                 `json:"goDropped"`
	FirAdded          int                 `json:"firAdded"`
	FilesGated        int                 `json:"filesGated"`
	RuleErrors        int                 `json:"ruleErrors"`
	JSONConfirmed     int                 `json:"jsonConfirmed"`
	JSONGoDropped     int                 `json:"jsonGoDropped"`
	JSONFirAdded      int                 `json:"jsonFirAdded"`
	Discrepancy       bool                `json:"discrepancy"`
	VerdictPending    int                 `json:"verdictPending"`
	GoDroppedFindings []normalizedFinding `json:"goDroppedFindings"`
	FirAddedFindings  []normalizedFinding `json:"firAddedFindings"`
}

type firSnapshot struct {
	Comment         string       `json:"_comment"`
	CorpusName      string       `json:"corpusName"`
	CommitSHA       string       `json:"commitSHA"`
	GradleModel     bool         `json:"gradleModel"`
	LabelsAvailable bool         `json:"labelsAvailable"`
	Summary         firSummary   `json:"summary"`
	Rules           []firRuleRow `json:"rules"`
}

var (
	firSummaryLine = regexp.MustCompile(`^verbose: FIR verdict: (\d+) authoritative files, (\d+) gated \(compiler error or crash\), (\d+) excluded \(scripts or not in a JVM source set\), (\d+) rule errors \(checker threw; Go kept for that rule and file\), (\d+) gated \(generated sources\)$`)
	firRuleLine    = regexp.MustCompile(`^verbose: FIR verdict (.+): confirmed=(\d+) go-dropped=(\d+) fir-added=(\d+) enriched-with-fix=(\d+) suppressed=(\d+) files-gated=(\d+) rule-errors=(\d+)$`)
)

func parseFIRVerdict(verbose string) (firSummary, []firRuleRow, error) {
	var summary firSummary
	var rows []firRuleRow
	seenSummary := false
	seenRules := map[string]bool{}
	for _, line := range strings.Split(verbose, "\n") {
		if m := firSummaryLine.FindStringSubmatch(line); m != nil {
			if seenSummary {
				return summary, nil, errors.New("duplicate FIR verdict summary")
			}
			seenSummary = true
			summary = firSummary{Authoritative: atoi(m[1]), Gated: atoi(m[2]), Excluded: atoi(m[3]), RuleErrors: atoi(m[4]), GeneratedGated: atoi(m[5])}
		} else if m := firRuleLine.FindStringSubmatch(line); m != nil {
			if seenRules[m[1]] {
				return summary, nil, fmt.Errorf("duplicate FIR verdict rule %q", m[1])
			}
			seenRules[m[1]] = true
			rows = append(rows, firRuleRow{Rule: m[1], Confirmed: atoi(m[2]), GoDropped: atoi(m[3]), FirAdded: atoi(m[4]), FilesGated: atoi(m[7]), RuleErrors: atoi(m[8])})
		}
	}
	if !seenSummary {
		return summary, nil, errors.New("FIR verdict summary missing from verbose output")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Rule < rows[j].Rule })
	return summary, rows, nil
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

type firJoinKey struct {
	Rule, RelPath string
	Line, Col     int
}
type firJoin struct {
	Confirmed int
	GoDropped []normalizedFinding
	FirAdded  []normalizedFinding
}

func joinFIRFindings(goFindings, firFindings []normalizedFinding, ported map[string]bool) map[string]firJoin {
	goByKey := map[firJoinKey][]normalizedFinding{}
	firByKey := map[firJoinKey][]normalizedFinding{}
	for _, f := range goFindings {
		if ported[f.Rule] {
			k := firJoinKey{f.Rule, f.RelPath, f.Line, f.Col}
			goByKey[k] = append(goByKey[k], f)
		}
	}
	for _, f := range firFindings {
		if ported[f.Rule] {
			k := firJoinKey{f.Rule, f.RelPath, f.Line, f.Col}
			firByKey[k] = append(firByKey[k], f)
		}
	}
	out := map[string]firJoin{}
	for rule := range ported {
		out[rule] = firJoin{}
	}
	for k, goAt := range goByKey {
		firAt := firByKey[k]
		matched := min(len(goAt), len(firAt))
		row := out[k.Rule]
		row.Confirmed += matched
		row.GoDropped = append(row.GoDropped, goAt[matched:]...)
		out[k.Rule] = row
	}
	for k, firAt := range firByKey {
		matched := min(len(goByKey[k]), len(firAt))
		row := out[k.Rule]
		row.FirAdded = append(row.FirAdded, firAt[matched:]...)
		out[k.Rule] = row
	}
	for rule, row := range out {
		sortFindings(row.GoDropped)
		sortFindings(row.FirAdded)
		out[rule] = row
	}
	return out
}

func firCheckerRules(root string) (map[string]bool, string, error) {
	jar := oracle.FindBackendJar(oracle.BackendFIR, []string{root})
	if jar == "" {
		return nil, "", errors.New("error: krit-fir jar missing; build with `cd tools/krit-fir && ./gradlew shadowJar` or set KRIT_FIR_JAR")
	}
	cmd := exec.CommandContext(context.Background(), "java", "-jar", jar, "--list-rules")
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("error: list FIR checkers from jar: %w", err)
	}
	rules := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			rules[line] = true
		}
	}
	if len(rules) == 0 {
		return nil, "", errors.New("error: FIR jar listed no rules")
	}
	return rules, jar, nil
}

func runFIRScan(root string, c availableCorpus, modelDir, jar string, fir bool) ([]normalizedFinding, string, error) {
	corpusRoot, err := parityCorpusRoot(root, c)
	if err != nil {
		return nil, "", err
	}
	args := []string{"-f", "json", "--no-cache", "--no-daemon", "--all-rules", "--base-path", c.ScanPath}
	if fir {
		args = append(args, "--fir", "-v")
	} else {
		args = append(args, "--no-fir")
	}
	if modelDir == "" {
		args = append(args, "--no-gradle-model")
	} else {
		args = append(args, "--gradle-model", modelDir)
	}
	args = append(args, c.Flags...)
	args = append(args, c.ScanPath)
	cmd := exec.CommandContext(context.Background(), filepath.Join(root, "krit"), args...)
	cmd.Dir = root
	if fir {
		cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, "", fmt.Errorf("error: %s scan of %s failed: %w\nstderr:\n%s", map[bool]string{true: "FIR", false: "Go"}[fir], c.Name, runErr, stderr.String())
		}
	}
	var report rawReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, "", fmt.Errorf("error: decode %s scan of %s: %w\nstderr:\n%s", map[bool]string{true: "FIR", false: "Go"}[fir], c.Name, err, stderr.String())
	}
	findings, err := normalizeFindings(root, corpusRoot, report.Findings)
	return findings, stderr.String(), err
}

func compareFIR(root string, selected []availableCorpus, requireModel bool, out io.Writer) error {
	var ready []availableCorpus
	models := map[string]string{}
	for _, c := range selected {
		corpusRoot, err := parityCorpusRoot(root, c)
		if err != nil {
			fmt.Fprintf(out, "Skipping %s: %v\n", c.Name, err)
			continue
		}
		modelDir := filepath.Join(corpusRoot, ".krit", "gradle-model")
		modelFiles, err := filepath.Glob(filepath.Join(modelDir, "*.json"))
		if err != nil || len(modelFiles) == 0 {
			if requireModel {
				fmt.Fprintf(out, "Skipping %s: Gradle model missing; run scripts/corpus-gradle-model.sh %s\n", c.Name, c.ScanPath)
				continue
			}
			modelDir = ""
		}
		ready = append(ready, c)
		models[c.Name] = modelDir
	}
	for _, c := range corpora {
		if c.EnvVar != "" && os.Getenv(c.EnvVar) == "" {
			fmt.Fprintf(out, "Skipping %s: %s is unset\n", c.Name, c.EnvVar)
		}
	}
	if len(ready) == 0 {
		fmt.Fprintln(out, "No corpora with exported Gradle models were available.")
		return nil
	}
	ported, jar, err := firCheckerRules(root)
	if err != nil {
		return err
	}
	var snapshots []firSnapshot
	for _, c := range ready {
		snapshot, err := compareFIRCorpus(root, c, models[c.Name], jar, ported)
		if err != nil {
			return err
		}
		path := filepath.Join(root, ".krit", "corpus-fir", c.Name+".json")
		if err := writeFIRSnapshot(path, snapshot); err != nil {
			return err
		}
		fmt.Fprintf(out, "Wrote .krit/corpus-fir/%s.json with %d FIR rules.\n", c.Name, len(snapshot.Rules))
		snapshots = append(snapshots, snapshot)
	}
	return writeFIRMarkdown(root, snapshots)
}

func compareFIRCorpus(root string, c availableCorpus, modelDir, jar string, ported map[string]bool) (firSnapshot, error) {
	goFindings, _, err := runFIRScan(root, c, modelDir, jar, false)
	if err != nil {
		return firSnapshot{}, err
	}
	firFindings, verbose, err := runFIRScan(root, c, modelDir, jar, true)
	if err != nil {
		return firSnapshot{}, err
	}
	summary, rows, err := parseFIRVerdict(verbose)
	if err != nil {
		return firSnapshot{}, fmt.Errorf("error: %s: %w\nverbose:\n%s", c.Name, err, verbose)
	}
	labels, labelsAvailable, err := loadLabels(labelsPath(root, c.Name))
	if err != nil {
		return firSnapshot{}, err
	}
	populateFIRRows(rows, joinFIRFindings(goFindings, firFindings, ported), labels, labelsAvailable)
	corpusRoot, err := parityCorpusRoot(root, c)
	if err != nil {
		return firSnapshot{}, err
	}
	comment := "FIR versus Go verdict and coverage. Regenerate via `make fir-validate`."
	if modelDir == "" {
		comment = fmt.Sprintf("FIR versus Go verdict and coverage. Regenerate via `go run ./internal/devtools/corpusprecision --fir-compare --corpus %s`.", c.Name)
	}
	return firSnapshot{Comment: comment, CorpusName: c.Name, CommitSHA: corpusCommitSHA(corpusRoot), GradleModel: modelDir != "", LabelsAvailable: labelsAvailable, Summary: summary, Rules: rows}, nil
}

func populateFIRRows(rows []firRuleRow, joined map[string]firJoin, labels []label, labelsAvailable bool) {
	labeled := map[labelSignature]bool{}
	for _, entry := range labels {
		labeled[signatureForLabel(entry)] = true
	}
	for i := range rows {
		j := joined[rows[i].Rule]
		rows[i].JSONConfirmed, rows[i].JSONGoDropped, rows[i].JSONFirAdded = j.Confirmed, len(j.GoDropped), len(j.FirAdded)
		rows[i].Discrepancy = rows[i].Confirmed != j.Confirmed || rows[i].GoDropped != len(j.GoDropped) || rows[i].FirAdded != len(j.FirAdded)
		if !labelsAvailable {
			rows[i].VerdictPending = len(j.GoDropped) + len(j.FirAdded)
		} else {
			for _, f := range j.GoDropped {
				if !labeled[signatureForFinding(f)] {
					rows[i].VerdictPending++
				}
			}
			for _, f := range j.FirAdded {
				if !labeled[signatureForFinding(f)] {
					rows[i].VerdictPending++
				}
			}
		}
		rows[i].GoDroppedFindings = capFindings(j.GoDropped)
		rows[i].FirAddedFindings = capFindings(j.FirAdded)
	}
}

func capFindings(findings []normalizedFinding) []normalizedFinding {
	if len(findings) > 25 {
		findings = findings[:25]
	}
	if findings == nil {
		return []normalizedFinding{}
	}
	return findings
}

func writeFIRSnapshot(path string, snapshot firSnapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeFIRMarkdown(root string, snapshots []firSnapshot) error {
	type total struct {
		Confirmed, GoDropped, FirAdded, Gated, Authoritative, Excluded, Pending int
	}
	totals := map[string]total{}
	for _, snapshot := range snapshots {
		for _, row := range snapshot.Rules {
			t := totals[row.Rule]
			t.Confirmed += row.Confirmed
			t.GoDropped += row.GoDropped
			t.FirAdded += row.FirAdded
			t.Gated += row.FilesGated
			t.Authoritative += snapshot.Summary.Authoritative
			t.Excluded += snapshot.Summary.Excluded
			t.Pending += row.VerdictPending
			totals[row.Rule] = t
		}
	}
	rules := make([]string, 0, len(totals))
	for rule := range totals {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	var b strings.Builder
	b.WriteString("# FIR vs Go validation\n\nGenerated by `make fir-validate`. Verdict counts come from FIR's authoritative merge; JSON join counts and discrepancies are in `.krit/corpus-fir/`. Gated and excluded coverage are global corpus counts reported for each invoked checker.\n\n")
	var corpusNames []string
	for _, snapshot := range snapshots {
		corpusNames = append(corpusNames, snapshot.CorpusName)
	}
	if len(corpusNames) > 0 {
		fmt.Fprintf(&b, "Corpora included: %s.\n\n", strings.Join(corpusNames, ", "))
	}
	if len(snapshots) == 0 {
		b.WriteString("No corpora with exported Gradle models were available for this run.\n\n")
	}
	b.WriteString("| rule | confirmed | go-dropped | fir-added | files-gated | gated% | excluded% | verdict-pending |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, rule := range rules {
		t := totals[rule]
		gatedPct, excludedPct := "n/a", "n/a"
		if denominator := t.Authoritative + t.Gated; denominator > 0 {
			gatedPct = fmt.Sprintf("%.1f%%", 100*float64(t.Gated)/float64(denominator))
		}
		if denominator := t.Authoritative + t.Gated + t.Excluded; denominator > 0 {
			excludedPct = fmt.Sprintf("%.1f%%", 100*float64(t.Excluded)/float64(denominator))
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %s | %s | %d |\n", rule, t.Confirmed, t.GoDropped, t.FirAdded, t.Gated, gatedPct, excludedPct, t.Pending)
	}
	b.WriteString("\n`verdict-pending` counts JSON-join findings lacking a matching `(rule, relPath, lineHash, col)` label. Where labels are unavailable, it uses the raw go-dropped + fir-added JSON count.\n")
	var unlabeled []string
	for _, snapshot := range snapshots {
		if !snapshot.LabelsAvailable {
			unlabeled = append(unlabeled, snapshot.CorpusName)
		}
	}
	if len(unlabeled) > 0 {
		fmt.Fprintf(&b, "\nLabels unavailable for: %s.\n", strings.Join(unlabeled, ", "))
	}
	var unmodelled []string
	for _, snapshot := range snapshots {
		if !snapshot.GradleModel {
			unmodelled = append(unmodelled, snapshot.CorpusName)
		}
	}
	if len(unmodelled) > 0 {
		fmt.Fprintf(&b, "\nGradle model unavailable for: %s. These scans did not use real Gradle classpaths.\n", strings.Join(unmodelled, ", "))
	}
	path := filepath.Join(root, "docs", "fir-validation.md")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
