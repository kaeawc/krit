// Package iterutil contains determinism-safe iteration helpers.
//
// Go's map iteration order is randomized. Any code that drives output,
// logging, error reporting, hashing, cache keys, or downstream ordering
// from a map range produces non-deterministic results across runs.
//
// This package centralizes the "sort then iterate" pattern so authors
// can opt into determinism explicitly via a single helper, and so a
// linter (forbidigo or similar) can ban bare `range map[K]V` in
// determinism-sensitive packages and steer callers here.
package iterutil

import (
	"cmp"
	"sort"
)

// SortedKeys returns the keys of m in ascending order.
//
// The returned slice is freshly allocated; callers may freely mutate it.
// For empty maps the returned slice is nil.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	if len(m) == 0 {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// ForEachSorted invokes fn(key, m[key]) for each key in m, in ascending
// key order. Callers should prefer this over a bare `for k, v := range m`
// in any code path that affects observable output, logs, errors, or
// hashes.
func ForEachSorted[K cmp.Ordered, V any](m map[K]V, fn func(K, V)) {
	for _, k := range SortedKeys(m) {
		fn(k, m[k])
	}
}
