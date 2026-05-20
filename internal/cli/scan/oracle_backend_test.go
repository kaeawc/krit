package scan

import (
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
)

func TestOracleBackendResolution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		flagValue string
		yamlValue string
		want      oracle.Backend
		wantErr   bool
	}{
		{
			name: "neither set falls through to default",
			want: oracle.DefaultBackend,
		},
		{
			name:      "config-only is honoured",
			yamlValue: "fir",
			want:      oracle.BackendFIR,
		},
		{
			name:      "flag wins over config",
			flagValue: "kaa",
			yamlValue: "fir",
			want:      oracle.BackendKAA,
		},
		{
			name:      "flag wins even with reversed roles",
			flagValue: "fir",
			yamlValue: "kaa",
			want:      oracle.BackendFIR,
		},
		{
			name:      "alias canonicalization passes through",
			flagValue: "krit-fir",
			want:      oracle.BackendFIR,
		},
		{
			name:      "invalid flag surfaces error",
			flagValue: "garbage",
			wantErr:   true,
		},
		{
			name:      "invalid config surfaces error when no flag override",
			yamlValue: "nonsense",
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.NewConfigFromData(map[string]interface{}{
				"oracle": map[string]interface{}{"backend": tc.yamlValue},
			})
			got, err := resolveOracleBackend(tc.flagValue, cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for flag=%q yaml=%q, got backend=%q", tc.flagValue, tc.yamlValue, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for flag=%q yaml=%q: %v", tc.flagValue, tc.yamlValue, err)
			}
			if got != tc.want {
				t.Errorf("flag=%q yaml=%q: got %q, want %q", tc.flagValue, tc.yamlValue, got, tc.want)
			}
		})
	}
}

func TestOracleBackendResolutionWithNilConfig(t *testing.T) {
	// A nil-config caller is shaped like the CLI defaulting path
	// before krit.yml is loaded; the helper must not panic.
	got, err := resolveOracleBackend("", nil)
	if err != nil {
		t.Fatalf("nil-config resolution errored: %v", err)
	}
	if got != oracle.DefaultBackend {
		t.Errorf("nil-config resolution = %q, want %q", got, oracle.DefaultBackend)
	}
}
