package oracle

import (
	"fmt"
	"path/filepath"
	"testing"
)

// #957: the oracle-only analyzeAll names krit-fir's bundle directory as an
// absolute path, and only to a daemon launched as krit-fir.
func TestAnalyzeAllWithCallFilterInNamesTheDirToKritFirOnly(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, tc := range []struct {
		name    string
		backend Backend
		in      string
		want    any
	}{
		{"fir", BackendFIR, filepath.Join(".krit", "fir-incremental"), filepath.Join(dir, ".krit", "fir-incremental")},
		{"fir without a dir", BackendFIR, "", nil},
		{"kaa", BackendKAA, ".krit/fir-incremental", nil},
		{"unknown backend", "", ".krit/fir-incremental", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got any
			d := pipeDaemon(t, func(req daemonRequest) string {
				if req.Method != "analyzeAll" {
					t.Errorf("method = %q", req.Method)
				}
				got = req.Params["incrementalDir"]
				return fmt.Sprintf(`{"id":%d,"result":{"version":1,"files":{}}}`, req.ID)
			})
			d.backend = tc.backend
			if _, err := d.AnalyzeAllWithCallFilterIn(nil, tc.in); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("incrementalDir = %v, want %v", got, tc.want)
			}
		})
	}
}
