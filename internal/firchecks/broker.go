package firchecks

// broker.go — coordinates the oracle and the --fir pass of a daemon-delegated
// scan, so the module is compiled once for both (#810).
//
// The delegated scan starts the pass right after parsing, while the index
// phase asks the resident oracle daemon for its facts. Both want the same
// compilation, so one of them has to ask for it, and the other has to wait
// to learn whether it was answered. The pass cannot ask: it does not know the
// oracle daemon, and the oracle's request is the one that is going to be sent
// anyway. So the oracle claims the pass's check request, sends it with its
// own, and says when it is done. The pass waits for that word before it
// compiles on its own, and compiles on its own whenever the answer is
// anything but a delivered response.
//
// The pass must never wait for a word that does not come. The oracle resolves
// the broker on every path it can take (a compile, a cache answer, another
// compile context, a skip, an early return) and the pipeline resolves it
// again once the index phase has returned, so the wait ends whatever the
// oracle did. Timeouts and cancellation end it too, as a decline.

import (
	"context"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

const (
	// brokerUnclaimedTimeout bounds the wait for the oracle's decision. It
	// only has to outlast the work between the parse and the oracle request
	// (resolver and filter building); every normal path resolves long before.
	brokerUnclaimedTimeout = 3 * time.Minute
	// brokerClaimedSlack is added to the daemon request timeout to bound the
	// wait for a compile the oracle has started.
	brokerClaimedSlack = time.Minute
)

// Broker is one scan's rendezvous between the oracle and the --fir pass. The
// zero value is not usable; nil is, and declines everything.
type Broker struct {
	unclaimedTimeout time.Duration
	claimedTimeout   time.Duration

	mu       sync.Mutex
	prefetch *Prefetch
	claimed  bool
	resolved bool
	reason   string
	// delivered is set once the oracle's check response reached the prefetch;
	// consumed once the pass answered its files from it.
	delivered, consumed bool
	claimedAt           time.Time

	claimedCh  chan struct{}
	resolvedCh chan struct{}
}

// NewBroker returns a broker with the production timeouts.
func NewBroker() *Broker {
	// The compile rides an oracle daemon request, which is bounded by the
	// oracle's own request timeout, not the checker daemon's.
	return NewBrokerWithTimeouts(brokerUnclaimedTimeout, oracle.DaemonRequestTimeout()+brokerClaimedSlack)
}

// NewBrokerWithTimeouts returns a broker whose wait gives up after unclaimed
// without the oracle's decision, or after claimed from the moment the oracle
// sent the shared request.
func NewBrokerWithTimeouts(unclaimed, claimed time.Duration) *Broker {
	return &Broker{
		unclaimedTimeout: unclaimed,
		claimedTimeout:   claimed,
		claimedCh:        make(chan struct{}),
		resolvedCh:       make(chan struct{}),
	}
}

// Arm registers the pass's prefetch: the check request the oracle may claim.
// Without it, or after the broker resolved, the oracle has nothing to carry.
//
// What does not depend on the oracle run is decided here, so a pass that can
// never ride starts compiling at once instead of waiting for the oracle:
//   - the generic oracle daemon registry does not key a JVM target;
//   - the oracle daemon must compile in the pass's context (matches says so;
//     nil means there is no oracle daemon).
//
// The partial-reanalyze and warm-cache declines stay with the oracle run.
func (b *Broker) Arm(p *Prefetch, matches ContextMatcher) {
	if b == nil || p == nil {
		return
	}
	checker := p.checker
	switch {
	case checker.JvmTarget != "":
		b.Done("fir check needs a target-scoped compile")
		return
	case matches == nil:
		b.Done("no oracle daemon to share a compile with")
		return
	case !matches(checker.JarPath, checker.SourceDirs, checker.Kotlin, checker.Classpath):
		b.Done("oracle daemon compiles in another context")
		return
	}
	b.mu.Lock()
	if !b.resolved {
		b.prefetch = p
	}
	b.mu.Unlock()
}

// Watch resolves the broker as a decline when ctx ends before it is resolved,
// so a canceled scan never leaves the pass waiting. It returns at once.
func (b *Broker) Watch(ctx context.Context) {
	if b == nil || ctx == nil {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			b.Done("canceled")
		case <-b.resolvedCh:
		}
	}()
}

// ContextMatcher reports whether the oracle's compilation is the one with this
// jar, source roots, classpath and Kotlin settings.
type ContextMatcher func(jarPath string, sourceDirs []string, kotlin gradlemodel.KotlinSettings, classpath []string) bool

