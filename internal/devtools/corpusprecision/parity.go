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
	"sort"
	"text/tabwriter"
)

// parityMapping maps Krit rules to retained Kotlin compiler diagnostic factories.
// Both JVM backends retain exactly UNREACHABLE_CODE, USELESS_ELVIS, and
// CAST_NEVER_SUCCEEDS (tools/krit-fir/.../OracleDiagnosticMessageCollector.kt
// and tools/krit-types/.../Main.kt); do not add factories until both Kotlin-side
// allowlists are extended. UnsafeCast's existing diagnostic map proves
// CAST_NEVER_SUCCEEDS; the FIR contract fixture proves USELESS_ELVIS for the
// non-null elvis shape; UnreachableCode is the Go analogue of UNREACHABLE_CODE.
var parityMapping = map[string][]string{
	"UnsafeCast":            {"CAST_NEVER_SUCCEEDS"},
	"UnreachableCode":       {"UNREACHABLE_CODE"},
	"UselessElvisOnNonNull": {"USELESS_ELVIS"},
}

type parityRawReport struct {
	Findings []parityRawFinding `json:"findings"`
}
type parityRawFinding struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Rule      string `json:"rule"`
	StartByte *int   `json:"startByte,omitempty"`
	EndByte   *int   `json:"endByte,omitempty"`
}
type parityGoFinding struct {
	Rule, RelPath                 string
	Line, Col, StartByte, EndByte int
	HasBytes                      bool
}
type compilerDiagnostic struct {
	FactoryName string `json:"factoryName"`
	Line        int    `json:"line"`
	Col         int    `json:"col"`
	StartByte   int    `json:"startByte"`
	EndByte     int    `json:"endByte"`
}
type parityCount struct {
	Rule                        string
	Agree, GoOnly, CompilerOnly int
}

func parityCorpusRoot(root string, c availableCorpus) (string, error) {
	corpusRoot := c.ScanPath
	if !filepath.IsAbs(corpusRoot) {
		corpusRoot = filepath.Join(root, corpusRoot)
	}
	corpusRoot = filepath.Clean(corpusRoot)
	info, err := os.Stat(corpusRoot)
	if err != nil {
		return "", fmt.Errorf("error: inspect corpus %s: %w", c.Name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("error: corpus %s path is not a directory: %s", c.Name, c.ScanPath)
	}
	return corpusRoot, nil
}

func parityRelPath(root, corpusRoot, path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, filepath.FromSlash(abs))
	}
	rel, err := filepath.Rel(corpusRoot, filepath.Clean(abs))
	if err != nil {
		return "", fmt.Errorf("make %q relative to corpus root: %w", path, err)
	}
	return filepath.ToSlash(rel), nil
}

func runKritForParity(root string, c availableCorpus) ([]parityGoFinding, error) {
	krit := filepath.Join(root, "krit")
	if _, err := os.Stat(krit); err != nil {
		return nil, fmt.Errorf("error: %s not found. Build with `go build -o krit ./cmd/krit/`", krit)
	}
	corpusRoot, err := parityCorpusRoot(root, c)
	if err != nil {
		return nil, err
	}
	args := []string{"-f", "json", "--no-cache", "--no-daemon", "--base-path", c.ScanPath}
	args = append(args, c.Flags...)
	args = append(args, c.ScanPath)
	cmd := exec.CommandContext(context.Background(), krit, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("error: %s exited: %w\nstderr:\n%s", commandString(krit, args), runErr, stderr.String())
		}
	}
	var report parityRawReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("error: decode %s output: %w\nstderr:\n%s", commandString(krit, args), err, stderr.String())
	}
	findings := make([]parityGoFinding, 0, len(report.Findings))
	for _, raw := range report.Findings {
		if _, ok := parityMapping[raw.Rule]; !ok {
			continue
		}
		rel, err := parityRelPath(root, corpusRoot, raw.File)
		if err != nil {
			return nil, fmt.Errorf("error: normalize %s findings: %w", c.Name, err)
		}
		f := parityGoFinding{Rule: raw.Rule, RelPath: rel, Line: raw.Line, Col: raw.Column}
		if raw.StartByte != nil && raw.EndByte != nil && *raw.EndByte > *raw.StartByte {
			f.StartByte, f.EndByte, f.HasBytes = *raw.StartByte, *raw.EndByte, true
		}
		findings = append(findings, f)
	}
	return findings, nil
}

func runKritDumpOracleDiagnostics(root string, c availableCorpus) (map[string][]compilerDiagnostic, error) {
	krit := filepath.Join(root, "krit")
	if _, err := os.Stat(krit); err != nil {
		return nil, fmt.Errorf("error: %s not found. Build with `go build -o krit ./cmd/krit/`", krit)
	}
	corpusRoot, err := parityCorpusRoot(root, c)
	if err != nil {
		return nil, err
	}
	args := []string{"--dump-oracle-diagnostics", "--base-path", c.ScanPath, c.ScanPath}
	cmd := exec.CommandContext(context.Background(), krit, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error: %s exited: %w\nstderr:\n%s", commandString(krit, args), err, stderr.String())
	}
	var raw map[string][]compilerDiagnostic
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("error: decode %s output: %w\nstderr:\n%s", commandString(krit, args), err, stderr.String())
	}
	factories := map[string]bool{}
	for _, names := range parityMapping {
		for _, name := range names {
			factories[name] = true
		}
	}
	result := make(map[string][]compilerDiagnostic)
	for path, diagnostics := range raw {
		rel, err := parityRelPath(root, corpusRoot, path)
		if err != nil {
			return nil, fmt.Errorf("error: normalize %s compiler diagnostics: %w", c.Name, err)
		}
		for _, diagnostic := range diagnostics {
			if factories[diagnostic.FactoryName] {
				result[rel] = append(result[rel], diagnostic)
			}
		}
	}
	return result, nil
}

