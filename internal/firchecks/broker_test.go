package firchecks

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// matchAll is the oracle saying it compiles in the pass's context.
func matchAll(string, []string, gradlemodel.KotlinSettings, []string) bool { return true }

const brokerTestWait = 5 * time.Second

// brokeredProject is a prefetchProject whose checker waits on a broker.
func brokeredProject(t *testing.T, unclaimed, claimed time.Duration) (*prefetchProject, *Broker) {
	t.Helper()
	p := newPrefetchProject(t)
	broker := NewBrokerWithTimeouts(unclaimed, claimed)
	prefetch := NewPrefetch(p.opts)
	if prefetch == nil {
		t.Fatal("NewPrefetch = nil")
	}
	p.checker.Prefetch, p.checker.Broker = prefetch, broker
	broker.Arm(prefetch, matchAll)
	return p, broker
}

// startCheck runs the pass's check on its own goroutine, as StartPass does.
func (p *prefetchProject) startCheck(t *testing.T) <-chan *Result {
	t.Helper()
	out := make(chan *Result, 1)
	go func() { out <- p.check(t, []string{p.a, p.b}, []string{verdictRule}) }()
	return out
}

func waitResult(t *testing.T, out <-chan *Result) *Result {
	t.Helper()
	select {
	case res := <-out:
		return res
	case <-time.After(brokerTestWait):
		t.Fatal("the pass is still waiting: the broker deadlocked")
		return nil
	}
}

// countingRun counts the compiles the pass runs on its own.
func countingRun(t *testing.T) *atomic.Int32 {
	t.Helper()
	old := runMissesForCache
	t.Cleanup(func() { runMissesForCache = old })
	var calls atomic.Int32
	runMissesForCache = func(_ string, _, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...CompileTarget) (*CheckResponse, error) {
		calls.Add(1)
		return &CheckResponse{}, nil
	}
	return &calls
}

func TestNilBrokerIsInert(t *testing.T) {
	var b *Broker
	b.Arm(nil, nil)
	b.Watch(context.Background())
	b.Done("x")
	if b.Claim(matchAll) != nil || b.Wait() != "" || b.Reason() != "" || b.Outcome() != "" || !b.Resolved() {
		t.Fatal("a nil broker must decline everything and never block")
	}
}

