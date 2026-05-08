package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/output"
)

func TestAppendRecordAndQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".krit", "metrics.jsonl")
	mustTime := func(value string) time.Time {
		t.Helper()
		ts, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	records := []Record{
		{Timestamp: mustTime("2024-01-15T00:00:00Z"), Version: "dev", Targets: []string{"."}, Summary: output.JSONSummary{Total: 4, ByRule: map[string]int{"LongMethod": 4}}},
		{Timestamp: mustTime("2024-04-01T00:00:00Z"), Version: "dev", Targets: []string{"."}, Summary: output.JSONSummary{Total: 2, ByRule: map[string]int{"LongMethod": 2}}},
	}
	for _, record := range records {
		if err := AppendRecord(path, record); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := Query(QueryOptions{Path: path, Rule: "LongMethod"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Count != 4 || rows[0].Delta != 0 {
		t.Fatalf("first row = %#v", rows[0])
	}
	if rows[1].Count != 2 || rows[1].Delta != -2 {
		t.Fatalf("second row = %#v", rows[1])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "\n"); got != 2 {
		t.Fatalf("newline count = %d, want 2 appended JSONL rows", got)
	}
}

func TestQuerySince(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	oldTime, _ := time.Parse(time.RFC3339, "2024-01-15T00:00:00Z")
	newTime, _ := time.Parse(time.RFC3339, "2024-04-01T00:00:00Z")
	_ = AppendRecord(path, Record{Timestamp: oldTime, Summary: output.JSONSummary{ByRule: map[string]int{"MagicNumber": 10}}})
	_ = AppendRecord(path, Record{Timestamp: newTime, Summary: output.JSONSummary{ByRule: map[string]int{"MagicNumber": 3}}})
	since, err := ParseSince("2024-04-01")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := Query(QueryOptions{Path: path, Rule: "MagicNumber", Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Count != 3 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestReadRecordsRejectsMalformedJSONL(t *testing.T) {
	_, err := ReadRecords(strings.NewReader(`{"timestamp":"2024-01-01T00:00:00Z"}` + "\nnot-json\n"))
	if err == nil {
		t.Fatal("expected malformed JSONL error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %v", err)
	}
}
