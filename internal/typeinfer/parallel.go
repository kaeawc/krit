package typeinfer

import (
	"context"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
	"golang.org/x/sync/errgroup"
)

// FileTypeInfo holds all type information extracted from a single file.
// It is produced independently per file with no shared state,
// enabling embarrassingly parallel indexing.
type FileTypeInfo struct {
	Path        string
	ImportTable *ImportTable
	RootScope   *ScopeTable
	Classes     []*ClassInfo
	ClassScopes []*ClassInfo             // every class declaration, including duplicate FQNs with distinct scopes
	SealedSubs  map[string][]string      // supertype name → subclass names
	EnumEntries map[string][]string      // enum class name → entry names
	TypeAliases map[string]*ResolvedType // alias name or FQN → target type
	Functions   map[string]*ResolvedType // function name → return type
	// FunctionDecls lists every declaration per function key in this file.
	FunctionDecls map[string][]FunctionDecl
	Extensions    []*ExtensionFuncInfo // extension functions
	// SuspendFunctions lists the sorted names of functions declared `suspend` in the file.
	SuspendFunctions []string
	// JavaPackage and JavaWildcardImports are a Java file's package and
	// on-demand imports, which decide what its classes' unqualified
	// supertypes name (resolveJavaSupertypes).
	JavaPackage         string
	JavaWildcardImports []string
}

// IndexFileParallel extracts type info from a single file without
// touching any shared state. Returns a FileTypeInfo that can be
// merged later.
func IndexFileParallel(file *scanner.File) *FileTypeInfo {
	if file != nil && file.Language == scanner.LangJava {
		facts := javafacts.SourceFactsForFile(file)
		classes := make([]*ClassInfo, 0, len(facts.Declared))
		for _, class := range facts.Declared {
			classes = append(classes, javaClassInfo(facts, class, file.Path))
		}
		sortClassInfos(classes)
		var stars []string
		if len(facts.WildcardImports) > 0 {
			stars = append(stars, facts.WildcardImports...)
			sort.Strings(stars)
		}
		return &FileTypeInfo{Path: file.Path, Classes: classes, JavaPackage: facts.Package, JavaWildcardImports: stars}
	}
	if file == nil || file.FlatTree == nil || file.FlatTree.Len() == 0 || file.FlatType(0) != "source_file" {
		return nil
	}

	// Use a temporary resolver for per-file indexing.
	// It writes to its own local maps, not shared state.
	tmp := &defaultResolver{
		imports:        make(map[string]*ImportTable),
		scopes:         make(map[string]*ScopeTable),
		classes:        make(map[string]*ClassInfo),
		classFQN:       make(map[string]*ClassInfo),
		classFQNScopes: make(map[string][]*ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		typeAliases:    make(map[string]*ResolvedType),
		functions:      make(map[string]*ResolvedType),
	}

	headers := scanFileHeadersFlat(file)
	it := headers.it
	rootScope := &ScopeTable{
		Entries:         make(map[string]*ResolvedType),
		EntryStarts:     make(map[string]uint32),
		SmartCasts:      make(map[string]bool),
		SmartCastStarts: make(map[string]uint32),
		SmartCastTypes:  make(map[string]*ResolvedType),
	}
	pkg := headers.pkg

	tmp.indexDeclarationsFlat(0, file, rootScope, it, pkg)
	tmp.buildScopesFlat(0, file, rootScope, it)
	tmp.buildFlatSmartCastScopes(file, rootScope)

	// Collect classes from the temporary resolver. Sort by FQN so the
	// returned slice has a stable, content-defined order — otherwise
	// `for _, ci := range tmp.classes` would expose Go map iteration
	// randomness to the merge step. Two declarations with the same
	// short Name but different FQNs (cross-package collisions) then
	// have a deterministic winner under last-write-wins. See #35.
	classes := make([]*ClassInfo, 0, len(tmp.classes))
	for _, ci := range tmp.classes {
		classes = append(classes, ci)
	}
	sortClassInfos(classes)
	classScopes := make([]*ClassInfo, 0)
	for _, declarations := range tmp.classFQNScopes {
		classScopes = append(classScopes, declarations...)
	}
	sortClassInfos(classScopes)

	return &FileTypeInfo{
		Path:        file.Path,
		ImportTable: it,
		RootScope:   rootScope,
		Classes:     classes,
		ClassScopes: classScopes,
		SealedSubs:  tmp.sealedVariants,
		EnumEntries: tmp.enumEntries,
		TypeAliases: tmp.typeAliases,
		Functions:   tmp.functions,
		Extensions:  tmp.extensions,

		FunctionDecls: fileFunctionDecls(tmp.functionDecls),

		SuspendFunctions: suspendFunctionNamesFlat(file),
	}
}

// fileFunctionDecls strips the file path from a single-file declaration map.
func fileFunctionDecls(refs map[string][]functionDeclRef) map[string][]FunctionDecl {
	if len(refs) == 0 {
		return nil
	}
	out := make(map[string][]FunctionDecl, len(refs))
	for key, list := range refs {
		decls := make([]FunctionDecl, len(list))
		for i, ref := range list {
			decls[i] = ref.FunctionDecl
		}
		out[key] = decls
	}
	return out
}

// suspendFunctionNamesFlat returns the sorted names of functions declared `suspend` in the file.
func suspendFunctionNamesFlat(file *scanner.File) []string {
	seen := make(map[string]bool)
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if !file.FlatHasModifier(fn, "suspend") {
			return
		}
		name := flatMemberName(file, fn)
		if name == "" {
			name = flatFindIdentifier(file, fn)
		}
		if name != "" {
			seen[name] = true
		}
	})
	return sortedKeys(seen)
}

