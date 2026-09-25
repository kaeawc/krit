package firchecks

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/oracle"
)

// RuleConfigs maps a rule ID to its configured options. It is sent to
// krit-fir as check.ruleConfigs (read back by FirRule.config()) and folded
// into the FIR cache fingerprint so an option change invalidates findings.
type RuleConfigs map[string]map[string]any

// wireRuleConfigs returns a JSON-encodable copy of rc (nil when empty).
// Config values come from YAML, so nested maps may be keyed by interface{}
// and floats may be non-finite; neither survives encoding/json as-is.
func wireRuleConfigs(rc RuleConfigs) map[string]any {
	var out map[string]any
	for id, opts := range rc {
		if len(opts) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string]any, len(rc))
		}
		out[id] = jsonSafe(opts)
	}
	return out
}

func jsonSafe(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = jsonSafe(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = jsonSafe(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = jsonSafe(val)
		}
		return out
	case []string:
		return t
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return fmt.Sprint(t)
		}
		return t
	case float32:
		return jsonSafe(float64(t))
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return t
	default:
		if _, err := json.Marshal(t); err == nil {
			return t
		}
		return fmt.Sprint(t)
	}
}

// FirInvocationFingerprint combines the classpath with the checker binary
// identity, the enabled rule set, the rules' options, and the file facts sent
// with the request: the test-file classification (checkers skip test files
// through FirRule.isTestFile, so reclassifying a file, e.g. by editing the
// configured test paths, changes verdicts without changing any source) and
// the scan's spelling of each file (checkers apply Go's path heuristics to it
// through FirRule.scanPath, so scanning the same files from another working
// directory can change verdicts). Stat avoids
// hashing the large fat jar. encoding/json sorts map keys, so the options
// encoding is deterministic regardless of map iteration order.
func FirInvocationFingerprint(classpath []string, jarPath string, rules []string, ruleConfigs RuleConfigs, facts FileFacts) string {
	jarIdentity := jarPath + ":missing"
	if info, err := os.Stat(jarPath); err == nil {
		jarIdentity = fmt.Sprintf("%s:%d:%d", jarPath, info.Size(), info.ModTime().UnixNano())
	}
	ids := slices.Clone(rules)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	options, err := json.Marshal(wireRuleConfigs(ruleConfigs))
	if err != nil {
		// Unreachable after jsonSafe; never let a stale fingerprint match.
		options = []byte(fmt.Sprintf("unencodable:%v", err))
	}
	tests := slices.Clone(facts.TestFiles)
	slices.Sort(tests)
	tests = slices.Compact(tests)
	// A JSON array keeps the list unambiguous whatever the paths contain.
	testsJSON, err := json.Marshal(tests)
	if err != nil {
		testsJSON = []byte(fmt.Sprintf("unencodable:%v", err))
	}
	fingerprint := ClasspathFingerprint(classpath) + "\x00" + jarIdentity + "\x00" +
		strings.Join(ids, "\x00") + "\x00" + string(options) + "\x00" + string(testsJSON)
	if len(facts.ScanPaths) > 0 {
		// encoding/json sorts the keys. Appended only when present, so a
		// scan whose spellings match the requested paths keeps its cache.
		scanJSON, err := json.Marshal(facts.ScanPaths)
		if err != nil {
			scanJSON = []byte(fmt.Sprintf("unencodable:%v", err))
		}
		fingerprint += "\x00scanPaths:" + string(scanJSON)
	}
	return hashutil.HashHex([]byte(fingerprint))
}

// CheckCacheFingerprint is the closure fingerprint every FIR finding cache
// entry is validated against: the invocation (classpath, jar, rules,
// options, file facts) plus the whole compilation the check
// runs, i.e. every `.kt` under sourceDirs and every requested file, by path
// and content. krit-fir compiles
// the module as one unit, so a file's findings can change when any other
// source in it changes; keying on the compilation makes any source edit
// invalidate every cached FIR verdict (coarse but correct, the same trade-off
// as krit-fir's oracle cache).
func CheckCacheFingerprint(sourceDirs, files, classpath []string, jarPath string, rules []string, ruleConfigs RuleConfigs, facts FileFacts) string {
	sources := oracle.CompilationSources(sourceDirs)
	seen := make(map[string]bool, len(sources)+len(files))
	for _, p := range sources {
		seen[p] = true
	}
	for _, p := range files {
		if !seen[p] {
			seen[p] = true
			sources = append(sources, p)
		}
	}
	compilation := oracle.CompilationFingerprint(sources, classpath, jarPath)
	return hashutil.HashHex([]byte(FirInvocationFingerprint(classpath, jarPath, rules, ruleConfigs, facts) + "\x00" + compilation))
}
