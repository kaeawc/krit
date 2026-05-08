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
	VerbStatus         = "status"
	VerbShutdown       = "shutdown"
	VerbAbiHash        = "abi-hash"
	VerbAnalyzeBuffer  = "analyze-buffer"
	VerbAnalyzeBuffers = "analyze-buffers"
)

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
