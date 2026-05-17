package arch

import (
	"sort"
	"strings"
)

// AbiSignatureSimpleNames returns the deduped, sorted slice of simple
// symbol names from sigs. "Simple name" is the last '.'-separated
// segment of each AbiSignature's FQN. Used by the warm-path freshness
// gate to feed names into CodeIndex.TransitiveDependents when a file's
// public ABI changes: every name that survived the prior run is a
// possible identifier dependents textually reference.
//
// Anonymous/empty FQN segments are skipped. The result is sorted for
// deterministic downstream behavior (stable StaleOraclePaths set across
// runs).
func AbiSignatureSimpleNames(sigs []AbiSignature) []string {
	if len(sigs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(sigs))
	for _, s := range sigs {
		name := simpleName(s.FQN)
		if name == "" {
			continue
		}
		seen[name] = true
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// simpleName returns the last '.'-separated segment of fqn. Empty
// string when fqn is empty or ends in '.'.
func simpleName(fqn string) string {
	if fqn == "" {
		return ""
	}
	idx := strings.LastIndex(fqn, ".")
	if idx < 0 {
		return fqn
	}
	if idx == len(fqn)-1 {
		return ""
	}
	return fqn[idx+1:]
}
