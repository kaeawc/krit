package scanner

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// kotlinParserPool pools tree-sitter Kotlin parsers to avoid repeated allocation.
var kotlinParserPool = sync.Pool{
	New: func() interface{} {
		p := sitter.NewParser()
		p.SetLanguage(kotlin.GetLanguage())
		return p
	},
}

// GetKotlinParser returns a pooled Kotlin parser. Callers must call PutKotlinParser when done.
func GetKotlinParser() *sitter.Parser {
	return kotlinParserPool.Get().(*sitter.Parser)
}

// PutKotlinParser returns a Kotlin parser to the pool for reuse.
func PutKotlinParser(p *sitter.Parser) {
	kotlinParserPool.Put(p)
}

// javaParserPool pools tree-sitter Java parsers to avoid repeated allocation.
var javaParserPool = sync.Pool{
	New: func() interface{} {
		p := sitter.NewParser()
		p.SetLanguage(java.GetLanguage())
		return p
	},
}

// Finding is the serialization-boundary representation of a single lint
// finding. Internally krit stores findings in columnar form via
// FindingColumns (see findings.go) — Finding is the per-row struct used
// at boundaries: output formatters (JSON/SARIF/plain/checkstyle) marshal
// from it, rule bodies produce it for Context.Emit (which immediately
// writes into a FindingCollector), and tests construct it to seed
// columns via CollectFindings. New internal code should prefer the
// columnar accessors; construct Finding only at serialization or emit
// boundaries.
type Finding struct {
	File       string
	Line       int
	Col        int
	RuleSet    string
	Rule       string
	Severity   string
	Message    string
	Fix        *Fix       // nil if no auto-fix available
	BinaryFix  *BinaryFix // nil if no binary fix available
	Confidence float64    // 0.0-1.0, 0 means not set
}

// Fix describes an auto-fix for a finding.
type Fix struct {
	// Line-based replacement: replace lines[StartLine-1:EndLine] with Replacement
	StartLine   int
	EndLine     int
	Replacement string
	// Byte-based replacement (more precise): replace content[StartByte:EndByte]
	StartByte int
	EndByte   int
	ByteMode  bool // if true, use byte offsets instead of line offsets
}

// Language identifies which source language a File holds. Used by the
// dispatcher to skip rules whose declared Languages list excludes this
// file. See issue #19 (UnifiedFileModel).
type Language uint8

const (
	// LangKotlin is the default for files parsed by ParseFile. Rules with
	// no declared Languages list default to targeting Kotlin only.
	LangKotlin Language = iota
	LangJava
	// LangXML covers both AndroidManifest.xml and res/ XML files. The
	// specific kind (manifest vs resource) lives in File.Metadata.
	LangXML
	LangGradle
	LangVersionCatalog
)

// String returns a short human-readable name for the language.
func (l Language) String() string {
	switch l {
	case LangKotlin:
		return "kotlin"
	case LangJava:
		return "java"
	case LangXML:
		return "xml"
	case LangGradle:
		return "gradle"
	case LangVersionCatalog:
		return "version-catalog"
	default:
		return "unknown"
	}
}

// File holds parsed source in flat form. The cgo parse tree is used
// only during flattening and is not retained on the File.
type File struct {
	Path        string
	Language    Language
	Content     []byte
	Lines       []string
	FlatTree    *FlatTree
	lineOffsets []int // cached byte offset of each line start

	// Metadata carries language-specific parsed structures (e.g.
	// *android.ManifestMeta, *android.ResourceMeta, *android.BuildConfig)
	// for non-source-language files. Nil for Kotlin/Java.
	Metadata any

	// SuppressionIdx is populated by the pipeline.Parse phase with a
	// per-file SuppressionIndex so cross-file and module-aware rules
	// can consult the same @Suppress map as per-file dispatch. Nil when
	// the caller (LSP/MCP) builds files without running Parse.
	SuppressionIdx *SuppressionIndex
}

// LineOffsets returns the byte offset of each line start, computed lazily and cached.
func (f *File) LineOffsets() []int {
	if f.lineOffsets != nil {
		return f.lineOffsets
	}
	offsets := []int{0}
	for i, b := range f.Content {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	f.lineOffsets = offsets
	return f.lineOffsets
}

// LineOffset returns the byte offset for the start of the given line index (0-based).
// If lineIdx is out of range, returns len(Content).
func (f *File) LineOffset(lineIdx int) int {
	offsets := f.LineOffsets()
	if lineIdx < len(offsets) {
		return offsets[lineIdx]
	}
	return len(f.Content)
}

// ParseFile parses a Kotlin file and returns the AST.
func ParseFile(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parser := GetKotlinParser()
	defer PutKotlinParser(parser)
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}

	return NewParsedFile(path, content, tree), nil
}

// NewParsedFile builds a scanner.File from already-parsed Kotlin source.
// The incoming tree is flattened immediately and not retained.
func NewParsedFile(path string, content []byte, tree *sitter.Tree) *File {
	lines := strings.Split(string(content), "\n")

	var flatTree *FlatTree
	if tree != nil {
		flatTree = flattenTree(tree.RootNode())
	}

	return &File{
		Path:     internString(path),
		Language: LangKotlin,
		Content:  content,
		Lines:    lines,
		FlatTree: flatTree,
	}
}

