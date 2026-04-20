package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"sync/atomic"
)

// Memo memoizes file content hashes for the duration of a single run.
// The cache is keyed by (path, size, mtime); a file whose stat fingerprint
// changes is re-hashed on the next lookup. A Memo is safe for concurrent
// use.
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

// HashFile returns the lowercase hex SHA-256 of the file at path. The
// returned digest is memoized against the file's (size, mtime); a later
// call for the same unchanged file hits the cache. If provider is non-nil
// and a hash actually needs to be computed, it is invoked to obtain the
// bytes instead of re-reading from disk — useful for callers that already
// have the file content in memory (e.g. the parse / cross-file caches).
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

	info, err := os.Stat(path)
	var size, modNano int64
	statOK := err == nil
	if statOK {
		size = info.Size()
		modNano = info.ModTime().UnixNano()

		m.mu.Lock()
		if e, ok := m.entries[path]; ok && e.size == size && e.modNano == modNano && e.hex != "" {
			m.mu.Unlock()
			m.hits.Add(1)
			return e.hex, nil
		}
		m.mu.Unlock()
	}

	m.misses.Add(1)

	var raw [32]byte
	var hx string
	if provider != nil {
		b, perr := provider()
		if perr != nil {
			return "", perr
		}
		raw = sha256.Sum256(b)
		hx = hex.EncodeToString(raw[:])
	} else {
		hx, err = HashFile(path)
		if err != nil {
			return "", err
		}
		b, _ := hex.DecodeString(hx)
		copy(raw[:], b)
	}

	if statOK {
		m.mu.Lock()
		m.entries[path] = memoEntry{size: size, modNano: modNano, hex: hx, raw: raw}
		m.mu.Unlock()
	}
	return hx, nil
}

// HashFileRaw is like HashFile but returns the raw 32-byte digest.
func (m *Memo) HashFileRaw(path string, provider func() ([]byte, error)) ([32]byte, error) {
	if m == nil {
		if provider != nil {
			b, err := provider()
			if err != nil {
				return [32]byte{}, err
			}
			return sha256.Sum256(b), nil
		}
		hx, err := HashFile(path)
		if err != nil {
			return [32]byte{}, err
		}
		var out [32]byte
		b, _ := hex.DecodeString(hx)
		copy(out[:], b)
		return out, nil
	}

	info, err := os.Stat(path)
	var size, modNano int64
	statOK := err == nil
	if statOK {
		size = info.Size()
		modNano = info.ModTime().UnixNano()

		m.mu.Lock()
		if e, ok := m.entries[path]; ok && e.size == size && e.modNano == modNano && e.hex != "" {
			m.mu.Unlock()
			m.hits.Add(1)
			return e.raw, nil
		}
		m.mu.Unlock()
	}

	m.misses.Add(1)

	var raw [32]byte
	var hx string
	if provider != nil {
		b, perr := provider()
		if perr != nil {
			return [32]byte{}, perr
		}
		raw = sha256.Sum256(b)
		hx = hex.EncodeToString(raw[:])
	} else {
		hx, err = HashFile(path)
		if err != nil {
			return [32]byte{}, err
		}
		b, _ := hex.DecodeString(hx)
		copy(raw[:], b)
	}

	if statOK {
		m.mu.Lock()
		m.entries[path] = memoEntry{size: size, modNano: modNano, hex: hx, raw: raw}
		m.mu.Unlock()
	}
	return raw, nil
}

// HashContent returns the hex SHA-256 of content and, if path is non-empty
// and a stat succeeds, memoizes the result so subsequent HashFile(path)
// calls within this Memo return the same digest without re-reading or
// re-hashing. Use this from callers that already hold the file bytes
// (e.g. after reading once into memory for parsing).
func (m *Memo) HashContent(path string, content []byte) string {
	hx := HashHex(content)
	if m == nil || path == "" {
		return hx
	}
	info, err := os.Stat(path)
	if err != nil {
		return hx
	}
	size := info.Size()
	modNano := info.ModTime().UnixNano()
	var raw [32]byte
	b, _ := hex.DecodeString(hx)
	copy(raw[:], b)

	m.mu.Lock()
	if e, ok := m.entries[path]; ok && e.size == size && e.modNano == modNano && e.hex != "" {
		m.mu.Unlock()
		m.hits.Add(1)
		return e.hex
	}
	m.entries[path] = memoEntry{size: size, modNano: modNano, hex: hx, raw: raw}
	m.mu.Unlock()
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
// a private Memo, so all redundant SHA-256 computations within a single
// run collapse to one per unique file.
func Default() *Memo { return defaultMemo }

// ResetDefault clears the shared Memo. The CLI calls this at the start of
// each scan invocation so memoized hashes do not bleed into a subsequent
// run where files may have changed.
func ResetDefault() { defaultMemo.Clear() }
