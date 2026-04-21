package oracle

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/scanner"
)

// OracleFilterRule is a minimal view of a rule that declared
// NeedsOracle. The caller (internal/rules.BuildOracleFilterRulesV2)
// pre-filters the registered v2 rules and passes only oracle-needing
// rules here — the filter inversion (roadmap:
// core-infra/oracle-filter-inversion.md) means rules without
// NeedsOracle never reach this package.
type OracleFilterRule struct {
	// Name is the rule identifier, used only for verbose reporting.
	Name string
	// Filter is the rule's declared oracle filter. A nil Filter (or
	// AllFiles: true) means the rule wants every file — the caller
	// should set AllFiles: true explicitly when no narrowing applies.
	Filter *OracleFilterSpec
}

// OracleFilterSpec is an in-package mirror of v2.OracleFilter. The
// value types are decoupled so internal/oracle does not import
// internal/rules. The fields match v2.OracleFilter 1:1.
type OracleFilterSpec struct {
	Identifiers []string
	AllFiles    bool
}

// OracleFilterSummary describes the outcome of a filter evaluation.
// Returned from CollectOracleFiles so the caller can decide whether to
// skip the filtering round trip (no reduction) and emit a verbose log.
type OracleFilterSummary struct {
	// TotalFiles is len(files).
	TotalFiles int
	// MarkedFiles is the number of files at least one rule wants oracle
	// access on. Always <= TotalFiles.
	MarkedFiles int
	// AllFiles is true when any enabled rule declared AllFiles: true
	// and the filter was short-circuited to the full set.
	AllFiles bool
	// Paths is the sorted list of absolute paths that any rule marked
	// for oracle access. When AllFiles is true, Paths is nil (the caller
	// should fall back to the unfiltered set).
	Paths []string
	// Fingerprint is a short hex-encoded SHA-256 prefix over the sorted
	// Paths. It is a stable identity for the oracle input set: narrowing
	// PRs can diff fingerprints to detect which files moved in or out
	// without a full finding diff. Empty when AllFiles is true (the
	// "full corpus" set is not fingerprinted — its identity is implicit).
	Fingerprint string
}

// CollectOracleFiles returns the subset of files any enabled rule wants
// oracle access on. Rules are represented by OracleFilterRule (the caller
// is expected to build this slice from v2.Registry via
// rules.GetOracleFilter).
//
// Matching semantics:
//
//  1. If any rule has AllFiles: true, the short-circuit returns
//     AllFiles: true and Paths: nil. Callers should treat this as "no
//     reduction — feed the full file set to the oracle".
//  2. Otherwise each file is marked when any rule's Identifiers list
//     contains a substring of the file's raw bytes. bytes.Contains is
//     conservative: false positives waste oracle work but never lose
//     findings.
//  3. Files that no rule marks are dropped from the returned Paths.
//
// The returned Paths are absolute (via filepath.Abs) so that krit-types
// can match them against its own source tree regardless of how the
// caller passed them in.
func CollectOracleFiles(rules []OracleFilterRule, files []*scanner.File) OracleFilterSummary {
	summary := OracleFilterSummary{TotalFiles: len(files)}

	if len(rules) == 0 || len(files) == 0 {
		return summary
	}

	// Short-circuit: any rule demanding AllFiles makes the filter
	// meaningless — bail out without scanning file contents.
	var identifiers [][]byte
	for _, r := range rules {
		f := r.Filter
		if f == nil || f.AllFiles {
			summary.AllFiles = true
			summary.MarkedFiles = summary.TotalFiles
			return summary
		}
		for _, id := range f.Identifiers {
			if id == "" {
				continue
			}
			identifiers = append(identifiers, []byte(id))
		}
	}

	// Dedup identifier substrings. Typical audited rule sets share
	// substrings ("suspend", " as ", "!!"); deduping saves repeated
	// substring scans without changing semantics.
	identifiers = dedupBytes(identifiers)

	if len(identifiers) == 0 {
		// All enabled rules are tree-sitter-only. No file needs the
		// oracle. Return an empty Paths set (not nil) so the caller can
		// still write an empty --files list.
		summary.Paths = []string{}
		summary.Fingerprint = fingerprintPaths(summary.Paths)
		return summary
	}

	matched := make([]string, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		if anyBytesContains(file.Content, identifiers) {
			abs, err := filepath.Abs(file.Path)
			if err != nil {
				abs = file.Path
			}
			matched = append(matched, abs)
		}
	}
	sort.Strings(matched)
	summary.MarkedFiles = len(matched)
	summary.Paths = matched
	summary.Fingerprint = fingerprintPaths(matched)
	return summary
}

// fingerprintPaths returns the first 16 hex chars of SHA-256 over the
// newline-joined sorted path list. 64 bits of collision space is
// plenty for human-diffable fingerprints across narrowing scans.
func fingerprintPaths(paths []string) string {
	h := hashutil.Hasher().New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// StableFingerprint returns a fingerprint of paths that is invariant
// under repository checkout location: each path is rewritten relative
// to root (slash-normalised) before hashing. Paths outside root fall
// back to their basename. An empty input returns the fingerprint of
// the empty set (matches fingerprintPaths(nil)).
//
// The CI oracle-fingerprint gate uses this instead of the absolute-
// path fingerprint emitted in perf output, since baseline files are
// checked in and must match across every contributor's checkout.
func StableFingerprint(paths []string, root string) string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	rels := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, rerr := filepath.Rel(absRoot, p)
		if rerr != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(p)
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	sort.Strings(rels)
	h := hashutil.Hasher().New()
	for _, p := range rels {
		h.Write([]byte(p))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// WriteFilterListFile writes the filter's matched paths to a temp file
// that krit-types can read via its --files flag. Returns the written path
// or an error. Callers should clean up the file after the oracle run.
// When AllFiles is true or the Paths list is nil, returns an empty
// string and no error — the caller is expected to skip the --files flag
// entirely in that case.
func WriteFilterListFile(summary OracleFilterSummary, tmpDir string) (string, error) {
	if summary.AllFiles || summary.Paths == nil {
		return "", nil
	}
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	f, err := os.CreateTemp(tmpDir, "krit-oracle-files-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, p := range summary.Paths {
		if _, err := f.WriteString(p); err != nil {
			return "", err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return "", err
		}
	}
	return f.Name(), nil
}

// dedupBytes returns ids with duplicate byte slices removed. The input
// order is preserved for the first occurrence of each unique value.
func dedupBytes(ids [][]byte) [][]byte {
	if len(ids) < 2 {
		return ids
	}
	seen := make(map[string]struct{}, len(ids))
	out := ids[:0]
	for _, id := range ids {
		k := string(id)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, id)
	}
	return out
}

// anyBytesContains reports whether content contains any of the given
// substrings. Short-circuits on the first hit.
func anyBytesContains(content []byte, needles [][]byte) bool {
	for _, n := range needles {
		if bytes.Contains(content, n) {
			return true
		}
	}
	return false
}