// CollectKotlinFiles finds all .kt and .kts files under the given paths.
func CollectKotlinFiles(paths []string, excludes []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isKotlinFile(p) {
				files = append(files, p)
			}
			continue
		}
		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := info.Name()
				if base == ".git" || base == "build" || base == "node_modules" ||
					base == ".idea" || base == ".gradle" || base == "out" ||
					base == ".kotlin" || base == "target" ||
					// Vendored / third-party / generated sources: not the
					// user's code, shouldn't be subject to style rules.
					base == "third-party" || base == "third_party" ||
					base == "vendor" || base == "external" {
					return filepath.SkipDir
				}
				return nil
			}
			if !isKotlinFile(path) {
				return nil
			}
			if isExcluded(path, excludes) {
				return nil
			}
			abs, _ := filepath.Abs(path)
			if !seen[abs] {
				seen[abs] = true
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// ScanFiles parses all files in parallel and returns parsed File objects.
func ScanFiles(paths []string, workers int) ([]*File, []error) {
	return scanFilesParallel(paths, workers, ParseFile)
}

func isKotlinFile(path string) bool {
	return strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts")
}

func isJavaFile(path string) bool {
	return strings.HasSuffix(path, ".java")
}

// CollectJavaFiles finds all .java files under the given paths.
func CollectJavaFiles(paths []string, excludes []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isJavaFile(p) {
				files = append(files, p)
			}
			continue
		}
		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := info.Name()
				if base == ".git" || base == "build" || base == "node_modules" ||
					base == ".idea" || base == ".gradle" || base == "out" ||
					base == ".kotlin" || base == "target" ||
					// Vendored / third-party / generated sources: not the
					// user's code, shouldn't be subject to style rules.
					base == "third-party" || base == "third_party" ||
					base == "vendor" || base == "external" {
					return filepath.SkipDir
				}
				return nil
			}
			if !isJavaFile(path) {
				return nil
			}
			if isExcluded(path, excludes) {
				return nil
			}
			abs, _ := filepath.Abs(path)
			if !seen[abs] {
				seen[abs] = true
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// ParseJavaFile parses a Java file and returns a File with its AST.
func ParseJavaFile(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parser := javaParserPool.Get().(*sitter.Parser)
	defer javaParserPool.Put(parser)
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var flatTree *FlatTree
	if tree != nil {
		flatTree = flattenTree(tree.RootNode())
	}

	return &File{
		Path:     internString(path),
		Language: LangJava,
		Content:  content,
		Lines:    lines,
		FlatTree: flatTree,
	}, nil
}

// ScanJavaFiles parses all Java files in parallel (for reference indexing only).
func ScanJavaFiles(paths []string, workers int) ([]*File, []error) {
	return scanFilesParallel(paths, workers, ParseJavaFile)
}

type indexedPath struct {
	index int
	path  string
}

type indexedFile struct {
	index int
	file  *File
}

type indexedError struct {
	index int
	err   error
}

type scanBatchResult struct {
	files []indexedFile
	errs  []indexedError
}

func scanFilesParallel(paths []string, workers int, parse func(string) (*File, error)) ([]*File, []error) {
	if len(paths) == 0 {
		return nil, nil
	}

	batches := partitionIndexedPaths(paths, workers)
	results := make([]scanBatchResult, len(batches))

	var (
		wg sync.WaitGroup
	)

	for batchIdx, batch := range batches {
		wg.Add(1)
		go func(resultIdx int, inputs []indexedPath) {
			defer wg.Done()
			// Keep worker-local state hot on one OS thread for the duration of the batch.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			local := scanBatchResult{
				files: make([]indexedFile, 0, len(inputs)),
				errs:  make([]indexedError, 0),
			}

			for _, input := range inputs {
				f, err := parse(input.path)
				if err != nil {
					local.errs = append(local.errs, indexedError{index: input.index, err: err})
					continue
				}
				local.files = append(local.files, indexedFile{index: input.index, file: f})
			}

			results[resultIdx] = local
		}(batchIdx, batch)
	}
	wg.Wait()

	fileSlots := make([]*File, len(paths))
	errSlots := make([]error, len(paths))
	for _, result := range results {
		for _, item := range result.files {
			fileSlots[item.index] = item.file
		}
		for _, item := range result.errs {
			errSlots[item.index] = item.err
		}
	}

	files := make([]*File, 0, len(paths))
	errs := make([]error, 0)
	for i := range paths {
		if fileSlots[i] != nil {
			files = append(files, fileSlots[i])
		}
		if errSlots[i] != nil {
			errs = append(errs, errSlots[i])
		}
	}
	return files, errs
}

func partitionIndexedPaths(paths []string, workers int) [][]indexedPath {
	if len(paths) == 0 {
		return nil
	}

	if workers < 1 {
		workers = 1
	}
	if workers > len(paths) {
		workers = len(paths)
	}

	batches := make([][]indexedPath, 0, workers)
	for worker := 0; worker < workers; worker++ {
		start := worker * len(paths) / workers
		end := (worker + 1) * len(paths) / workers
		batch := make([]indexedPath, 0, end-start)
		for idx := start; idx < end; idx++ {
			batch = append(batch, indexedPath{index: idx, path: paths[idx]})
		}
		batches = append(batches, batch)
	}
	return batches
}

func isExcluded(path string, excludes []string) bool {
	// Test-data directories contain deliberately malformed Kotlin used to
	// exercise compiler/IDE behavior — not user code and not subject to
	// style rules. Skip common paths.
	if strings.Contains(path, "/test/data/") ||
		strings.Contains(path, "/testData/") ||
		strings.Contains(path, "/testdata/") ||
		strings.Contains(path, "/test-data/") ||
		strings.Contains(path, "/compiler-tests/") ||
		strings.Contains(path, "/compilerTests/") {
		return true
	}
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, strings.Trim(pattern, "*")) {
			return true
		}
	}
	return false
}

func bytesEqualString(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}

// IsCommentLine returns true if the trimmed line is a comment (// or * prefix).
func IsCommentLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*")
}

// ReadLines reads a file and returns lines (for gitignore, etc.)
func ReadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}
