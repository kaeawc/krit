package scan

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

// `krit .` with a Gradle model: the oracle finds convention roots from the raw
// scan argument (relative) and the FIR checker from the absolute scan path,
// and both add the model's absolute roots. They compile the same roots, so
// the FIR check must ride on the oracle compile instead of compiling the
// repository a second time (#1659).
func TestFIRCheckRidesTheOracleCompileForARelativeScanWithAModel(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app", "src", "main", "kotlin")
	lib := filepath.Join(root, "lib", "src", "main", "kotlin")
	jar := filepath.Join(root, "krit-fir.jar")
	for path, text := range map[string]string{filepath.Join(app, "A.kt"): "fun a() {}\n", filepath.Join(lib, "B.kt"): "fun b() {}\n", jar: "jar"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(oracle.BackendFIR.JarEnvVar(), jar)
	t.Chdir(root)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	modelDirs := []string{filepath.Join(wd, "app", "src", "main", "kotlin"), filepath.Join(wd, "lib", "src", "main", "kotlin")}

	f := registerScanFlags(flag.NewFlagSet("scan", flag.ContinueOnError))
	f.modelSourceDirs = modelDirs
	r := &runner{f: f, cfg: config.NewConfig(), paths: absoluteScanPaths([]string{"."}), tracker: perf.New(false)}
	rider := r.firCheckRider()
	if rider == nil {
		t.Fatal("firCheckRider = nil with --fir on")
	}
	checker := r.productionFIRChecker()

	// The oracle's roots, as pipeline.IndexPhase.runJvmAnalyze computes them
	// from IndexInput.OracleScanPaths (the raw `.`).
	oracleDirs := oracle.FilterFIRSourceDirs(oracle.UnionSourceDirs(oracle.FindSourceDirs([]string{"."}), modelDirs))
	if len(oracleDirs) != len(modelDirs) {
		t.Fatalf("oracle roots = %v, want each of %v once", oracleDirs, modelDirs)
	}
	if rider(jar, oracleDirs, checker.Classpath, checker.CompileTarget()) == nil {
		t.Fatalf("the FIR check compiles on its own: oracle roots %v, checker roots %v", oracleDirs, checker.SourceDirs)
	}
}
