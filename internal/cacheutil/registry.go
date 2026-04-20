package cacheutil

import (
	"errors"
	"log"
	"sync"
)

// ClearContext is passed to every Registered.Clear() call. Subsystems that
// need runtime-resolved paths (e.g. the repo root) read them from here
// rather than maintaining their own globals.
type ClearContext struct {
	RepoDir string
}

// Registered is anything that can be enumerated and cleared wholesale.
type Registered interface {
	Name() string
	Clear(ctx ClearContext) error
}

var (
	mu       sync.Mutex
	registry []Registered
)

// Register adds c to the global registry. Idempotent-by-Name: if a cache with
// the same name is already registered, it is replaced and a warning is logged.
func Register(c Registered) {
	mu.Lock()
	defer mu.Unlock()
	for i, existing := range registry {
		if existing.Name() == c.Name() {
			log.Printf("cacheutil: replacing already-registered cache %q", c.Name())
			registry[i] = c
			return
		}
	}
	registry = append(registry, c)
}

// ClearAll invokes Clear() on every registered cache. Uses errors.Join;
// never short-circuits. Errors from individual caches are accumulated.
func ClearAll(ctx ClearContext) error {
	mu.Lock()
	snapshot := make([]Registered, len(registry))
	copy(snapshot, registry)
	mu.Unlock()

	var errs []error
	for _, c := range snapshot {
		if err := c.Clear(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// AllRegistered returns a snapshot of every currently-registered cache.
// Used for --verbose output and tests.
func AllRegistered() []Registered {
	mu.Lock()
	defer mu.Unlock()
	snap := make([]Registered, len(registry))
	copy(snap, registry)
	return snap
}
