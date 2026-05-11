package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

// RedactionMarker is the prefix prepended to every hashed identifier
// so a reader can tell at a glance that a value was redacted (vs. a
// legitimate identifier that happens to look like hex). Diff readers
// that join across snapshots key on the full string so the marker is
// part of the join key — two redacted snapshots with the same source
// FQN still match.
const RedactionMarker = "krit-redacted:"

// hashedNameLen caps the hex-encoded portion of redacted identifiers
// at 16 characters (64 bits of SHA-256). 64 bits gives ~50% collision
// probability around 2^32 symbols — well above any real-world Kotlin
// project. Truncating keeps blob sizes manageable.
const hashedNameLen = 16

// RedactBlob replaces every source-identifier field on b with a
// stable one-way hash and flips b.Redacted. Idempotent: a blob that
// is already Redacted is returned unchanged so a CaptureOptions.Redact
// caller that runs RedactBlob defensively doesn't double-hash.
//
// Fields redacted:
//
//   - RepoRoot — absolute path on the capture machine, leaks the
//     user's directory layout. Cleared (not hashed) because it is
//     not a join key.
//   - File.Path — repo-relative path. Each path segment is hashed
//     individually so the directory shape (depth, fan-out) survives
//     while names do not.
//   - Symbol.Name / FQN / Owner / Package / Signature / File — all
//     source identifiers. Hashing preserves stable join keys for
//     diff while making reverse lookup infeasible.
//
// Fields kept clear: SchemaVersion, KritVersion, CommitSHA,
// CapturedAt, Symbol.Kind / Visibility / Line / Language /
// IsOverride / IsTest, File.Language / Lines / Bytes, Module.Path /
// Dir / Dependencies / Consumers. Module paths are gradle
// coordinates — public by convention and not a code-secrecy
// concern; if a downstream user needs them hashed too they can
// extend this function.
func RedactBlob(b *Blob) {
	if b == nil || b.Redacted {
		return
	}
	b.RepoRoot = ""
	for i := range b.Files {
		b.Files[i].Path = redactPath(b.Files[i].Path)
	}
	for i := range b.Symbols {
		s := &b.Symbols[i]
		s.Name = redactName(s.Name)
		s.FQN = redactName(s.FQN)
		s.Owner = redactName(s.Owner)
		s.Package = redactName(s.Package)
		s.Signature = redactName(s.Signature)
		s.File = redactPath(s.File)
	}
	sort.Slice(b.Symbols, func(i, j int) bool {
		if b.Symbols[i].FQN != b.Symbols[j].FQN {
			return b.Symbols[i].FQN < b.Symbols[j].FQN
		}
		return b.Symbols[i].Signature < b.Symbols[j].Signature
	})
	sort.Slice(b.Files, func(i, j int) bool { return b.Files[i].Path < b.Files[j].Path })
	b.Redacted = true
}

// RedactFindings rewrites Findings.ByRuleFile so the per-file path
// keys are hashed using the same scheme as RedactBlob. Rule IDs in
// ByRule and ByRuleFile stay in clear text — they are public krit
// identifiers and contain no caller-controlled content. Idempotent.
//
// Callers that pass the result to Diff must redact both snapshots
// consistently; mixing redacted and raw findings panics the diff
// guard on Findings.Redacted (see Diff).
func RedactFindings(f *Findings) {
	if f == nil || f.Redacted {
		return
	}
	if len(f.ByRuleFile) > 0 {
		redacted := make(map[string]map[string]int, len(f.ByRuleFile))
		for rule, perFile := range f.ByRuleFile {
			if len(perFile) == 0 {
				continue
			}
			rewritten := make(map[string]int, len(perFile))
			for path, count := range perFile {
				rewritten[redactPath(path)] += count
			}
			redacted[rule] = rewritten
		}
		f.ByRuleFile = redacted
	}
	f.Redacted = true
}

// redactName returns a stable hash of s. Empty input returns "" so
// fields that are legitimately blank (e.g. a symbol with no owner)
// stay distinguishable from hashed-empty.
func redactName(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return RedactionMarker + hex.EncodeToString(sum[:])[:hashedNameLen]
}

// redactPath hashes each segment of s individually and rejoins them
// with forward slashes. Preserves directory depth and fan-out (useful
// for "tests under module X grew by N" reporting) while hiding the
// names themselves. Empty input returns "".
//
// Paths in snapshots are already repo-relative + forward-slashed
// (relPath in capture.go enforces that), so segmentation is a
// trivial Split. If a non-slash separator slips in the result is
// still deterministic because hash(seg) is independent of the
// separator choice.
func redactPath(p string) string {
	if p == "" {
		return ""
	}
	// Normalise just in case a backslash leaked through on Windows.
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = redactName(seg)
	}
	return strings.Join(parts, "/")
}
