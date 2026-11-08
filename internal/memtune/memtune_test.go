package memtune

import "testing"

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
