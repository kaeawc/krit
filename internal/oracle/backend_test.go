package oracle

import (
	"strings"
	"testing"
)

// TestDefaultBackendIsFIR locks the default oracle backend to FIR, the
// only type-oracle backend.
func TestDefaultBackendIsFIR(t *testing.T) {
	t.Parallel()
	if DefaultBackend != BackendFIR {
		t.Errorf("DefaultBackend = %q, want %q", DefaultBackend, BackendFIR)
	}
}

func TestParseBackend(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    Backend
		wantErr bool
	}{
		{name: "empty resolves to default backend", input: "", want: DefaultBackend},
		{name: "canonical fir", input: "fir", want: BackendFIR},
		{name: "case-insensitive Fir", input: "Fir", want: BackendFIR},
		{name: "trims whitespace", input: "  fir\t", want: BackendFIR},
		{name: "alias krit-fir", input: "krit-fir", want: BackendFIR},
		{name: "alias k2", input: "k2", want: BackendFIR},
		{name: "unknown rejected", input: "foo", wantErr: true},
		{name: "no silent fallback for typo", input: "fir2", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseBackend(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseBackend(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseBackendRejectsRemovedKAASpellings guards that the removed
// krit-types (KAA) backend spellings fail loudly instead of silently
// selecting krit-fir.
func TestParseBackendRejectsRemovedKAASpellings(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"kaa", "KAA", "krit-types", "types", " Krit-Types "} {
		got, err := ParseBackend(input)
		if err == nil {
			t.Errorf("ParseBackend(%q) = %q, want error", input, got)
			continue
		}
		if !strings.Contains(err.Error(), "removed") {
			t.Errorf("ParseBackend(%q) error = %q, want it to mention the removal", input, err)
		}
	}
}

func TestBackendString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   Backend
		want string
	}{
		{in: BackendFIR, want: "fir"},
		// Empty backend renders as the default so verbose log lines
		// don't print a blank value when callers forget to set one.
		{in: "", want: string(DefaultBackend)},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("(%q).String() = %q, want %q", string(tc.in), got, tc.want)
		}
	}
}

func TestBackendJarName(t *testing.T) {
	t.Parallel()
	if got, want := BackendFIR.JarName(), "krit-fir.jar"; got != want {
		t.Errorf("BackendFIR.JarName() = %q, want %q", got, want)
	}
	if got, want := Backend("").JarName(), "krit-fir.jar"; got != want {
		t.Errorf("empty backend JarName() = %q, want %q", got, want)
	}
}
