package perf

import (
	"sync"
	"time"
)

// TimingEntry records the duration of a named phase.
type TimingEntry struct {
	Name       string        `json:"name"`
	DurationMs int64         `json:"durationMs"`
	Children   []TimingEntry `json:"children,omitempty"`
}

// Tracker records hierarchical performance timings.
type Tracker interface {
	// Track runs fn and records its duration under name.
	Track(name string, fn func() error) error
	// Serial returns a child tracker that appends entries under name.
	Serial(name string) Tracker
	// End finalizes this child tracker and returns the parent.
	End() Tracker
	// GetTimings returns all recorded timing entries.
	GetTimings() []TimingEntry
	// IsEnabled reports whether timing is active.
	IsEnabled() bool
}

// New returns a Tracker. When enabled is false, the tracker is a no-op
// with zero overhead.
func New(enabled bool) Tracker {
	if !enabled {
		return &noopTracker{}
	}
	return &realTracker{}
}

// --- real tracker ---

type realTracker struct {
	mu       sync.Mutex
	entries  []TimingEntry
	parent   *realTracker
	name     string
	start    time.Time
	children []TimingEntry
}

func (t *realTracker) IsEnabled() bool { return true }

func (t *realTracker) Track(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	dur := time.Since(start).Milliseconds()

	t.mu.Lock()
	t.entries = append(t.entries, TimingEntry{Name: name, DurationMs: dur})
	t.mu.Unlock()

	return err
}

func (t *realTracker) Serial(name string) Tracker {
	child := &realTracker{
		parent: t,
		name:   name,
		start:  time.Now(),
	}
	return child
}

func (t *realTracker) End() Tracker {
	if t.parent == nil {
		return t
	}
	dur := time.Since(t.start).Milliseconds()
	entry := TimingEntry{
		Name:       t.name,
		DurationMs: dur,
		Children:   t.entries,
	}
	t.parent.mu.Lock()
	t.parent.entries = append(t.parent.entries, entry)
	t.parent.mu.Unlock()
	return t.parent
}

func (t *realTracker) GetTimings() []TimingEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TimingEntry, len(t.entries))
	copy(out, t.entries)
	return out
}

// AddEntry adds a pre-computed timing entry to the tracker.
// This is useful for recording wall-clock time that spans multiple Track calls.
func AddEntry(t Tracker, name string, dur time.Duration) {
	if rt, ok := t.(*realTracker); ok {
		rt.mu.Lock()
		rt.entries = append(rt.entries, TimingEntry{Name: name, DurationMs: dur.Milliseconds()})
		rt.mu.Unlock()
	}
}

// --- no-op tracker ---

type noopTracker struct{}

func (n *noopTracker) IsEnabled() bool                      { return false }
func (n *noopTracker) Track(_ string, fn func() error) error { return fn() }
func (n *noopTracker) Serial(_ string) Tracker              { return n }
func (n *noopTracker) End() Tracker                         { return n }
func (n *noopTracker) GetTimings() []TimingEntry             { return nil }
