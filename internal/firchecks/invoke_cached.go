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
	"slices"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// Result carries the findings and per-file gating markers from InvokeCached.
type Result struct {
	Findings []scanner.Finding
	// Crashed maps file path → error message for files that crashed the FIR checker.
	Crashed map[string]string
	// ErrorFiles maps file path → first compiler ERROR (or exclusion reason)
	// for requested files whose checker verdict is not authoritative.
	ErrorFiles map[string]string
	// Rules is the set of requested rules the jar has a checker for: the
	// rules whose verdict FIR owns on authoritative files.
	Rules []string
	// RuleErrors maps rule ID -> file path -> the exception that rule's
	// checker threw on the file (CheckResponse.RuleErrors). Go keeps its own
	// findings for exactly those (file, rule) pairs.
	RuleErrors map[string]map[string]string
}

func newResult() *Result {
	return &Result{Crashed: map[string]string{}, ErrorFiles: map[string]string{}, RuleErrors: map[string]map[string]string{}}
}

// addRuleError records that rule's checker threw on path.
func (r *Result) addRuleError(rule, path, msg string) {
	if msg == "" {
		msg = "checker threw"
	}
	if r.RuleErrors == nil {
		r.RuleErrors = map[string]map[string]string{}
	}
	byPath := r.RuleErrors[rule]
	if byPath == nil {
		byPath = map[string]string{}
		r.RuleErrors[rule] = byPath
	}
	byPath[path] = msg
}

// addResponse folds a daemon/one-shot response into r.
func (r *Result) addResponse(resp *CheckResponse) {
	contentCache := map[string][]byte{}
	for _, f := range resp.Findings {
		r.Findings = append(r.Findings, toScannerFindingWithRange(f, contentCache))
	}
	for path, msg := range resp.Crashed {
		r.Crashed[path] = msg
	}
	for path := range resp.ErrorFiles {
		r.ErrorFiles[path] = errorFileMessage(resp.ErrorFiles, path)
	}
	for rule, byPath := range resp.RuleErrors {
		for path, msg := range byPath {
			r.addRuleError(rule, path, msg)
		}
	}
	r.addRules(resp.Rules)
}

func (r *Result) addRules(rules []string) {
	if len(rules) == 0 {
		return
	}
	r.Rules = append(r.Rules, rules...)
	slices.Sort(r.Rules)
	r.Rules = slices.Compact(r.Rules)
}

// InvokeCached is the cache-aware entry point for running FIR checks.
//
// jarPath is the krit-fir.jar (required when misses need JVM analysis).
// files is the set of .kt file paths to check (pre-filtered by CollectFirCheckFiles).
// sourceDirs / classpath / rules / ruleConfigs / testFiles are forwarded to
// the daemon's check request; rules, ruleConfigs, and testFiles (the subset of
// files krit classifies as test sources) are also part of the cache
// fingerprint.
// repoDir is used to locate the cache; empty disables caching.
// useDaemon controls whether to prefer the persistent daemon (vs one-shot).
// verbose enables progress logging to stderr.
func InvokeCached(
	jarPath string,
	files []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	ruleConfigs RuleConfigs,
	testFiles []string,
	repoDir string,
	useDaemon bool,
	verbose bool,
) (*Result, error) {
	if len(files) == 0 {
		return newResult(), nil
	}

	// If no repo dir, skip cache and go straight to JVM.
	if repoDir == "" {
		return runUncached(jarPath, files, sourceDirs, classpath, rules, ruleConfigs, testFiles, useDaemon, verbose)
	}

	cacheDir, err := CacheDir(repoDir)
	if err != nil {
		if verbose {
			reporter().Verbosef("verbose: fir cache dir init failed (%v), falling back to uncached\n", err)
		}
		return runUncached(jarPath, files, sourceDirs, classpath, rules, ruleConfigs, testFiles, useDaemon, verbose)
	}

	cacheFingerprint := CheckCacheFingerprint(sourceDirs, files, classpath, jarPath, rules, ruleConfigs, testFiles)
	hits, misses := ClassifyFilesForFingerprint(cacheDir, files, cacheFingerprint)
	if verbose {
		reporter().Verbosef("verbose: fir cache: %d hits, %d misses (%d files)\n",
			len(hits), len(misses), len(files))
	}

	// Fast path: all hits.
	if len(misses) == 0 {
		return assembleFromCache(hits), nil
	}

	// Slow path: analyze misses via daemon or one-shot.
	resp, err := runMisses(jarPath, misses, sourceDirs, classpath, rules, ruleConfigs, testFiles, useDaemon, verbose)
	if err != nil {
		return nil, err
	}

	// Write new cache entries.
	written := WriteFreshEntriesForFingerprint(cacheDir, misses, resp, cacheFingerprint)
	if verbose && written > 0 {
		reporter().Verbosef("verbose: fir cache: wrote %d new entries\n", written)
	}

	// Assemble hits + fresh findings.
	result := assembleFromCache(hits)
	result.addResponse(resp)
	return result, nil
}

