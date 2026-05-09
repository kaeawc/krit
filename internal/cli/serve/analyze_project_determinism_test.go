package serve

import (
	"reflect"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_DeterministicAcrossRuns extends PR #36's
// per-phase determinism guarantees end-to-end through the daemon
// verb: 50 sequential calls against an unchanging fixture must
// produce byte-identical findings JSON every time.
//
// Iteration count is 50 (not the 100 the plan suggested) because
// each call runs the full pipeline including parse, dispatch, and
// cross-file phases — at ~80ms each, 100 would push the test
// runtime past 8s. 50 is enough to surface scheduler-driven
// non-determinism if any of the contributing phases regress.
func TestAnalyzeProject_DeterministicAcrossRuns(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Det.kt",
		"package demo\n\nclass Det {\n    fun a() {}\n    fun b() {}\n    fun c() {}\n}\n")
	writeKotlinFile(t, state.root, "Other.kt",
		"package demo\n\nfun helper(x: Int): Int = x + 1\n")

	const N = 50

	var first []byte
	for i := 0; i < N; i++ {
		var got daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &got); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		stripped := stripTimingFields(t, got.Findings)
		canon := mustJSON(t, stripped)
		if i == 0 {
			first = []byte(canon)
			continue
		}
		if string(first) != canon {
			t.Fatalf("iter %d: findings diverged from first call\n--- first ---\n%s\n--- iter %d ---\n%s",
				i, string(first), i, canon)
		}
	}
}

// TestAnalyzeProject_ConcurrentRequestsAllSucceed exercises the
// single-flight gate: 4 goroutines each fire one analyze-project
// call. analyzeMu serialises the work; the test confirms every
// caller gets a successful response and all four bodies are
// byte-equal (after timing strip). A regression that drops the
// mutex would either crash on a data race (Go race detector
// catches it under `go test -race`) or produce divergent results.
func TestAnalyzeProject_ConcurrentRequestsAllSucceed(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Concurrent.kt",
		"package demo\n\nclass Concurrent { fun work() {} }\n")

	const N = 4
	type slot struct {
		body []byte
		err  error
	}
	results := make([]slot, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			var got daemon.AnalyzeProjectResult
			if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
				daemon.AnalyzeProjectArgs{}, &got); err != nil {
				results[i] = slot{err: err}
				return
			}
			canon := mustJSON(t, stripTimingFields(t, got.Findings))
			results[i] = slot{body: []byte(canon)}
		}()
	}
	wg.Wait()

	var ref []byte
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("goroutine %d: %v", i, r.err)
		}
		if i == 0 {
			ref = r.body
			continue
		}
		if !reflect.DeepEqual(r.body, ref) {
			t.Errorf("goroutine %d body differs from goroutine 0", i)
		}
	}
}

// TestAnalyzeProject_ConcurrentRequestsRaceFreeUnderRace is a
// no-assertion stress test that exercises the verb under -race so
// the Go race detector sees concurrent access patterns. Failures
// surface as data race reports; in CI this provides defence-in-
// depth beyond the single-flight contract.
func TestAnalyzeProject_ConcurrentRequestsRaceFreeUnderRace(t *testing.T) {
	if testing.Short() {
		t.Skip("skip race-stress in -short mode")
	}
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "R.kt", "package demo\nclass R\n")

	const N = 8
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			var got daemon.AnalyzeProjectResult
			_ = daemon.Call(socket, daemon.VerbAnalyzeProject,
				daemon.AnalyzeProjectArgs{}, &got)
		}()
	}
	wg.Wait()
}
