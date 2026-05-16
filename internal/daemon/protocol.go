// Package daemon implements a long-lived krit process that keeps parse
// trees, the cross-file index, oracle state, and typeinfer caches resident
// in memory. CLI verbs in the build-integration cluster prefer the daemon
// when a socket is reachable and fall back to in-process execution
// otherwise.
//
// The protocol is line-delimited JSON over a Unix socket. Each request is
// a single JSON object terminated by a newline; each response is a single
// JSON object terminated by a newline.
package daemon

import "encoding/json"

// Request names a verb and carries its arguments as opaque JSON.
type Request struct {
	Verb string          `json:"verb"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Response is the wire form of a verb result. OK=false carries an Error
// message and an empty Data; OK=true carries the verb-specific Data.
type Response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Built-in verb names.
const (
	VerbStatus                  = "status"
	VerbShutdown                = "shutdown"
	VerbAbiHash                 = "abi-hash"
	VerbAnalyzeBuffer           = "analyze-buffer"
	VerbAnalyzeBuffers          = "analyze-buffers"
	VerbAnalyzeProject          = "analyze-project"
	VerbListRules               = "list-rules"
	VerbListExperiments         = "list-experiments"
	VerbValidateConfig          = "validate-config"
	VerbOracleFilterFingerprint = "oracle-filter-fingerprint"
)

// ErrBinaryHashMismatchPrefix is the error.Error() prefix the daemon
// emits when it rejects a request whose ClientBinaryHash does not
// match its own. CLI callers detect this prefix to fall back to
// in-process execution after printing a short "binary diverged"
// warning. The full error reads
// "binary hash mismatch (daemon=<x> client=<y>)" so logs carry both
// hashes for diagnostics. Mirrors the JSON-RPC error code -32001 the
// issue calls out, transported via the daemon's
// `{ok:false,error:...}` envelope instead of a code field.
const ErrBinaryHashMismatchPrefix = "binary hash mismatch"

// AbiHashArgs is the argument shape for the abi-hash verb.
type AbiHashArgs struct {
	Target string `json:"target"`
}

// AbiHashResult is the response payload for the abi-hash verb.
type AbiHashResult struct {
	Target string `json:"target"`
	Module string `json:"module,omitempty"`
	File   string `json:"file,omitempty"`
	Hash   string `json:"hash"`
	Inputs int    `json:"inputs"`
}

// StatusResult reports daemon readiness and basic warm-up stats.
type StatusResult struct {
	Ready       bool    `json:"ready"`
	Root        string  `json:"root"`
	Modules     int     `json:"modules"`
	Files       int     `json:"files"`
	WarmSeconds float64 `json:"warm_seconds"`
	// KritVersion is the daemon binary's compile-time version
	// string (e.g. "v0.42.0", "dev").
	KritVersion string `json:"krit_version,omitempty"`
	// BinaryHash is the SHA-256 of the daemon's running krit
	// binary. Clients with a different binary hash should restart
	// the daemon to avoid stale-protocol-or-rule-set drift.
	BinaryHash string `json:"binary_hash,omitempty"`
	// HasLibraryFacts reports whether the daemon's WorkspaceState
	// holds a cached *librarymodel.Facts. Useful for clients that
	// want to confirm cross-file warm state is populated before
	// running cross-file rules.
	HasLibraryFacts bool `json:"has_library_facts,omitempty"`
	// HasCodeIndex reports the same for *scanner.CodeIndex.
	HasCodeIndex bool `json:"has_code_index,omitempty"`
}

// AnalyzeBufferArgs carries an in-memory file body for single-buffer
// per-file rule dispatch. The daemon parses the buffer (or reuses a
// cached parse when content is identical) and runs the same per-file
// rule pass the LSP / MCP single-file paths use.
type AnalyzeBufferArgs struct {
	// Path is the filesystem path the buffer represents. Used as the
	// cache key and as the file label in findings. Empty paths default
	// to "input.kt".
	Path string `json:"path"`
	// Content is the buffer body. UTF-8 Kotlin source.
	Content string `json:"content"`
}

// AnalyzeBufferResult carries the per-file findings for an
// analyze-buffer call. Findings is the canonical JSON form of
// scanner.FindingColumns so the wire shape matches `krit -f json`
// output. CacheHit is true when the daemon's WorkspaceState served
// this buffer from a prior parse.
type AnalyzeBufferResult struct {
	Findings json.RawMessage `json:"findings"`
	CacheHit bool            `json:"cache_hit"`
}

// AnalyzeBuffersArgs is the batched form of AnalyzeBufferArgs. The
// daemon processes Buffers in order and returns one result per buffer.
// Clients with N staged files trade N dial+RTT cycles for one.
type AnalyzeBuffersArgs struct {
	Buffers []AnalyzeBufferArgs `json:"buffers"`
}

// AnalyzeBuffersResult mirrors AnalyzeBuffersArgs: one result per
// input buffer in matching order. A buffer-level error (e.g. parse
// failure) populates Error and leaves Findings empty for that entry,
// so the caller still gets dispositive results for the rest of the
// batch instead of one bad file failing the whole call.
type AnalyzeBuffersResult struct {
	Results []AnalyzeBufferEntry `json:"results"`
}

// AnalyzeBufferEntry is one slot in AnalyzeBuffersResult.Results.
type AnalyzeBufferEntry struct {
	Findings json.RawMessage `json:"findings,omitempty"`
	CacheHit bool            `json:"cache_hit"`
	Error    string          `json:"error,omitempty"`
}

// AnalyzeProjectArgs drives the analyze-project verb. The verb runs
// the same whole-project scan pipeline as `krit -f json` against the
// daemon's resident parse cache (and, in future commits, resident
// resolver and oracle), so the JSON shape returned in
// AnalyzeProjectResult.Findings matches the CLI output byte-for-byte
// modulo timing fields.
//
// Empty fields take daemon defaults; fields mirror the most useful
// CLI flags one-to-one so client wrappers can translate flag-by-flag.
type AnalyzeProjectArgs struct {
	// Paths is the explicit scan target. Empty means "use the
	// daemon's --root".
	Paths []string `json:"paths,omitempty"`
	// Format is the output format. Empty defaults to "json".
	Format string `json:"format,omitempty"`
	// BaselinePath, when non-empty, points at a baseline file used
	// to suppress known findings.
	BaselinePath string `json:"baseline,omitempty"`
	// DiffRef, when non-empty, restricts findings to files changed
	// since the given git ref.
	DiffRef string `json:"diff,omitempty"`
	// MinConfidence drops findings below the threshold from output.
	MinConfidence float64 `json:"min_confidence,omitempty"`
	// WarningsAsErrors promotes warning-severity findings to errors
	// before format dispatch.
	WarningsAsErrors bool `json:"warnings_as_errors,omitempty"`
	// IncludeGenerated retains files under */generated/* during parse.
	IncludeGenerated bool `json:"include_generated,omitempty"`
	// AllRules opts into experimental rules in addition to the
	// default core set.
	AllRules bool `json:"all_rules,omitempty"`
	// Experimental enables experimental flags whose default-off behavior
	// the daemon would otherwise suppress.
	Experimental bool `json:"experimental,omitempty"`
	// Strict enables the strict preset: rules whose effective Noisiness
	// is NoisinessNoisy are excluded unless explicitly named in
	// EnableRules.
	Strict bool `json:"strict,omitempty"`
	// EnableRules / DisableRules are comma-separated rule-id lists.
	EnableRules  string `json:"enable_rules,omitempty"`
	DisableRules string `json:"disable_rules,omitempty"`
	// CustomRuleJars are Kotlin custom-rule plugin jars loaded by the
	// resident krit-types daemon.
	CustomRuleJars []string `json:"custom_rule_jars,omitempty"`
	// RequireWarm, when true, makes the verb fail fast (with a typed
	// error) instead of paying cold-warm cost. Clients that want a
	// hard SLA set this — useful for IDE workflows that can show a
	// "warming up" indicator instead of blocking on the first call.
	RequireWarm bool `json:"require_warm,omitempty"`
	// ClientBinaryHash is the SHA-256 of the calling CLI's krit
	// binary. When non-empty and the daemon's hash is non-empty, the
	// daemon refuses the request when they differ — prevents a
	// freshly-installed CLI from talking to a stale daemon. Empty
	// disables the handshake (back-compat for clients that don't
	// know their hash).
	ClientBinaryHash string `json:"client_binary_hash,omitempty"`
	// ShowPerf mirrors --perf: when true the daemon wires a
	// perf.Tracker into the pipeline so OutputPhase emits the
	// hierarchical timing tree in the JSON header (under
	// "performance"). PerfRules + ProfileDispatch ride on top of
	// this flag — both require ShowPerf to be true.
	ShowPerf bool `json:"show_perf,omitempty"`
	// PerfRules mirrors --perf-rules: when true (and ShowPerf is
	// true) the daemon also returns the per-rule execution-stat
	// ranking. Requires the dispatcher to record per-rule timings,
	// which the tracker arms automatically when active.
	PerfRules bool `json:"perf_rules,omitempty"`
}

// AnalyzeProjectResult is the response payload. Findings is the raw
// bytes the OutputPhase formatter wrote, so callers can
// `json.Unmarshal(res.Findings, &theirSchema)` with no shape change
// from `krit -f json`.
type AnalyzeProjectResult struct {
	Findings json.RawMessage     `json:"findings"`
	Stats    AnalyzeProjectStats `json:"stats"`
}

// AnalyzeProjectStats reports per-call observability data — useful
// for clients that want to log warm-vs-cold cadence, or for tests
// asserting that the dirty-set behaves as expected after a single-
// file change.
type AnalyzeProjectStats struct {
	FilesScanned    int     `json:"files_scanned"`
	FindingsCount   int     `json:"findings_count"`
	WallSeconds     float64 `json:"wall_seconds"`
	CodeIndexHit    bool    `json:"code_index_hit"`
	LibraryFactsHit bool    `json:"library_facts_hit"`
	// ResolverHit reports whether the resident type-resolver slot was
	// consulted and served (cached pointer reused). Mirrors
	// CodeIndexHit semantics — true means the slot was populated when
	// the verb ran; the run may have rebuilt mid-verb on fingerprint
	// mismatch.
	ResolverHit bool `json:"resolver_hit"`
	// OracleFilterHit reports whether the resident oracle call-filter
	// slot was populated at verb entry. Useful for confirming that
	// PR-C's filter cache is warm after the first oracle-enabled call.
	OracleFilterHit bool `json:"oracle_filter_hit"`
	// DirtyFiles is the count of files Touched in WorkspaceState
	// since the last analyze-project call (drained at the start of
	// this call). Useful for clients that want to show "N files
	// changed since last scan" UX.
	DirtyFiles int `json:"dirty_files"`
	// Cold is true on the first analyze-project call after daemon
	// startup; subsequent calls report false.
	Cold bool `json:"cold"`
	// ParseHits and ParseMisses are the per-call delta against the
	// resident parse cache (combined Kotlin + Java). Both stay 0 when
	// no parse cache is attached.
	ParseHits   int64 `json:"parse_hits"`
	ParseMisses int64 `json:"parse_misses"`
	// FindingsBundleHit is true when the whole-run findings cache
	// served the result without redoing dispatch or cross-file work.
	// A true value means the call was a structural reuse of a prior
	// identical-input run.
	FindingsBundleHit bool `json:"findings_bundle_hit"`
	// PhaseTimingsMs is the per-phase wall-time breakdown for this
	// call. Phases skipped on a findings-bundle hit (dispatch,
	// crossfile, android) report 0. Useful for diagnosing which phase
	// dominates a slow warm call without a full pprof capture.
	PhaseTimingsMs PhaseTimingsMs `json:"phase_timings_ms"`
}

// PhaseTimingsMs mirrors pipeline.PhaseTimingsMs on the wire so
// daemon clients can introspect per-phase cost without a separate
// fetch. All values are wall-clock milliseconds.
type PhaseTimingsMs struct {
	Parse     int64 `json:"parse"`
	Index     int64 `json:"index"`
	Dispatch  int64 `json:"dispatch"`
	CrossFile int64 `json:"crossfile"`
	Android   int64 `json:"android"`
	Fixup     int64 `json:"fixup"`
	Output    int64 `json:"output"`
}

// MetaResult is the response payload shared by the read-only meta
// verbs (list-rules, list-experiments, validate-config,
// oracle-filter-fingerprint). The daemon captures the formatted bytes
// the in-process flag handlers would have written and the exit code
// they would have returned; the CLI replays them against its own
// stdout/stderr + exit so daemon-routed and in-process invocations
// remain byte-equivalent.
type MetaResult struct {
	Stdout   []byte `json:"stdout,omitempty"`
	Stderr   []byte `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// ListRulesArgs mirrors the --list-rules flag knobs. CustomRuleJars is
// intentionally omitted: the daemon-compatibility gate already keeps
// callers with --custom-rule-jars on the in-process path because plugin
// rule discovery requires the krit-types JVM.
type ListRulesArgs struct {
	// Verbose mirrors -v: include fix levels, precision, maturity,
	// description.
	Verbose bool `json:"verbose,omitempty"`
	// Maturity filters by lifecycle (stable, experimental, deprecated).
	// Empty means no filter.
	Maturity string `json:"maturity,omitempty"`
	// TaxonomyID filters by --cwe (CWE / OWASP / SEI-CERT / MITRE).
	TaxonomyID string `json:"taxonomy,omitempty"`
}

// ListExperimentsArgs mirrors the --list-experiments flag knobs.
type ListExperimentsArgs struct {
	// Format is the output format ("json" or "plain"). Empty defaults
	// to "json" matching the CLI's default.
	Format string `json:"format,omitempty"`
}

// ValidateConfigArgs is the validate-config payload. The daemon uses
// its resident config (loaded from --root); the CLI may pass an
// explicit path via ConfigPath to override.
type ValidateConfigArgs struct {
	// ConfigPath, when non-empty, is the absolute path of the
	// krit.yml the daemon should validate. Empty means the daemon
	// uses its already-loaded resident config.
	ConfigPath string `json:"config_path,omitempty"`
}

// OracleFilterFingerprintArgs drives the oracle-filter-fingerprint
// verb. Paths is the explicit scan target; empty falls back to the
// daemon's --root. AllRules toggles the rule-set label between
// "default" and "all-rules" and broadens activeRules.
type OracleFilterFingerprintArgs struct {
	Paths    []string `json:"paths,omitempty"`
	AllRules bool     `json:"all_rules,omitempty"`
}
