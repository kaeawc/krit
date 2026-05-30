package scanner

// AffectedSet computes a conservative, #608-safe superset of the files whose
// cross-file findings could change when changedFiles change, given the prior
// (pre-edit) and current (post-edit) code indexes.
//
// The returned set always includes changedFiles themselves, plus — computed
// against BOTH the prior and current index — for every changed file B:
//
//   - declaration -> referrers: every file that references a name B declares.
//     If B's declaration changed or was removed, those referrers may resolve
//     differently or flip a finding.
//   - reference -> declarers: every file that declares a name B references.
//     If B added or removed a reference, a declaring file's finding may flip
//     (e.g. an unused-declaration rule whose only use was in B).
//
// Unioning over both indexes is essential for correctness. An edit that
// REMOVES a declaration or reference leaves no trace in the current index, so
// the dependents that must be re-analyzed are only discoverable through the
// prior index. Conversely an edit that ADDS one is only in the current index.
// Including both never drops a real dependent; it only over-approximates,
// which is safe (the affected files are re-dispatched, never served stale).
//
// Either index may be nil — a cold start has no prior, and a degenerate call
// may have no current. Nil indexes contribute nothing. Empty changedFiles
// yields nil. The result order is unspecified.
func AffectedSet(prior, current *CodeIndex, changedFiles []string) []string {
	if len(changedFiles) == 0 {
		return nil
	}
	changed := make(map[string]bool, len(changedFiles))
	affected := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		if f == "" {
			continue
		}
		changed[f] = true
		affected[f] = true
	}
	if len(changed) == 0 {
		return nil
	}

	for _, idx := range []*CodeIndex{prior, current} {
		if idx == nil {
			continue
		}
		idx.collectAffected(changed, affected)
	}

	out := make([]string, 0, len(affected))
	for f := range affected {
		out = append(out, f)
	}
	return out
}

// collectAffected adds, into affected, every file related to one of the
// changed files through this index's declaration/reference tables, in both
// dependency directions. See AffectedSet for the directionality and the
// #608-safety argument.
func (idx *CodeIndex) collectAffected(changed, affected map[string]bool) {
	// Direction 1: declaration -> referrers.
	for file := range changed {
		for _, name := range idx.DeclaredNames(file) {
			for ref := range idx.ReferenceFiles(name) {
				affected[ref] = true
			}
		}
	}

	// Direction 2: reference -> declarers.
	for name := range idx.referencedNamesInFiles(changed) {
		for _, sym := range idx.SymbolsNamed(name) {
			if sym.File != "" {
				affected[sym.File] = true
			}
		}
	}
}

// referencedNamesInFiles returns the set of names referenced by any file in
// the given set, scanning the reference table once. The reference slice is
// read lock-free: like the symbol lookup maps it is stable for the lifetime of
// a published index, and AffectedSet's current-index argument is owned by the
// analyzing goroutine until publish.
func (idx *CodeIndex) referencedNamesInFiles(files map[string]bool) map[string]bool {
	if idx == nil || len(files) == 0 {
		return nil
	}
	names := make(map[string]bool)
	for _, ref := range idx.References {
		if files[ref.File] {
			names[ref.Name] = true
		}
	}
	return names
}
