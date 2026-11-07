package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Expression is the part of an expression fact that parity compares.
type Expression struct {
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	CallTarget string `json:"callTarget"`
	Resolved   bool   `json:"callTargetResolved"`
	Suspend    bool   `json:"callTargetSuspend"`
}

// Oracle maps a normalized file path to its expression facts, keyed "line:column".
type Oracle map[string]map[string]Expression

// LoadFile reads the expression facts of an oracle JSON file.
func LoadFile(path, root string) (Oracle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, root)
}

// identityHash matches the object identity suffix some compiler types print (for a star
// projection), which differs per run.
var identityHash = regexp.MustCompile(`@[0-9a-f]{5,8}\b`)

// Parse reads the expression facts of oracle JSON, with file paths relative to root when given.
func Parse(data []byte, root string) (Oracle, error) {
	var doc struct {
		Files map[string]struct {
			Expressions map[string]Expression `json:"expressions"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := make(Oracle, len(doc.Files))
	for p, f := range doc.Files {
		for k, e := range f.Expressions {
			e.Type = identityHash.ReplaceAllString(e.Type, "@")
			f.Expressions[k] = e
		}
		if root != "" {
			if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
				p = rel
			}
		}
		out[filepath.ToSlash(p)] = f.Expressions
	}
	return out, nil
}

// Kinds of expression difference.
const (
	KindOnlyA     = "only-in-a"
	KindOnlyB     = "only-in-b"
	KindType      = "type"
	KindTypeAErr  = "type(a-unresolved)"
	KindTypeBErr  = "type(b-unresolved)"
	KindNullable  = "nullable"
	KindTarget    = "call-target"
	KindResolved  = "call-resolved"
	KindSuspend   = "call-suspend"
	kindFileOnlyA = "file-only-in-a"
	kindFileOnlyB = "file-only-in-b"
)

// Report is the comparison result: counts per kind and the files behind each kind.
type Report struct {
	Files, SameFiles int
	Counts           map[string]int
	FilesByKind      map[string][]string
	Detail           []string
}

// Compare lists the differences between two oracles. Detail lines are produced for files whose
// path contains detail (when not empty).
func Compare(a, b Oracle, detail string) Report {
	r := Report{Counts: map[string]int{}, FilesByKind: map[string][]string{}}
	note := func(kind, file string) {
		r.Counts[kind]++
		if fs := r.FilesByKind[kind]; len(fs) == 0 || fs[len(fs)-1] != file {
			r.FilesByKind[kind] = append(fs, file)
		}
	}
	seen := map[string]bool{}
	for _, file := range sortedKeys(a) {
		seen[file] = true
		bx, ok := b[file]
		if !ok {
			note(kindFileOnlyA, file)
			continue
		}
		r.Files++
		before := totalCount(r.Counts)
		for _, key := range sortedKeys(a[file]) {
			ea := a[file][key]
			eb, ok := bx[key]
			if !ok {
				note(KindOnlyA, file)
				addDetail(&r, detail, file, key, "only in a: "+describe(ea))
				continue
			}
			if kind := typeKind(ea, eb); kind != KindType {
				// A side that failed to resolve the type has no meaningful derived facts.
				note(kind, file)
				addDetail(&r, detail, file, key, kind+": a="+describe(ea)+" b="+describe(eb))
				continue
			}
			for _, d := range []struct {
				kind string
				diff bool
			}{
				{KindType, ea.Type != eb.Type}, {KindNullable, ea.Nullable != eb.Nullable},
				{KindTarget, ea.CallTarget != eb.CallTarget}, {KindResolved, ea.Resolved != eb.Resolved},
				{KindSuspend, ea.Suspend != eb.Suspend},
			} {
				if d.diff {
					note(d.kind, file)
					addDetail(&r, detail, file, key, d.kind+": a="+describe(ea)+" b="+describe(eb))
				}
			}
		}
		for _, key := range sortedKeys(bx) {
			if _, ok := a[file][key]; !ok {
				note(KindOnlyB, file)
				addDetail(&r, detail, file, key, "only in b: "+describe(bx[key]))
			}
		}
		if totalCount(r.Counts) == before {
			r.SameFiles++
		}
	}
	for _, file := range sortedKeys(b) {
		if !seen[file] {
			note(kindFileOnlyB, file)
		}
	}
	return r
}

// typeKind separates a type that only one side failed to resolve from a real disagreement.
func typeKind(a, b Expression) string {
	aErr, bErr := unresolved(a.Type), unresolved(b.Type)
	switch {
	case bErr && !aErr:
		return KindTypeBErr
	case aErr && !bErr:
		return KindTypeAErr
	}
	return KindType
}

func unresolved(typ string) bool {
	return strings.Contains(typ, "<error>") || strings.Contains(typ, "???")
}

func totalCount(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func addDetail(r *Report, detail, file, key, text string) {
	if detail != "" && strings.Contains(file, detail) {
		r.Detail = append(r.Detail, fmt.Sprintf("%s %s %s", file, key, text))
	}
}

func describe(e Expression) string {
	return fmt.Sprintf("{%s nullable=%t target=%s resolved=%t suspend=%t}", e.Type, e.Nullable, e.CallTarget, e.Resolved, e.Suspend)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Print writes the summary, example files per kind, and any detail lines.
func (r Report) Print(w io.Writer, examples int) {
	fmt.Fprintf(w, "files compared: %d (identical: %d)\n", r.Files, r.SameFiles)
	kinds := sortedKeys(r.Counts)
	for _, kind := range kinds {
		fmt.Fprintf(w, "%-18s %8d entries in %d files\n", kind, r.Counts[kind], len(r.FilesByKind[kind]))
		for i, f := range r.FilesByKind[kind] {
			if i >= examples {
				break
			}
			fmt.Fprintf(w, "    %s\n", f)
		}
	}
	for _, line := range r.Detail {
		fmt.Fprintln(w, line)
	}
}