// Claim is the oracle's offer to answer the pass's check on its own
// compilation. It returns the rider to send with the oracle request, or nil
// when the check cannot ride (the broker is then resolved as a decline, so
// the pass compiles on its own at once):
//   - the pass registered no check, or the broker already resolved;
//   - the oracle daemon is not the one the pass was armed against (matches);
//   - the pass would not compile anything because its check cache is warm.
//
// A claimed broker stays pending until Done, so the oracle must call Done
// whatever happens to the request.
func (b *Broker) Claim(matches ContextMatcher) *oracle.CheckRider {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	prefetch, resolved := b.prefetch, b.resolved
	b.mu.Unlock()
	if resolved {
		return nil
	}
	if prefetch == nil {
		b.Done("no fir check planned")
		return nil
	}
	checker := prefetch.checker
	switch {
	case matches == nil || !matches(checker.JarPath, checker.SourceDirs, checker.Kotlin, checker.Classpath):
		b.Done("oracle daemon compiles in another context")
		return nil
	case !prefetch.hasUncachedFiles():
		b.Done("fir check cache is warm")
		return nil
	}
	// The context is the checker's own (it was just matched), so the prefetch
	// always grants the rider.
	rider := prefetch.Rider(checker.JarPath, checker.SourceDirs, checker.Classpath, checker.CompileTarget())
	return &oracle.CheckRider{
		Decline:   b.Done,
		Cancelled: b.Resolved,
		Sent:      b.markClaimed,
		Prepare: func() ([]byte, func([]byte)) {
			if b.Resolved() {
				return nil, nil // the pass gave up: do not hash the module for nothing
			}
			request, deliver := rider.Prepare()
			if len(request) == 0 || deliver == nil {
				b.Done("nothing to check")
				return nil, nil
			}
			return request, func(response []byte) {
				// A response that arrives after the pass gave up is dropped:
				// the pass already compiles on its own.
				b.mu.Lock()
				stale := b.resolved
				b.mu.Unlock()
				if stale {
					return
				}
				deliver(response)
				if !prefetch.hasResponse() {
					b.Done("oracle response unusable")
					return
				}
				b.mu.Lock()
				b.delivered = true
				b.mu.Unlock()
			}
		},
	}
}

// markClaimed starts the clock on the oracle's compile: the request is on the
// wire.
func (b *Broker) markClaimed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.resolved || b.claimed {
		return
	}
	b.claimed, b.claimedAt = true, time.Now()
	close(b.claimedCh)
}

// Consumed records that the pass answered its files from the oracle's
// response.
func (b *Broker) Consumed() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.consumed = true
	b.mu.Unlock()
}

// Done resolves the broker: the oracle is finished with the shared compile,
// or will not run one. Idempotent; the first reason sticks. After Done the
// pass uses a delivered response if there is one, and otherwise compiles on
// its own.
func (b *Broker) Done(reason string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.resolved {
		return
	}
	b.resolved, b.reason = true, reason
	close(b.resolvedCh)
}

// Wait blocks until the oracle has resolved the broker, or gives up after the
// timeouts, and returns why it ended. It never blocks forever on an oracle
// that never answers; a nil broker returns at once.
func (b *Broker) Wait() string {
	if b == nil {
		return ""
	}
	timer := time.NewTimer(b.unclaimedTimeout)
	defer timer.Stop()
	select {
	case <-b.resolvedCh:
	case <-b.claimedCh:
		// The oracle is compiling: give it as long as a daemon request may take.
		b.mu.Lock()
		remaining := b.claimedTimeout - time.Since(b.claimedAt)
		b.mu.Unlock()
		compile := time.NewTimer(remaining)
		defer compile.Stop()
		select {
		case <-b.resolvedCh:
		case <-compile.C:
			b.Done("timed out waiting for the oracle compile")
		}
	case <-timer.C:
		b.Done("timed out waiting for the oracle")
	}
	return b.Reason()
}

// Reason is why the broker resolved; empty while it is pending.
func (b *Broker) Reason() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reason
}

// Outcome is "shared" once the pass answered its files from the oracle's
// compile, "delivered, not used" when a response arrived but the pass compiled
// anyway (its inputs drifted), and otherwise the reason the pass compiles on
// its own.
func (b *Broker) Outcome() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.consumed:
		return "shared"
	case b.delivered:
		return "delivered, not used"
	case b.reason == "":
		// Asked for before the oracle decided: the pass never waited, because
		// its check cache answered every file.
		return "not needed: fir check cache warm"
	}
	return b.reason
}

// Resolved reports whether the broker has resolved.
func (b *Broker) Resolved() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resolved
}
