package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/metrics"
	"github.com/kaeawc/krit/internal/output"
)

func TestToolMetricsQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	t1, _ := time.Parse(time.RFC3339, "2024-01-15T00:00:00Z")
	t2, _ := time.Parse(time.RFC3339, "2024-04-01T00:00:00Z")
	_ = metrics.AppendRecord(path, metrics.Record{Timestamp: t1, Summary: output.JSONSummary{ByRule: map[string]int{"LongMethod": 412}}})
	_ = metrics.AppendRecord(path, metrics.Record{Timestamp: t2, Summary: output.JSONSummary{ByRule: map[string]int{"LongMethod": 164}}})

	server := &Server{}
	args, _ := json.Marshal(metricsArgs{Operation: "query", Path: path, Rule: "LongMethod", Since: "2024-01-01"})
	result := server.toolMetrics(args)
	if result.IsError {
		t.Fatalf("unexpected error: %#v", result)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"count": 412`) || !strings.Contains(text, `"delta": -248`) {
		t.Fatalf("unexpected metrics result: %s", text)
	}
}

func TestToolMetricsRequiresRule(t *testing.T) {
	server := &Server{}
	result := server.toolMetrics(json.RawMessage(`{"operation":"query"}`))
	if !result.IsError {
		t.Fatal("expected error")
	}
	if !strings.Contains(result.Content[0].Text, "rule") {
		t.Fatalf("unexpected error: %#v", result)
	}
}
