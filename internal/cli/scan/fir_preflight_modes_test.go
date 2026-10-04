package scan

import (
	"flag"
	"testing"
)

func TestShouldPreflightFIRModes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"normal scan", nil, true},
		{"explicit fir", []string{"--fir"}, true},
		{"no fir", []string{"--no-fir"}, false},
		{"init", []string{"--init"}, false},
		{"version", []string{"--version"}, false},
		{"completions", []string{"--completions=bash"}, false},
		{"list rules", []string{"--list-rules"}, false},
		{"generate schema", []string{"--generate-schema"}, false},
		{"validate config", []string{"--validate-config"}, false},
		{"doctor", []string{"--doctor"}, false},
		{"clear cache", []string{"--clear-cache"}, false},
		{"clear matrix cache", []string{"--clear-matrix-cache"}, false},
		{"list experiments", []string{"--list-experiments"}, false},
		{"promote experiment", []string{"--promote-experiment=demo"}, false},
		{"deprecate experiment", []string{"--deprecate-experiment=demo"}, false},
		{"experiment matrix", []string{"--experiment-matrix=demo"}, false},
		{"new experiment", []string{"--new-experiment=demo"}, false},
		{"oracle filter fingerprint", []string{"--oracle-filter-fingerprint"}, false},
		{"output types", []string{"--output-types=types.json"}, false},
		{"dump oracle diagnostics", []string{"--dump-oracle-diagnostics"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			f := registerScanFlags(fs)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			if got := shouldPreflightFIR(f); got != tt.want {
				t.Fatalf("shouldPreflightFIR(%v) = %t, want %t", tt.args, got, tt.want)
			}
		})
	}
}
