package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

var corpora = []corpus{
	{Name: "kotlin-webservice", Path: "playground/kotlin-webservice"},
	{Name: "android-app", Path: "playground/android-app"},
	{Name: "metro", EnvVar: "KRIT_CORPUS_METRO"},
}

type corpus struct {
	Name   string
	Path   string
	EnvVar string
	Flags  []string
}

type availableCorpus struct {
	corpus
	ScanPath string
}

type rawReport struct {
	Findings []rawFinding `json:"findings"`
}

type rawFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	RuleSet  string `json:"ruleSet"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
}

type normalizedFinding struct {
	Rule     string `json:"rule"`
	RuleSet  string `json:"ruleSet"`
	RelPath  string `json:"relPath"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	LineHash string `json:"lineHash"`
}

type snapshotFile struct {
	Comment    string              `json:"_comment"`
	CorpusName string              `json:"corpusName"`
	CommitSHA  string              `json:"commitSHA"`
	Findings   []normalizedFinding `json:"findings"`
}

type corpusRun struct {
	Corpus   availableCorpus
	Snapshot snapshotFile
}

type label struct {
	Rule     string `json:"rule"`
	RelPath  string `json:"relPath"`
	LineHash string `json:"lineHash"`
	Col      int    `json:"col"`
	Verdict  string `json:"verdict"`
	Note     string `json:"note,omitempty"`
}

type labelSignature struct {
	Rule     string
	RelPath  string
	LineHash string
	Col      int
}

type findingDiff struct {
	Added   []normalizedFinding
	Removed []normalizedFinding
}

type precisionCount struct {
	Rule      string
	TP        int
	FP        int
	Unlabeled int
}

