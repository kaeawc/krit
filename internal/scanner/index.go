package scanner

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/kaeawc/krit/internal/perf"
)

// classRefPatterns matches class references in XML files.
// Compiled once at package level to avoid recompilation per XML file walk.
var classRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`android:name="([^"]+)"`),
	regexp.MustCompile(`class="([^"]+)"`),
	regexp.MustCompile(`app:argType="([^"]+)"`),
	regexp.MustCompile(`app:destination="@id/([^"]+)"`),
	regexp.MustCompile(`tools:context="([^"]+)"`),
	regexp.MustCompile(`<([a-z][a-zA-Z0-9_.]+\.[A-Z][a-zA-Z0-9]*)`), // FQN as XML tag
}

// Symbol represents a declared symbol in the codebase.
type Symbol struct {
	Name       string
	Kind       string // "function", "class", "property", "object", "interface"
	Visibility string // "public", "private", "internal", "protected"
	File       string
	Line       int
	StartByte  int
	EndByte    int
	IsOverride bool
	IsTest     bool
	IsMain     bool
}

// Reference represents a usage of a name in the codebase.
type Reference struct {
	Name      string
	File      string
	Line      int
	InComment bool // true if this reference is inside a comment node
}

// CodeIndex holds the cross-file symbol table.
type CodeIndex struct {
	Symbols    []Symbol
	References []Reference
	Files      []*File

	// Lookup maps
	symbolsByName                map[string][]Symbol
	refCountByName               map[string]int
	refFilesByName               map[string]map[string]bool // name -> set of files referencing it
	nonCommentRefFilesByName     map[string]map[string]bool // name -> set of files with non-comment references
	nonCommentRefCountByNameFile map[string]map[string]int  // name -> file -> non-comment ref count

	// Bloom filter for fast "is this name referenced?" checks.
	// False positives are OK (we fall back to exact check), false negatives are not.
	refBloom *bloom.BloomFilter
}

// BuildIndex constructs a cross-file index from parsed Kotlin files,
// optionally including Java files for reference-only indexing.
func BuildIndex(files []*File, workers int, javaFiles ...*File) *CodeIndex {
	return BuildIndexWithTracker(files, workers, nil, javaFiles...)
}

// BuildIndexCached behaves like BuildIndexWithTracker but tries the
// on-disk cross-file index cache first. When cacheDir is empty, the
// cache is bypassed entirely and this reduces to BuildIndexWithTracker.
// On a miss (or when persistence fails) the full build path runs and
// the result is written back. Returns the index and a bool reporting
// whether the cache was hit.
func BuildIndexCached(cacheDir string, files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) (*CodeIndex, bool) {
	if cacheDir == "" {
		return BuildIndexWithTracker(files, workers, tracker, javaFiles...), false
	}

	// Pre-load XML files so fingerprint and reference extraction share
	// one disk walk. Also gives the cache a complete file-set snapshot.
	xmlFiles := loadXMLFilesForCache(files)
	fingerprint, _ := computeCrossFileFingerprint(files, javaFiles, xmlFiles)

	if syms, refs, ok := LoadCrossFileCache(cacheDir, fingerprint); ok {
		idx := BuildIndexFromDataWithTracker(syms, refs, tracker)
		idx.Files = append(idx.Files, files...)
		return idx, true
	}

	// Miss → full build. Reuse the pre-loaded XML so we don't re-walk.
	symbols, refs := collectIndexDataInternal(files, workers, tracker, xmlFiles, javaFiles...)
	idx := BuildIndexFromDataWithTracker(symbols, refs, tracker)
	idx.Files = append(idx.Files, files...)

	meta := CrossFileCacheMeta{
		KotlinFiles: len(files),
		JavaFiles:   len(javaFiles),
		XMLFiles:    len(xmlFiles),
	}
	// Best-effort persistence; any error just means the next run rebuilds.
	_ = SaveCrossFileCache(cacheDir, fingerprint, meta, symbols, refs)
	return idx, false
}

// BuildIndexWithTracker constructs a cross-file index and records sub-phase timings when tracker is enabled.
func BuildIndexWithTracker(files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) *CodeIndex {
	symbols, refs := collectIndexDataWithTracker(files, workers, tracker, javaFiles...)
	idx := BuildIndexFromDataWithTracker(symbols, refs, tracker)
	idx.Files = append(idx.Files, files...)
	return idx
}

// BuildIndexFromData constructs a CodeIndex from pre-collected symbols and
// references. This lets callers reuse indexing work instead of rescanning ASTs.
func BuildIndexFromData(symbols []Symbol, refs []Reference) *CodeIndex {
	return BuildIndexFromDataWithTracker(symbols, refs, nil)
}

