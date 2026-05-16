// Package traces overlays runtime behavior onto krit's static structural
// graph. It ingests OpenTelemetry spans, JFR-derived stack samples, and
// JUnit step boundaries; reduces them to runtime states and transitions;
// reconciles those against the static symbol index; and exposes queries
// (overlay, orphans, phantoms, divergence) plus an opt-in rule
// capability (NeedsRuntimeEvidence).
//
// Conceptually this mirrors AutoMobile's NavigationGraphManager:
// screen-to-screen edges become state-to-state edges, and
// ScreenFingerprint becomes RuntimeStateFingerprint keyed on
// (top_symbol, caller_chain_hash, role_tag).
package traces

// SchemaVersion versions the on-disk store format. Readers see a higher
// version and refuse to decode rather than risk reading partial data.
const SchemaVersion = 1

// RoleTag classifies the execution context of a runtime state. Mirrors
// the role concept from AutoMobile's screen fingerprint — the same code
// path under "startup" vs "request" vs "test" is intentionally a
// different state, because behavior diverges by role.
type RoleTag string

const (
	RoleUnknown    RoleTag = "unknown"
	RoleStartup    RoleTag = "startup"
	RoleRequest    RoleTag = "request"
	RoleBackground RoleTag = "background"
	RoleTest       RoleTag = "test"
)

// TransitionKind names the relationship between two adjacent states in
// the reduced event stream.
type TransitionKind string

const (
	KindCall   TransitionKind = "call"
	KindReturn TransitionKind = "return"
	KindAsync  TransitionKind = "async"
	KindSpawn  TransitionKind = "spawn"
)

// SourceKind names the ingest format an IngestSource originated from.
type SourceKind string

const (
	SourceOTel  SourceKind = "otel"
	SourceJFR   SourceKind = "jfr"
	SourceJUnit SourceKind = "junit"
)

// Event is the canonical internal representation that every ingest
// adapter reduces to. The reducer collapses bursts of equivalent
// events into states and emits transitions on state change.
type Event struct {
	// SourceID joins back to IngestSource.ID. Different sources may
	// emit overlapping frames; carrying the source preserves
	// provenance for divergence queries.
	SourceID string
	// TimestampNS is the event's monotonic-ish wall-clock nanoseconds
	// when known, zero otherwise. The reducer treats zero as
	// "unordered after previous event" and uses arrival order.
	TimestampNS int64
	// FrameStack is top-of-stack first. Each entry is a resolvable
	// symbol name (FQN preferred, plain function name acceptable);
	// the reconciler later determines whether the symbol is in the
	// static index.
	FrameStack []string
	// Kind is the transition kind that produced this event. Most OTel
	// span starts reduce to KindCall.
	Kind TransitionKind
	// Role classifies execution context; carried through to the
	// fingerprint so the same code at startup and at request time are
	// distinct states.
	Role RoleTag
}

// IngestSource records a single ingest run. Stored so divergence
// queries can scope by commit, env, or kind.
type IngestSource struct {
	ID        string     `json:"id"`
	Kind      SourceKind `json:"kind"`
	StartedAt int64      `json:"started_at"`
	CommitSHA string     `json:"commit_sha,omitempty"`
	Env       string     `json:"env,omitempty"`
	// Path is the absolute or repo-relative path of the ingested
	// file when applicable. Empty for endpoint ingest.
	Path string `json:"path,omitempty"`
	// EventCount is the number of canonical events the ingest
	// produced before reduction.
	EventCount int `json:"event_count"`
}

// RuntimeState is the distilled "what's running" view a series of
// events collapses to. Equality is defined by Fingerprint; everything
// else (counts, timestamps) is summarisable metadata.
type RuntimeState struct {
	Fingerprint     string  `json:"fingerprint"`
	TopSymbol       string  `json:"top_symbol"`
	CallerChainHash string  `json:"caller_chain_hash"`
	Role            RoleTag `json:"role"`
	FirstSeen       int64   `json:"first_seen"`
	LastSeen        int64   `json:"last_seen"`
	Count           int     `json:"count"`
	// Sources lists every ingest source ID that observed this state.
	// Two-source observation is a useful signal for divergence queries.
	Sources []string `json:"sources,omitempty"`
	// CallerFrames carries the unhashed top-N caller frames so the
	// reconcile pass can score fuzzy candidates by caller-chain
	// proximity (module locality), not just name similarity. Bounded
	// to CallerChainDepth entries.
	CallerFrames []string `json:"caller_frames,omitempty"`
}

// RuntimeTransition records that the runtime moved from one state to
// another. Two transitions are "the same" iff (FromFP, ToFP, Kind)
// match — counts aggregate, timestamps widen.
type RuntimeTransition struct {
	FromFP    string         `json:"from_fp"`
	ToFP      string         `json:"to_fp"`
	Kind      TransitionKind `json:"kind"`
	Count     int            `json:"count"`
	FirstSeen int64          `json:"first_seen"`
	LastSeen  int64          `json:"last_seen"`
	Sources   []string       `json:"sources,omitempty"`
}

// StateSuggestion mirrors AutoMobile's NavigationSuggestion: when a
// frame's top symbol can't be exactly placed in the static symbol
// index, we record candidate matches with evidence so a later pass —
// or a human — can reconcile.
type StateSuggestion struct {
	Fingerprint     string  `json:"fingerprint"`
	CandidateSymbol string  `json:"candidate_symbol"`
	Evidence        string  `json:"evidence"`
	Confidence      float64 `json:"confidence"`
}

// Resolution classifies a state's reconciliation outcome against the
// static symbol index. Stored separately from RuntimeState so that
// re-running reconcile against a new commit's index does not require
// rewriting the state table.
type Resolution string

const (
	// ResolvedExact means the top symbol matched a static-index FQN
	// (or a unique simple name when FQN was not provided).
	ResolvedExact Resolution = "exact"
	// ResolvedFuzzy means the top symbol matched a static-index
	// symbol through name similarity / caller-chain heuristics; one
	// or more StateSuggestion rows describe the candidates.
	ResolvedFuzzy Resolution = "fuzzy"
	// Unresolved means no static-index symbol matched — the state is
	// a "phantom" from the static graph's perspective.
	Unresolved Resolution = "unresolved"
)
