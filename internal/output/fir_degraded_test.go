package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

func degradedFixture() *FirDegraded {
	return &FirDegraded{Reason: "no classpath", FallbackRules: []string{"RuleA", "RuleB"}}
}

func TestJSONSummaryFirDegraded(t *testing.T) {
	cols := scanner.CollectFindings(nil)
	for _, tc := range []struct {
		name string
		d    *FirDegraded
		want bool
	}{{"degraded", degradedFixture(), true}, {"healthy", nil, false}} {
		for _, compact := range []bool{false, true} {
			var buf bytes.Buffer
			err := FormatJSONColumnsWithOptions(&buf, &cols, "v", 0, 0, time.Now(), nil, nil, nil, nil, nil, nil,
				JSONOptions{Compact: compact, FirDegraded: tc.d})
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Summary struct {
					FirDegraded *FirDegraded `json:"firDegraded"`
				} `json:"summary"`
			}
			if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if got := report.Summary.FirDegraded != nil; got != tc.want {
				t.Fatalf("%s compact=%v: firDegraded present=%v\n%s", tc.name, compact, got, buf.String())
			}
			if tc.want && len(report.Summary.FirDegraded.FallbackRules) != 2 {
				t.Fatalf("fallback rules lost: %+v", report.Summary.FirDegraded)
			}
		}
	}
}

func TestSARIFFirDegradedNotification(t *testing.T) {
	cols := scanner.CollectFindings(nil)
	var degraded, healthy bytes.Buffer
	if err := FormatSARIFColumnsDegraded(&degraded, &cols, "v", degradedFixture()); err != nil {
		t.Fatal(err)
	}
	if err := FormatSARIFColumns(&healthy, &cols, "v"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(degraded.String(), "toolExecutionNotifications") || !strings.Contains(degraded.String(), "FIR rules were skipped: no classpath") {
		t.Fatalf("missing notification: %s", degraded.String())
	}
	if strings.Contains(healthy.String(), "invocations") {
		t.Fatalf("healthy SARIF must be unchanged: %s", healthy.String())
	}
}
