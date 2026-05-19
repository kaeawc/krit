package scanner

import (
	"regexp"
	"strings"

	"github.com/bits-and-blooms/bloom/v3"
)

var sourceImportRe = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_$][A-Za-z0-9_$.*]*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?\s*;?`)

// SymbolsNamed returns declarations indexed under a simple or fully-qualified
// name. A nil result means no matching declaration is known.
func (idx *CodeIndex) SymbolsNamed(name string) []Symbol {
	if idx == nil || name == "" {
		return nil
	}
	return idx.symbolsByName[name]
}

// SymbolByFQN returns the declaration with the exact fully-qualified name.
func (idx *CodeIndex) SymbolByFQN(fqn string) (Symbol, bool) {
	if idx == nil || fqn == "" {
		return Symbol{}, false
	}
	sym, ok := idx.symbolsByFQN[fqn]
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

func (idx *CodeIndex) buildReferenceLookups(addToBloom bool) {
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

func (idx *CodeIndex) addReferenceLookup(ref Reference) {
	idx.refMu.Lock()
	defer idx.refMu.Unlock()

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

func (idx *CodeIndex) removeReferenceLookup(ref Reference) {
	idx.refMu.Lock()
	defer idx.refMu.Unlock()

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

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) MayHaveReference(name string) bool {
	if idx == nil {
		return false
	}
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
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
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
	return idx.refCountByName[name]
}

// ReferenceFiles returns the set of files that reference a name. The returned
// map is a defensive copy: callers may iterate it without holding the
// index lock and concurrent writers will not corrupt their snapshot.
func (idx *CodeIndex) ReferenceFiles(name string) map[string]bool {
	if idx == nil {
		return nil
	}
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
	src := idx.refFilesByName[name]
	if src == nil {
		return nil
	}
	out := make(map[string]bool, len(src))
	for f, v := range src {
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
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
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
	return idx.anyReferenceOutsideFile(symbolReferenceNames(sym), sym.File, ignoreCommentRefs)
}

// anyReferenceOutsideFile is shared between the loose and strict
// IsSymbolReferenced* variants: returns true on the first name that has
// an outside-file reference (honouring the comment-exclusion flag).
func (idx *CodeIndex) anyReferenceOutsideFile(names []string, file string, ignoreCommentRefs bool) bool {
	if idx == nil {
		return false
	}
	for _, name := range names {
		if ignoreCommentRefs {
			if idx.IsReferencedOutsideFileExcludingComments(name, file) {
				return true
			}
		} else if idx.IsReferencedOutsideFile(name, file) {
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
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
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
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
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
		for _, ref := range idx.References {
			if ref.Name == name && ref.File == file {
				count++
			}
		}
	}
	return count
}

// UnusedSymbols returns symbols that are never referenced from any other file.
// If ignoreCommentRefs is true, references inside comments don't count as usage.
func (idx *CodeIndex) UnusedSymbols(ignoreCommentRefs bool) []Symbol {
	var unused []Symbol
	for _, sym := range idx.Symbols {
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

// symbolReferenceNamesStrict drops the simple-name path when an FQN is
// available, so references that only matched by collision with a
// like-named symbol in another package are not counted. When the
// symbol has no FQN the strict view falls back to the simple name —
// the rule has no other handle to disambiguate.
func symbolReferenceNamesStrict(sym Symbol) []string {
	if sym.Name == "" {
		return nil
	}
	if sym.FQN != "" && sym.FQN != sym.Name {
		return []string{sym.FQN}
	}
	return []string{sym.Name}
}

// IsSymbolReferencedOutsideFileFQN is the strict variant of
// IsSymbolReferencedOutsideFile: it requires an FQN match (when the
// symbol has one), so a reference to a like-named declaration in a
// different package does not count as usage. Use at --depth=thorough
// when reducing false-negative dead-code is worth the loss of the
// simple-name fallback for symbols without an FQN.
func (idx *CodeIndex) IsSymbolReferencedOutsideFileFQN(sym Symbol, ignoreCommentRefs bool) bool {
	return idx.anyReferenceOutsideFile(symbolReferenceNamesStrict(sym), sym.File, ignoreCommentRefs)
}

// BloomStats returns the bloom filter memory usage in bytes.
func (idx *CodeIndex) BloomStats() (refBits, crossBits uint) {
	if idx == nil {
		return
	}
	idx.refMu.RLock()
	defer idx.refMu.RUnlock()
	if idx.refBloom != nil {
		refBits = idx.refBloom.Cap()
	}
	return
}
