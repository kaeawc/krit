package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

// `krit --daemon --fir` must behave like a direct `krit --fir`: the wire
// carries the FIR switches so the daemon runs the same verdict pass.
func TestBuildDaemonAnalyzeArgs_ForwardsFir(t *testing.T) {
	cases := []struct {
		name                 string
		fir, noFir, noDaemon bool
		wantFir, wantNoDaemo bool
	}{
		{name: "off by default"},
		{name: "--fir", fir: true, wantFir: true},
		{name: "--fir --no-fir", fir: true, noFir: true},
		{name: "--fir --no-fir-daemon", fir: true, noDaemon: true, wantFir: true, wantNoDaemo: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.Fir, *f.NoFir, *f.NoFirDaemon = tc.fir, tc.noFir, tc.noDaemon
			if !daemonCompatibleFlags(f) {
				t.Fatal("FIR flags must not bypass daemon delegation")
			}
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.Fir != tc.wantFir || args.NoFirDaemon != tc.wantNoDaemo {
				t.Fatalf("Fir=%v NoFirDaemon=%v, want %v %v", args.Fir, args.NoFirDaemon, tc.wantFir, tc.wantNoDaemo)
			}
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(raw), `"fir":true`); got != tc.wantFir {
				t.Fatalf("wire %s: fir present=%v, want %v", raw, got, tc.wantFir)
			}
		})
	}
}