// IndexFilesParallel indexes all files in parallel and merges results
// into the resolver. This is the fast path: O(n/cores) for indexing,
// O(n) for merge.
func (r *defaultResolver) IndexFilesParallel(files []*scanner.File, workers int) {
	r.IndexFilesParallelWithTracker(files, workers, nil)
}

func (r *defaultResolver) IndexFilesParallelWithTracker(files []*scanner.File, workers int, tracker perf.Tracker) {
	results := make([]*FileTypeInfo, len(files))
	r.runTracked(tracker, "perFileExtraction", func() { r.extractFilesParallel(files, workers, results) })
	r.runTracked(tracker, "mergeIndexedData", func() { r.mergeFileResults(results) })
	r.runTracked(tracker, "resolveSupertypes", r.resolveSupertypes)
}

// IndexFilesParallelCachedWithTracker indexes files through the per-file
// on-disk cache. A changed file invalidates only its own cached FileTypeInfo;
// merge and supertype resolution still run over the current file set so the
// resolver remains deterministic.
func (r *defaultResolver) IndexFilesParallelCachedWithTracker(files []*scanner.File, workers int, cacheDir string, tracker perf.Tracker) (hits, misses int) {
	results := make([]*FileTypeInfo, len(files))
	r.runTracked(tracker, "perFileExtraction", func() {
		hits, misses = r.extractFilesParallelCached(files, workers, cacheDir, results)
	})
	r.runTracked(tracker, "mergeIndexedData", func() { r.mergeFileResults(results) })
	r.runTracked(tracker, "resolveSupertypes", r.resolveSupertypes)
	return hits, misses
}

func (r *defaultResolver) runTracked(tracker perf.Tracker, name string, fn func()) {
	if tracker != nil && tracker.IsEnabled() {
		tracker.TrackVoid(name, fn)
	} else {
		fn()
	}
}

func (r *defaultResolver) extractFilesParallel(files []*scanner.File, workers int, results []*FileTypeInfo) {
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	for i, f := range files {
		idx, file := i, f
		g.Go(func() error {
			results[idx] = IndexFileParallel(file)
			return nil
		})
	}
	_ = g.Wait()
}