// One compile answers both: the oracle claims the pass's check, compiles once,
// and the pass, which was already waiting, reuses the response without a
// compile of its own.
func TestBrokerOneCompileAnswersTheOracleAndThePass(t *testing.T) {
	p, broker := brokeredProject(t, time.Minute, time.Minute)
	passCompiles := countingRun(t)
	var oracleCompiles atomic.Int32

	out := p.startCheck(t)
	time.Sleep(20 * time.Millisecond) // the pass reaches its wait first, as in a delegated scan

	rider := broker.Claim(matchAll)
	if rider == nil {
		t.Fatalf("Claim declined: %q", broker.Reason())
	}
	rider.Sent()
	request, deliver := rider.Prepare()
	if len(request) == 0 || deliver == nil {
		t.Fatal("the rider carries no request")
	}
	oracleCompiles.Add(1) // the one compile of the module
	deliver(p.response(verdictRule))
	broker.Done("oracle daemon analysis finished")

	res := waitResult(t, out)
	if len(res.Findings) != 1 || res.Findings[0].Rule != verdictRule {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if got := oracleCompiles.Load() + passCompiles.Load(); got != 1 {
		t.Fatalf("the module was compiled %d times, want 1 (oracle %d, pass %d)", got, oracleCompiles.Load(), passCompiles.Load())
	}
	if broker.Outcome() != "shared" {
		t.Fatalf("outcome = %q", broker.Outcome())
	}
}

// Every way the oracle can finish without delivering releases the waiting pass,
// which then compiles on its own: the pass is never stuck.
func TestBrokerEveryResolutionPathReleasesThePass(t *testing.T) {
	paths := map[string]func(p *prefetchProject, b *Broker){
		"cache hit": func(_ *prefetchProject, b *Broker) { b.Done("oracle cache hit") },
		"skipped":   func(_ *prefetchProject, b *Broker) { b.Done("oracle skipped") },
		"early return": func(_ *prefetchProject, b *Broker) {
			// The oracle path returned before it sent anything.
			b.Done("oracle daemon path returned")
		},
		"deferred decline after the index phase": func(_ *prefetchProject, b *Broker) {
			time.Sleep(30 * time.Millisecond)
			b.Done("index phase returned")
		},
		"other compile context": func(_ *prefetchProject, b *Broker) {
			if rider := b.Claim(func(string, []string, gradlemodel.KotlinSettings, []string) bool { return false }); rider != nil {
				panic("claimed a compile in another context")
			}
		},
		"unarmed": func(_ *prefetchProject, b *Broker) {
			b.mu.Lock()
			b.prefetch = nil
			b.mu.Unlock()
			if b.Claim(matchAll) != nil {
				panic("claimed without a planned check")
			}
		},
		"krit-fir does not know the request (older jar)": func(_ *prefetchProject, b *Broker) {
			b.Claim(matchAll).Decline("krit-fir does not carry the check")
		},
		"claimed but the compile failed": func(_ *prefetchProject, b *Broker) {
			rider := b.Claim(matchAll)
			rider.Sent()
			rider.Prepare()
			b.Done("oracle failed") // no response ever delivered
		},
		"claimed, nothing delivered (not shared)": func(_ *prefetchProject, b *Broker) {
			rider := b.Claim(matchAll)
			rider.Sent()
			rider.Prepare()
			b.Done("oracle daemon analysis finished")
		},
	}
	for name, resolve := range paths {
		t.Run(name, func(t *testing.T) {
			p, broker := brokeredProject(t, time.Minute, time.Minute)
			compiles := countingRun(t)
			out := p.startCheck(t)
			go resolve(p, broker)
			waitResult(t, out)
			if compiles.Load() != 1 {
				t.Fatalf("the pass compiled %d times on its own, want 1", compiles.Load())
			}
			if !broker.Resolved() || broker.Outcome() == "shared" {
				t.Fatalf("resolved=%v outcome=%q", broker.Resolved(), broker.Outcome())
			}
		})
	}
}

func TestBrokerClaimDeclinesWhatCannotRide(t *testing.T) {
	t.Run("no matcher", func(t *testing.T) {
		_, broker := brokeredProject(t, time.Minute, time.Minute)
		if broker.Claim(nil) != nil || !broker.Resolved() {
			t.Fatal("claimed without knowing the oracle's context")
		}
	})
	t.Run("already resolved", func(t *testing.T) {
		_, broker := brokeredProject(t, time.Minute, time.Minute)
		broker.Done("index phase returned")
		if broker.Claim(matchAll) != nil || broker.Reason() != "index phase returned" {
			t.Fatalf("claimed after the broker resolved: %q", broker.Reason())
		}
	})
	t.Run("warm check cache", func(t *testing.T) {
		p, broker := brokeredProject(t, time.Minute, time.Minute)
		p.checker.NoCache, p.checker.RepoDir = false, t.TempDir()
		plan, _ := planPass(p.opts)
		cacheDir, err := CacheDir(p.checker.RepoDir)
		if err != nil {
			t.Fatal(err)
		}
		fingerprint := CheckCacheFingerprint(p.checker.SourceDirs, plan.requested, p.checker.Classpath, p.checker.JarPath, plan.rules, plan.configs, plan.facts, p.checker.CompileTarget())
		if !p.checker.Prefetch.hasUncachedFiles() {
			t.Fatal("an empty cache should have misses")
		}
		if n := WriteFreshEntriesForFingerprint(cacheDir, plan.requested, &CheckResponse{Succeeded: 2}, fingerprint); n != 2 {
			t.Fatalf("wrote %d cache entries", n)
		}
		if p.checker.Prefetch.hasUncachedFiles() {
			t.Fatal("a fully cached check should have no misses")
		}
		if broker.Claim(matchAll) != nil || broker.Reason() != "fir check cache is warm" {
			t.Fatalf("claimed a check the cache answers: %q", broker.Reason())
		}
	})
}

// A pass whose check the cache answers in full never waits at all.
func TestBrokerPassWithNothingToCompileDoesNotWait(t *testing.T) {
	p, _ := brokeredProject(t, time.Hour, time.Hour) // would hang the test if it waited
	p.checker.NoCache, p.checker.RepoDir = false, t.TempDir()
	plan, _ := planPass(p.opts)
	cacheDir, err := CacheDir(p.checker.RepoDir)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := CheckCacheFingerprint(p.checker.SourceDirs, plan.requested, p.checker.Classpath, p.checker.JarPath, plan.rules, plan.configs, plan.facts, p.checker.CompileTarget())
	WriteFreshEntriesForFingerprint(cacheDir, plan.requested, &CheckResponse{Succeeded: 2}, fingerprint)
	compiles := countingRun(t)
	waitResult(t, p.startCheck(t))
	if compiles.Load() != 0 {
		t.Fatalf("a fully cached check compiled %d times", compiles.Load())
	}
}

// An oracle that never answers cannot hang the pass: both timeouts fall back.
func TestBrokerTimeoutFallsBack(t *testing.T) {
	t.Run("oracle never decides", func(t *testing.T) {
		p, broker := brokeredProject(t, 30*time.Millisecond, time.Minute)
		compiles := countingRun(t)
		waitResult(t, p.startCheck(t))
		if compiles.Load() != 1 || broker.Outcome() != "timed out waiting for the oracle" {
			t.Fatalf("compiles=%d outcome=%q", compiles.Load(), broker.Outcome())
		}
	})
	t.Run("oracle claims and never finishes", func(t *testing.T) {
		p, broker := brokeredProject(t, time.Minute, 30*time.Millisecond)
		compiles := countingRun(t)
		rider := broker.Claim(matchAll)
		_, deliver := rider.Prepare()
		rider.Sent()
		waitResult(t, p.startCheck(t))
		if compiles.Load() != 1 || broker.Outcome() != "timed out waiting for the oracle compile" {
			t.Fatalf("compiles=%d outcome=%q", compiles.Load(), broker.Outcome())
		}
		// The late response is dropped: the pass already compiled on its own.
		deliver(p.response(verdictRule))
		if p.checker.Prefetch.resp != nil || broker.Outcome() == "shared" {
			t.Fatal("a response that arrived after the timeout was accepted")
		}
	})
}

func TestBrokerCancelFallsBack(t *testing.T) {
	p, broker := brokeredProject(t, time.Minute, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	broker.Watch(ctx)
	compiles := countingRun(t)
	out := p.startCheck(t)
	time.Sleep(20 * time.Millisecond)
	cancel()
	waitResult(t, out)
	if compiles.Load() != 1 || broker.Outcome() != "canceled" {
		t.Fatalf("compiles=%d outcome=%q", compiles.Load(), broker.Outcome())
	}
}

// Watch must not leak its goroutine when the broker resolves first.
func TestBrokerWatchEndsWhenResolved(t *testing.T) {
	broker := NewBroker()
	broker.Watch(context.Background())
	broker.Done("index phase returned")
	select {
	case <-broker.resolvedCh:
	case <-time.After(brokerTestWait):
		t.Fatal("not resolved")
	}
}

// Done is idempotent and the first reason wins, from any number of goroutines.
func TestBrokerDoneIsIdempotentAndConcurrent(t *testing.T) {
	broker := NewBrokerWithTimeouts(time.Minute, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			broker.Done("first")
			broker.Done("second")
			broker.Wait()
		}()
	}
	wg.Wait()
	if broker.Reason() != "first" && broker.Reason() != "second" {
		t.Fatalf("reason = %q", broker.Reason())
	}
	before := broker.Reason()
	broker.Done("third")
	if broker.Reason() != before {
		t.Fatal("a later Done changed the reason")
	}
}

// Waiting for an already-resolved broker returns at once.
func TestBrokerWaitAfterResolveReturnsImmediately(t *testing.T) {
	broker := NewBrokerWithTimeouts(time.Hour, time.Hour)
	broker.Done("oracle skipped")
	done := make(chan string, 1)
	go func() { done <- broker.Wait() }()
	select {
	case got := <-done:
		if got != "oracle skipped" {
			t.Fatalf("Wait = %q", got)
		}
	case <-time.After(brokerTestWait):
		t.Fatal("Wait blocked on a resolved broker")
	}
}

// Declines that do not depend on the oracle run are decided when the pass is
// armed, so a pass that can never ride does not wait for the oracle at all.
func TestBrokerStructuralDeclinesAreDecidedAtArmTime(t *testing.T) {
	cases := map[string]struct {
		jvmTarget string
		matches   ContextMatcher
		reason    string
	}{
		"jvm target":       {"17", matchAll, "fir check needs a target-scoped compile"},
		"another context":  {"", func(string, []string, gradlemodel.KotlinSettings, []string) bool { return false }, "oracle daemon compiles in another context"},
		"no oracle daemon": {"", nil, "no oracle daemon to share a compile with"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := newPrefetchProject(t)
			p.checker.JvmTarget = tc.jvmTarget
			broker := NewBrokerWithTimeouts(time.Hour, time.Hour) // would hang the test if the pass waited
			prefetch := NewPrefetch(p.opts)
			p.checker.Prefetch, p.checker.Broker = prefetch, broker
			broker.Arm(prefetch, tc.matches)
			if !broker.Resolved() || broker.Reason() != tc.reason {
				t.Fatalf("resolved=%v reason=%q, want %q", broker.Resolved(), broker.Reason(), tc.reason)
			}
			compiles := countingRun(t)
			waitResult(t, p.startCheck(t)) // returns promptly: nothing to wait for
			if compiles.Load() != 1 {
				t.Fatalf("pass compiles = %d", compiles.Load())
			}
		})
	}
}

