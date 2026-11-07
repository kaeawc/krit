package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Finding is the part of a krit finding that parity compares.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Key identifies a finding after normalization.
type Key = Finding

// Diff is the multiset difference between two finding lists, grouped by rule.
type Diff struct {
	OnlyA, OnlyB map[string][]Key
	Matching     int
}

// LoadFile reads the findings of a krit JSON output.
func LoadFile(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc.Findings, nil
}

// Normalize makes file paths relative to root (when given) with forward slashes and collapses
// whitespace in messages.
func Normalize(in []Finding, root string) []Finding {
	out := make([]Finding, len(in))
	for i, f := range in {
		if root != "" {
			if rel, err := filepath.Rel(root, f.File); err == nil && !strings.HasPrefix(rel, "..") {
				f.File = rel
			}
		}
		f.File = filepath.ToSlash(f.File)
		f.Message = strings.Join(strings.Fields(f.Message), " ")
		out[i] = f
	}
	return out
}

// Compare treats each list as a multiset, so a finding reported twice on one side and once on the
// other is a difference.
func Compare(a, b []Finding) Diff {
	d := Diff{OnlyA: map[string][]Key{}, OnlyB: map[string][]Key{}}
	counts := map[Key]int{}
	for _, f := range a {
		counts[f]++
	}
	for _, f := range b {
		if counts[f] > 0 {
			counts[f]--
			d.Matching++
		} else {
			d.OnlyB[f.Rule] = append(d.OnlyB[f.Rule], f)
		}
	}
	for k, n := range counts {
		for ; n > 0; n-- {
			d.OnlyA[k.Rule] = append(d.OnlyA[k.Rule], k)
		}
	}
	for _, m := range []map[string][]Key{d.OnlyA, d.OnlyB} {
		for _, keys := range m {
			sort.Slice(keys, func(i, j int) bool {
				if keys[i].File != keys[j].File {
					return keys[i].File < keys[j].File
				}
				if keys[i].Line != keys[j].Line {
					return keys[i].Line < keys[j].Line
				}
				return keys[i].Message < keys[j].Message
			})
		}
	}
	return d
}

// Rules lists every rule with a difference, sorted.
func (d Diff) Rules() []string {
	seen := map[string]bool{}
	for r := range d.OnlyA {
		seen[r] = true
	}
	for r := range d.OnlyB {
		seen[r] = true
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// CountA is the number of findings only in the reference.
func (d Diff) CountA() int { return countAll(d.OnlyA) }

// CountB is the number of findings only in the candidate.
func (d Diff) CountB() int { return countAll(d.OnlyB) }

// Total is the number of differing findings on both sides.
func (d Diff) Total() int { return d.CountA() + d.CountB() }

func countAll(m map[string][]Key) int {
	n := 0
	for _, keys := range m {
		n += len(keys)
	}
	return n
}
