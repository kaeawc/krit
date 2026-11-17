package memtune

import (
	"math"
	"runtime/debug"
	"testing"
)

func TestPercentForLive(t *testing.T) {
	const gib = uint64(1) << 30
	cases := []struct {
		live uint64
		want int
	}{
		{0, 100},
		{gib, 100},
		{4 * gib, 100},
		{8 * gib, 50},
		{12 * gib, 30},
		{64 * gib, 20},
	}
	for _, c := range cases {
		if got := PercentForLive(c.live); got != c.want {
			t.Errorf("PercentForLive(%d GiB) = %d, want %d", c.live/gib, got, c.want)
		}
	}
}

func TestStartStopIdempotent(t *testing.T) {
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "")
	stop := Start()
	stop()
	stop()
}

func TestSoftLimitFor(t *testing.T) {
	const gib = uint64(1) << 30
	if got, want := SoftLimitFor(8*gib), int64(6*gib); got != want {
		t.Errorf("SoftLimitFor(8 GiB) = %d, want %d", got, want)
	}
	if got := SoftLimitFor(0); got != 0 {
		t.Errorf("SoftLimitFor(0) = %d, want 0", got)
	}
}

// withCgroupLimit replaces the cgroup reader and restores the runtime memory
// limit after the test.
func withCgroupLimit(t *testing.T, limit uint64, ok bool) {
	t.Helper()
	prev := cgroupLimit
	before := debug.SetMemoryLimit(-1)
	cgroupLimit = func() (uint64, bool) { return limit, ok }
	t.Cleanup(func() {
		cgroupLimit = prev
		debug.SetMemoryLimit(before)
	})
}

func TestStartSetsSoftLimitInsideMemoryLimitedCgroup(t *testing.T) {
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "")
	const limit = uint64(4) << 30
	withCgroupLimit(t, limit, true)
	before := debug.SetMemoryLimit(-1)

	stop := Start()
	if got := debug.SetMemoryLimit(-1); got != SoftLimitFor(limit) {
		t.Errorf("memory limit during scan = %d, want %d", got, SoftLimitFor(limit))
	}
	stop()
	if got := debug.SetMemoryLimit(-1); got != before {
		t.Errorf("memory limit after stop = %d, want the previous %d", got, before)
	}
}

func TestStartKeepsGOMEMLIMIT(t *testing.T) {
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "1GiB")
	withCgroupLimit(t, uint64(4)<<30, true)
	before := debug.SetMemoryLimit(-1)

	stop := Start()
	defer stop()
	if got := debug.SetMemoryLimit(-1); got != before {
		t.Errorf("memory limit = %d, want the runtime's %d: GOMEMLIMIT must win", got, before)
	}
}

func TestStartWithoutCgroupLimitLeavesMemoryLimit(t *testing.T) {
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "")
	withCgroupLimit(t, 0, false)
	debug.SetMemoryLimit(math.MaxInt64)

	stop := Start()
	defer stop()
	if got := debug.SetMemoryLimit(-1); got != math.MaxInt64 {
		t.Errorf("memory limit = %d, want no limit", got)
	}
}