// BuildIndexFromDataWithTracker constructs a CodeIndex from pre-collected symbols and
// references and records sub-phase timings when tracker is enabled.
func BuildIndexFromDataWithTracker(symbols []Symbol, refs []Reference, tracker perf.Tracker) *CodeIndex {
	if tracker != nil && tracker.IsEnabled() {
		var idx *CodeIndex
		_ = tracker.Track("lookupMapBuild", func() error {
			idx = buildCodeIndex(symbols, refs)
			return nil
		})
		return idx
	}
	return buildCodeIndex(symbols, refs)
}

func collectIndexData(files []*File, workers int, javaFiles ...*File) ([]Symbol, []Reference) {
	return collectIndexDataWithTracker(files, workers, nil, javaFiles...)
}

func collectIndexDataWithTracker(files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) ([]Symbol, []Reference) {
	return collectIndexDataInternal(files, workers, tracker, nil, javaFiles...)
}

// collectIndexDataInternal is the shared body. A non-nil preloadedXML
// skips the per-run XML disk walk and reuses the caller's read bytes;
// nil falls back to a fresh walk.
func collectIndexDataInternal(files []*File, workers int, tracker perf.Tracker, preloadedXML []*xmlCacheFile, javaFiles ...*File) ([]Symbol, []Reference) {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		symbols []Symbol
		refs    []Reference
		sem     = make(chan struct{}, workers)
	)

	runKotlin := func() {
		for _, f := range files {
			wg.Add(1)
			sem <- struct{}{}
			go func(file *File) {
				defer wg.Done()
				defer func() { <-sem }()

				syms, fileRefs := indexFile(file)
				mu.Lock()
				symbols = append(symbols, syms...)
				refs = append(refs, fileRefs...)
				mu.Unlock()
			}(f)
		}
		wg.Wait()
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("kotlinIndexCollection", func() error {
			runKotlin()
			return nil
		})
	} else {
		runKotlin()
	}

	// Index Java files for references only (no symbol declarations)
	runJava := func() {
		for _, jf := range javaFiles {
			wg.Add(1)
			sem <- struct{}{}
			go func(file *File) {
				defer wg.Done()
				defer func() { <-sem }()

				var javaRefs []Reference
				collectJavaReferencesFlat(file, &javaRefs)
				mu.Lock()
				refs = append(refs, javaRefs...)
				mu.Unlock()
			}(jf)
		}
		wg.Wait()
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("javaReferenceCollection", func() error {
			runJava()
			return nil
		})
	} else {
		runJava()
	}

	// Index XML files for class/name references (Android layouts, navigation, manifest).
	runXML := func() {
		if preloadedXML != nil {
			refs = append(refs, collectXmlReferencesFromLoaded(preloadedXML)...)
		} else {
			refs = append(refs, collectXmlReferences(files)...)
		}
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("xmlReferenceCollection", func() error {
			runXML()
			return nil
		})
	} else {
		runXML()
	}
	return symbols, refs
}

func buildCodeIndex(symbols []Symbol, refs []Reference) *CodeIndex {
	idx := &CodeIndex{
		Symbols:                      symbols,
		References:                   refs,
		symbolsByName:                make(map[string][]Symbol),
		refCountByName:               make(map[string]int),
		refFilesByName:               make(map[string]map[string]bool),
		nonCommentRefFilesByName:     make(map[string]map[string]bool),
		nonCommentRefCountByNameFile: make(map[string]map[string]int),
	}

	// Build lookup maps + bloom filters.
	// Estimate bloom filter size: number of unique name+file pairs.
	estimatedRefs := uint(len(idx.References))
	if estimatedRefs < 1000 {
		estimatedRefs = 1000
	}
	idx.refBloom = bloom.NewWithEstimates(estimatedRefs, 0.01) // 1% false positive

	for _, sym := range idx.Symbols {
		idx.symbolsByName[sym.Name] = append(idx.symbolsByName[sym.Name], sym)
	}
	for _, ref := range idx.References {
		idx.refCountByName[ref.Name]++
		if idx.refFilesByName[ref.Name] == nil {
			idx.refFilesByName[ref.Name] = make(map[string]bool)
		}
		idx.refFilesByName[ref.Name][ref.File] = true
		if !ref.InComment {
			if idx.nonCommentRefFilesByName[ref.Name] == nil {
				idx.nonCommentRefFilesByName[ref.Name] = make(map[string]bool)
			}
			idx.nonCommentRefFilesByName[ref.Name][ref.File] = true
			if idx.nonCommentRefCountByNameFile[ref.Name] == nil {
				idx.nonCommentRefCountByNameFile[ref.Name] = make(map[string]int)
			}
			idx.nonCommentRefCountByNameFile[ref.Name][ref.File]++
		}
		idx.refBloom.AddString(ref.Name)
	}

	return idx
}

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) MayHaveReference(name string) bool {
	if idx == nil || idx.refBloom == nil {
		return false
	}
	return idx.refBloom.TestString(name)
}

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) ReferenceCount(name string) int {
	return idx.refCountByName[name]
}

