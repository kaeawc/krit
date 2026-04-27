package firchecks

// invoke_cached.go — warm path through the FIR finding cache and daemon.
//
// InvokeCached:
//   1. Classifies files into cache hits and misses.
//   2. On all-hit: returns assembled findings from cache; no JVM launched.
//   3. On misses: sends them to the persistent daemon (falling back to
//      one-shot if the daemon is unavailable).
//   4. Writes new cache entries for miss results.
//   5. Assembles and returns all findings as []scanner.Finding.

import (
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// Result carries the findings and per-file crash markers from InvokeCached.
type Result struct {
	Findings []scanner.Finding
	// Crashed maps file path → error message for files that crashed the FIR checker.
	Crashed map[string]string
}

// InvokeCached is the cache-aware entry point for running FIR checks.
//
// jarPath is the krit-fir.jar (required when misses need JVM analysis).
// files is the set of .kt file paths to check (pre-filtered by CollectFirCheckFiles).
// sourceDirs / classpath / rules are forwarded to the daemon's check request.
// repoDir is used to locate the cache; empty disables caching.
// useDaemon controls whether to prefer the persistent daemon (vs one-shot).
// verbose enables progress logging to stderr.
func InvokeCached(
	jarPath string,
	files []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	repoDir string,
	useDaemon bool,
	verbose bool,
) (*Result, error) {
	if len(files) == 0 {
		return &Result{Crashed: map[string]string{}}, nil
	}

	// If no repo dir, skip cache and go straight to JVM.
	if repoDir == "" {
		return runUncached(jarPath, files, sourceDirs, classpath, rules, useDaemon, verbose)
	}

	cacheDir, err := CacheDir(repoDir)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: fir cache dir init failed (%v), falling back to uncached\n", err)
		}
		return runUncached(jarPath, files, sourceDirs, classpath, rules, useDaemon, verbose)
	}

	hits, misses := ClassifyFiles(cacheDir, files)
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: fir cache: %d hits, %d misses (%d files)\n",
			len(hits), len(misses), len(files))
	}

	// Fast path: all hits.
	if len(misses) == 0 {
		return assembleFromCache(hits), nil
	}

	// Slow path: analyze misses via daemon or one-shot.
	resp, err := runMisses(jarPath, misses, sourceDirs, classpath, rules, useDaemon, verbose)
	if err != nil {
		return nil, err
	}

	// Write new cache entries.
	written := WriteFreshEntries(cacheDir, misses, resp)
	if verbose && written > 0 {
		fmt.Fprintf(os.Stderr, "verbose: fir cache: wrote %d new entries\n", written)
	}

	// Assemble hits + fresh findings.
	result := assembleFromCache(hits)
	contentCache := map[string][]byte{}
	for _, f := range resp.Findings {
		result.Findings = append(result.Findings, toScannerFindingWithRange(f, contentCache))
	}
	for path, msg := range resp.Crashed {
		result.Crashed[path] = msg
	}
	return result, nil
}

func runUncached(
	jarPath string,
	files []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	useDaemon bool,
	verbose bool,
) (*Result, error) {
	resp, err := runMisses(jarPath, files, sourceDirs, classpath, rules, useDaemon, verbose)
	if err != nil {
		return nil, err
	}
	result := &Result{Crashed: map[string]string{}}
	contentCache := map[string][]byte{}
	for _, f := range resp.Findings {
		result.Findings = append(result.Findings, toScannerFindingWithRange(f, contentCache))
	}
	for path, msg := range resp.Crashed {
		result.Crashed[path] = msg
	}
	return result, nil
}

func runMisses(
	jarPath string,
	misses []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	useDaemon bool,
	verbose bool,
) (*CheckResponse, error) {
	// Try persistent daemon.
	if useDaemon && jarPath != "" {
		d, err := ConnectOrStartFirDaemon(jarPath, sourceDirs, verbose)
		if err == nil {
			defer d.Release()
			refs := buildFileRefs(misses)
			resp, err := d.Check(refs, sourceDirs, classpath, rules)
			if err == nil {
				return resp, nil
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: fir daemon check failed (%v), falling back to one-shot\n", err)
			}
		} else if verbose {
			fmt.Fprintf(os.Stderr, "verbose: fir daemon unavailable (%v), using one-shot\n", err)
		}
	}

	// One-shot fallback.
	if jarPath == "" {
		return nil, fmt.Errorf("krit-fir.jar not found; build with: cd tools/krit-fir && ./gradlew shadowJar")
	}
	return InvokeOneShot(jarPath, misses, sourceDirs, classpath, rules, verbose)
}

func buildFileRefs(files []string) []fileRef {
	refs := make([]fileRef, 0, len(files))
	for _, p := range files {
		hash, _ := ContentHash(p)
		refs = append(refs, fileRef{Path: p, ContentHash: hash})
	}
	return refs
}

