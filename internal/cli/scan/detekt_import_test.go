package scan

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func detektScanProject(t *testing.T, withBaseline bool) (root string, cfg *config.Config) {
	t.Helper()
	root = t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "config", "detekt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := "complexity:\n  LongMethod:\n    threshold: 120\n  Unmapped:\n    active: true\nbuild:\n  maxIssues: 0\n"
	if err := os.WriteFile(filepath.Join(dir, "detekt.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if withBaseline {
		if err := os.WriteFile(filepath.Join(dir, "baseline.xml"), []byte("<SmellBaseline/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadAndMergeDefaultsWith(config.LoadOptions{}, "", root)
	if err != nil {
		t.Fatal(err)
	}
	return root, cfg
}

func newTestFlags(t *testing.T, args ...string) *scanFlags {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDetektFlagsMapToLoadOptions(t *testing.T) {
	f := newTestFlags(t, "--detekt-config", "x/detekt.yml", "--no-detekt-config")
	opts := detektLoadOptions(f)
	if opts.DetektConfigFile != "x/detekt.yml" || !opts.NoDetekt {
		t.Errorf("opts = %+v", opts)
	}
	if opts := detektLoadOptions(newTestFlags(t)); opts != (config.LoadOptions{}) {
		t.Errorf("default opts = %+v, want zero", opts)
	}
}

func TestApplyDetektBaseline(t *testing.T) {
	_, cfg := detektScanProject(t, true)
	want := cfg.Detekt().Baseline
	if want == "" {
		t.Fatal("baseline.xml next to the detekt config should be discovered")
	}

	f := newTestFlags(t)
	applyDetektBaseline(f, cfg)
	if *f.Baseline != want {
		t.Errorf("Baseline = %q, want %q", *f.Baseline, want)
	}

	explicit := newTestFlags(t, "--baseline", "mine.xml")
	applyDetektBaseline(explicit, cfg)
	if *explicit.Baseline != "mine.xml" {
		t.Errorf("explicit --baseline replaced by %q", *explicit.Baseline)
	}

	for _, args := range [][]string{{"--create-baseline", "out.xml"}, {"--baseline-audit"}, {"--delta", "main"}} {
		f := newTestFlags(t, args...)
		applyDetektBaseline(f, cfg)
		if *f.Baseline != "" {
			t.Errorf("%v must not pick up the detekt baseline, got %q", args, *f.Baseline)
		}
	}
}

func TestApplyDetektBaselineWithoutDetekt(t *testing.T) {
	f := newTestFlags(t)
	applyDetektBaseline(f, config.NewConfig())
	if *f.Baseline != "" {
		t.Errorf("Baseline = %q, want empty", *f.Baseline)
	}
}

func TestReportDetektImportVerbose(t *testing.T) {
	root, cfg := detektScanProject(t, false)
	var quiet, loud bytes.Buffer
	reportDetektImport(false, cfg, &quiet)
	if quiet.Len() != 0 {
		t.Errorf("non-verbose run printed %q", quiet.String())
	}
	reportDetektImport(true, cfg, &loud)
	out := loud.String()
	for _, want := range []string{filepath.Join(root, "config", "detekt", "detekt.yml"), `"build"`} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "verbose: detekt: ") {
		t.Errorf("lines should carry the verbose prefix:\n%s", out)
	}
}

func TestConfigServedByDaemonRejectsDetektFlags(t *testing.T) {
	root := t.TempDir()
	if !configServedByDaemon(newTestFlags(t), root) {
		t.Fatal("no flags: daemon serves the config")
	}
	for _, args := range [][]string{{"--detekt-config", "d.yml"}, {"--no-detekt-config"}} {
		if configServedByDaemon(newTestFlags(t, args...), root) {
			t.Errorf("%v must force an in-process scan", args)
		}
	}
}