// ReferenceFiles returns the set of files that reference a name.
func (idx *CodeIndex) ReferenceFiles(name string) map[string]bool {
	return idx.refFilesByName[name]
}

// IsReferencedOutsideFile checks if a name is referenced in any file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFile(name, file string) bool {
	// Fast path: bloom filter says name not referenced at all
	if !idx.refBloom.TestString(name) {
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

// IsReferencedOutsideFileExcludingComments checks if a name has any non-comment
// reference in a file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFileExcludingComments(name, file string) bool {
	// Fast path: bloom filter says name not referenced at all
	if !idx.refBloom.TestString(name) {
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
	files := idx.nonCommentRefCountByNameFile[name]
	if files == nil {
		return 0
	}
	return files[file]
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
		hasExternalRef := false
		if ignoreCommentRefs {
			hasExternalRef = idx.IsReferencedOutsideFileExcludingComments(sym.Name, sym.File)
		} else {
			hasExternalRef = idx.IsReferencedOutsideFile(sym.Name, sym.File)
		}

		if !hasExternalRef {
			// Check if referenced within its own file beyond the declaration itself
			localRefs := 0
			if ignoreCommentRefs {
				localRefs = idx.CountNonCommentRefsInFile(sym.Name, sym.File)
			} else {
				for _, ref := range idx.References {
					if ref.Name == sym.Name && ref.File == sym.File {
						localRefs++
					}
				}
			}
			// The declaration itself counts as 1 non-comment ref. If there are more, it's used locally.
			if localRefs > 1 {
				continue
			}
			unused = append(unused, sym)
		}
	}
	return unused
}

// BloomStats returns the bloom filter memory usage in bytes.
func (idx *CodeIndex) BloomStats() (refBits, crossBits uint) {
	if idx.refBloom != nil {
		refBits = idx.refBloom.Cap()
	}
	return
}

func indexFile(file *File) ([]Symbol, []Reference) {
	var symbols []Symbol
	var references []Reference

	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		return symbols, references
	}

	collectDeclarationsFlat(file, &symbols)
	collectReferencesFlat(file, &references)

	return symbols, references
}

func collectDeclarationsFlat(file *File, symbols *[]Symbol) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		switch nodeType {
		case "function_declaration":
			name := file.FlatNodeText(file.FlatFindChild(idx, "simple_identifier"))
			if name == "" {
				return
			}
			sym := Symbol{
				Name:       name,
				Kind:       "function",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				IsOverride: file.FlatHasModifier(idx, "override"),
				IsMain:     name == "main",
			}
			sym.IsTest = strings.Contains(file.FlatNodeText(idx), "@Test")
			*symbols = append(*symbols, sym)
		case "class_declaration":
			name := file.FlatNodeText(file.FlatFindChild(idx, "type_identifier"))
			if name == "" {
				name = file.FlatNodeText(file.FlatFindChild(idx, "simple_identifier"))
			}
			if name == "" {
				return
			}
			kind := "class"
			text := file.FlatNodeText(idx)
			if strings.Contains(text, "interface ") {
				kind = "interface"
			}
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       kind,
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
			})
		case "object_declaration":
			name := file.FlatNodeText(file.FlatFindChild(idx, "type_identifier"))
			if name == "" {
				name = file.FlatNodeText(file.FlatFindChild(idx, "simple_identifier"))
			}
			if name == "" {
				return
			}
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "object",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
			})
		case "property_declaration":
			parent, ok := file.FlatParent(idx)
			if !ok {
				return
			}
			parentType := file.FlatType(parent)
			if parentType != "source_file" && parentType != "class_body" &&
				!(parentType == "statements" && hasFlatAncestorType(file, parent, "class_body")) {
				return
			}
			name := file.FlatNodeText(file.FlatFindChild(idx, "simple_identifier"))
			if name == "" {
				varDecl := file.FlatFindChild(idx, "variable_declaration")
				if varDecl != 0 {
					name = file.FlatNodeText(file.FlatFindChild(varDecl, "simple_identifier"))
				}
			}
			if name == "" || name == "_" {
				return
			}
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "property",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				IsOverride: file.FlatHasModifier(idx, "override"),
			})
		}
	})
}

