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

// The resident daemon carries no JVM target, so a run with one must take the
// target-scoped one-shot path rather than analyze under the host JDK's target.
func TestUseResidentOracleDaemonRequiresNoJvmTarget(t *testing.T) {
	cases := []struct {
		in   IndexInput
		want bool
	}{
		{IndexInput{}, false},
		{IndexInput{UseDaemon: true}, true},
		{IndexInput{UseDaemon: true, OracleJvmTarget: "17"}, false},
		{IndexInput{OracleJvmTarget: "17"}, false},
	}
	for _, tc := range cases {
		if got := useResidentOracleDaemon(tc.in); got != tc.want {
			t.Errorf("useResidentOracleDaemon(UseDaemon=%v, JvmTarget=%q) = %v, want %v", tc.in.UseDaemon, tc.in.OracleJvmTarget, got, tc.want)
		}
	}
}
