package diag

import (
	"bytes"
	"testing"
)

func TestNilReporterIsSilent(t *testing.T) {
	var r *Reporter
	r.Verbosef("should not panic %d", 1)
	r.Warnf("should not panic %d", 2)
	if r.VerboseEnabled() {
		t.Fatal("VerboseEnabled() on nil receiver should be false")
	}
}

func TestVerbosef_RoutesToVerboseWriter(t *testing.T) {
	var v, w bytes.Buffer
	r := &Reporter{Verbose: &v, Warning: &w}
	r.Verbosef("hello %s\n", "world")

	if got := v.String(); got != "hello world\n" {
		t.Fatalf("verbose buffer = %q, want %q", got, "hello world\n")
	}
	if w.Len() != 0 {
		t.Fatalf("warning buffer should be empty, got %q", w.String())
	}
}

func TestWarnf_RoutesToWarningWriter(t *testing.T) {
	var v, w bytes.Buffer
	r := &Reporter{Verbose: &v, Warning: &w}
	r.Warnf("uh oh %d\n", 42)

	if got := w.String(); got != "uh oh 42\n" {
		t.Fatalf("warning buffer = %q, want %q", got, "uh oh 42\n")
	}
	if v.Len() != 0 {
		t.Fatalf("verbose buffer should be empty, got %q", v.String())
	}
}

func TestVerbosef_NilVerboseStreamIsSilent(t *testing.T) {
	var w bytes.Buffer
	r := &Reporter{Warning: &w}
	r.Verbosef("ignored")

	if w.Len() != 0 {
		t.Fatalf("warning buffer should be empty when only verbose stream is silenced, got %q", w.String())
	}
	if r.VerboseEnabled() {
		t.Fatal("VerboseEnabled() should be false when Verbose writer is nil")
	}
}

func TestWarnf_NilWarningStreamIsSilent(t *testing.T) {
	var v bytes.Buffer
	r := &Reporter{Verbose: &v}
	r.Warnf("ignored")

	if v.Len() != 0 {
		t.Fatalf("verbose buffer should not capture warnings, got %q", v.String())
	}
}

func TestVerboseEnabled(t *testing.T) {
	cases := []struct {
		name string
		r    *Reporter
		want bool
	}{
		{"nil receiver", nil, false},
		{"no writers", &Reporter{}, false},
		{"only warning", &Reporter{Warning: &bytes.Buffer{}}, false},
		{"only verbose", &Reporter{Verbose: &bytes.Buffer{}}, true},
		{"both", &Reporter{Verbose: &bytes.Buffer{}, Warning: &bytes.Buffer{}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.VerboseEnabled(); got != tc.want {
				t.Fatalf("VerboseEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewStderr_VerboseFalse(t *testing.T) {
	r := NewStderr(false)
	if r == nil {
		t.Fatal("NewStderr returned nil")
	}
	if r.Warning == nil {
		t.Fatal("NewStderr(false) should set Warning to stderr")
	}
	if r.Verbose != nil {
		t.Fatal("NewStderr(false) should leave Verbose nil")
	}
	if r.VerboseEnabled() {
		t.Fatal("NewStderr(false) should not enable verbose")
	}
}

func TestNewStderr_VerboseTrue(t *testing.T) {
	r := NewStderr(true)
	if r == nil {
		t.Fatal("NewStderr returned nil")
	}
	if r.Warning == nil {
		t.Fatal("NewStderr(true) should set Warning to stderr")
	}
	if r.Verbose == nil {
		t.Fatal("NewStderr(true) should set Verbose to stderr")
	}
	if !r.VerboseEnabled() {
		t.Fatal("NewStderr(true) should enable verbose")
	}
}

func TestFormatArgsPassedThroughVerbatim(t *testing.T) {
	var v bytes.Buffer
	r := &Reporter{Verbose: &v}
	r.Verbosef("no newline, no prefix")
	if got := v.String(); got != "no newline, no prefix" {
		t.Fatalf("Verbosef should not add newline or prefix, got %q", got)
	}
}
