package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/perf"
)

type Record struct {
	Timestamp  time.Time          `json:"timestamp"`
	Version    string             `json:"version"`
	Commit     string             `json:"commit,omitempty"`
	Targets    []string           `json:"targets"`
	Summary    output.JSONSummary `json:"summary"`
	PerfTiming []perf.TimingEntry `json:"perfTiming,omitempty"`
}

type QueryOptions struct {
	Path  string
	Rule  string
	Since time.Time
}

type QueryRow struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
	Delta     int       `json:"delta,omitempty"`
}

func AppendRecord(path string, record Record) error {
	if path == "" {
		return fmt.Errorf("metrics output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(record)
}

func Query(opts QueryOptions) ([]QueryRow, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("metrics input path is required")
	}
	if opts.Rule == "" {
		return nil, fmt.Errorf("rule is required")
	}
	f, err := os.Open(opts.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	records, err := ReadRecords(f)
	if err != nil {
		return nil, err
	}
	rows := make([]QueryRow, 0, len(records))
	for _, record := range records {
		if !opts.Since.IsZero() && record.Timestamp.Before(opts.Since) {
			continue
		}
		rows = append(rows, QueryRow{
			Timestamp: record.Timestamp,
			Count:     record.Summary.ByRule[opts.Rule],
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Timestamp.Before(rows[j].Timestamp)
	})
	for i := 1; i < len(rows); i++ {
		rows[i].Delta = rows[i].Count - rows[i-1].Count
	}
	return rows, nil
}

func ReadRecords(r io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(r)
	var records []Record
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if text == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(text), &record); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func ParseSince(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q; use YYYY-MM-DD or RFC3339", value)
}
