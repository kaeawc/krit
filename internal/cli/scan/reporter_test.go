package scan

import (
	"os"
	"testing"
)

func TestInstallDiagnosticReporterNonVerbose(t *testing.T) {
	r := installDiagnosticReporter(false)
	if r == nil {
		t.Fatal("expected non-nil reporter")
	}
	if r.Warning != os.Stderr {
		t.Errorf("Warning = %v; want os.Stderr", r.Warning)
	}
	if r.Verbose != nil {
		t.Errorf("Verbose = %v; want nil when verbose=false", r.Verbose)
	}
}

func TestInstallDiagnosticReporterVerbose(t *testing.T) {
	r := installDiagnosticReporter(true)
	if r == nil {
		t.Fatal("expected non-nil reporter")
	}
	if r.Warning != os.Stderr {
		t.Errorf("Warning = %v; want os.Stderr", r.Warning)
	}
	if r.Verbose != os.Stderr {
		t.Errorf("Verbose = %v; want os.Stderr when verbose=true", r.Verbose)
	}
}
