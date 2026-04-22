package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

// InvocationOptions carries optional diagnostics for oracle invocations.
// A nil or disabled tracker keeps the existing low-overhead behavior.
type InvocationOptions struct {
	Tracker perf.Tracker
}

func (o InvocationOptions) tracker() perf.Tracker {
	if o.Tracker == nil {
		return perf.New(false)
	}
	return o.Tracker
}

func trackOracle(t perf.Tracker, name string, fn func() error) error {
	if t == nil {
		return fn()
	}
	return t.Track(name, fn)
}

func addOracleEntry(t perf.Tracker, name string, start time.Time, metrics map[string]int64, attrs map[string]string) {
	perf.AddEntryDetails(t, name, time.Since(start), metrics, attrs)
}

func addOracleInstant(t perf.Tracker, name string, metrics map[string]int64, attrs map[string]string) {
	perf.AddEntryDetails(t, name, 0, metrics, attrs)
}

func addKotlinTimingsFromFile(t perf.Tracker, path string) {
	if t == nil || !t.IsEnabled() || path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		addOracleInstant(t, "kotlinTimingsReadError", nil, map[string]string{"error": err.Error()})
		return
	}
	addKotlinTimings(t, data)
}

func addKotlinTimings(t perf.Tracker, data []byte) {
	if t == nil || !t.IsEnabled() || len(data) == 0 {
		return
	}
	var entries []perf.TimingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		addOracleInstant(t, "kotlinTimingsParseError", nil, map[string]string{"error": err.Error()})
		return
	}
	if len(entries) == 0 {
		return
	}
	kt := t.Serial("kotlinTimings")
	perf.AddEntries(kt, entries)
	kt.End()
}

func tempTimingsPath() (string, func(), error) {
	f, err := os.CreateTemp("", "krit-types-timings-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("tempfile (timings): %w", err)
	}
	name := f.Name()
	_ = f.Close()
	return name, func() { _ = os.Remove(name) }, nil
}
