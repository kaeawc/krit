package oracle

import (
	"bytes"
	"context"
	"os"
	"sort"
	"strings"

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

// Known limits of demoteNameCollidingHits:
//
//   - The oracle filter list can drop a declaring file that has no rule
//     identifiers, so it is never a miss and its names are never collected
//     (https://github.com/kaeawc/krit/issues/847).
//   - The check is run-local: it only sees the files analyzed in this run. A
//     partial or subpath scan that analyzes the new file caches it, so a
//     later full scan finds no miss and never demotes the callers
//     (https://github.com/kaeawc/krit/issues/847).
//   - Declaring Java sources are not scanned; only Kotlin files contribute
//     names, though Java classes can also add overload candidates.
//   - Deleted files are not considered. Removing a declaration can change
//     resolution only for callers that resolved to it, and those carry the
//     deleted file in their closure.

type declEntry struct {
	pkg, name string
	operator  bool
}

// declaredSet is the set of top-level names the changed files declare.
type declaredSet struct {
	entries []declEntry
}

// demoteNameCollidingHits moves cached KAA hits back to the miss list when a
// file being re-analyzed this run declares a top-level name the hit uses
// from a scope that can see it.
//
// A krit-types cache entry's dependency closure records only the
// declarations its file already resolved to. A new file that adds an overload
// (or a same-named function, extension, property, or type) in the same
// package, or in one the file imports, can change what a cached caller
// resolves to without touching anything in that caller's closure, so
// closure verification alone keeps the caller's stale facts. The files
// analyzed this run are exactly the ones that are new or changed (everything
// else verified as a hit), so their top-level declared names are the names
// that may have appeared. A hit is demoted when it
//
//   - mentions such a name as an identifier outside comments, or, for an
//     `operator` declaration, contains a syntax pattern that calls it (see
//     operatorGates), and
//   - is in the declaring package, imports the name (explicitly, by
//     wildcard, or through a default import), or spells it
//     package-qualified, and
//   - cannot be read, which fails closed.
//
// The test is deliberately conservative on the hit side (lexical, so a name
// in a string template or an unrelated receiver's member can demote a hit)
// and precise on the declaring side (flat-AST top-level, non-private
// declarations only). Over-invalidation costs one re-analysis of that file;
// an unrelated added file demotes nothing. After re-analysis the new file is
// in the caller's closure, so later edits of it invalidate through the
// ordinary closure check.
//
// Whole-compilation backends already refresh every file's facts together and
// are not affected.
func demoteNameCollidingHits(opts InvocationOptions, hits []*CacheEntry, misses []string, tracker perf.Tracker, verbose bool) ([]*CacheEntry, []string) {
	if opts.Backend.ReturnsWholeCompilation() || len(hits) == 0 || len(misses) == 0 {
		return hits, misses
	}
	declared := declaredNamesOf(misses)
	if len(declared.entries) == 0 {
		return hits, misses
	}
	kept := make([]*CacheEntry, 0, len(hits))
	var demoted []string
	for _, hit := range hits {
		if !hit.Crashed && hitSeesDeclaredName(hit.FilePath, declared) {
			demoted = append(demoted, hit.FilePath)
			continue
		}
		kept = append(kept, hit)
	}
	if len(demoted) == 0 {
		return hits, misses
	}
	sort.Strings(demoted)
	addOracleInstant(tracker, "nameCollisionInvalidated", map[string]int64{"files": int64(len(demoted))}, nil)
	if verbose {
		reporter().Verbosef("verbose: cache classify: %d hit(s) use a name declared by a changed file; re-analyzing\n", len(demoted))
	}
	return kept, append(append([]string(nil), misses...), demoted...)
}

// declaredNamesOf collects the visible top-level names the given Kotlin
// files declare. Files that cannot be parsed contribute nothing; they are
// already misses and the closure check covers their callers.
func declaredNamesOf(paths []string) declaredSet {
	var set declaredSet
	seen := map[declEntry]int{}
	for _, path := range paths {
		if !strings.HasSuffix(path, ".kt") {
			continue
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			continue
		}
		pkg, decls := scanner.TopLevelDeclaredNames(file)
		for _, d := range decls {
			key := declEntry{pkg: pkg, name: d.Name}
			if i, ok := seen[key]; ok {
				set.entries[i].operator = set.entries[i].operator || d.Operator
				continue
			}
			seen[key] = len(set.entries)
			set.entries = append(set.entries, declEntry{pkg: pkg, name: d.Name, operator: d.Operator})
		}
	}
	return set
}

// hitSeesDeclaredName reports whether the file at path uses one of the
// declared names from a scope that can see it. The checks run cheapest
// first: raw substring and syntax-pattern presence, then the lexical
// identifier scan only for names that are present, then the header prefix
// (package and imports) only for names actually mentioned.
func hitSeesDeclaredName(path string, declared declaredSet) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		// Fail closed: a hit we cannot inspect cannot be trusted.
		return true
	}
	// wanted holds the identifiers and gate tokens the lexical scan looks
	// for; gated marks entries whose raw syntax gate is already present.
	wanted := map[string]bool{}
	gated := make([]bool, len(declared.entries))
	anyGated := false
	for i, e := range declared.entries {
		if bytes.Contains(content, []byte(e.name)) {
			wanted[e.name] = true
		}
		if !e.operator {
			continue
		}
		gate := gatesFor(e.name)
		for _, g := range gate.raw {
			if bytes.Contains(content, []byte(g)) {
				gated[i] = true
				anyGated = true
				break
			}
		}
		for _, tok := range gate.tokens {
			wanted[tok] = true
		}
	}
	if len(wanted) == 0 && !anyGated {
		return false
	}
	var used map[string]bool
	if len(wanted) > 0 {
		used = identifiersUsed(content, wanted)
	}
	var header scanner.SourceHeader
	headerRead := false
	for i, e := range declared.entries {
		viaToken := e.operator && anyToken(used, gatesFor(e.name).tokens)
		if !used[e.name] && !gated[i] && !viaToken {
			continue
		}
		if !headerRead {
			header = scanner.ParseSourceHeader(content)
			headerRead = true
		}
		if declaredNameVisible(header, e, content) {
			return true
		}
	}
	return false
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
