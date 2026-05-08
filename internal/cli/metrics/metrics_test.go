package metricscli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/metrics"
	"github.com/kaeawc/krit/internal/output"
)

func TestRunQueryPrintsDeltas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	t1, _ := time.Parse(time.RFC3339, "2024-01-15T00:00:00Z")
	t2, _ := time.Parse(time.RFC3339, "2024-04-01T00:00:00Z")
	if err := metrics.AppendRecord(path, metrics.Record{Timestamp: t1, Summary: output.JSONSummary{ByRule: map[string]int{"LongMethod": 412}}}); err != nil {
		t.Fatal(err)
	}
	if err := metrics.AppendRecord(path, metrics.Record{Timestamp: t2, Summary: output.JSONSummary{ByRule: map[string]int{"LongMethod": 164}}}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"query", "LongMethod", "--in", path, "--since", "2024-01-01"}, "dev", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	want := "2024-01-15: 412\n2024-04-01: 164 (-248)\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunQueryAcceptsFlagsBeforeRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	t1, _ := time.Parse(time.RFC3339, "2024-01-15T00:00:00Z")
	if err := metrics.AppendRecord(path, metrics.Record{Timestamp: t1, Summary: output.JSONSummary{ByRule: map[string]int{"MagicNumber": 3}}}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"query", "--in", path, "MagicNumber"}, "dev", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); got != "2024-01-15: 3\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"wat"}, "dev", &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