func assembleFromCache(hits []*FirCacheEntry) *Result {
	result := &Result{Crashed: map[string]string{}}
	contentCache := map[string][]byte{}
	for _, entry := range hits {
		if entry.Crashed {
			result.Crashed[entry.FilePath] = entry.CrashError
			continue
		}
		for _, f := range entry.Findings {
			result.Findings = append(result.Findings, toScannerFindingWithRange(f, contentCache))
		}
	}
	return result
}

func toScannerFindingWithRange(f FirFinding, contents map[string][]byte) scanner.Finding {
	finding := ToScannerFinding(f)
	if finding.EndByte > finding.StartByte {
		return finding
	}
	start, end, ok := firPointRange(f.Path, f.Line, f.Col, contents)
	if !ok {
		return finding
	}
	finding.StartByte = start
	finding.EndByte = end
	return finding
}

func firPointRange(path string, line, col int, contents map[string][]byte) (int, int, bool) {
	if line <= 0 || col <= 0 || path == "" {
		return 0, 0, false
	}
	content, ok := contents[path]
	if !ok {
		var err error
		content, err = os.ReadFile(path)
		if err != nil {
			return 0, 0, false
		}
		if contents != nil {
			contents[path] = content
		}
	}
	lineStart := 0
	currentLine := 1
	for lineStart < len(content) && currentLine < line {
		if content[lineStart] == '\n' {
			currentLine++
		}
		lineStart++
	}
	if currentLine != line {
		return 0, 0, false
	}
	start := lineStart
	currentCol := 1
	for start < len(content) && currentCol < col {
		if content[start] == '\n' || content[start] == '\r' {
			return 0, 0, false
		}
		_, width := utf8.DecodeRune(content[start:])
		if width <= 0 {
			width = 1
		}
		start += width
		currentCol++
	}
	if start >= len(content) {
		return 0, 0, false
	}
	end := start
	for end < len(content) {
		r, width := utf8.DecodeRune(content[end:])
		if width <= 0 {
			width = 1
		}
		if !(r == '_' || r == '$' || r == '.' || r == '#' ||
			r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			break
		}
		end += width
	}
	if end <= start {
		_, width := utf8.DecodeRune(content[start:])
		if width <= 0 {
			width = 1
		}
		end = start + width
	}
	return start, end, true
}

var firLineDedupeRules = map[string]struct{}{
	"CollectInOnCreateWithoutLifecycle": {},
	"ComposeRememberWithoutKey":         {},
	"InjectDispatcher":                  {},
}

// MergeFindings merges FIR findings into the existing allFindings slice,
// deduplicating on (file, line, col, rule). Pilot FIR rules also dedupe on
// (file, line, rule) because compiler source ranges can point at a callee
// token while the tree-sitter rule points at the containing call expression.
// Go tree-sitter findings win on collision (they're already in allFindings).
func MergeFindings(allFindings []scanner.Finding, firFindings []scanner.Finding) []scanner.Finding {
	type key struct {
		file, rule string
		line, col  int
	}
	type lineKey struct {
		file, rule string
		line       int
	}
	type byteKey struct {
		file, rule string
		start, end int
	}
	existing := make(map[key]struct{}, len(allFindings))
	existingLines := make(map[lineKey]struct{}, len(allFindings))
	existingLineWithoutBytes := make(map[lineKey]struct{}, len(allFindings))
	existingBytes := make(map[byteKey]struct{}, len(allFindings))
	for _, f := range allFindings {
		existing[key{f.File, f.Rule, f.Line, f.Col}] = struct{}{}
		if f.EndByte > f.StartByte {
			existingBytes[byteKey{f.File, f.Rule, f.StartByte, f.EndByte}] = struct{}{}
		}
		if _, ok := firLineDedupeRules[f.Rule]; ok {
			lk := lineKey{f.File, f.Rule, f.Line}
			existingLines[lk] = struct{}{}
			if f.EndByte <= f.StartByte {
				existingLineWithoutBytes[lk] = struct{}{}
			}
		}
	}
	for _, f := range firFindings {
		k := key{f.File, f.Rule, f.Line, f.Col}
		bk := byteKey{f.File, f.Rule, f.StartByte, f.EndByte}
		if f.EndByte > f.StartByte {
			if _, ok := existingBytes[bk]; ok {
				continue
			}
		}
		if _, ok := existing[k]; !ok {
			lk := lineKey{f.File, f.Rule, f.Line}
			if _, lineDedupe := firLineDedupeRules[f.Rule]; lineDedupe {
				if f.EndByte > f.StartByte {
					if _, ok := existingLineWithoutBytes[lk]; ok {
						continue
					}
				}
				if _, ok := existingLines[lk]; ok {
					if f.EndByte <= f.StartByte {
						continue
					}
				}
			}
			allFindings = append(allFindings, f)
			existing[k] = struct{}{}
			if f.EndByte > f.StartByte {
				existingBytes[bk] = struct{}{}
			}
			if _, lineDedupe := firLineDedupeRules[f.Rule]; lineDedupe {
				existingLines[lk] = struct{}{}
				if f.EndByte <= f.StartByte {
					existingLineWithoutBytes[lk] = struct{}{}
				}
			}
		}
	}
	return allFindings
}
