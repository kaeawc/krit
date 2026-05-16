// Package reconcile maps runtime states onto the static symbol
// index. Mirrors AutoMobile's reconcile pass: first an exact match,
// then a fuzzy match that emits StateSuggestion rows for human / later
// review.
//
// Exact: state.TopSymbol == Symbol.FQN, or state.TopSymbol matches a
// unique Symbol.Name when FQN-style lookup misses.
//
// Fuzzy: score every candidate symbol with the same simple name as
// the runtime frame's leaf identifier. Score components:
//
//  1. Name similarity — the longest common suffix between the
//     runtime symbol and the candidate's FQN, divided by the longer
//     of the two. Catches `Foo.bar` vs `com.acme.Foo.bar` and
//     `bar(int)` vs `bar` differences.
//  2. Caller-chain proximity — when reduced states are reconcile-d
//     together, fuzzy candidates earn a bonus if any of their static
//     callers also appear as a resolved caller in the state's caller
//     chain. We only do the cheaper "module locality" form: same
//     package prefix as a previously-resolved state.
//  3. Module locality — same package prefix as the calling frame.
package reconcile

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/traces"
)

// Index is the static-symbol view reconcile needs. Built once per CLI
// invocation from the snapshot's symbol list or a freshly-built
// CodeIndex.
type Index struct {
	byFQN  map[string]scanner.Symbol
	byName map[string][]scanner.Symbol
}

// BuildIndex builds the reconcile index over a flat symbol list. The
// list may come from a scanner.CodeIndex (Symbols) or from a snapshot
// Blob (converted by the caller into scanner.Symbol).
func BuildIndex(symbols []scanner.Symbol) *Index {
	idx := &Index{
		byFQN:  make(map[string]scanner.Symbol, len(symbols)),
		byName: make(map[string][]scanner.Symbol, len(symbols)),
	}
	for _, s := range symbols {
		if s.FQN != "" {
			idx.byFQN[s.FQN] = s
		}
		if s.Name != "" {
			idx.byName[s.Name] = append(idx.byName[s.Name], s)
		}
	}
	return idx
}

// Result classifies one state's reconciliation outcome.
type Result struct {
	Fingerprint string
	Resolution  traces.Resolution
	Match       scanner.Symbol // populated when Resolution is ResolvedExact
	Suggestions []traces.StateSuggestion
}

// Reconcile maps every state in `states` against `idx`. Returns a
// slice of results in the same order so callers can persist
// resolutions and suggestions back to the Store.
func Reconcile(idx *Index, states []traces.RuntimeState) []Result {
	if idx == nil {
		idx = BuildIndex(nil)
	}
	out := make([]Result, 0, len(states))
	for _, st := range states {
		r := Result{Fingerprint: st.Fingerprint}
		if sym, ok := idx.byFQN[st.TopSymbol]; ok {
			r.Resolution = traces.ResolvedExact
			r.Match = sym
			out = append(out, r)
			continue
		}
		leaf := simpleName(st.TopSymbol)
		if matches := idx.byName[leaf]; len(matches) == 1 {
			r.Resolution = traces.ResolvedExact
			r.Match = matches[0]
			out = append(out, r)
			continue
		}
		// Fuzzy: rank candidates by name similarity.
		candidates := idx.byName[leaf]
		if len(candidates) == 0 {
			r.Resolution = traces.Unresolved
			out = append(out, r)
			continue
		}
		scored := make([]traces.StateSuggestion, 0, len(candidates))
		for _, c := range candidates {
			sim := suffixSimilarity(st.TopSymbol, c.FQN)
			locality := moduleLocality(c, st.CallerFrames)
			conf := sim
			evidence := "name-similarity"
			if locality > 0 {
				// Caller-chain proximity is the AutoMobile equivalent of a
				// candidate appearing inside the same NavigationContext —
				// a strong signal that ties this candidate to the
				// observed call site. Combine so a name-and-caller
				// match decisively beats name-only matches.
				conf = sim*0.5 + locality*0.5 + 0.1
				if conf > 1 {
					conf = 1
				}
				evidence = "name-similarity+caller-chain"
			}
			scored = append(scored, traces.StateSuggestion{
				Fingerprint:     st.Fingerprint,
				CandidateSymbol: c.FQN,
				Evidence:        evidence,
				Confidence:      conf,
			})
		}
		sort.Slice(scored, func(i, j int) bool {
			if scored[i].Confidence != scored[j].Confidence {
				return scored[i].Confidence > scored[j].Confidence
			}
			return scored[i].CandidateSymbol < scored[j].CandidateSymbol
		})
		r.Resolution = traces.ResolvedFuzzy
		r.Suggestions = scored
		out = append(out, r)
	}
	return out
}

