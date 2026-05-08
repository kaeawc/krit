package score

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "json", "Output format: json or number")
	configPath := fs.String("config", "", "Path to krit.yml")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *format != "json" && *format != "number" {
		fmt.Fprintf(stderr, "score: unsupported format %q; use json or number\n", *format)
		return 1
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	report, code, err := scanReport(paths, *configPath)
	if err != nil {
		fmt.Fprintf(stderr, "score: %v\n", err)
		return code
	}
	score := output.ScoreFindings(report.Findings, report.Files, report.Rules)
	if *format == "number" {
		fmt.Fprintf(stdout, "%d\n", score.Score)
		return 0
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(score); err != nil {
		fmt.Fprintf(stderr, "score: encoding JSON: %v\n", err)
		return 1
	}
	return 0
}

type scanJSONReport struct {
	Files    int               `json:"files"`
	Rules    int               `json:"rules"`
	Findings []scanner.Finding `json:"findings"`
}

func scanReport(targets []string, configPath string) (scanJSONReport, int, error) {
	exe, err := os.Executable()
	if err != nil {
		return scanJSONReport{}, 1, err
	}
	tmpDir, err := os.MkdirTemp("", "krit-score-*")
	if err != nil {
		return scanJSONReport{}, 1, err
	}
	defer os.RemoveAll(tmpDir)
	reportPath := filepath.Join(tmpDir, "scan.json")
	args := []string{"-f", "json", "-o", reportPath}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	args = append(args, targets...)
	cmd := exec.CommandContext(context.Background(), exe, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		// Findings make the normal scan exit 1, but the JSON report is valid.
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			code := 1
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			}
			return scanJSONReport{}, code, fmt.Errorf("scan failed: %w: %s", err, stderr.String())
		}
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return scanJSONReport{}, 1, err
	}
	var report scanJSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		return scanJSONReport{}, 1, fmt.Errorf("parsing scan JSON: %w", err)
	}
	return report, 0, nil
}
