package store

// StoreKind distinguishes the logical cache domain inside a unified store.
type StoreKind uint8

const (
	KindIncremental StoreKind = iota // incremental analysis findings
	KindOracle                       // JVM type-analysis results
	KindMatrix                       // experiment matrix baseline counts
	KindBaseline                     // detekt-style suppression membership
)

// Key uniquely identifies one entry in the store.
//
// FileHash is the full SHA-256 of the source file's bytes.
// RuleSetHash encodes the active rule IDs and their configuration (16 bytes).
// Kind scopes the entry to its owning subsystem so KindIncremental and
// KindOracle entries for the same file never collide.
type Key struct {
	FileHash    [32]byte
	RuleSetHash [16]byte
	Kind        StoreKind
}

// KindStats holds entry count and byte size for one StoreKind.
type KindStats struct {
	EntryCount int
	TotalBytes int64
}

// StoreStats summarises utilisation for krit cache stats.
type StoreStats struct {
	EntryCount int
	TotalBytes int64
	HitRate    float64              // populated by callers that track hits/misses
	ByKind     map[StoreKind]KindStats // per-kind breakdown
}

// Store is a content-hash-keyed byte store.  Each subsystem encodes its own
// payload into []byte; the store owns only persistence and invalidation.
type Store interface {
	// Get retrieves a cached value by key. Returns (nil, false) on miss.
	Get(key Key) ([]byte, bool)

	// Put stores a value, overwriting any existing entry for the key.
	Put(key Key, value []byte) error

	// Invalidate removes all entries whose on-disk path encodes one of
	// the given rule IDs in their RuleSetHash prefix directory. This is
	// a best-effort scan; it never returns an error for individual
	// missing entries.
	Invalidate(ruleIDs ...string) error

	// Stats returns a point-in-time summary of cache utilisation.
	Stats() (StoreStats, error)
}
