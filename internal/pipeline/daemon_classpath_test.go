package pipeline

import (
	"errors"
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// The --daemon path starts the krit-fir daemon with the effective oracle
// classpath, as the one-shot path does (#852).
func TestRunDaemonOracleStartsDaemonWithClasspath(t *testing.T) {
	var got []string
	called := false
	orig := invokeOracleDaemon
	invokeOracleDaemon = func(_ []string, classpath []string, _ bool, _ ...oracle.DaemonOption) (*oracle.Daemon, error) {
		called = true
		got = classpath
		return nil, errors.New("stop")
	}
	t.Cleanup(func() { invokeOracleDaemon = orig })
	in := IndexInput{OracleClasspath: []string{"/m/a.jar", "/m/b.jar"}}
	_ = (IndexPhase{}).runDaemonOracle(in, nil, []string{t.TempDir()}, func() []*scanner.File { return nil }, perf.New(false), nil, &IndexResult{})
	if !called || !slices.Equal(got, in.OracleClasspath) {
		t.Fatalf("daemon started=%v with classpath %v, want %v", called, got, in.OracleClasspath)
	}
}