func (r *defaultResolver) extractFilesParallelCached(files []*scanner.File, workers int, cacheDir string, results []*FileTypeInfo) (int, int) {
	var hitCount, missCount atomic.Int64
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	resident := r.residentCache
	for i, f := range files {
		idx, file := i, f
		g.Go(func() error {
			if file == nil {
				return nil
			}
			// Resident cache (daemon-resident, path-keyed,
			// watcher-invalidated) is hottest path: a map lookup
			// instead of a disk read + zstd-gob decode. Trades
			// content-hash verification for the watcher's
			// best-effort invalidation contract — same trade the
			// resident parsed-trees cache already makes for files.
			if resident != nil {
				if info, ok := resident.LookupFileTypeInfo(file.Path); ok {
					results[idx] = info
					hitCount.Add(1)
					return nil
				}
			}
			if info, ok := loadFileTypeInfoCached(cacheDir, file); ok {
				results[idx] = info
				if resident != nil {
					resident.StoreFileTypeInfo(file.Path, info)
				}
				hitCount.Add(1)
				return nil
			}
			info := IndexFileParallel(file)
			results[idx] = info
			if info != nil {
				_ = saveFileTypeInfoCached(cacheDir, file, info)
				if resident != nil {
					resident.StoreFileTypeInfo(file.Path, info)
				}
			}
			missCount.Add(1)
			return nil
		})
	}
	_ = g.Wait()
	return int(hitCount.Load()), int(missCount.Load())
}

// mergeClasses adds fi's classes to the simple-name and FQN maps.
//
// Classes merge in canonical FQN order. Last-write-wins on short-name
// collisions is preserved, but the "winner" is the (path-largest,
// FQN-largest) class — a deterministic function of the input set rather
// than goroutine scheduling.
func (r *defaultResolver) mergeClasses(fi *FileTypeInfo) {
	java := strings.HasSuffix(fi.Path, ".java")
	if java {
		if r.javaClasses == nil {
			r.javaClasses = make(map[string][]*ClassInfo)
		}
		r.javaClasses[fi.Path] = nil
	}
	for _, ci := range orderedClassInfos(fi.Classes) {
		if java {
			// The resolver serves a copy whose supertypes
			// resolveJavaSupertypes resolves against the merged classes; fi
			// keeps them as written, so a cached FileTypeInfo never carries
			// another run's resolution.
			c := *ci
			c.Supertypes = append([]string(nil), ci.Supertypes...)
			ci = &c
			r.javaClasses[fi.Path] = append(r.javaClasses[fi.Path], ci)
		}
		// A Kotlin class keeps its simple name over a Java one, so indexing
		// Java leaves Kotlin's simple-name lookups as they were.
		if !kotlinKeeps(r.classes[ci.Name], ci) {
			r.classes[ci.Name] = ci
		}
		if ci.FQN != "" && !kotlinKeeps(r.classFQN[ci.FQN], ci) {
			r.classFQN[ci.FQN] = ci
		}
	}
}

