package scanner

// PriorRemovedContributions records, per file, the symbol names/FQNs a file
// declared and the names it referenced in the prior index, captured just
// before an incremental rebuild removed those contributions.
//
// It exists because the daemon mutates the prior CodeIndex in place
// (BuildIndexIncremental adopts and rewrites the resident snapshot), so by the
// time dispatch runs, the edges an edit DELETED are gone from the index. Those
// removed edges are exactly what AffectedSetIncremental needs to stay
// #608-safe: an edit that deletes file B's only reference to symbol S must
// still re-analyze the file that declares S (its "S is unused" finding can
// flip), and that link is only discoverable from B's prior references.
type PriorRemovedContributions struct {
	// Declared maps a file path to the distinct names/FQNs it declared in
	// the prior index.
	Declared map[string][]string
	// Referenced maps a file path to the distinct names it referenced in
	// the prior index.
	Referenced map[string][]string
}

// SnapshotRemovedContributions records the declared and referenced names that
// the given files contribute to idx. Call it on the PRIOR index immediately
// before BuildIndexIncremental removes those files' contributions. The cost is
// bounded by the changed files' symbol/reference counts (a handful of files
// per edit), so it is cheap enough to capture unconditionally.
func (idx *CodeIndex) SnapshotRemovedContributions(files map[string]bool) PriorRemovedContributions {
	out := PriorRemovedContributions{
		Declared:   make(map[string][]string),
		Referenced: make(map[string][]string),
	}
	if idx == nil || len(files) == 0 {
		return out
	}
	for f := range files {
		if names := idx.DeclaredNames(f); len(names) > 0 {
			out.Declared[f] = names
		}
	}
	// Referenced names per file: single pass over the reference slice,
	// bucketed by the changed-file set.
	perFile := make(map[string]map[string]bool)
	for _, ref := range idx.References {
		if !files[ref.File] {
			continue
		}
		set := perFile[ref.File]
		if set == nil {
			set = make(map[string]bool)
			perFile[ref.File] = set
		}
		set[ref.Name] = true
	}
	for f, set := range perFile {
		names := make([]string, 0, len(set))
		for n := range set {
			names = append(names, n)
		}
		out.Referenced[f] = names
	}
	return out
}

// setLastRemoved stashes the prior removed contributions on the index. Called
// by the incremental overlay build after BuildIndexIncremental mutates the
// prior in place.
func (idx *CodeIndex) setLastRemoved(removed PriorRemovedContributions) {
	if idx == nil {
		return
	}
	r := removed
	idx.lastRemoved = &r
}

// LastRemovedContributions returns the contributions dropped by the most
// recent incremental rebuild that produced this index, or nil when the index
// came from a full build (nothing removed) or never went through the overlay
// path. The pipeline reads it to compute a #608-safe affected set.
func (idx *CodeIndex) LastRemovedContributions() *PriorRemovedContributions {
	if idx == nil {
		return nil
	}
	return idx.lastRemoved
}

// AffectedSetIncremental computes a conservative, #608-safe superset of the
// files whose cross-file findings could change when changedFiles change, given
// the post-edit (current) index and the prior contributions the edit removed.
//
// It is the daemon-path companion to AffectedSet, which needs a full prior
// index. Because the surviving endpoint of every removed edge still lives in
// current (e.g. an edit that removes B's reference to S leaves S's declaring
// file present), the removed-edge dependents are found by looking the removed
// names up in current — no synthetic prior index is required.
//
// For each changed file B it unions four dependency directions, all resolved
// against current's lookup tables:
//
//   - B's current declarations -> their referrers
//   - B's removed declarations -> their referrers (edit deleted the decl)
//   - B's current references -> their declarers
//   - B's removed references -> their declarers (edit deleted the reference)
//
// changedFiles are always present in the result. removed may be nil (no prior
// removals, e.g. a purely additive edit). current may be nil (degenerate).
func AffectedSetIncremental(current *CodeIndex, changedFiles []string, removed *PriorRemovedContributions) []string {
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
	if current == nil {
		out := make([]string, 0, len(affected))
		for f := range affected {
			out = append(out, f)
		}
		return out
	}

	addReferrers := func(name string) {
		for ref := range current.ReferenceFiles(name) {
			affected[ref] = true
		}
	}
	addDeclarers := func(name string) {
		for _, sym := range current.SymbolsNamed(name) {
			if sym.File != "" {
				affected[sym.File] = true
			}
		}
	}

	for file := range changed {
		// Current declarations -> referrers.
		for _, name := range current.DeclaredNames(file) {
			addReferrers(name)
		}
		// Removed declarations -> referrers.
		if removed != nil {
			for _, name := range removed.Declared[file] {
				addReferrers(name)
			}
		}
	}

	// Current references -> declarers.
	for name := range current.referencedNamesInFiles(changed) {
		addDeclarers(name)
	}
	// Removed references -> declarers.
	if removed != nil {
		for file := range changed {
			for _, name := range removed.Referenced[file] {
				addDeclarers(name)
			}
		}
	}

	out := make([]string, 0, len(affected))
	for f := range affected {
		out = append(out, f)
	}
	return out
}
