package pipeline

import (
	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// maybeExpandStaleByAbiDependents is the entry-point gate: returns
// in.StaleOraclePaths unchanged when transitive expansion can't run
// (no stale seed, no codeIndex, or no prior ABI hashes), otherwise
// delegates to expandStaleByAbiDependents.
func maybeExpandStaleByAbiDependents(in IndexInput, idx *scanner.CodeIndex) []string {
	if len(in.StaleOraclePaths) == 0 || idx == nil || len(in.PriorAbiHashes) == 0 {
		return in.StaleOraclePaths
	}
	return expandStaleByAbiDependents(in, idx)
}

// expandStaleByAbiDependents extends in.StaleOraclePaths with the
// transitive dependent set of any stale file whose current public ABI
// hash differs from in.PriorAbiHashes. Returns the original slice
// unchanged when nothing was promoted to transitive scope.
//
// Inputs must satisfy: idx non-nil, in.PriorAbiHashes non-empty,
// in.StaleOraclePaths non-empty. Caller is responsible for the gate.
// The function is idempotent — re-running on its own output yields
// the same set.
//
// Dependents are resolved by querying idx.TransitiveDependents with
// the simple-name set extracted from the CURRENT file's ABI signatures.
// Using current (vs prior) names handles the common cases (add,
// modify, rename-with-overlap) and gracefully degrades on
// rename/delete: a removed symbol's old name is still a textual
// reference target in any dependent file, but its absence from the
// current signature set means we'd miss it. That tradeoff matches the
// available data — the manifest only stores the per-file ABI hash,
// not the signature list itself. Deletion-aware invalidation would
// require persisting the full signature set; not worth the manifest
// bloat for the marginal case.
func expandStaleByAbiDependents(in IndexInput, idx *scanner.CodeIndex) []string {
	staleSet := make(map[string]bool, len(in.StaleOraclePaths))
	for _, p := range in.StaleOraclePaths {
		staleSet[p] = true
	}
	parsedByPath := make(map[string]*scanner.File, len(in.KotlinFiles))
	for _, f := range in.KotlinFiles {
		if f != nil {
			parsedByPath[f.Path] = f
		}
	}

	var abiChanged []string
	for _, path := range in.StaleOraclePaths {
		f, ok := parsedByPath[path]
		if !ok {
			// Stale path wasn't parsed this run (deleted file, off-scan,
			// etc.) — can't compute current ABI; keep per-file scope.
			continue
		}
		prior, hadPrior := in.PriorAbiHashes[path]
		if !hadPrior {
			// New file (no prior ABI to compare) — treat any reference
			// to its current public names as transitively stale too.
			abiChanged = append(abiChanged, path)
			continue
		}
		current := arch.HashAbiSignatures(arch.ExtractAbiSignatures([]*scanner.File{f}))
		if current != prior {
			abiChanged = append(abiChanged, path)
		}
	}
	if len(abiChanged) == 0 {
		return in.StaleOraclePaths
	}

	addedDependents := 0
	for _, path := range abiChanged {
		f := parsedByPath[path]
		names := arch.AbiSignatureSimpleNames(arch.ExtractAbiSignatures([]*scanner.File{f}))
		if len(names) == 0 {
			continue
		}
		for _, dep := range idx.TransitiveDependents(names, path) {
			if staleSet[dep] {
				continue
			}
			staleSet[dep] = true
			addedDependents++
		}
	}
	if addedDependents == 0 {
		return in.StaleOraclePaths
	}

	expanded := make([]string, 0, len(staleSet))
	for p := range staleSet {
		expanded = append(expanded, p)
	}
	perf.AddEntryDetails(in.Tracker, "freshnessGateTransitiveExpansion", 0, map[string]int64{
		"abiChanged":   int64(len(abiChanged)),
		"addedDeps":    int64(addedDependents),
		"originalSize": int64(len(in.StaleOraclePaths)),
		"expandedSize": int64(len(expanded)),
	}, nil)
	return expanded
}
