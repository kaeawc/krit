package scan

import (
	"flag"
	"runtime"
	"testing"
)

func TestRegisterScanFlagsAllFieldsPopulated(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if f == nil {
		t.Fatal("registerScanFlags returned nil")
	}
	// Spot-check that every field got a non-nil pointer. A typo in the
	// big bind block would leave one nil, which would panic at runtime
	// the moment Run dereferenced it.
	checks := []struct {
		name string
		ptr  any
	}{
		{"Format", f.Format},
		{"Report", f.Report},
		{"Perf", f.Perf},
		{"PerfRules", f.PerfRules},
		{"CPUProfile", f.CPUProfile},
		{"MemProfile", f.MemProfile},
		{"Jobs", f.Jobs},
		{"Quiet", f.Quiet},
		{"Verbose", f.Verbose},
		{"FixLevel", f.FixLevel},
		{"Baseline", f.Baseline},
		{"NoCache", f.NoCache},
		{"StoreDir", f.StoreDir},
		{"List", f.List},
		{"InputTypes", f.InputTypes},
		{"OutputTypes", f.OutputTypes},
		{"Fir", f.Fir},
		{"WarningsAsErrors", f.WarningsAsErrors},
		{"MinConfidence", f.MinConfidence},
		{"Diff", f.Diff},
		{"DisableRules", f.DisableRules},
		{"MaxCost", f.MaxCost},
		{"Experiment", f.Experiment},
		{"ExperimentMatrix", f.ExperimentMatrix},
		{"PromoteExperiment", f.PromoteExperiment},
		{"NewExperiment", f.NewExperiment},
		{"SampleRule", f.SampleRule},
		{"RuleAudit", f.RuleAudit},
		{"BaselineAudit", f.BaselineAudit},
	}
	for _, c := range checks {
		if c.ptr == nil {
			t.Errorf("%s pointer is nil", c.name)
		}
	}
}

func TestRegisterScanFlagsKeyDefaults(t *testing.T) {
	// The defaults are part of the user-facing CLI contract — guard
	// against accidental drift if someone re-binds a flag.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)

	if got := *f.Format; got != "json" {
		t.Errorf("Format default = %q; want \"json\"", got)
	}
	if got := *f.FixLevel; got != "idiomatic" {
		t.Errorf("FixLevel default = %q; want \"idiomatic\"", got)
	}
	if got := *f.Jobs; got != runtime.NumCPU() {
		t.Errorf("Jobs default = %d; want runtime.NumCPU() = %d", got, runtime.NumCPU())
	}
	if got := *f.SampleCount; got != 10 {
		t.Errorf("SampleCount default = %d; want 10", got)
	}
	if got := *f.SampleContext; got != 3 {
		t.Errorf("SampleContext default = %d; want 3", got)
	}
	if got := *f.ExperimentRuns; got != 3 {
		t.Errorf("ExperimentRuns default = %d; want 3", got)
	}
	if got := *f.NewExperimentIntent; got != "fp-reduction" {
		t.Errorf("NewExperimentIntent default = %q; want \"fp-reduction\"", got)
	}
	if got := *f.RuleAuditMin; got != 1 {
		t.Errorf("RuleAuditMin default = %d; want 1", got)
	}
}

func TestRegisterScanFlagsFormatAlias(t *testing.T) {
	// --format is an alias for -f via flag.StringVar; both names must
	// share storage so that --format=plain mutates the same value -f
	// would. This is the contract the auto-format logic in Run depends on.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--format", "sarif"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := *f.Format; got != "sarif" {
		t.Fatalf("Format after --format=sarif = %q; want \"sarif\"", got)
	}
}
