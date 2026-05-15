// Package breakage records failure-localization events keyed to the
// snapshot timeline. Events name a failure (test, build, runtime, or a
// krit-finding regression) and carry just enough structure for the
// internal/bisect package to fuse location signals across them.
package breakage

// EventsSchemaVersion versions the on-disk events.json layout. Bumped
// when a reader couldn't tolerate a new field; readers seeing a higher
// version fall back to an empty event set rather than erroring out.
const EventsSchemaVersion = 1

const (
	SourceCI          = "ci"
	SourceLocal       = "local"
	SourceKritFinding = "krit-finding"
	SourceRuntime     = "runtime"
	SourceSelfCapture = "self-capture"
)

const (
	KindTestFailure           = "test-failure"
	KindBuildFailure          = "build-failure"
	KindRuntimeFailure        = "runtime-failure"
	KindKritFindingRegression = "krit-finding-regression"
)

// Event is a single recorded breakage. All fields are stable across
// ingest sources so the bisect layer can join them on (signature,
// failure_kind) without dispatching by source.
type Event struct {
	ID          string `json:"id"`
	OccurredAt  int64  `json:"occurred_at"`
	CommitSHA   string `json:"commit_sha"`
	FailureKind string `json:"failure_kind"`
	Signature   string `json:"signature"`
	Module      string `json:"module,omitempty"`
	File        string `json:"file,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Source      string `json:"source"`
	// Frames is the optional stack-frame ladder for runtime failures.
	// Tier-1 signal fusion walks these in order, top frame first, until
	// it finds one mapped to a symbol in the snapshot graph.
	Frames []string `json:"frames,omitempty"`
	// Message is a human-readable failure description. Not used for
	// ranking; surfaced in bisect output so operators can correlate
	// against their original error logs.
	Message string `json:"message,omitempty"`
}
