package pipeline

import "github.com/kaeawc/krit/internal/perf"

// phaseLogf calls fn when non-nil; nil fn is a no-op.
func phaseLogf(fn func(string, ...any), format string, args ...any) {
	if fn != nil {
		fn(format, args...)
	}
}

// phaseTrackSerial runs fn under a child tracker named name when t is
// non-nil; otherwise it runs fn directly.
func phaseTrackSerial(t perf.Tracker, name string, fn func() error) error {
	if t == nil {
		return fn()
	}
	child := t.Serial(name)
	err := fn()
	child.End()
	return err
}
