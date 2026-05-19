package metrics

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/output"
)

type failingCloseBuffer struct {
	bytes.Buffer
	closeErr error
}

func (f *failingCloseBuffer) Close() error { return f.closeErr }

type failingWriter struct {
	writeErr error
	closeErr error
	closed   bool
}

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *failingWriter) Close() error {
	f.closed = true
	return f.closeErr
}

func TestAppendRecordToSurfacesCloseError(t *testing.T) {
	buf := &failingCloseBuffer{closeErr: errors.New("disk full")}
	err := appendRecordTo(buf, Record{Version: "v"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("want close error, got %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected encoder to write before close fails")
	}
}

func TestAppendRecordToReturnsEncodeErrorEvenIfCloseAlsoFails(t *testing.T) {
	w := &failingWriter{writeErr: errors.New("encode boom"), closeErr: errors.New("close boom")}
	err := appendRecordTo(w, Record{Version: "v"})
	if err == nil || !strings.Contains(err.Error(), "encode boom") {
		t.Fatalf("want encode error to win, got %v", err)
	}
	if !w.closed {
		t.Fatalf("Close must still run even when Encode fails")
	}
}

func TestAppendRecordToReturnsNilOnCleanCloseAndEncode(t *testing.T) {
	buf := &failingCloseBuffer{}
	if err := appendRecordTo(buf, Record{Version: "v"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

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