func main() {
	update := flag.Bool("update", false, "rewrite corpus snapshots with current findings")
	precision := flag.Bool("precision", false, "print precision from current findings and triage labels")
	corpusName := flag.String("corpus", "", "restrict the run to one corpus name")
	flag.Parse()

	if *update && *precision {
		fmt.Fprintln(os.Stderr, "error: --update and --precision are mutually exclusive")
		os.Exit(2)
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	selected, err := selectCorpora(*corpusName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	observed, err := collect(root, selected)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	switch {
	case *update:
		if err := updateSnapshots(root, observed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	case *precision:
		if err := printPrecision(root, observed, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	default:
		clean, err := checkSnapshots(root, observed, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if !clean {
			os.Exit(1)
		}
		fmt.Printf("Corpus precision snapshot gate: OK (%d corpora).\n", len(observed))
	}
}

func repoRoot() (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error: resolve repo root: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func selectCorpora(name string) ([]availableCorpus, error) {
	if name != "" {
		found := false
		for _, c := range corpora {
			if c.Name == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("error: unknown corpus %q", name)
		}
	}

	selected := make([]availableCorpus, 0, len(corpora))
	for _, c := range corpora {
		if name != "" && c.Name != name {
			continue
		}
		path := c.Path
		if c.EnvVar != "" {
			path = os.Getenv(c.EnvVar)
			if path == "" {
				continue
			}
		}
		selected = append(selected, availableCorpus{corpus: c, ScanPath: path})
	}
	return selected, nil
}

func collect(root string, selected []availableCorpus) ([]corpusRun, error) {
	runs := make([]corpusRun, 0, len(selected))
	for _, c := range selected {
		snapshot, err := runKrit(root, c)
		if err != nil {
			return nil, err
		}
		runs = append(runs, corpusRun{Corpus: c, Snapshot: snapshot})
	}
	return runs, nil
}

func runKrit(root string, c availableCorpus) (snapshotFile, error) {
	krit := filepath.Join(root, "krit")
	if _, err := os.Stat(krit); err != nil {
		return snapshotFile{}, fmt.Errorf("error: %s not found. Build with `go build -o krit ./cmd/krit/`", krit)
	}

	corpusRoot := c.ScanPath
	if !filepath.IsAbs(corpusRoot) {
		corpusRoot = filepath.Join(root, corpusRoot)
	}
	corpusRoot = filepath.Clean(corpusRoot)
	info, err := os.Stat(corpusRoot)
	if err != nil {
		return snapshotFile{}, fmt.Errorf("error: inspect corpus %s: %w", c.Name, err)
	}
	if !info.IsDir() {
		return snapshotFile{}, fmt.Errorf("error: corpus %s path is not a directory: %s", c.Name, c.ScanPath)
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
			return snapshotFile{}, fmt.Errorf("error: %s exited: %w\nstderr:\n%s", commandString(krit, args), runErr, stderr.String())
		}
	}

	var report rawReport
	if err := json.Unmarshal(out, &report); err != nil {
		return snapshotFile{}, fmt.Errorf("error: decode %s output: %w\nstderr:\n%s", commandString(krit, args), err, stderr.String())
	}
	findings, err := normalizeFindings(root, corpusRoot, report.Findings)
	if err != nil {
		return snapshotFile{}, fmt.Errorf("error: normalize %s findings: %w", c.Name, err)
	}

	return snapshotFile{
		Comment:    "Normalized Krit findings for precision tracking. Regenerate via `go run ./internal/devtools/corpusprecision --update`.",
		CorpusName: c.Name,
		CommitSHA:  corpusCommitSHA(corpusRoot),
		Findings:   findings,
	}, nil
}

func normalizeFindings(root, corpusRoot string, raw []rawFinding) ([]normalizedFinding, error) {
	findings := make([]normalizedFinding, 0, len(raw))
	lineCache := make(map[string][][]byte)
	readFailures := make(map[string]bool)

	for _, finding := range raw {
		absolutePath := finding.File
		if !filepath.IsAbs(absolutePath) {
			absolutePath = filepath.Join(root, filepath.FromSlash(absolutePath))
		}
		absolutePath = filepath.Clean(absolutePath)
		relPath, err := filepath.Rel(corpusRoot, absolutePath)
		if err != nil {
			return nil, fmt.Errorf("make %q relative to corpus root: %w", finding.File, err)
		}
		relPath = filepath.ToSlash(relPath)
		findings = append(findings, normalizedFinding{
			Rule:     finding.Rule,
			RuleSet:  finding.RuleSet,
			RelPath:  relPath,
			Line:     finding.Line,
			Col:      finding.Column,
			Severity: finding.Severity,
			LineHash: sourceLineHash(corpusRoot, relPath, finding.Line, lineCache, readFailures),
		})
	}

	sortFindings(findings)
	return findings, nil
}

func sourceLineHash(corpusRoot, relPath string, line int, cache map[string][][]byte, failures map[string]bool) string {
	path := filepath.Join(corpusRoot, filepath.FromSlash(relPath))
	lines, ok := cache[path]
	if !ok && !failures[path] {
		data, err := os.ReadFile(path)
		if err != nil {
			failures[path] = true
			return "unreadable"
		}
		lines = bytes.Split(data, []byte("\n"))
		cache[path] = lines
	}
	if failures[path] || line < 1 || line > len(lines) {
		return "unreadable"
	}
	trimmed := strings.TrimSpace(string(lines[line-1]))
	hash := sha256.Sum256([]byte(trimmed))
	return fmt.Sprintf("%x", hash[:])[:12]
}

func sortFindings(findings []normalizedFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.RelPath != right.RelPath {
			return left.RelPath < right.RelPath
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Col != right.Col {
			return left.Col < right.Col
		}
		return left.Rule < right.Rule
	})
}

func corpusCommitSHA(corpusRoot string) string {
	cmd := exec.CommandContext(context.Background(), "git", "-C", corpusRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

func updateSnapshots(root string, observed []corpusRun) error {
	for _, run := range observed {
		path := snapshotPath(root, run.Corpus.Name)
		if err := writeSnapshot(path, run.Snapshot); err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}
		fmt.Printf("Wrote %s with %d findings.\n", filepath.ToSlash(relPath), len(run.Snapshot.Findings))
	}
	return nil
}

func writeSnapshot(path string, snapshot snapshotFile) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("error: encode snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("error: create snapshot directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("error: write %s: %w", path, err)
	}
	return nil
}

func loadSnapshot(path string) (snapshotFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshotFile{}, fmt.Errorf("error: read %s: %w", path, err)
	}
	var snapshot snapshotFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshotFile{}, fmt.Errorf("error: decode %s: %w", path, err)
	}
	if snapshot.Findings == nil {
		snapshot.Findings = []normalizedFinding{}
	}
	sortFindings(snapshot.Findings)
	return snapshot, nil
}

func checkSnapshots(root string, observed []corpusRun, stderr io.Writer) (bool, error) {
	drifted := false
	for _, run := range observed {
		baseline, err := loadSnapshot(snapshotPath(root, run.Corpus.Name))
		if err != nil {
			return false, err
		}
		if baseline.CorpusName != run.Corpus.Name {
			return false, fmt.Errorf("error: snapshot for %s declares corpusName %q", run.Corpus.Name, baseline.CorpusName)
		}

		diff := diffFindings(baseline.Findings, run.Snapshot.Findings)
		if len(diff) == 0 {
			continue
		}
		if !drifted {
			fmt.Fprintln(stderr, "Corpus precision snapshot drift detected:")
			fmt.Fprintln(stderr)
		}
		drifted = true
		fmt.Fprintf(stderr, "  corpus=%s\n", run.Corpus.Name)
		printFindingDiff(stderr, diff)
	}
	if drifted {
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "If this shift is intentional, update the baseline:")
		fmt.Fprintln(stderr, "    go run ./internal/devtools/corpusprecision --update")
		fmt.Fprintln(stderr, "Then commit `.krit/corpus-snapshots/*.json` with your change.")
	}
	return !drifted, nil
}

func diffFindings(baseline, observed []normalizedFinding) map[string]findingDiff {
	baselineCounts := make(map[normalizedFinding]int, len(baseline))
	observedCounts := make(map[normalizedFinding]int, len(observed))
	for _, finding := range baseline {
		baselineCounts[finding]++
	}
	for _, finding := range observed {
		observedCounts[finding]++
	}

	diff := make(map[string]findingDiff)
	for finding, count := range observedCounts {
		for i := baselineCounts[finding]; i < count; i++ {
			entry := diff[finding.Rule]
			entry.Added = append(entry.Added, finding)
			diff[finding.Rule] = entry
		}
	}
	for finding, count := range baselineCounts {
		for i := observedCounts[finding]; i < count; i++ {
			entry := diff[finding.Rule]
			entry.Removed = append(entry.Removed, finding)
			diff[finding.Rule] = entry
		}
	}
	for rule, entry := range diff {
		sortFindings(entry.Added)
		sortFindings(entry.Removed)
		diff[rule] = entry
	}
	return diff
}

func printFindingDiff(w io.Writer, diff map[string]findingDiff) {
	rules := make([]string, 0, len(diff))
	for rule := range diff {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	for _, rule := range rules {
		entry := diff[rule]
		fmt.Fprintf(w, "    rule=%s\n", rule)
		if len(entry.Added) > 0 {
			fmt.Fprintln(w, "      added:")
			for _, finding := range entry.Added {
				fmt.Fprintf(w, "        {%s:%d}\n", finding.RelPath, finding.Line)
			}
		}
		if len(entry.Removed) > 0 {
			fmt.Fprintln(w, "      removed:")
			for _, finding := range entry.Removed {
				fmt.Fprintf(w, "        {%s:%d}\n", finding.RelPath, finding.Line)
			}
		}
	}
}

func printPrecision(root string, observed []corpusRun, out io.Writer) error {
	for i, run := range observed {
		path := labelsPath(root, run.Corpus.Name)
		labels, found, err := loadLabels(path)
		if err != nil {
			return err
		}
		if !found {
			relPath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relPath = path
			}
			fmt.Fprintf(out, "Corpus %s: no labels at %s; skipping precision.\n", run.Corpus.Name, filepath.ToSlash(relPath))
			continue
		}
		if i > 0 {
			fmt.Fprintln(out)
		}
		counts, overall := computePrecision(run.Snapshot.Findings, labels)
		printPrecisionTable(out, run.Corpus.Name, counts, overall, len(run.Snapshot.Findings))
	}
	return nil
}

func loadLabels(path string) ([]label, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("error: read %s: %w", path, err)
	}
	var labels []label
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, false, fmt.Errorf("error: decode %s: %w", path, err)
	}
	seen := make(map[labelSignature]bool, len(labels))
	for i, entry := range labels {
		switch entry.Verdict {
		case "tp", "fp", "unknown":
		default:
			return nil, false, fmt.Errorf("error: decode %s: label %d has invalid verdict %q (want tp, fp, or unknown)", path, i+1, entry.Verdict)
		}
		signature := signatureForLabel(entry)
		if seen[signature] {
			return nil, false, fmt.Errorf("error: decode %s: duplicate label signature for rule=%q relPath=%q lineHash=%q", path, entry.Rule, entry.RelPath, entry.LineHash)
		}
		seen[signature] = true
	}
	return labels, true, nil
}

