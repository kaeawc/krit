package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

func oracleBackendEntries(tr perf.Tracker) []perf.TimingEntry {
	var out []perf.TimingEntry
	for _, e := range tr.GetTimings() {
		if e.Name == "oracleBackend" {
			out = append(out, e)
		}
	}
	return out
}

func TestRecordResidentOracleBackend(t *testing.T) {
	tr := perf.New(true)
	recordResidentOracleBackend(tr, oracle.BackendFIR, nil)
	if got := oracleBackendEntries(tr); len(got) != 0 {
		t.Fatalf("failed analysis recorded the oracle as having run: %v", got)
	}
	recordResidentOracleBackend(tr, "", &oracle.Data{})
	if got := oracleBackendEntries(tr); len(got) != 0 {
		t.Fatalf("unknown backend recorded: %v", got)
	}
	recordResidentOracleBackend(tr, oracle.BackendFIR, &oracle.Data{})
	got := oracleBackendEntries(tr)
	if len(got) != 1 || got[0].Attributes["backend"] != "fir" || got[0].Attributes["source"] != "resident-daemon" {
		t.Fatalf("entries = %+v", got)
	}
}