// simpleName returns the last dot-separated component of sym,
// stripped of any "(...)" signature suffix. Duplicates
// clishared.SimpleName to avoid a cli→non-cli import cycle.
func simpleName(sym string) string {
	if i := strings.IndexByte(sym, '('); i >= 0 {
		sym = sym[:i]
	}
	if i := strings.LastIndexByte(sym, '.'); i >= 0 {
		return sym[i+1:]
	}
	return sym
}

// suffixSimilarity returns the ratio of the longest shared
// dot-separated suffix to the longer of the two FQNs, in [0,1].
// Walks both strings from the right using LastIndexByte so the hot
// fuzzy-ranking loop does not allocate per candidate.
func suffixSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	remA, remB := stripSig(a), stripSig(b)
	matched := 0
	for remA != "" && remB != "" {
		ai := strings.LastIndexByte(remA, '.')
		bi := strings.LastIndexByte(remB, '.')
		if remA[ai+1:] != remB[bi+1:] {
			break
		}
		matched++
		if ai < 0 {
			remA = ""
		} else {
			remA = remA[:ai]
		}
		if bi < 0 {
			remB = ""
		} else {
			remB = remB[:bi]
		}
	}
	if matched == 0 {
		return 0
	}
	totalA := segmentCount(remA) + matched
	totalB := segmentCount(remB) + matched
	denom := totalA
	if totalB > denom {
		denom = totalB
	}
	return float64(matched) / float64(denom)
}

// segmentCount returns the number of dot-separated segments in s.
// Empty string counts as 0.
func segmentCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, ".") + 1
}

// moduleLocality returns a [0,1] score based on how much of the
// candidate's package (or owner) appears in any caller frame. The
// match is dot-segment-prefix: we look for the longest leading
// segment sequence common to both. This is the AutoMobile reconcile
// "same NavigationContext" signal, ported to packages.
func moduleLocality(c scanner.Symbol, callers []string) float64 {
	if len(callers) == 0 {
		return 0
	}
	candPkg := c.Package
	if candPkg == "" {
		candPkg = stripLast(stripSig(c.FQN))
	}
	if candPkg == "" {
		return 0
	}
	candSeg := strings.Split(candPkg, ".")
	best := 0
	for _, caller := range callers {
		callerPkg := stripLast(stripSig(caller))
		if callerPkg == "" {
			continue
		}
		seg := strings.Split(callerPkg, ".")
		matched := 0
		for i := 0; i < len(candSeg) && i < len(seg); i++ {
			if candSeg[i] != seg[i] {
				break
			}
			matched++
		}
		if matched > best {
			best = matched
		}
	}
	if best == 0 {
		return 0
	}
	denom := len(candSeg)
	if denom == 0 {
		return 0
	}
	return float64(best) / float64(denom)
}

// stripLast drops the last dot-separated segment of s.
func stripLast(s string) string {
	i := strings.LastIndexByte(s, '.')
	if i < 0 {
		return ""
	}
	return s[:i]
}

func stripSig(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 {
		return s[:i]
	}
	return s
}