func computePrecision(findings []normalizedFinding, labels []label) ([]precisionCount, precisionCount) {
	labelsBySignature := make(map[labelSignature]string, len(labels))
	for _, entry := range labels {
		labelsBySignature[signatureForLabel(entry)] = entry.Verdict
	}

	byRule := make(map[string]precisionCount)
	overall := precisionCount{Rule: "overall"}
	for _, finding := range findings {
		count := byRule[finding.Rule]
		count.Rule = finding.Rule
		switch labelsBySignature[signatureForFinding(finding)] {
		case "tp":
			count.TP++
			overall.TP++
		case "fp":
			count.FP++
			overall.FP++
		default:
			count.Unlabeled++
			overall.Unlabeled++
		}
		byRule[finding.Rule] = count
	}

	counts := make([]precisionCount, 0, len(byRule))
	for _, count := range byRule {
		counts = append(counts, count)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].Rule < counts[j].Rule })
	return counts, overall
}

func printPrecisionTable(out io.Writer, corpusName string, counts []precisionCount, overall precisionCount, total int) {
	fmt.Fprintf(out, "Corpus: %s\n", corpusName)
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RULE\tTP\tFP\tUNLABELED\tPRECISION")
	for _, count := range counts {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\n", count.Rule, count.TP, count.FP, count.Unlabeled, precisionValue(count))
	}
	_ = w.Flush()
	fmt.Fprintf(out, "Overall: tp=%d fp=%d unlabeled=%d precision=%s\n", overall.TP, overall.FP, overall.Unlabeled, precisionValue(overall))
	labeled := overall.TP + overall.FP
	if total == 0 {
		fmt.Fprintln(out, "Coverage: n/a (0/0 findings labeled tp/fp)")
		return
	}
	fmt.Fprintf(out, "Coverage: %.1f%% (%d/%d findings labeled tp/fp)\n", 100*float64(labeled)/float64(total), labeled, total)
}

func precisionValue(count precisionCount) string {
	labeled := count.TP + count.FP
	if labeled == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", float64(count.TP)/float64(labeled))
}

func signatureForFinding(finding normalizedFinding) labelSignature {
	return labelSignature{Rule: finding.Rule, RelPath: finding.RelPath, LineHash: finding.LineHash, Col: finding.Col}
}

func signatureForLabel(entry label) labelSignature {
	return labelSignature{Rule: entry.Rule, RelPath: entry.RelPath, LineHash: entry.LineHash, Col: entry.Col}
}

func snapshotPath(root, corpusName string) string {
	return filepath.Join(root, ".krit", "corpus-snapshots", corpusName+".json")
}

func labelsPath(root, corpusName string) string {
	return filepath.Join(root, ".krit", "corpus-labels", corpusName+".json")
}

func commandString(path string, args []string) string {
	return path + " " + joinArgs(args)
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
