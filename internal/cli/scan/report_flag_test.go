package scan

import (
	"flag"
	"testing"
)

func TestReportFlagKeepsAliasAndCollectsTargets(t *testing.T) {
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--report", "sarif", "--report", "json=a.json", "--report", "checkstyle=out/c=1.xml"}); err != nil {
		t.Fatal(err)
	}
	if *f.Report != "sarif" {
		t.Fatalf("bare --report = %q, want the -f alias sarif", *f.Report)
	}
	want := []reportTarget{{"json", "a.json"}, {"checkstyle", "out/c=1.xml"}}
	if len(f.Reports) != len(want) || f.Reports[0] != want[0] || f.Reports[1] != want[1] {
		t.Fatalf("Reports = %v, want %v", f.Reports, want)
	}
	if !primaryOutputRequested(f, fs) {
		t.Fatal("a bare --report FORMAT asks for the primary output")
	}
}

func TestPrimaryOutputSkippedForReportFilesOnly(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"--report", "json=a.json"}, false},
		{[]string{"--report", "json=a.json", "-o", "b.json"}, true},
		{[]string{"--report", "json=a.json", "-f", "plain"}, true},
		{[]string{"--report", "json=a.json", "--format", "sarif"}, true},
	} {
		fs := flag.NewFlagSet("krit", flag.ContinueOnError)
		f := registerScanFlags(fs)
		if err := fs.Parse(tc.args); err != nil {
			t.Fatal(err)
		}
		if got := primaryOutputRequested(f, fs); got != tc.want {
			t.Errorf("%v: primaryOutputRequested = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestReportTargetsForceInProcessScan(t *testing.T) {
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if !daemonCompatibleFlags(f) {
		t.Fatal("default flags should be daemon compatible")
	}
	if err := fs.Parse([]string{"--report", "json=a.json"}); err != nil {
		t.Fatal(err)
	}
	if daemonCompatibleFlags(f) {
		t.Fatal("--report FORMAT=PATH must run in-process")
	}
}