func (r *defaultResolver) mergeFileResults(results []*FileTypeInfo) {
	// `results` is slot-indexed by the parallel extraction step, so its
	// ordering matches the caller's `files` argument. To make the merge
	// independent of that contract — and immune to short-name
	// collisions across files where the previous code's "last in
	// `results` wins" was effectively non-deterministic — process
	// results in canonical Path-ascending order with content-sorted
	// inner data. See #35.
	ordered := make([]*FileTypeInfo, 0, len(results))
	for _, fi := range results {
		if fi != nil {
			ordered = append(ordered, fi)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })

	for _, fi := range ordered {
		if r.fileInfos == nil {
			r.fileInfos = make(map[string]*FileTypeInfo, len(ordered))
		}
		r.fileInfos[fi.Path] = fi
		r.imports[fi.Path] = fi.ImportTable
		r.scopes[fi.Path] = fi.RootScope

		r.mergeClasses(fi)
		for _, ci := range orderedClassInfos(fi.ClassScopes) {
			if ci.FQN != "" {
				if r.classFQNScopes == nil {
					r.classFQNScopes = make(map[string][]*ClassInfo)
				}
				r.classFQNScopes[ci.FQN] = append(r.classFQNScopes[ci.FQN], ci)
			}
		}

		for _, typeName := range sortedKeys(fi.SealedSubs) {
			variants := append([]string(nil), fi.SealedSubs[typeName]...)
			sort.Strings(variants)
			r.sealedVariants[typeName] = append(r.sealedVariants[typeName], variants...)
		}

		for _, typeName := range sortedKeys(fi.EnumEntries) {
			entries := append([]string(nil), fi.EnumEntries[typeName]...)
			sort.Strings(entries)
			r.enumEntries[typeName] = entries
		}

		for _, name := range sortedKeysResolved(fi.TypeAliases) {
			r.typeAliases[name] = fi.TypeAliases[name]
		}

		for _, name := range sortedKeysResolved(fi.Functions) {
			r.functions[name] = fi.Functions[name]
		}

		if r.fileFunctionDecls == nil {
			r.fileFunctionDecls = make(map[string]map[string][]FunctionDecl)
		}
		r.fileFunctionDecls[fi.Path] = fi.FunctionDecls

		if r.fileExtensions == nil {
			r.fileExtensions = make(map[string][]*ExtensionFuncInfo)
		}
		r.fileExtensions[fi.Path] = fi.Extensions

		r.recordParamAnnotations(fi)

		for _, name := range fi.SuspendFunctions {
			if r.suspendFuncs == nil {
				r.suspendFuncs = make(map[string]bool)
			}
			r.suspendFuncs[name] = true
		}
	}
	r.summarizeFunctionDecls()
}

// DeclaresSuspendFunction reports whether any indexed source file declares a function with this name as `suspend`.
func (r *defaultResolver) DeclaresSuspendFunction(name string) bool {
	return r.suspendFuncs[name]
}

// sortClassInfos / orderedClassInfos / sortedKeys helpers keep the
// merge step's iteration order a deterministic function of input
// content rather than goroutine scheduling or map randomization.

func sortClassInfos(classes []*ClassInfo) {
	sort.SliceStable(classes, func(i, j int) bool {
		a, b := classes[i], classes[j]
		if a.FQN != b.FQN {
			return a.FQN < b.FQN
		}
		return a.Name < b.Name
	})
}

