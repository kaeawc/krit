package firchecks

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// livePIDs starts n sleeping processes to stand in for verified daemons.
func livePIDs(t *testing.T, n int) []int {
	t.Helper()
	pids := make([]int, n)
	for i := range pids {
		proc := exec.Command("sleep", "60")
		if err := proc.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() { _ = proc.Wait(); close(done) }()
		t.Cleanup(func() { _ = proc.Process.Kill(); <-done })
		pids[i] = proc.Process.Pid
	}
	return pids
}

// Concurrent cold starts for one key all verify at once: exactly one
// publishes, the registry names that daemon consistently, and the others stay
// private (and are closed on Release).
func TestConcurrentVerifiersPublishExactlyOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	const n = 12
	pids := livePIDs(t, n)
	daemons := make([]*FirDaemon, n)
	for i := range daemons {
		daemons[i] = &FirDaemon{sourcesHash: "race", aotCachePath: "/c.aot", aotPID: pids[i], port: 20000 + i, unpublished: true}
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, d := range daemons {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			d.markAOTVerified()
		}()
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, d := range daemons {
		if !d.aotVerified {
			t.Errorf("daemon %d not verified", i)
		}
		if !d.unpublished {
			if winner >= 0 {
				t.Fatalf("daemons %d and %d both published", winner, i)
			}
			winner = i
		}
	}
	if winner < 0 {
		t.Fatal("no daemon published")
	}
	pid, ok := registeredFirPID("race")
	port, perr := os.ReadFile(firPortPath("race"))
	meta, merr := os.ReadFile(firAOTPath("race"))
	if !ok || pid != pids[winner] || perr != nil || merr != nil ||
		strings.TrimSpace(string(port)) != strconv.Itoa(20000+winner) ||
		!strings.Contains(string(meta), fmt.Sprintf(`"pid":%d`, pids[winner])) || !strings.Contains(string(meta), `"verified":true`) {
		t.Fatalf("registry inconsistent: pid=%d,%v port=%q,%v meta=%q,%v (winner %d pid %d)", pid, ok, port, perr, meta, merr, winner, pids[winner])
	}
	if _, err := os.Stat(firClaimPath("race")); !os.IsNotExist(err) {
		t.Fatalf("claim left behind: %v", err)
	}
}

func TestStaleRegistryClaimIsRecovered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(firClaimPath("stale"), []byte(strconv.Itoa(dead.Process.Pid)+"\n"), 0o644))
	release, ok := claimFirRegistry("stale")
	if !ok {
		t.Fatal("stale claim not recovered")
	}
	// A live holder blocks a second claimant until it releases.
	start := time.Now()
	done := make(chan bool)
	go func() {
		release2, got := claimFirRegistry("stale")
		if got {
			release2()
		}
		done <- got
	}()
	time.Sleep(100 * time.Millisecond)
	release()
	if !<-done || time.Since(start) < 90*time.Millisecond {
		t.Fatal("second claimant did not wait for the live holder")
	}
}
