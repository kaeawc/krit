package scanner

import (
	"regexp"
	"strings"

	"github.com/bits-and-blooms/bloom/v3"
)

var sourceImportRe = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_$][A-Za-z0-9_$.*]*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?\s*;?`)

// SymbolsNamed returns declarations indexed under a simple or fully-qualified
// name. A nil result means no matching declaration is known.
//
// The returned slice is a snapshot — concurrent mutation via
// BuildIndexIncremental cannot tear it.
func (idx *CodeIndex) SymbolsNamed(name string) []Symbol {
	if idx == nil || name == "" {
		return nil
	}
	idx.mu.RLock()
	syms := idx.symbolsByName[name]
	if syms == nil {
		idx.mu.RUnlock()
		return nil
	}
	out := make([]Symbol, len(syms))
	copy(out, syms)
	idx.mu.RUnlock()
	return out
}

// SymbolByFQN returns the declaration with the exact fully-qualified name.
func (idx *CodeIndex) SymbolByFQN(fqn string) (Symbol, bool) {
	if idx == nil || fqn == "" {
		return Symbol{}, false
	}
	idx.mu.RLock()
	sym, ok := idx.symbolsByFQN[fqn]
	idx.mu.RUnlock()
	return sym, ok
}

// ResolveType resolves a type name from a Kotlin or Java source file using
// source-visible package and import information plus declarations in the
// mixed-language CodeIndex.
func (idx *CodeIndex) ResolveType(file *File, name string) []ResolvedSymbol {
	if idx == nil || name == "" {
		return nil
	}
	imports, wildcards := sourceImports(file)
	if imported := imports[name]; imported != "" {
		return idx.resolveSymbols([]string{imported}, isTypeSymbol)
	}
	if strings.Contains(name, ".") {
		return idx.resolveSymbols([]string{name}, isTypeSymbol)
	}
	var candidates []string
	if !strings.Contains(name, ".") {
		if pkg := packageNameForFile(file); pkg != "" {
			candidates = append(candidates, pkg+"."+name)
		}
		for _, wildcard := range wildcards {
			candidates = append(candidates, wildcard+"."+name)
		}
	}
	resolved := idx.resolveSymbols(candidates, isTypeSymbol)
	if len(resolved) > 0 {
		return resolved
	}
	return idx.resolveSymbols([]string{name}, isTypeSymbol)
}

// ResolveCallable resolves a function/method/property callable from the mixed
// source index. arity < 0 disables arity filtering.
func (idx *CodeIndex) ResolveCallable(file *File, receiver, name string, arity int) []ResolvedSymbol {
	if idx == nil || name == "" {
		return nil
	}
	var ownerCandidates map[string]bool
	if receiver != "" {
		ownerCandidates = make(map[string]bool)
		for _, sym := range idx.ResolveType(file, receiver) {
			ownerCandidates[sym.FQN] = true
			ownerCandidates[sym.Symbol.Name] = true
		}
		if len(ownerCandidates) == 0 {
			ownerCandidates[receiver] = true
		}
	}
	imports, _ := sourceImports(file)
	if imported := imports[name]; imported != "" {
		return idx.resolveSymbols([]string{imported}, func(sym Symbol) bool {
			return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
		})
	}
	if strings.Contains(name, ".") {
		return idx.resolveSymbols([]string{name}, func(sym Symbol) bool {
			return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
		})
	}
	var candidates []string
	if pkg := packageNameForFile(file); pkg != "" {
		candidates = append(candidates, pkg+"."+name)
	}
	accept := func(sym Symbol) bool {
		return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
	}
	resolved := idx.resolveSymbols(candidates, accept)
	if len(resolved) > 0 {
		return resolved
	}
	return idx.resolveSymbols([]string{name}, accept)
}

func isTypeSymbol(sym Symbol) bool {
	switch sym.Kind {
	case "class", "interface", "object", "enum", "record", "annotation":
		return true
	default:
		return false
	}
}

func isCallableSymbol(sym Symbol) bool {
	switch sym.Kind {
	case "function", "method", "property", "field", "constructor":
		return true
	default:
		return false
	}
}

func callableMatches(sym Symbol, ownerCandidates map[string]bool, arity int) bool {
	if arity >= 0 && sym.Arity != arity {
		return false
	}
	if len(ownerCandidates) > 0 && !ownerCandidates[sym.Owner] {
		return false
	}
	return true
}

func (idx *CodeIndex) resolveSymbols(names []string, accept func(Symbol) bool) []ResolvedSymbol {
	seen := map[string]bool{}
	var out []ResolvedSymbol
	for _, name := range names {
		for _, sym := range idx.SymbolsNamed(name) {
			key := sym.FQN + "|" + sym.Signature + "|" + sym.File
			if seen[key] || !accept(sym) {
				continue
			}
			seen[key] = true
			out = append(out, ResolvedSymbol{
				FQN:      sym.FQN,
				Language: sym.Language,
				Owner:    sym.Owner,
				Kind:     sym.Kind,
				Symbol:   sym,
			})
		}
	}
	return out
}

func sourceImports(file *File) (map[string]string, []string) {
	explicit := map[string]string{}
	if file == nil {
		return explicit, nil
	}
	var wildcards []string
	for _, match := range sourceImportRe.FindAllStringSubmatch(string(file.Content), -1) {
		target := strings.TrimSpace(match[1])
		if target == "" {
			continue
		}
		if strings.HasSuffix(target, ".*") {
			wildcards = append(wildcards, strings.TrimSuffix(target, ".*"))
			continue
		}
		simple := target
		if alias := strings.TrimSpace(match[2]); alias != "" {
			simple = alias
		} else if dot := strings.LastIndex(target, "."); dot >= 0 {
			simple = target[dot+1:]
		}
		explicit[simple] = target
	}
	return explicit, wildcards
}

// buildReferenceLookupsLocked rebuilds the per-name reference aggregates
// from idx.References. Caller must hold idx.mu for writing (or be the
// constructor — initial build runs before any pointer to idx escapes).
func (idx *CodeIndex) buildReferenceLookupsLocked(addToBloom bool) {
	if len(idx.References) == 0 {
		idx.refCountByName = make(map[string]int)
		idx.refFilesByName = make(map[string]map[string]bool)
		idx.nonCommentRefFilesByName = make(map[string]map[string]bool)
		idx.nonCommentRefCountByNameFile = make(map[string]map[string]int)
		return
	}

	estimatedNames := estimateUniqueReferenceNames(len(idx.References))
	nameToAgg := make(map[string]int, estimatedNames)
	aggs := make([]referenceAggregate, 0, estimatedNames)

	for _, ref := range idx.References {
		aggIdx, ok := nameToAgg[ref.Name]
		if !ok {
			aggIdx = len(aggs)
			nameToAgg[ref.Name] = aggIdx
			aggs = append(aggs, referenceAggregate{name: ref.Name})
		}
		aggs[aggIdx].add(ref)
	}

	idx.refCountByName = make(map[string]int, len(aggs))
	idx.refFilesByName = make(map[string]map[string]bool, len(aggs))
	idx.nonCommentRefFilesByName = make(map[string]map[string]bool, len(aggs))
	idx.nonCommentRefCountByNameFile = make(map[string]map[string]int, len(aggs))

	for i := range aggs {
		agg := &aggs[i]
		idx.refCountByName[agg.name] = agg.count
		idx.refFilesByName[agg.name] = agg.files
		if len(agg.nonCommentFiles) > 0 {
			idx.nonCommentRefFilesByName[agg.name] = agg.nonCommentFiles
			idx.nonCommentRefCountByNameFile[agg.name] = agg.nonCommentCounts
		}
		if addToBloom {
			idx.refBloom.AddString(agg.name)
		}
	}
}

type referenceAggregate struct {
	name             string
	count            int
	files            map[string]bool
	nonCommentFiles  map[string]bool
	nonCommentCounts map[string]int
}

func (a *referenceAggregate) add(ref Reference) {
	a.count++
	if a.files == nil {
		a.files = make(map[string]bool, 1)
	}
	a.files[ref.File] = true

	if ref.InComment {
		return
	}
	if a.nonCommentFiles == nil {
		a.nonCommentFiles = make(map[string]bool, 1)
		a.nonCommentCounts = make(map[string]int, 1)
	}
	a.nonCommentFiles[ref.File] = true
	a.nonCommentCounts[ref.File]++
}

// addReferenceLookupLocked records a single reference into the per-name
// maps and bloom filter. Caller must hold idx.mu for writing.
func (idx *CodeIndex) addReferenceLookupLocked(ref Reference) {
	if idx.refCountByName == nil {
		idx.refCountByName = make(map[string]int)
	}
	if idx.refFilesByName == nil {
		idx.refFilesByName = make(map[string]map[string]bool)
	}
	if idx.nonCommentRefFilesByName == nil {
		idx.nonCommentRefFilesByName = make(map[string]map[string]bool)
	}
	if idx.nonCommentRefCountByNameFile == nil {
		idx.nonCommentRefCountByNameFile = make(map[string]map[string]int)
	}
	if idx.refBloom == nil {
		idx.refBloom = bloom.NewWithEstimates(1000, 0.01)
	}

	idx.refCountByName[ref.Name]++
	files := idx.refFilesByName[ref.Name]
	if files == nil {
		files = make(map[string]bool, 1)
		idx.refFilesByName[ref.Name] = files
	}
	files[ref.File] = true
	if !ref.InComment {
		ncFiles := idx.nonCommentRefFilesByName[ref.Name]
		if ncFiles == nil {
			ncFiles = make(map[string]bool, 1)
			idx.nonCommentRefFilesByName[ref.Name] = ncFiles
		}
		ncFiles[ref.File] = true
		counts := idx.nonCommentRefCountByNameFile[ref.Name]
		if counts == nil {
			counts = make(map[string]int, 1)
			idx.nonCommentRefCountByNameFile[ref.Name] = counts
		}
		counts[ref.File]++
	}
	idx.refBloom.AddString(ref.Name)
}

// removeReferenceLookupLocked removes a single reference from the per-name
// maps. The bloom filter is left untouched — it tolerates extra bits as
// false positives. Caller must hold idx.mu for writing.
func (idx *CodeIndex) removeReferenceLookupLocked(ref Reference) {
	if idx.refCountByName != nil {
		if next := idx.refCountByName[ref.Name] - 1; next > 0 {
			idx.refCountByName[ref.Name] = next
		} else {
			delete(idx.refCountByName, ref.Name)
		}
	}
	if files := idx.refFilesByName[ref.Name]; files != nil {
		delete(files, ref.File)
		if len(files) == 0 {
			delete(idx.refFilesByName, ref.Name)
		}
	}
	if ref.InComment {
		return
	}
	if files := idx.nonCommentRefFilesByName[ref.Name]; files != nil {
		delete(files, ref.File)
		if len(files) == 0 {
			delete(idx.nonCommentRefFilesByName, ref.Name)
		}
	}
	if counts := idx.nonCommentRefCountByNameFile[ref.Name]; counts != nil {
		delete(counts, ref.File)
		if len(counts) == 0 {
			delete(idx.nonCommentRefCountByNameFile, ref.Name)
		}
	}
}

func estimateUniqueReferenceNames(refCount int) int {
	if refCount <= 1024 {
		return refCount
	}
	estimated := refCount / 16
	if estimated < 1024 {
		return 1024
	}
	if estimated > 262144 {
		return 262144
	}
	return estimated
}

// MayHaveReference reports whether name appears in the bloom filter. False
// positives are possible; false negatives are not.
func (idx *CodeIndex) MayHaveReference(name string) bool {
	if idx == nil {
		return false
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.refBloom == nil {
		return false
	}
	return idx.refBloom.TestString(name)
}

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) ReferenceCount(name string) int {
	if idx == nil {
		return 0
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.refCountByName[name]
}

// ReferenceFiles returns a snapshot of the set of files that reference a
// name. The returned map is owned by the caller — mutating it does not
// affect the index, and concurrent index mutation cannot tear it.
func (idx *CodeIndex) ReferenceFiles(name string) map[string]bool {
	if idx == nil {
		return nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	files := idx.refFilesByName[name]
	if len(files) == 0 {
		return nil
	}
	out := make(map[string]bool, len(files))
	for f, v := range files {
		out[f] = v
	}
	return out
}

// SymbolReferenceCount returns the total number of references that can identify
// sym by either simple name or fully-qualified name.
func (idx *CodeIndex) SymbolReferenceCount(sym Symbol) int {
	if idx == nil {
		return 0
	}
	count := 0
	for _, name := range symbolReferenceNames(sym) {
		count += idx.ReferenceCount(name)
	}
	return count
}

// IsReferencedOutsideFile checks if a name is referenced in any file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFile(name, file string) bool {
	if idx == nil {
		return false
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	// Fast path: bloom filter says name not referenced at all
	if idx.refBloom == nil || !idx.refBloom.TestString(name) {
		return false
	}
	files := idx.refFilesByName[name]
	if files == nil {
		return false
	}
	for f := range files {
		if f != file {
			return true
		}
	}
	return false
}

// IsSymbolReferencedOutsideFile checks whether sym is referenced from another
// file by either simple name or fully-qualified name.
func (idx *CodeIndex) IsSymbolReferencedOutsideFile(sym Symbol, ignoreCommentRefs bool) bool {
	if idx == nil {
		return false
	}
	for _, name := range symbolReferenceNames(sym) {
		if ignoreCommentRefs {
			if idx.IsReferencedOutsideFileExcludingComments(name, sym.File) {
				return true
			}
		} else if idx.IsReferencedOutsideFile(name, sym.File) {
			return true
		}
	}
	return false
}

// IsReferencedOutsideFileExcludingComments checks if a name has any non-comment
// reference in a file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFileExcludingComments(name, file string) bool {
	if idx == nil {
		return false
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	// Fast path: bloom filter says name not referenced at all
	if idx.refBloom == nil || !idx.refBloom.TestString(name) {
		return false
	}
	files := idx.nonCommentRefFilesByName[name]
	if files == nil {
		return false
	}
	for f := range files {
		if f != file {
			return true
		}
	}
	return false
}

// CountNonCommentRefsInFile counts references to a name in a file that are NOT inside comments.
func (idx *CodeIndex) CountNonCommentRefsInFile(name, file string) int {
	if idx == nil {
		return 0
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	files := idx.nonCommentRefCountByNameFile[name]
	if files == nil {
		return 0
	}
	return files[file]
}

func (idx *CodeIndex) countRefsInFileForSymbol(sym Symbol, file string, ignoreCommentRefs bool) int {
	if idx == nil {
		return 0
	}
	count := 0
	for _, name := range symbolReferenceNames(sym) {
		if ignoreCommentRefs {
			count += idx.CountNonCommentRefsInFile(name, file)
			continue
		}
		idx.mu.RLock()
		for _, ref := range idx.References {
			if ref.Name == name && ref.File == file {
				count++
			}
		}
		idx.mu.RUnlock()
	}
	return count
}

// UnusedSymbols returns symbols that are never referenced from any other file.
// If ignoreCommentRefs is true, references inside comments don't count as usage.
func (idx *CodeIndex) UnusedSymbols(ignoreCommentRefs bool) []Symbol {
	if idx == nil {
		return nil
	}
	// Snapshot the symbol slice under the read lock so the iteration is
	// not racing with concurrent BuildIndexIncremental adds (which
	// append-with-may-realloc into idx.Symbols).
	idx.mu.RLock()
	symbols := make([]Symbol, len(idx.Symbols))
	copy(symbols, idx.Symbols)
	idx.mu.RUnlock()

	var unused []Symbol
	for _, sym := range symbols {
		if sym.IsOverride || sym.IsMain || sym.IsTest {
			continue
		}
		if sym.Visibility == "private" {
			continue // handled by single-file rules
		}

		// Check for references outside the declaring file
		hasExternalRef := idx.IsSymbolReferencedOutsideFile(sym, ignoreCommentRefs)

		if !hasExternalRef {
			// Check if referenced within its own file beyond the declaration itself
			localRefs := idx.countRefsInFileForSymbol(sym, sym.File, ignoreCommentRefs)
			// The declaration itself counts as 1 non-comment ref. If there are more, it's used locally.
			if localRefs > 1 {
				continue
			}
			unused = append(unused, sym)
		}
	}
	return unused
}

func symbolReferenceNames(sym Symbol) []string {
	if sym.Name == "" {
		return nil
	}
	if sym.FQN == "" || sym.FQN == sym.Name {
		return []string{sym.Name}
	}
	return []string{sym.Name, sym.FQN}
}

// BloomStats returns the bloom filter memory usage in bytes.
func (idx *CodeIndex) BloomStats() (refBits, crossBits uint) {
	if idx == nil {
		return
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.refBloom != nil {
		refBits = idx.refBloom.Cap()
	}
	return
}