// The outcome is "shared" only once the pass has used the oracle's response.
func TestBrokerOutcomeReportsWhatThePassUsed(t *testing.T) {
	p, broker := brokeredProject(t, time.Minute, time.Minute)
	rider := broker.Claim(matchAll)
	rider.Sent()
	_, deliver := rider.Prepare()
	deliver(p.response(verdictRule))
	broker.Done("oracle daemon analysis finished")
	if got := broker.Outcome(); got != "delivered, not used" {
		t.Fatalf("before the pass: %q", got)
	}
	stubMisses(t, false)
	p.check(t, []string{p.a, p.b}, []string{verdictRule})
	if got := broker.Outcome(); got != "shared" {
		t.Fatalf("after the pass: %q", got)
	}
}

// A response the prefetch cannot use (here: no registered checkers) resolves
// the broker as a decline instead of claiming a share.
func TestBrokerUnusableResponseIsADecline(t *testing.T) {
	p, broker := brokeredProject(t, time.Minute, time.Minute)
	rider := broker.Claim(matchAll)
	rider.Sent()
	_, deliver := rider.Prepare()
	deliver([]byte(`{"id":1,"succeeded":0,"rules":[],"findings":[],"crashed":{},"errorFiles":{}}`))
	if broker.Outcome() != "oracle response unusable" {
		t.Fatalf("outcome = %q", broker.Outcome())
	}
	_ = p
}

// A pass that finishes from a warm cache before the oracle decides still
// reports a real outcome.
func TestBrokerOutcomeWhenThePassNeverWaited(t *testing.T) {
	_, broker := brokeredProject(t, time.Hour, time.Hour)
	if got := broker.Outcome(); got != "not needed: fir check cache warm" {
		t.Fatalf("outcome = %q", got)
	}
}

// The rider reports it is cancelled once the pass gave up.
func TestBrokerRiderIsCancelledOnceResolved(t *testing.T) {
	_, broker := brokeredProject(t, time.Hour, time.Hour)
	rider := broker.Claim(matchAll)
	if rider.Cancelled() {
		t.Fatal("cancelled before the pass gave up")
	}
	broker.Done("timed out waiting for the oracle")
	if !rider.Cancelled() {
		t.Fatal("not cancelled after the pass gave up")
	}
}
