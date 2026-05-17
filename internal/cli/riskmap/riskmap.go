package riskmap

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type Entry struct {
	File       string `json:"file"`
	Churn      int    `json:"churn"`
	Complexity int    `json:"complexity"`
	Risk       int    `json:"risk"`
}

type Report struct {
	Since string  `json:"since"`
	Files []Entry `json:"files"`
}

func Run(args []string) int {
	fs := flag.NewFlagSet("risk-map", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	since := fs.String("since", "90d", "Git history window, passed to git log --since")
	format := fs.String("format", "plain", "Output format: plain or json")
	root := fs.String("root", "", "Repository root. Defaults to current directory.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	wd := *root
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}
	report, err := Analyze(wd, *since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return 0
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	case "plain":
		for _, entry := range report.Files {
			fmt.Printf("%s\tchurn=%d complexity=%d risk=%d\n", entry.File, entry.Churn, entry.Complexity, entry.Risk)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q\n", *format)
		return 1
	}
	return 0
}

func Analyze(root, since string) (Report, error) {
	churn, err := gitChurn(root, since)
	if err != nil {
		return Report{}, err
	}
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return Report{}, err
	}
	files, errs := scanner.ScanFiles(context.Background(), paths, 1)
	if len(errs) > 0 {
		return Report{}, errs[0]
	}
	var entries []Entry
	for _, file := range files {
		if file == nil {
			continue
		}
		rel := relPath(root, file.Path)
		c := churn[rel]
		if c == 0 {
			if abs, err := filepath.Abs(file.Path); err == nil {
				c = churn[filepath.ToSlash(abs)]
			}
		}
		if c == 0 {
			continue
		}
		complexity := fileComplexity(file.Lines)
		entries = append(entries, Entry{
			File:       rel,
			Churn:      c,
			Complexity: complexity,
			Risk:       c * complexity,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Risk != entries[j].Risk {
			return entries[i].Risk > entries[j].Risk
		}
		if entries[i].Churn != entries[j].Churn {
			return entries[i].Churn > entries[j].Churn
		}
		return entries[i].File < entries[j].File
	})
	return Report{Since: since, Files: entries}, nil
}

func gitChurn(root, since string) (map[string]int, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "log", "--since="+since, "--name-only", "--pretty=format:")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log unavailable")
	}
	churn := make(map[string]int)
	seenInCommit := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			seenInCommit = make(map[string]bool)
			continue
		}
		if !strings.HasSuffix(line, ".kt") && !strings.HasSuffix(line, ".kts") {
			continue
		}
		line = filepath.ToSlash(line)
		if seenInCommit[line] {
			continue
		}
		seenInCommit[line] = true
		churn[line]++
		if abs, err := filepath.Abs(filepath.Join(root, line)); err == nil {
			churn[filepath.ToSlash(abs)]++
		}
	}
	return churn, nil
}

var decisionRe = regexp.MustCompile(`\b(if|else\s+if|when|for|while|catch)\b|&&|\|\||\?:`)

func fileComplexity(lines []string) int {
	complexity := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		complexity += len(decisionRe.FindAllString(trimmed, -1))
		if strings.Contains(trimmed, " fun ") || strings.HasPrefix(trimmed, "fun ") {
			complexity++
		}
	}
	if complexity == 0 {
		return 1
	}
	return complexity
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
