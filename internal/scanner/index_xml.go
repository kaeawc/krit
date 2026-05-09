package scanner

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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

// xmlCacheFile is a pre-loaded XML source whose content and hash are
// consumed by both the cross-file cache fingerprint and the reference
// walk, so each file is read from disk once.
type xmlCacheFile struct {
	Path    string
	Content []byte
	Hash    string
}

// collectXMLReferences scans for XML files in the project and extracts class name references.
// Android references Kotlin/Java classes from XML in: layouts, navigation graphs, manifest, etc.
func collectXMLReferences(ktFiles []*File) []Reference {
	return collectXMLReferencesFromLoaded(loadXMLFilesForCache(ktFiles))
}

func xmlRootsFromKotlinFiles(ktFiles []*File) map[string]bool {
	roots := make(map[string]bool)
	for _, f := range ktFiles {
		dir := filepath.Dir(f.Path)
		for dir != "/" && dir != "." {
			if filepath.Base(dir) == "src" {
				roots[filepath.Dir(dir)] = true
				break
			}
			dir = filepath.Dir(dir)
		}
	}
	return roots
}

func isXMLPrunedDir(base string) bool {
	switch base {
	case ".git", "build", "node_modules", ".idea", ".gradle", "out",
		".kotlin", "target", "third-party", "third_party", "vendor", "external":
		return true
	}
	return strings.HasPrefix(base, "values")
}

func walkXMLFilesInRoot(root string) []*xmlCacheFile {
	var local []*xmlCacheFile
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && isXMLPrunedDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isXMLReferenceCandidate(path) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // skip-and-continue: per-file read/parse error inside Walk callback
		}
		local = append(local, &xmlCacheFile{
			Path:    path,
			Content: content,
			Hash:    contentHashForFile(path, content),
		})
		return nil
	})
	return local
}

// loadXMLFilesForCache walks the project for XML reference-candidate
// files, reads them, and hashes each. The result feeds both the cache
// fingerprint and the reference extraction in a single I/O pass.
func loadXMLFilesForCache(ktFiles []*File) []*xmlCacheFile {
	if len(ktFiles) == 0 {
		return nil
	}

	roots := xmlRootsFromKotlinFiles(ktFiles)

	// Walk each project root in its own goroutine. Roots are
	// independent subtrees (one per Gradle module, typically) so the
	// walks do not contend. Per-root results are appended under a
	// single mutex.
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []*xmlCacheFile
	)
	for r := range roots {
		wg.Add(1)
		go func(root string) {
			defer wg.Done()
			local := walkXMLFilesInRoot(root)
			if len(local) > 0 {
				mu.Lock()
				out = append(out, local...)
				mu.Unlock()
			}
		}(r)
	}
	wg.Wait()

	// Goroutine completion order is non-deterministic, so `out`
	// accumulates in whatever order the per-root walks finish. Sort by
	// path so downstream consumers (cache fingerprints, reference
	// extraction, rules that pick first-match) see a stable XML
	// reference sequence across runs. See #31.
	sortXMLCacheFiles(out)
	return out
}

// sortXMLCacheFiles orders xmlCacheFile entries by path. It is the
// canonical ordering for any aggregated XML file slice in the
// scanner — exposed as a named helper so the determinism property is
// documented and reusable for tests.
func sortXMLCacheFiles(files []*xmlCacheFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

func collectXMLReferencesFromLoaded(files []*xmlCacheFile) []Reference {
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
				fullName := strings.TrimPrefix(className, ".")
				if fullName != "" && strings.Contains(fullName, ".") {
					*refs = append(*refs, Reference{
						Name: fullName,
						File: path,
						Line: lineNo,
					})
				}
				if idx := strings.LastIndex(className, "."); idx >= 0 {
					className = className[idx+1:]
				}
				className = strings.TrimPrefix(className, ".")
				if className == "" {
					continue
				}
				*refs = append(*refs, Reference{
					Name: className,
					File: path,
					Line: lineNo,
				})
			}
		}
	}
}