func orderedClassInfos(in []*ClassInfo) []*ClassInfo {
	if len(in) <= 1 {
		return in
	}
	out := append([]*ClassInfo(nil), in...)
	sortClassInfos(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysResolved(m map[string]*ResolvedType) []string {
	return sortedKeys(m)
}

func (r *defaultResolver) resolveSupertypeNames() {
	for _, ci := range r.classes {
		if strings.HasSuffix(ci.File, ".java") {
			continue
		}
		for i, st := range ci.Supertypes {
			if !strings.Contains(st, ".") {
				if info, ok := r.classes[st]; ok && info.FQN != "" {
					ci.Supertypes[i] = info.FQN
				}
			}
		}
	}
}

// resolveSupertypes qualifies the merged classes' supertypes: Kotlin's
// through the simple-name map, Java's with javac's lookup order.
func (r *defaultResolver) resolveSupertypes() {
	r.resolveSupertypeNames()
	r.resolveJavaSupertypes()
}

// resolveJavaSupertypes resolves each merged Java class's unqualified
// supertypes as javac does: a member type of an enclosing class, then a
// class of the same package (never a parent package), then one an on-demand
// import names. A name none of them declares stays as written, unless a
// source class elsewhere has that simple name: then it is qualified with the
// file's only on-demand import, or else with its package, so a simple-name
// lookup can't take an unrelated class of another package (a Kotlin k.A for
// Java j.B extends A).
func (r *defaultResolver) resolveJavaSupertypes() {
	for path, classes := range r.javaClasses {
		fi := r.fileInfos[path]
		if fi == nil {
			continue
		}
		for _, ci := range classes {
			for i, st := range ci.Supertypes {
				if !strings.Contains(st, ".") {
					ci.Supertypes[i] = r.resolveJavaSupertype(ci, fi, st)
				}
			}
		}
	}
}

func (r *defaultResolver) resolveJavaSupertype(ci *ClassInfo, fi *FileTypeInfo, name string) string {
	// The enclosing classes, then the file's package, where the walk stops.
	for scope := ci.FQN; scope != fi.JavaPackage; {
		dot := strings.LastIndexByte(scope, '.')
		if dot < 0 {
			break
		}
		scope = scope[:dot]
		if r.classFQN[scope+"."+name] != nil {
			return scope + "." + name
		}
	}
	if fi.JavaPackage == "" && r.classFQN[name] != nil {
		return name
	}
	for _, pkg := range fi.JavaWildcardImports {
		if r.classFQN[pkg+"."+name] != nil {
			return pkg + "." + name
		}
	}
	if r.classes[name] == nil {
		return name
	}
	if _, javaLang := KotlinAutoImports[name]; javaLang {
		return name
	}
	switch {
	case len(fi.JavaWildcardImports) == 1:
		return fi.JavaWildcardImports[0] + "." + name
	case fi.JavaPackage != "":
		return fi.JavaPackage + "." + name
	}
	return name
}

// kotlinKeeps reports whether prev, a Kotlin class, keeps its name over ci,
// a Java one.
func kotlinKeeps(prev, ci *ClassInfo) bool {
	return prev != nil && strings.HasSuffix(ci.File, ".java") && !strings.HasSuffix(prev.File, ".java")
}

// javaClassInfo is the resolver's view of a Java source class. Java's
// defaults differ from Kotlin's: a class is open unless declared final, and
// an interface (an annotation type included) is abstract, so a rule that
// reads Kind and the modifier flags with Kotlin defaults in mind does not
// take a Java class for a final one.
func javaClassInfo(facts *javafacts.JavaFileFacts, class javafacts.JavaClassFact, path string) *ClassInfo {
	info := &ClassInfo{
		Name:       class.Name,
		FQN:        class.FQN,
		Kind:       "class",
		Supertypes: javaSupertypeNames(facts, class.Supertypes),
		IsSealed:   class.Sealed,
		IsAbstract: class.Abstract,
		IsNested:   class.Nested,
		IsInner:    class.Inner,
		Outer:      class.Outer,
		File:       path,
	}
	switch class.Kind {
	case "interface", "annotation":
		info.Kind = "interface"
		info.IsAbstract = true
	case "enum":
		info.Kind = "enum"
	case "record":
		// Records are implicitly final.
	default:
		info.IsOpen = !class.Final && !class.Sealed
	}
	if class.Sealed {
		info.Kind = "sealed " + info.Kind
	}
	return info
}

// javaSupertypeNames copies a Java class's written supertypes, qualifying a
// simple name through the file's single-type imports. The copy keeps
// resolveSupertypeNames from rewriting javafacts' cached slice, and the
// qualified name puts the import into the class's cross-file facts, so
// swapping only the import a supertype resolves through changes them.
func javaSupertypeNames(facts *javafacts.JavaFileFacts, written []string) []string {
	if len(written) == 0 {
		return nil
	}
	out := make([]string, len(written))
	for i, st := range written {
		out[i] = st
		if strings.Contains(st, ".") {
			continue
		}
		if fqn := facts.Imports[st]; fqn != "" {
			out[i] = fqn
		}
	}
	return out
}