func runUncached(
	jarPath string,
	files []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	ruleConfigs RuleConfigs,
	testFiles []string,
	useDaemon bool,
	verbose bool,
) (*Result, error) {
	resp, err := runMisses(jarPath, files, sourceDirs, classpath, rules, ruleConfigs, testFiles, useDaemon, verbose)
	if err != nil {
		return nil, err
	}
	result := newResult()
	result.addResponse(resp)
	return result, nil
}

func runMisses(
	jarPath string,
	misses []string,
	sourceDirs []string,
	classpath []string,
	rules []string,
	ruleConfigs RuleConfigs,
	testFiles []string,
	useDaemon bool,
	verbose bool,
) (*CheckResponse, error) {
	testFiles = requestedTestFiles(testFiles, misses)
	// Try persistent daemon.
	if useDaemon && jarPath != "" {
		d, err := connectOrStartFirCheckDaemon(jarPath, sourceDirs, classpath, verbose)
		if err == nil {
			defer func() { _ = d.Release() }()
			refs := buildFileRefs(misses)
			resp, err := d.Check(refs, sourceDirs, classpath, rules, ruleConfigs, testFiles)
			if err == nil {
				return resp, nil
			}
			if verbose {
				reporter().Verbosef("verbose: fir daemon check failed (%v), falling back to one-shot\n", err)
			}
		} else if verbose {
			reporter().Verbosef("verbose: fir daemon unavailable (%v), using one-shot\n", err)
		}
	}

	// One-shot fallback.
	if jarPath == "" {
		return nil, fmt.Errorf("krit-fir.jar not found; build with: cd tools/krit-fir && ./gradlew shadowJar")
	}
	return InvokeOneShot(jarPath, misses, sourceDirs, classpath, rules, ruleConfigs, testFiles, verbose)
}

// requestedTestFiles keeps the test files among requested, so a check request
// only classifies the files it actually asks krit-fir to check.
func requestedTestFiles(testFiles, requested []string) []string {
	if len(testFiles) == 0 {
		return nil
	}
	want := make(map[string]bool, len(requested))
	for _, p := range requested {
		want[p] = true
	}
	var out []string
	for _, p := range testFiles {
		if want[p] {
			out = append(out, p)
		}
	}
	return out
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
	result := newResult()
	contentCache := map[string][]byte{}
	for _, entry := range hits {
		result.addRules(entry.Rules)
		if entry.Crashed {
			result.Crashed[entry.FilePath] = entry.CrashError
			continue
		}
		if entry.ErrorMessage != "" {
			result.ErrorFiles[entry.FilePath] = entry.ErrorMessage
		}
		for rule, msg := range entry.RuleErrors {
			result.addRuleError(rule, entry.FilePath, msg)
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
	content, ok := firLoadContent(path, contents)
	if !ok {
		return 0, 0, false
	}
	lineStart, ok := firFindLineStart(content, line)
	if !ok {
		return 0, 0, false
	}
	start, ok := firAdvanceToCol(content, lineStart, col)
	if !ok {
		return 0, 0, false
	}
	end := firScanIdentifier(content, start)
	return start, end, true
}

func firLoadContent(path string, contents map[string][]byte) ([]byte, bool) {
	if content, ok := contents[path]; ok {
		return content, true
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if contents != nil {
		contents[path] = content
	}
	return content, true
}

func firFindLineStart(content []byte, line int) (int, bool) {
	lineStart := 0
	currentLine := 1
	for lineStart < len(content) && currentLine < line {
		if content[lineStart] == '\n' {
			currentLine++
		}
		lineStart++
	}
	return lineStart, currentLine == line
}

func firAdvanceToCol(content []byte, lineStart, col int) (int, bool) {
	pos := lineStart
	currentCol := 1
	for pos < len(content) && currentCol < col {
		if content[pos] == '\n' || content[pos] == '\r' {
			return 0, false
		}
		_, width := utf8.DecodeRune(content[pos:])
		if width <= 0 {
			width = 1
		}
		pos += width
		currentCol++
	}
	if pos >= len(content) {
		return 0, false
	}
	return pos, true
}

func firScanIdentifier(content []byte, start int) int {
	end := start
	for end < len(content) {
		r, width := utf8.DecodeRune(content[end:])
		if width <= 0 {
			width = 1
		}
		if r != '_' && r != '$' && r != '.' && r != '#' &&
			(r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
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
	return end
}
