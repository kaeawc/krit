package scan

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestParseDepthPreset(t *testing.T) {
	cases := []struct {
		in    string
		want  DepthPreset
		ok    bool
		empty bool
	}{
		{"", "", true, true},
		{"fast", DepthFast, true, false},
		{"balanced", DepthBalanced, true, false},
		{"thorough", DepthThorough, true, false},
		{"FAST", "", false, false},
		{"high", "", false, false},
	}
	for _, tc := range cases {
		got, ok := parseDepthPreset(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseDepthPreset(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestResolveDepthPresetCLIWins(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("analysis", "_top", "_unused", "noop") // ensure section exists; depth resolution reads its own key
	// Set analysis.depth via top-level data: GetTopLevelString reads the
	// raw map, so plant the value directly.
	cfg.Data()["analysis"] = map[string]interface{}{"depth": "balanced"}
	var buf bytes.Buffer
	got := resolveDepthPreset("fast", cfg, &buf)
	if got != DepthFast {
		t.Fatalf("CLI value should win: got %q, want %q", got, DepthFast)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected warning: %q", buf.String())
	}
}

func TestResolveDepthPresetConfigUsedWhenCLIEmpty(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Data()["analysis"] = map[string]interface{}{"depth": "thorough"}
	var buf bytes.Buffer
	got := resolveDepthPreset("", cfg, &buf)
	if got != DepthThorough {
		t.Fatalf("config value should be used: got %q, want %q", got, DepthThorough)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected warning: %q", buf.String())
	}
}

func TestResolveDepthPresetDefaultsToBalanced(t *testing.T) {
	cfg := config.NewConfig()
	var buf bytes.Buffer
	got := resolveDepthPreset("", cfg, &buf)
	if got != DepthBalanced {
		t.Fatalf("missing config + missing flag should default to balanced: got %q", got)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected warning: %q", buf.String())
	}
}

func TestResolveDepthPresetUnknownValueWarns(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Data()["analysis"] = map[string]interface{}{"depth": "wrongo"}
	var buf bytes.Buffer
	got := resolveDepthPreset("", cfg, &buf)
	if got != DepthBalanced {
		t.Fatalf("unknown config value should fall back to balanced: got %q", got)
	}
	if !strings.Contains(buf.String(), "wrongo") {
		t.Errorf("expected warning to mention bad value, got %q", buf.String())
	}
}

func TestResolveDepthPresetUnknownCLIWarns(t *testing.T) {
	cfg := config.NewConfig()
	var buf bytes.Buffer
	got := resolveDepthPreset("ultra", cfg, &buf)
	if got != DepthBalanced {
		t.Fatalf("unknown CLI value should fall back to balanced: got %q", got)
	}
	if !strings.Contains(buf.String(), "ultra") {
		t.Errorf("expected warning to mention bad value, got %q", buf.String())
	}
}

func TestApplyDepthPresetFastSetsNoTypeOracle(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthFast, f, fs)
	if !*f.NoTypeOracle {
		t.Errorf("DepthFast should set NoTypeOracle=true; got false")
	}
}

func TestApplyDepthPresetBalancedLeavesFlagsAlone(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthBalanced, f, fs)
	if *f.NoTypeOracle {
		t.Errorf("DepthBalanced should leave NoTypeOracle=false; got true")
	}
}

func TestApplyDepthPresetThoroughLeavesFlagsAlone(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthThorough, f, fs)
	if *f.NoTypeOracle {
		t.Errorf("DepthThorough should leave NoTypeOracle=false; got true")
	}
}

func TestApplyDepthPresetExplicitFlagWins(t *testing.T) {
	// User passes --no-type-oracle=false explicitly. Even if the depth
	// preset is fast, the explicit flag must win.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--no-type-oracle=false"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthFast, f, fs)
	if *f.NoTypeOracle {
		t.Errorf("explicit --no-type-oracle=false should win over DepthFast; got NoTypeOracle=true")
	}
}

func TestApplyDepthPresetExplicitTrueAlsoRespected(t *testing.T) {
	// User passes --no-type-oracle (=true). Depth balanced would normally
	// leave it alone anyway, but make sure the precedence holds.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--no-type-oracle"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthBalanced, f, fs)
	if !*f.NoTypeOracle {
		t.Errorf("explicit --no-type-oracle should remain true under DepthBalanced; got false")
	}
}

func TestApplyDepthPresetNilFlagSetAppliesUnconditionally(t *testing.T) {
	// Tests that pass nil FlagSet (no real CLI parse) get unconditional
	// application — this is the documented test-mode behavior.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--no-type-oracle=false"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthFast, f, nil)
	if !*f.NoTypeOracle {
		t.Errorf("nil FlagSet should apply unconditionally; got NoTypeOracle=false")
	}
}

func TestApplyDepthPresetThoroughEnablesFir(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthThorough, f, fs)
	if !*f.Fir {
		t.Errorf("DepthThorough should default Fir=true; got false")
	}
	if *f.NoFir {
		t.Errorf("DepthThorough should leave NoFir=false; got true")
	}
}

func TestApplyDepthPresetBalancedLeavesFirAlone(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthBalanced, f, fs)
	if *f.Fir {
		t.Errorf("DepthBalanced should leave Fir=false; got true")
	}
}

func TestApplyDepthPresetFastLeavesFirAlone(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthFast, f, fs)
	if *f.Fir {
		t.Errorf("DepthFast should leave Fir=false; got true")
	}
}

func TestApplyDepthPresetThoroughExplicitNoFirWins(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--no-fir"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthThorough, f, fs)
	if *f.Fir {
		t.Errorf("explicit --no-fir should suppress thorough's Fir default; got Fir=true")
	}
	if !*f.NoFir {
		t.Errorf("explicit --no-fir should remain set; got false")
	}
}

func TestApplyDepthPresetThoroughExplicitFirFalseWins(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--fir=false"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthThorough, f, fs)
	if *f.Fir {
		t.Errorf("explicit --fir=false should win over DepthThorough; got Fir=true")
	}
}

func TestApplyDepthPresetThoroughExplicitFirTrueRespected(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--fir"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyDepthPreset(DepthThorough, f, fs)
	if !*f.Fir {
		t.Errorf("explicit --fir should remain true under DepthThorough; got false")
	}
}
