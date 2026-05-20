package oracle

import (
	"testing"
)

func TestParseBackend(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    Backend
		wantErr bool
	}{
		{name: "empty defaults to KAA", input: "", want: DefaultBackend},
		{name: "canonical kaa", input: "kaa", want: BackendKAA},
		{name: "canonical fir", input: "fir", want: BackendFIR},
		{name: "case-insensitive KAA", input: "KAA", want: BackendKAA},
		{name: "case-insensitive Fir", input: "Fir", want: BackendFIR},
		{name: "trims whitespace", input: "  fir\t", want: BackendFIR},
		{name: "alias krit-types", input: "krit-types", want: BackendKAA},
		{name: "alias krit-fir", input: "krit-fir", want: BackendFIR},
		{name: "alias types", input: "types", want: BackendKAA},
		{name: "alias k2", input: "k2", want: BackendFIR},
		{name: "unknown rejected", input: "foo", wantErr: true},
		// Aliases mustn't bleed across backends — the test above
		// fails closed if someone accidentally maps "k2" to KAA.
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

func TestBackendString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   Backend
		want string
	}{
		{in: BackendKAA, want: "kaa"},
		{in: BackendFIR, want: "fir"},
		// Empty backend renders as the default so verbose log lines
		// don't print a blank value when callers forget to set one.
		{in: "", want: "kaa"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("(%q).String() = %q, want %q", string(tc.in), got, tc.want)
		}
	}
}

func TestBackendJarName(t *testing.T) {
	t.Parallel()
	if got, want := BackendKAA.JarName(), "krit-types.jar"; got != want {
		t.Errorf("BackendKAA.JarName() = %q, want %q", got, want)
	}
	if got, want := BackendFIR.JarName(), "krit-fir.jar"; got != want {
		t.Errorf("BackendFIR.JarName() = %q, want %q", got, want)
	}
	// Unknown backend falls back to the KAA jar — surfacing it as an
	// error in JarName would force every call site to handle the
	// failure mode, but the canonical surface is ParseBackend.
	if got := Backend("garbage").JarName(); got != "krit-types.jar" {
		t.Errorf("unknown backend jar name should fall back to krit-types.jar, got %q", got)
	}
}
