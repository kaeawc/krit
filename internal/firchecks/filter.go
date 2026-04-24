package firchecks

// filter.go — per-rule file filter for FIR checkers.
//
// Each FIR checker declares an Identifiers list; CollectFirCheckFiles
// partitions the file set so only files matching any declared identifier
// go to the JVM. Conservative default (AllFiles: true) for unaudited rules.
// Mirrors internal/oracle/filter.go.

import (
	"bytes"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
)

// FirFilterSpec is the per-rule declaration of which files a FIR checker
// needs to see. Mirrors oracle.OracleFilterSpec.
type FirFilterSpec struct {
	// Identifiers is a list of substrings; any file whose content contains
	// at least one is forwarded to the FIR checker. Conservative: false
	// positives waste JVM work but never lose findings.
	Identifiers []string
	// AllFiles: true means the checker needs every file (no reduction).
	AllFiles bool
}

// FirFilterRule is the minimal rule view used by CollectFirCheckFiles.
type FirFilterRule struct {
	Name   string
	Filter *FirFilterSpec
}

// FirFilterSummary describes the outcome of a filter evaluation.
type FirFilterSummary struct {
	TotalFiles  int
	MarkedFiles int
	AllFiles    bool
	// Paths is the sorted list of absolute paths any rule marked for FIR analysis.
	// Nil when AllFiles is true.
	Paths []string
}

// CollectFirCheckFiles returns the subset of files any enabled FIR rule
// wants to see. Matches oracle.CollectOracleFiles semantics exactly.
func CollectFirCheckFiles(rules []FirFilterRule, files []*scanner.File) FirFilterSummary {
	summary := FirFilterSummary{TotalFiles: len(files)}
	if len(rules) == 0 || len(files) == 0 {
		return summary
	}

	var identifiers [][]byte
	for _, r := range rules {
		f := r.Filter
		if f == nil || f.AllFiles {
			summary.AllFiles = true
			summary.MarkedFiles = summary.TotalFiles
			return summary
		}
		for _, id := range f.Identifiers {
			if id != "" {
				identifiers = append(identifiers, []byte(id))
			}
		}
	}
	identifiers = dedupFirBytes(identifiers)

	if len(identifiers) == 0 {
		summary.Paths = []string{}
		return summary
	}

	matched := make([]string, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		if anyBytesContainsFir(file.Content, identifiers) {
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
	return summary
}

func dedupFirBytes(ids [][]byte) [][]byte {
	if len(ids) < 2 {
		return ids
	}
	seen := make(map[string]struct{}, len(ids))
	out := ids[:0]
	for _, id := range ids {
		k := string(id)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func anyBytesContainsFir(content []byte, needles [][]byte) bool {
	for _, n := range needles {
		if bytes.Contains(content, n) {
			return true
		}
	}
	return false
}
