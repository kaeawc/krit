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
	// delivered is set once the oracle's check response reached the prefetch.
	delivered bool

	claimedCh  chan struct{}
	resolvedCh chan struct{}
}

// NewBroker returns a broker with the production timeouts.
func NewBroker() *Broker {
	return NewBrokerWithTimeouts(brokerUnclaimedTimeout, daemonRequestTimeout()+brokerClaimedSlack)
}

// NewBrokerWithTimeouts returns a broker whose wait gives up after unclaimed
// without the oracle's decision, or after claimed once the oracle has started
// the shared compile.
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
func (b *Broker) Arm(p *Prefetch) {
	if b == nil || p == nil {
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
//   - the oracle compiles in another context than the pass (matches says no);
//   - the generic oracle daemon cannot honor the pass's JVM target;
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
	case checker.JvmTarget != "":
		// The oracle daemon registry does not key the JVM target.
		b.Done("fir check needs a target-scoped compile")
		return nil
	case matches == nil || !matches(checker.JarPath, checker.SourceDirs, checker.Kotlin, checker.Classpath):
		b.Done("oracle daemon compiles in another context")
		return nil
	case !prefetch.hasUncachedFiles():
		b.Done("fir check cache is warm")
		return nil
	}
	rider := prefetch.Rider(checker.JarPath, checker.SourceDirs, checker.Classpath, checker.CompileTarget())
	if rider == nil {
		b.Done("oracle daemon compiles in another context")
		return nil
	}
	return &oracle.CheckRider{Abandon: func() { b.Done("krit-fir does not carry the check") }, Prepare: func() ([]byte, func([]byte)) {
		request, deliver := rider.Prepare()
		if len(request) == 0 || deliver == nil {
			b.Done("nothing to check")
			return nil, nil
		}
		b.mu.Lock()
		if b.resolved {
			b.mu.Unlock()
			return nil, nil
		}
		if !b.claimed {
			b.claimed = true
			close(b.claimedCh)
		}
		b.mu.Unlock()
		return request, func(response []byte) {
			// A response that arrives after the pass gave up is dropped:
			// the pass already compiles on its own.
			b.mu.Lock()
			stale := b.resolved
			if !stale {
				b.delivered = true
			}
			b.mu.Unlock()
			if !stale {
				deliver(response)
			}
		}
	}}
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
		compile := time.NewTimer(b.claimedTimeout)
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

// Outcome is "shared" when the oracle's compile delivered the pass's check,
// and otherwise the reason the pass compiles on its own.
func (b *Broker) Outcome() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.delivered {
		return "shared"
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
