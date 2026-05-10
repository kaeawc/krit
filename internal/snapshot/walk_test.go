package snapshot

import "testing"

func TestGitSemaphoreCapacity(t *testing.T) {
	cases := []struct{ workers, want int }{
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 2},
		{8, 4},
		{9, 4},
	}
	for _, tc := range cases {
		got := cap(gitSemaphore(tc.workers))
		if got != tc.want {
			t.Errorf("gitSemaphore(%d) cap = %d, want %d", tc.workers, got, tc.want)
		}
	}
}
