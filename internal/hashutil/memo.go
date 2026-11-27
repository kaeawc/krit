package hashutil

import (
	"encoding/hex"
	"os"
	"sync"
	"sync/atomic"
)

// Memo memoizes file content hashes for the duration of a single run.
// The cache is keyed by (path, size, mtime); a file whose stat fingerprint
// changes is re-hashed on the next lookup, and a file modified within
// RacyWindow of the lookup is never memoized. A Memo is safe for
// concurrent use.
//
// Memo is deliberately scoped to a single invocation — callers build one,
// pass it to every subsystem that would otherwise hash the same file
// independently, and discard it when the run completes. Persisting a Memo
// across runs would return stale hashes if a file was modified between
// invocations under the same (path, size, mtime) triple.
//
// The nil *Memo is a valid disabled memo: every method falls through to
// the unmemoized hashutil helpers and no entries are retained. This keeps
// callers that haven't yet been wired to a shared Memo working unchanged.
type Memo struct {
	mu      sync.Mutex
	entries map[string]memoEntry
	hits    atomic.Uint64
	misses  atomic.Uint64
	// generation counts Clear calls, so run-scoped caches built beside the Memo can tell one run from the next.
	generation atomic.Uint64
}

type memoEntry struct {
	size    int64
	modNano int64
	hex     string
	raw     [32]byte
}

// NewMemo returns an empty Memo.
func NewMemo() *Memo {
	return &Memo{entries: make(map[string]memoEntry)}
}

// HashFile returns the lowercase hex digest of the file at path using
// the active ContentHasher. The returned digest is memoized against the
// file's (size, mtime); a later call for the same unchanged file hits
// the cache. A file whose mtime is within RacyWindow of the stat is
// hashed on every call and never memoized, since a same-size rewrite in
// the same timestamp tick would keep its stat. If provider is non-nil
// and a hash actually needs to be computed, it is invoked to obtain the
// bytes instead of re-reading from disk — useful for callers that
// already have the file content in memory (e.g. the parse / cross-file
// caches).
//
// A nil *Memo falls through to an unmemoized hash.
func (m *Memo) HashFile(path string, provider func() ([]byte, error)) (string, error) {
	if m == nil {
		if provider != nil {
			b, err := provider()
			if err != nil {
				return "", err
			}
			return HashHex(b), nil
		}
		return HashFile(path)
	}
	e, err := m.hash(path, provider)
	return e.hex, err
}

// HashFileRaw is like HashFile but returns the raw 32-byte digest.
func (m *Memo) HashFileRaw(path string, provider func() ([]byte, error)) ([32]byte, error) {
	if m == nil {
		if provider != nil {
			b, err := provider()
			if err != nil {
				return [32]byte{}, err
			}
			return activeHasher.Sum(b), nil
		}
		hx, err := HashFile(path)
		if err != nil {
			return [32]byte{}, err
		}
		return rawOf(hx), nil
	}
	e, err := m.hash(path, provider)
	return e.raw, err
}

// hash serves HashFile and HashFileRaw for a non-nil Memo.
func (m *Memo) hash(path string, provider func() ([]byte, error)) (memoEntry, error) {
	key, memoizable := m.statKey(path)
	if e, ok := m.lookup(path, key, memoizable); ok {
		return e, nil
	}
	m.misses.Add(1)

	var e memoEntry
	if provider != nil {
		b, err := provider()
		if err != nil {
			return memoEntry{}, err
		}
		e.raw = activeHasher.Sum(b)
		e.hex = hex.EncodeToString(e.raw[:])
	} else {
		hx, err := HashFile(path)
		if err != nil {
			return memoEntry{}, err
		}
		e.hex, e.raw = hx, rawOf(hx)
	}
	m.store(path, key, memoizable, e)
	return e, nil
}

// statKey stats path and reports whether a memo entry may be served or
// stored for it: the stat must succeed and be old enough to prove the
// content (see StatTrusted).
func (m *Memo) statKey(path string) (memoEntry, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return memoEntry{}, false
	}
	key := memoEntry{size: info.Size(), modNano: info.ModTime().UnixNano()}
	return key, StatTrustedNow(info)
}

// lookup returns the memoized digest for path when its entry matches key.
func (m *Memo) lookup(path string, key memoEntry, memoizable bool) (memoEntry, bool) {
	if !memoizable {
		return memoEntry{}, false
	}
	m.mu.Lock()
	e, ok := m.entries[path]
	m.mu.Unlock()
	if !ok || e.size != key.size || e.modNano != key.modNano || e.hex == "" {
		return memoEntry{}, false
	}
	m.hits.Add(1)
	return e, true
}

// store memoizes digest e for path under key, unless key cannot prove the content.
func (m *Memo) store(path string, key memoEntry, memoizable bool, e memoEntry) {
	if !memoizable {
		return
	}
	e.size, e.modNano = key.size, key.modNano
	m.mu.Lock()
	m.entries[path] = e
	m.mu.Unlock()
}

// HashContent returns the hex digest of content under the active
// ContentHasher and, if path is non-empty and a stat succeeds, memoizes
// the result so subsequent HashFile(path) calls within this Memo return
// the same digest without re-reading or re-hashing. Use this from
// callers that already hold the file bytes (e.g. after reading once
// into memory for parsing).
func (m *Memo) HashContent(path string, content []byte) string {
	if m == nil || path == "" {
		return HashHex(content)
	}
	key, memoizable := m.statKey(path)
	if e, ok := m.lookup(path, key, memoizable); ok {
		// Stat-first lookup: a hit returns the prior hex without ever
		// re-hashing the content. On a large repo that turns
		// resolverFingerprint's per-file SHA-256 into a per-file stat
		// (10x cheaper). Only stats older than RacyWindow are memoized,
		// so a same-size rewrite inside one mtime tick is re-hashed.
		return e.hex
	}
	hx := HashHex(content)
	m.store(path, key, memoizable, memoEntry{hex: hx, raw: rawOf(hx)})
	m.misses.Add(1)
	return hx
}

// Clear drops all entries and resets hit/miss counters.
func (m *Memo) Clear() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.entries = make(map[string]memoEntry)
	m.mu.Unlock()
	m.hits.Store(0)
	m.misses.Store(0)
	m.generation.Add(1)
}

// Generation returns the number of times the Memo has been cleared. A cache scoped to one run, like the Memo itself,
// stores the generation it was built in and is stale once Generation moves (the CLI and the daemon clear the shared
// Memo at the start of every scan). A nil *Memo always reports 0.
func (m *Memo) Generation() uint64 {
	if m == nil {
		return 0
	}
	return m.generation.Load()
}

// Stats returns the current hit and miss counts.
func (m *Memo) Stats() (hits, misses uint64) {
	if m == nil {
		return 0, 0
	}
	return m.hits.Load(), m.misses.Load()
}

// Len returns the number of cached entries.
func (m *Memo) Len() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

var defaultMemo = NewMemo()

// Default returns the process-scoped shared Memo. Subsystems that need to
// cooperate on file hashing should use Default() rather than instantiating
// a private Memo, so all redundant content-hash computations within a
// single run collapse to one per unique file.
func Default() *Memo { return defaultMemo }

// ResetDefault clears the shared Memo. The CLI calls this at the start of
// each scan invocation so memoized hashes do not bleed into a subsequent
// run where files may have changed.
func ResetDefault() { defaultMemo.Clear() }

// rawOf decodes a 64-character hex digest; a malformed one yields the zero digest.
func rawOf(hx string) [32]byte {
	var raw [32]byte
	if b, err := hex.DecodeString(hx); err == nil {
		copy(raw[:], b)
	}
	return raw
}