func collectReferencesFlat(file *File, refs *[]Reference) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		inComment := nodeType == "line_comment" || nodeType == "multiline_comment" || file.FlatHasAncestorOfType(idx, "line_comment") || file.FlatHasAncestorOfType(idx, "multiline_comment")
		if nodeType != "simple_identifier" && nodeType != "type_identifier" {
			return
		}
		name := file.FlatNodeText(idx)
		if name == "" {
			return
		}
		*refs = append(*refs, Reference{
			Name:      name,
			File:      file.Path,
			Line:      file.FlatRow(idx) + 1,
			InComment: inComment,
		})
	})
}

func flatVisibility(file *File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "internal"):
		return "internal"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	default:
		return "public"
	}
}

func hasFlatAncestorType(file *File, idx uint32, want string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == want {
			return true
		}
	}
	return false
}

func collectJavaReferencesFlat(file *File, refs *[]Reference) {
	if file == nil || file.FlatTree == nil {
		return
	}
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "identifier" && nodeType != "type_identifier" {
			return
		}
		name := file.FlatNodeText(idx)
		if name == "" {
			return
		}
		*refs = append(*refs, Reference{
			Name: name,
			File: file.Path,
			Line: file.FlatRow(idx) + 1,
		})
	})
}

// xmlCacheFile is a pre-loaded XML source whose content and hash are
// consumed by both the cross-file cache fingerprint and the reference
// walk, so each file is read from disk once.
type xmlCacheFile struct {
	Path    string
	Content []byte
	Hash    string
}

// collectXmlReferences scans for XML files in the project and extracts class name references.
// Android references Kotlin/Java classes from XML in: layouts, navigation graphs, manifest, etc.
func collectXmlReferences(ktFiles []*File) []Reference {
	return collectXmlReferencesFromLoaded(loadXMLFilesForCache(ktFiles))
}

// loadXMLFilesForCache walks the project for XML reference-candidate
// files, reads them, and hashes each. The result feeds both the cache
// fingerprint and the reference extraction in a single I/O pass.
func loadXMLFilesForCache(ktFiles []*File) []*xmlCacheFile {
	if len(ktFiles) == 0 {
		return nil
	}

	// Find project roots from kotlin file paths
	roots := make(map[string]bool)
	for _, f := range ktFiles {
		// Walk up to find src/ parent
		dir := filepath.Dir(f.Path)
		for dir != "/" && dir != "." {
			if filepath.Base(dir) == "src" {
				roots[filepath.Dir(dir)] = true
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	var out []*xmlCacheFile
	for root := range roots {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if info != nil && info.IsDir() {
					base := info.Name()
					if base == ".git" || base == "build" || base == "node_modules" ||
						base == ".idea" || base == ".gradle" || base == "out" ||
						base == ".kotlin" || base == "target" ||
						base == "third-party" || base == "third_party" ||
						base == "vendor" || base == "external" ||
						strings.HasPrefix(base, "values") {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !isXMLReferenceCandidate(path) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			out = append(out, &xmlCacheFile{
				Path:    path,
				Content: content,
				Hash:    contentHashBytes(content),
			})
			return nil
		})
	}
	return out
}

func collectXmlReferencesFromLoaded(files []*xmlCacheFile) []Reference {
	if len(files) == 0 {
		return nil
	}
	var refs []Reference
	for _, f := range files {
		appendXMLReferences(&refs, f.Path, f.Content)
	}
	return refs
}

func isXMLReferenceCandidate(path string) bool {
	if !strings.HasSuffix(path, ".xml") {
		return false
	}
	base := filepath.Base(path)
	if base == "AndroidManifest.xml" {
		return true
	}
	dir := filepath.Base(filepath.Dir(path))
	switch {
	case strings.HasPrefix(dir, "layout"):
		return true
	case strings.HasPrefix(dir, "menu"):
		return true
	case strings.HasPrefix(dir, "navigation"):
		return true
	case dir == "xml":
		return true
	case strings.HasPrefix(dir, "values"):
		return false
	default:
		return false
	}
}

func appendXMLReferences(refs *[]Reference, path string, content []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for _, re := range classRefPatterns {
			matches := re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				className := m[1]
				if idx := strings.LastIndex(className, "."); idx >= 0 {
					className = className[idx+1:]
				}
				className = strings.TrimPrefix(className, ".")
				if className != "" {
					*refs = append(*refs, Reference{
						Name: className,
						File: path,
						Line: lineNo,
					})
				}
			}
		}
	}
}
