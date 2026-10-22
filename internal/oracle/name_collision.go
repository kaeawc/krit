package oracle

import (
	"bytes"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// kotlinDefaultImports are the packages Kotlin imports into every file. A new
// top-level declaration in one of them is visible without an import.
var kotlinDefaultImports = map[string]bool{
	"kotlin": true, "kotlin.annotation": true, "kotlin.collections": true,
	"kotlin.comparisons": true, "kotlin.io": true, "kotlin.ranges": true,
	"kotlin.sequences": true, "kotlin.text": true, "kotlin.jvm": true,
	"java.lang": true,
}

// operatorGates maps an operator-convention function name to the source
// patterns through which a caller can reach it without writing its name. A
// hit that contains any pattern may call the function by operator syntax, so
// for `operator` declarations the pattern stands in for the identifier. The
// patterns are raw byte matches and deliberately loose: `(` for invoke
// matches nearly every file, which only matters when a changed file declares
// an `operator fun invoke` visible to it.
//
// A gate is either raw bytes that need no surrounding spacing (`+`, `[`) or
// a token (`in`, `by`, `for`, `->`, `val`) found by the lexical scan, which
// skips comments and plain strings and does not depend on how the source is
// spaced or wrapped.
type opGate struct {
	raw    []string
	tokens []string
}

var operatorGates = map[string]opGate{
	"plus": {raw: []string{"+"}}, "minus": {raw: []string{"-"}}, "times": {raw: []string{"*"}},
	"div": {raw: []string{"/"}}, "rem": {raw: []string{"%"}}, "mod": {raw: []string{"%"}},
	"unaryPlus": {raw: []string{"+"}}, "unaryMinus": {raw: []string{"-"}}, "not": {raw: []string{"!"}},
	"inc": {raw: []string{"++"}}, "dec": {raw: []string{"--"}},
	"plusAssign": {raw: []string{"+="}}, "minusAssign": {raw: []string{"-="}},
	"timesAssign": {raw: []string{"*="}}, "divAssign": {raw: []string{"/="}},
	"remAssign": {raw: []string{"%="}}, "modAssign": {raw: []string{"%="}},
	// invoke also covers a trailing-lambda call, `x { }`.
	"get": {raw: []string{"["}}, "set": {raw: []string{"["}}, "invoke": {raw: []string{"(", "{"}},
	"getValue": {tokens: []string{"by"}}, "setValue": {tokens: []string{"by"}},
	"provideDelegate": {tokens: []string{"by"}},
	"compareTo":       {raw: []string{"<", ">"}},
	"contains":        {tokens: []string{"in"}},
	"iterator":        {tokens: []string{"for"}}, "next": {tokens: []string{"for"}},
	"hasNext":    {tokens: []string{"for"}},
	"rangeTo":    {raw: []string{".."}},
	"rangeUntil": {raw: []string{".."}},
}

// componentGate covers destructuring: `val (a, b) =`, `for ((a, b) in`, and
// lambda parameters `(a, b) ->`.
var componentGate = opGate{tokens: []string{"val", "var", "for", "->"}}

func gatesFor(name string) opGate {
	if g, ok := operatorGates[name]; ok {
		return g
	}
	if strings.HasPrefix(name, "component") && len(name) > len("component") {
		digits := name[len("component"):]
		if strings.Trim(digits, "0123456789") == "" {
			return componentGate
		}
	}
	return opGate{}
}

// Known limits of the name-collision check:
//
//   - A declaration only counts when the scanner's flat-AST index reports it
//     as a visible top-level one: members, locals, and private declarations
//     never do. Java sources contribute their top-level types only.
//   - The mention side is lexical (see identifiersUsed), so a name in a string
//     template or on an unrelated receiver can re-analyze a file once.
//   - The ledger describes the files a scan collected. A file outside every
//     scanned root keeps the record of the last scan that covered it; if it
//     changed since, the change is noticed by the next scan that covers it.
//   - A cached entry's facts still depend on which files the analysis saw;
//     the digest covers the declarations the ledger knows, not the scan
//     universe a file was analyzed in.

// declEntry is a (package, name) pair of a visible top-level declaration.
type declEntry struct {
	pkg, name string
}

// demoteStaleNameDeps moves cached KAA hits back to the miss list when the
// declarations visible to them changed since their facts were computed.
//
// A krit-types cache entry's dependency closure records only the
// declarations its file already resolved to. A declaration added later in a
// scope the file can see (an overload in its package, a same-named function,
// extension, property, or type in an imported package) can change what a
// cached caller resolves to without touching anything in that caller's
// closure, so closure verification alone keeps stale facts. Each entry
// therefore stores NameDigest, the digest of the ledger items its file
// mentioned and could see when it was analyzed (see declLedger), and a hit
// whose digest against the current ledger differs is demoted. That covers
// declarations that are new, deleted, or re-signed, whether the declaring file
// was analyzed in this run, an earlier invocation, or not at all (an oracle
// filter list can drop it): the ledger is refreshed from every collected
// source file before any of that is decided.
//
// A hit is compared through the same visibility rules as before: the
// declaring package is the file's own, a default-import package, imported
// explicitly or by wildcard, or spelled package-qualified; operator functions
// also count when the file contains syntax that calls them (see
// operatorGates). A hit that cannot be read fails closed, and a crashed entry
// has nothing to go stale.
//
// Whole-compilation backends already refresh every file's facts together and
// are not affected.
func demoteStaleNameDeps(opts InvocationOptions, ledger *declLedger, hits []*CacheEntry, misses []string, tracker perf.Tracker, verbose bool) ([]*CacheEntry, []string) {
	if ledger == nil || opts.Backend.ReturnsWholeCompilation() || len(hits) == 0 {
		return hits, misses
	}
	start := time.Now()
	stale := make([]bool, len(hits))
	runParallel(len(hits), func(i int) {
		hit := hits[i]
		if hit.Crashed {
			return
		}
		current, ok := ledger.currentDigest(hit.FilePath)
		stale[i] = !ok || current != hit.NameDigest
	})
	kept := make([]*CacheEntry, 0, len(hits))
	var demoted []string
	for i, hit := range hits {
		if stale[i] {
			demoted = append(demoted, hit.FilePath)
			continue
		}
		kept = append(kept, hit)
	}
	addOracleEntry(tracker, "nameDigestVerify", start, map[string]int64{
		"hits":     int64(len(hits)),
		"demoted":  int64(len(demoted)),
		"computed": ledger.computed.Load(),
	}, nil)
	if len(demoted) == 0 {
		return hits, misses
	}
	sort.Strings(demoted)
	addOracleInstant(tracker, "nameCollisionInvalidated", map[string]int64{"files": int64(len(demoted))}, nil)
	if verbose {
		reporter().Verbosef("verbose: cache classify: %d hit(s) see a declaration that changed since they were analyzed; re-analyzing\n", len(demoted))
	}
	return kept, append(append([]string(nil), misses...), demoted...)
}

func anyToken(used map[string]bool, tokens []string) bool {
	for _, tok := range tokens {
		if used[tok] {
			return true
		}
	}
	return false
}

// declaredNameVisible reports whether a file with the given header can see
// the top-level declaration e.
func declaredNameVisible(header scanner.SourceHeader, e declEntry, content []byte) bool {
	if header.Package == e.pkg || kotlinDefaultImports[e.pkg] {
		return true
	}
	qualified := e.name
	if e.pkg != "" {
		qualified = e.pkg + "." + e.name
	}
	for _, imp := range header.Imports {
		if imp.Wildcard {
			if imp.Path == e.pkg {
				return true
			}
		} else if imp.Path == qualified {
			return true
		}
	}
	return e.pkg != "" && containsQualified(content, qualified)
}

// containsQualified reports whether qualified appears in content as a whole
// dotted name, so `a.p.pick` does not count as `p.pick`.
func containsQualified(content []byte, qualified string) bool {
	needle := []byte(qualified)
	for from := 0; from < len(content); {
		i := bytes.Index(content[from:], needle)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(needle)
		beforeOK := start == 0 || (!isIdentByte(content[start-1]) && content[start-1] != '.')
		afterOK := end == len(content) || !isIdentByte(content[end])
		if beforeOK && afterOK {
			return true
		}
		from = start + 1
	}
	return false
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= 0x80 ||
		(b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentStart(b byte) bool {
	return b == '_' || b == '$' || b >= 0x80 || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