func computeParity(goFindings []parityGoFinding, diagnosticsByPath map[string][]compilerDiagnostic) ([]parityCount, parityCount) {
	rules := make([]string, 0, len(parityMapping))
	for rule := range parityMapping {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	counts := make([]parityCount, 0, len(rules))
	overall := parityCount{Rule: "overall"}
	for _, rule := range rules {
		count := computeParityForRule(rule, goFindings, diagnosticsByPath)
		overall.Agree += count.Agree
		overall.GoOnly += count.GoOnly
		overall.CompilerOnly += count.CompilerOnly
		counts = append(counts, count)
	}
	return counts, overall
}

func computeParityForRule(rule string, goFindings []parityGoFinding, diagnosticsByPath map[string][]compilerDiagnostic) parityCount {
	count := parityCount{Rule: rule}
	wanted := make(map[string]bool, len(parityMapping[rule]))
	for _, name := range parityMapping[rule] {
		wanted[name] = true
	}
	goByPath := make(map[string][]parityGoFinding)
	diagByPath := make(map[string][]compilerDiagnostic)
	for _, finding := range goFindings {
		if finding.Rule == rule {
			goByPath[finding.RelPath] = append(goByPath[finding.RelPath], finding)
		}
	}
	for path, diagnostics := range diagnosticsByPath {
		for _, diagnostic := range diagnostics {
			if wanted[diagnostic.FactoryName] {
				diagByPath[path] = append(diagByPath[path], diagnostic)
			}
		}
	}
	paths := make(map[string]bool)
	for path := range goByPath {
		paths[path] = true
	}
	for path := range diagByPath {
		paths[path] = true
	}
	for path := range paths {
		agree, goOnly, compilerOnly := matchParityPath(goByPath[path], diagByPath[path])
		count.Agree += agree
		count.GoOnly += goOnly
		count.CompilerOnly += compilerOnly
	}
	return count
}

func matchParityPath(goFindings []parityGoFinding, diagnostics []compilerDiagnostic) (agree, goOnly, compilerOnly int) {
	sort.Slice(goFindings, func(i, j int) bool {
		if goFindings[i].Line != goFindings[j].Line {
			return goFindings[i].Line < goFindings[j].Line
		}
		return goFindings[i].Col < goFindings[j].Col
	})
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Col < diagnostics[j].Col
	})
	used := make([]bool, len(diagnostics))
	for _, finding := range goFindings {
		matched := false
		for i, diagnostic := range diagnostics {
			if !used[i] && parityLocationsMatch(finding, diagnostic) {
				used[i], matched, agree = true, true, agree+1
				break
			}
		}
		if !matched {
			goOnly++
		}
	}
	for _, wasUsed := range used {
		if !wasUsed {
			compilerOnly++
		}
	}
	return agree, goOnly, compilerOnly
}

func parityLocationsMatch(finding parityGoFinding, diagnostic compilerDiagnostic) bool {
	if finding.HasBytes && diagnostic.EndByte > diagnostic.StartByte {
		return finding.StartByte < diagnostic.EndByte && diagnostic.StartByte < finding.EndByte
	}
	return finding.Line == diagnostic.Line
}

func parityAgreement(count parityCount) string {
	total := count.Agree + count.GoOnly + count.CompilerOnly
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", float64(count.Agree)/float64(total))
}
func printParityTable(out io.Writer, corpusName string, counts []parityCount, overall parityCount) {
	fmt.Fprintf(out, "Corpus: %s\n", corpusName)
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RULE\tAGREE\tGO_ONLY\tCOMPILER_ONLY\tAGREEMENT")
	for _, count := range counts {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\n", count.Rule, count.Agree, count.GoOnly, count.CompilerOnly, parityAgreement(count))
	}
	_ = w.Flush()
	fmt.Fprintf(out, "Overall: agree=%d go-only=%d compiler-only=%d agreement=%s\n", overall.Agree, overall.GoOnly, overall.CompilerOnly, parityAgreement(overall))
}
func printCompilerParity(root string, selected []availableCorpus, out io.Writer) error {
	for i, c := range selected {
		if i > 0 {
			fmt.Fprintln(out)
		}
		goFindings, err := runKritForParity(root, c)
		if err != nil {
			return err
		}
		diagnostics, err := runKritDumpOracleDiagnostics(root, c)
		if err != nil {
			return err
		}
		counts, overall := computeParity(goFindings, diagnostics)
		printParityTable(out, c.Name, counts, overall)
	}
	return nil
}
