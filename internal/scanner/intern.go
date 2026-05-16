package scanner

import (
	"strings"
	"sync"
	"unsafe"
)

var globalStringPool = NewStringPool()

// StringPool deduplicates repeated string values across scanner hot paths.
type StringPool struct {
	mu    sync.RWMutex
	table map[string]string
}

// NewStringPool creates a pool ready for concurrent use.
func NewStringPool() *StringPool {
	return &StringPool{table: make(map[string]string)}
}

// Intern returns a canonical copy of s. The stored value is cloned on first
// insert so callers can safely pass zero-copy string views backed by file bytes.
func (p *StringPool) Intern(s string) string {
	if s == "" {
		return ""
	}

	p.mu.RLock()
	if v, ok := p.table[s]; ok {
		p.mu.RUnlock()
		return v
	}
	p.mu.RUnlock()

	cloned := strings.Clone(s)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.table == nil {
		p.table = make(map[string]string)
	}
	if v, ok := p.table[s]; ok {
		return v
	}
	p.table[cloned] = cloned
	return cloned
}

// LocalPool caches recently-seen values without synchronization and falls back
// to the shared global pool when a string is first observed.
type LocalPool struct {
	table    map[string]string
	fallback *StringPool
}

// NewLocalPool creates an unsynchronized pool backed by fallback.
func NewLocalPool(fallback *StringPool) *LocalPool {
	if fallback == nil {
		fallback = globalStringPool
	}
	return &LocalPool{
		table:    make(map[string]string),
		fallback: fallback,
	}
}

// Intern returns a canonical string value and promotes it into the local cache.
func (p *LocalPool) Intern(s string) string {
	if s == "" {
		return ""
	}
	if p.table == nil {
		p.table = make(map[string]string)
	}
	if v, ok := p.table[s]; ok {
		return v
	}
	if p.fallback == nil {
		p.fallback = globalStringPool
	}
	v := p.fallback.Intern(s)
	p.table[v] = v
	return v
}

func internString(s string) string {
	return globalStringPool.Intern(s)
}

// InternString returns a canonical copy of s from the global string pool.
// Other packages can use this to reduce duplicate string allocations.
func InternString(s string) string {
	return globalStringPool.Intern(s)
}

func internBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return globalStringPool.Intern(bytesToStringView(b))
}

// bytesToStringView returns a zero-copy string aliasing b's backing
// array. The result MUST NOT outlive the caller's reference to b, and
// b MUST NOT be mutated while the view is in use. Used only as the
// lookup key into the StringPool: Intern's RLock-and-Load path reads
// the view, and on insert the value is replaced with strings.Clone'd
// storage before being stored, so no aliasing escapes the pool.
//
// UTF-8 validity: tree-sitter feeds valid-UTF-8 source bytes (we only
// parse Kotlin/Java/XML/Gradle source files, which are required to be
// UTF-8 by their respective specs). The view never escapes to a
// boundary that re-validates UTF-8; map lookups operate on raw bytes.
func bytesToStringView(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
