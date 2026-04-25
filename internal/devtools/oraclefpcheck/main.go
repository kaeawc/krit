package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var corpora = []corpus{
	{Repo: "playground/kotlin-webservice", RuleSet: "default"},
	{Repo: "playground/kotlin-webservice", RuleSet: "all-rules", Flags: []string{"--all-rules"}},
	{Repo: "playground/android-app", RuleSet: "default"},
	{Repo: "playground/android-app", RuleSet: "all-rules", Flags: []string{"--all-rules"}},
}

type corpus struct {
	Repo    string
	RuleSet string
	Flags   []string
}

type baselineFile struct {
	Comment string             `json:"_comment"`
	Entries []fingerprintEntry `json:"entries"`
}

type fingerprintEntry struct {
	Repo        string `json:"repo"`
	RuleSet     string `json:"ruleSet"`
	Fingerprint string `json:"fingerprint"`
	MarkedFiles int    `json:"markedFiles"`
	TotalFiles  int    `json:"totalFiles"`
	AllFiles    bool   `json:"allFiles"`
}

type fingerprintReport struct {
	RuleSet     string   `json:"ruleSet"`
	TotalFiles  int      `json:"totalFiles"`
	MarkedFiles int      `json:"markedFiles"`
	AllFiles    bool     `json:"allFiles"`
	Fingerprint string   `json:"fingerprint"`
	OracleRules []string `json:"oracleRules"`
	Root        string   `json:"root"`
}

func main() {
	update := flag.Bool("update", false, "rewrite .krit/oracle-fingerprints.json with current fingerprints")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	observed, err := collect(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	baselinePath := filepath.Join(root, ".krit", "oracle-fingerprints.json")
	if *update {
		if err := writeBaseline(baselinePath, observed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("Wrote %s with %d entries.\n", filepath.ToSlash(".krit/oracle-fingerprints.json"), len(observed))
		return
	}

	baseline, err := loadBaseline(baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !check(observed, baseline) {
		os.Exit(1)
	}
	fmt.Printf("Oracle fingerprint gate: OK (%d corpora).\n", len(observed))
}

func repoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error: resolve repo root: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func collect(root string) ([]fingerprintEntry, error) {
	entries := make([]fingerprintEntry, 0, len(corpora))
	for _, c := range corpora {
		report, err := runKrit(root, c)
		if err != nil {
			return nil, err
		}
		entries = append(entries, fingerprintEntry{
			Repo:        c.Repo,
			RuleSet:     c.RuleSet,
			Fingerprint: report.Fingerprint,
			MarkedFiles: report.MarkedFiles,
			TotalFiles:  report.TotalFiles,
			AllFiles:    report.AllFiles,
		})
	}
	return entries, nil
}

func runKrit(root string, c corpus) (fingerprintReport, error) {
	krit := filepath.Join(root, "krit")
	if _, err := os.Stat(krit); err != nil {
		return fingerprintReport{}, fmt.Errorf("error: %s not found. Build with `go build -o krit ./cmd/krit/`.", krit)
	}

	args := append([]string{"--oracle-filter-fingerprint"}, c.Flags...)
	args = append(args, c.Repo)
	cmd := exec.Command(krit, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fingerprintReport{}, fmt.Errorf("error: %s exited: %w\nstderr:\n%s", commandString(krit, args), err, stderr.String())
	}

	var report fingerprintReport
	if err := json.Unmarshal(out, &report); err != nil {
		return fingerprintReport{}, fmt.Errorf("error: decode %s output: %w", commandString(krit, args), err)
	}
	return report, nil
}

func loadBaseline(path string) (map[string]fingerprintEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error: read %s: %w", path, err)
	}
	var file baselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("error: decode %s: %w", path, err)
	}

	out := make(map[string]fingerprintEntry, len(file.Entries))
	for _, entry := range file.Entries {
		out[key(entry.Repo, entry.RuleSet)] = entry
	}
	return out, nil
}

func writeBaseline(path string, entries []fingerprintEntry) error {
	payload := baselineFile{
		Comment: "Oracle filter input-set fingerprints per (repo, rule-set). Regenerate via `go run ./internal/devtools/oraclefpcheck --update`. See issue #333.",
		Entries: entries,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("error: encode baseline: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("error: write %s: %w", path, err)
	}
	return nil
}

func check(observed []fingerprintEntry, baseline map[string]fingerprintEntry) bool {
	drifted := false
	for _, current := range observed {
		previous, ok := baseline[key(current.Repo, current.RuleSet)]
		if !ok || previous.Fingerprint != current.Fingerprint {
			if !drifted {
				fmt.Fprintln(os.Stderr, "Oracle filter fingerprint drift detected:")
				fmt.Fprintln(os.Stderr)
			}
			drifted = true
			oldFingerprint := "<missing>"
			oldMarked := "-"
			oldTotal := "-"
			if ok {
				oldFingerprint = previous.Fingerprint
				oldMarked = fmt.Sprint(previous.MarkedFiles)
				oldTotal = fmt.Sprint(previous.TotalFiles)
			}
			fmt.Fprintf(os.Stderr, "  repo=%s ruleSet=%s\n", current.Repo, current.RuleSet)
			fmt.Fprintf(os.Stderr, "    old: fingerprint=%s marked=%s/%s\n", oldFingerprint, oldMarked, oldTotal)
			fmt.Fprintf(os.Stderr, "    new: fingerprint=%s marked=%d/%d allFiles=%t\n", current.Fingerprint, current.MarkedFiles, current.TotalFiles, current.AllFiles)
		}
	}
	if drifted {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "If this shift is intentional, update the baseline:")
		fmt.Fprintln(os.Stderr, "    go run ./internal/devtools/oraclefpcheck --update")
		fmt.Fprintln(os.Stderr, "Then commit `.krit/oracle-fingerprints.json` with your change.")
	}
	return !drifted
}

func key(repo, ruleSet string) string {
	return repo + "\x00" + ruleSet
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
