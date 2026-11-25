package scan

import (
	"flag"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/output"
)

func coverageFlags(t *testing.T, args ...string) *scanFlags {
	t.Helper()
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestReportFirCoverageLineAndThreshold(t *testing.T) {
	low := &output.FirCoverage{Authoritative: 1, Gated: 3}
	full := &output.FirCoverage{Authoritative: 4}
	for _, tc := range []struct {
		name     string
		args     []string
		cov      *output.FirCoverage
		degraded bool
		code     int
		want     []string
		notWant  []string
	}{
		{name: "line without threshold", cov: low, want: []string{"info: FIR analyzed 1 of 4 files (25%); 3 gated"}},
		{name: "quiet", args: []string{"-q"}, cov: low, notWant: []string{"FIR analyzed"}},
		{name: "nothing gated", cov: full, notWant: []string{"FIR analyzed"}},
		{name: "below threshold", args: []string{"--min-fir-coverage", "0.8"}, cov: low, code: 2,
			want: []string{"error: --min-fir-coverage=0.8: FIR analyzed 25.0% of the files it checked (1 of 4; 3 gated)"}},
		{name: "at threshold", args: []string{"--min-fir-coverage", "0.25"}, cov: low},
		{name: "degraded", args: []string{"--min-fir-coverage", "0.5"}, degraded: true, code: 2, want: []string{"the FIR pass did not run"}},
		{name: "fir on, no coverage", args: []string{"--min-fir-coverage", "0.5"}, code: 2, want: []string{"the FIR pass did not run"}},
		{name: "no-fir", args: []string{"--no-fir", "--min-fir-coverage", "0.5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			code := reportFirCoverage(&out, coverageFlags(t, tc.args...), tc.cov, tc.degraded)
			if code != tc.code {
				t.Fatalf("code = %d, want %d; output %q", code, tc.code, out.String())
			}
			for _, w := range tc.want {
				if !strings.Contains(out.String(), w) {
					t.Errorf("output %q lacks %q", out.String(), w)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(out.String(), w) {
					t.Errorf("output %q has %q", out.String(), w)
				}
			}
		})
	}
}

func TestMinFirCoverageRunsInProcess(t *testing.T) {
	if daemonCompatibleFlags(coverageFlags(t, "--min-fir-coverage", "0.9")) {
		t.Fatal("--min-fir-coverage must run in-process")
	}
}
