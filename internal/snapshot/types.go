// Package snapshot captures per-commit structural state (modules,
// files, symbols, scalar metrics) and persists it under
// `.krit/snapshots/` for later timeline and diff queries.
package snapshot

// SchemaVersion is bumped when Blob's wire format changes in a way
// that breaks decode of existing on-disk snapshots. Cross-version
// diffs compare this before reading the rest.
const SchemaVersion = 1

// Blob is the captured structural snapshot of a project at one commit,
// persisted as zstd(gob(Blob)).
//
// Module.Dir, File.Path, and Symbol.File are repo-relative; RepoRoot is
// the absolute path on the capture machine and is not portable.
type Blob struct {
	SchemaVersion int
	KritVersion   string
	CommitSHA     string
	CapturedAt    int64
	RepoRoot      string

	Modules []Module
	Files   []File
	Symbols []Symbol
}

type Module struct {
	Path         string
	Dir          string
	Dependencies []ModuleDep
	Consumers    []string
}

type ModuleDep struct {
	Path          string
	Configuration string
}

type File struct {
	Path     string
	Module   string
	Language string
	Lines    int
	Bytes    int
}

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
