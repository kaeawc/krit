package serve

import (
	"github.com/fsnotify/fsnotify"
)

// fsnotifyBackend adapts *fsnotify.Watcher to watcherBackend. The
// translation is mechanical — fsnotify already gives us a per-
// directory model with a unified Op bitmask, so the adapter just
// renames ops and forwards events through a buffered channel sized
// to fsnotify's own buffer (10 events) so we don't add a stall point.
type fsnotifyBackend struct {
	w      *fsnotify.Watcher
	events chan backendEvent
	errors chan error
	done   chan struct{}
}

// newFsnotifyBackend wraps a fresh fsnotify.Watcher and starts the
// translation goroutine. Returns the bare error from fsnotify on
// platforms without inotify/kqueue.
func newFsnotifyBackend() (watcherBackend, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	b := &fsnotifyBackend{
		w:      w,
		events: make(chan backendEvent, 16),
		errors: make(chan error, 4),
		done:   make(chan struct{}),
	}
	go b.translate()
	return b, nil
}

func (b *fsnotifyBackend) Add(path string) error { return b.w.Add(path) }

func (b *fsnotifyBackend) Close() error {
	err := b.w.Close()
	<-b.done
	return err
}

func (b *fsnotifyBackend) Events() <-chan backendEvent { return b.events }
func (b *fsnotifyBackend) Errors() <-chan error        { return b.errors }

// translate forwards fsnotify events through our normalized channel,
// converting the fsnotify.Op bitmask to backendOp. The goroutine
// exits when fsnotify closes both its Events and Errors channels —
// fsnotify.Watcher.Close() guarantees this ordering.
func (b *fsnotifyBackend) translate() {
	defer close(b.done)
	defer close(b.events)
	defer close(b.errors)
	for {
		select {
		case ev, ok := <-b.w.Events:
			if !ok {
				// Drain errors before exiting so the daemon doesn't
				// lose a final teardown diagnostic.
				for err := range b.w.Errors {
					b.errors <- err
				}
				return
			}
			b.events <- backendEvent{Path: ev.Name, Op: fsnotifyOp(ev.Op)}
		case err, ok := <-b.w.Errors:
			if !ok {
				for ev := range b.w.Events {
					b.events <- backendEvent{Path: ev.Name, Op: fsnotifyOp(ev.Op)}
				}
				return
			}
			b.errors <- err
		}
	}
}

// fsnotifyOp translates an fsnotify.Op bitmask to backendOp. The two
// ops have the same shape (bitmask, multi-flag) so this is a direct
// mapping. Any future fsnotify ops not listed here are dropped — the
// watcher doesn't care about them today.
func fsnotifyOp(op fsnotify.Op) backendOp {
	var out backendOp
	if op&fsnotify.Create != 0 {
		out |= opCreate
	}
	if op&fsnotify.Write != 0 {
		out |= opWrite
	}
	if op&fsnotify.Remove != 0 {
		out |= opRemove
	}
	if op&fsnotify.Rename != 0 {
		out |= opRename
	}
	if op&fsnotify.Chmod != 0 {
		out |= opChmod
	}
	return out
}
