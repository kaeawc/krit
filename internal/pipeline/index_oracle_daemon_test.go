package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
)

// TestRunJvmAnalyzeForwardsNoPersistentOracleDaemon pins that --no-fir-daemon
// reaches the cached oracle invocation, which otherwise starts a persistent
// krit-fir daemon for small miss sets.
func TestRunJvmAnalyzeForwardsNoPersistentOracleDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_MODULE_SHARDS", "")
	root := t.TempDir()
	src := filepath.Join(root, "src", "main", "kotlin")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "A.kt"), []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)

	prev := invokeCachedOracle
	t.Cleanup(func() { invokeCachedOracle = prev })
	for _, want := range []bool{false, true} {
		var got *oracle.InvocationOptions
		invokeCachedOracle = func(_ string, _ []string, _, _, _ string, _ bool, _ *store.FileStore, opts oracle.InvocationOptions) (string, error) {
			got = &opts
			return "", errors.New("stop")
		}
		in := IndexInput{NoPersistentOracleDaemon: want}
		_ = IndexPhase{}.runJvmAnalyze(in, nil, []string{root}, func() []*scanner.File { return nil }, perf.New(false))
		if got == nil {
			t.Fatalf("NoPersistentOracleDaemon=%v: the cached oracle was not invoked", want)
		}
		if got.NoPersistentDaemon != want {
			t.Errorf("NoPersistentOracleDaemon=%v: InvocationOptions.NoPersistentDaemon = %v", want, got.NoPersistentDaemon)
		}
	}
}
