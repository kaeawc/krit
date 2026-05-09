// Package snapshot captures the structural state of a project at a given
// commit (the "graph blob" from PRD 1) and stores it under .krit/snapshots/
// for later timeline and diff queries. Phase A of the snapshot work covers
// only blob capture and storage; metrics rollup and the manifest/index land
// in subsequent phases.
package snapshot

// SchemaVersion is bumped whenever Blob's wire format changes in a way that
// breaks backwards-compatible decode of existing on-disk snapshots. Older
// blobs with a smaller version are still loadable by the current decoder
// for fields that have not changed; consumers compare versions before
// running cross-version diffs.
const SchemaVersion = 1

// Blob is the captured structural snapshot of a project at one commit.
// Stored on disk as zstd(gob(Blob)) to match the existing parse-cache
// conventions in internal/cacheutil.
type Blob struct {
	SchemaVersion int
	KritVersion   string
	CommitSHA     string
	CapturedAt    int64 // unix milliseconds
	RepoRoot      string

	Modules []Module
	Files   []File
	Symbols []Symbol
}

// Module records a Gradle module and its outgoing/incoming module edges.
// Paths are Gradle paths (":app", ":core:util"); Dir is relative to the
// blob's RepoRoot to keep snapshots portable across machines.
type Module struct {
	Path         string
	Dir          string
	Dependencies []ModuleDep
	Consumers    []string
}

// ModuleDep is one outgoing module-to-module edge.
type ModuleDep struct {
	Path          string
	Configuration string
}

// File records the parsed source files contributing to the snapshot.
// Path is relative to RepoRoot.
type File struct {
	Path     string
	Module   string
	Language string
	Lines    int
	Bytes    int
}

// Symbol mirrors scanner.Symbol but without the byte offsets and intern
// pointers, which are not meaningful across snapshots. File is relative
// to RepoRoot.
type Symbol struct {
	Name       string
	Kind       string
	Visibility string
	File       string
	Line       int
	Language   string
	Package    string
	FQN        string
	Owner      string
	Signature  string
	IsOverride bool
	IsTest     bool
}
