package serve

import (
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// The daemon runs the oracle inside the project analysis, so --no-cache must
// switch its cache off as a one-shot scan's does (#1831).
func TestNoCacheOracleFollowsNoCache(t *testing.T) {
	for _, tc := range []struct {
		args daemon.AnalyzeProjectArgs
		want bool
	}{
		{daemon.AnalyzeProjectArgs{}, false},
		{daemon.AnalyzeProjectArgs{NoCacheOracle: true}, true},
		{daemon.AnalyzeProjectArgs{NoCache: true}, true},
	} {
		if got := noCacheOracle(tc.args); got != tc.want {
			t.Errorf("noCacheOracle(NoCache=%v, NoCacheOracle=%v) = %v, want %v", tc.args.NoCache, tc.args.NoCacheOracle, got, tc.want)
		}
	}
}
