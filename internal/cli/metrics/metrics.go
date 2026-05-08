package metricscli

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
	"time"

	"github.com/kaeawc/krit/internal/metrics"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/perf"
)

const defaultMetricsPath = ".krit/metrics.jsonl"

func Run(args []string, version string) int {
	return run(args, version, os.Stdout, os.Stderr)
}

func run(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 1
	}
	switch args[0] {
	case "log":
		return runLog(args[1:], version, stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "metrics: unknown subcommand %q\n", args[0])
		usage(stderr)
		return 1
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  krit metrics log [--out .krit/metrics.jsonl] [--config krit.yml] [paths...]")
	fmt.Fprintln(w, "  krit metrics query <RuleName> [--in .krit/metrics.jsonl] [--since YYYY-MM-DD]")
}

func runLog(args []string, version string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("metrics log", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("out", defaultMetricsPath, "Append metrics JSONL to this file")
	configPath := fs.String("config", "", "krit.yml path to pass through to scan")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}
	report, code, err := scanJSONReport(targets, *configPath)
	if err != nil {
		fmt.Fprintf(stderr, "metrics log: %v\n", err)
		return code
	}
	record := metrics.Record{
		Timestamp:  time.Now().UTC(),
		Version:    firstNonEmpty(report.Version, version),
		Commit:     currentGitCommit(),
		Targets:    append([]string(nil), targets...),
		Summary:    report.Summary,
		PerfTiming: report.PerfTiming,
	}
	if err := metrics.AppendRecord(*outPath, record); err != nil {
		fmt.Fprintf(stderr, "metrics log: writing %s: %v\n", *outPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "appended metrics to %s\n", *outPath)
	return 0
}

type scanReport struct {
	Version    string             `json:"version"`
	Summary    output.JSONSummary `json:"summary"`
	PerfTiming []perf.TimingEntry `json:"perfTiming,omitempty"`
}

func scanJSONReport(targets []string, configPath string) (scanReport, int, error) {
	exe, err := os.Executable()
	if err != nil {
		return scanReport{}, 1, err
	}
	tmpDir, err := os.MkdirTemp("", "krit-metrics-*")
	if err != nil {
		return scanReport{}, 1, err
	}
	defer os.RemoveAll(tmpDir)
	reportPath := filepath.Join(tmpDir, "scan.json")
	args := []string{"-f", "json", "--perf", "-o", reportPath}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	args = append(args, targets...)
	cmd := exec.CommandContext(context.Background(), exe, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		// A normal krit scan exits 1 when findings are present; the JSON report
		// is still complete and is the data metrics log needs.
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			code := 1
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			}
			return scanReport{}, code, fmt.Errorf("scan failed: %w: %s", err, stderr.String())
		}
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return scanReport{}, 1, err
	}
	var report scanReport
	if err := json.Unmarshal(data, &report); err != nil {
		return scanReport{}, 1, fmt.Errorf("parsing scan JSON: %w", err)
	}
	return report, 0, nil
}

func runQuery(args []string, stdout, stderr io.Writer) int {
	positional, flags := splitRuleArg(args)
	fs := flag.NewFlagSet("metrics query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inPath := fs.String("in", defaultMetricsPath, "Read metrics JSONL from this file")
	sinceText := fs.String("since", "", "Only include records on or after YYYY-MM-DD or RFC3339 timestamp")
	if err := fs.Parse(flags); err != nil {
		return 1
	}
	if len(positional) == 0 {
		fmt.Fprintln(stderr, "metrics query: rule name is required")
		return 1
	}
	since, err := metrics.ParseSince(*sinceText)
	if err != nil {
		fmt.Fprintf(stderr, "metrics query: %v\n", err)
		return 1
	}
	rows, err := metrics.Query(metrics.QueryOptions{Path: *inPath, Rule: positional[0], Since: since})
	if err != nil {
		fmt.Fprintf(stderr, "metrics query: %v\n", err)
		return 1
	}
	for i, row := range rows {
		date := row.Timestamp.UTC().Format("2006-01-02")
		if i == 0 {
			fmt.Fprintf(stdout, "%s: %d\n", date, row.Count)
		} else {
			fmt.Fprintf(stdout, "%s: %d (%+d)\n", date, row.Count, row.Delta)
		}
	}
	return 0
}

func splitRuleArg(args []string) (positional, flags []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--in" || arg == "--since" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		if arg == "-h" || arg == "--help" || hasFlagValue(arg, "--in=") || hasFlagValue(arg, "--since=") {
			flags = append(flags, arg)
			continue
		}
		if len(positional) == 0 && len(arg) > 0 && arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
	}
	return positional, flags
}

func hasFlagValue(arg, prefix string) bool {
	return len(arg) > len(prefix) && arg[:len(prefix)] == prefix
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func currentGitCommit() string {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}
