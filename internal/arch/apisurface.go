package arch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// APIEntry represents a single public API element.
type APIEntry struct {
	Kind      string // "class", "interface", "object", "function", "property"
	Name      string // fully qualified or simple name
	Signature string // e.g., "fun getUser(id: String): User"
	File      string
	Line      int
}

// APIChange values for APIDiff.Change.
const (
	ChangeAdded   = "added"
	ChangeRemoved = "removed"
)

// APIDiff represents a change between two API surfaces.
type APIDiff struct {
	Change string // one of ChangeAdded, ChangeRemoved
	Entry  APIEntry
}

// apiEntryKey returns a stable key for matching entries across surfaces.
func apiEntryKey(e APIEntry) string {
	return e.Kind + "\t" + e.Name
}

// ExtractSurface extracts the public API surface from parsed files.
// Returns public/protected declarations sorted by kind+name for determinism.
func ExtractSurface(symbols []scanner.Symbol) []APIEntry {
	var entries []APIEntry

	for _, sym := range symbols {
		if sym.Visibility != "public" && sym.Visibility != "protected" {
			continue
		}
		if sym.IsTest || sym.IsOverride {
			continue
		}

		entries = append(entries, APIEntry{
			Kind: sym.Kind,
			Name: sym.Name,
			File: sym.File,
			Line: sym.Line,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Name < entries[j].Name
	})

	return entries
}

// FormatSurface produces a deterministic text representation of an API surface.
// Entries should already be sorted (ExtractSurface guarantees this); if passed
// arbitrary input, callers should sort first.
func FormatSurface(entries []APIEntry) string {
	var buf strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&buf, "%s\t%s\n", e.Kind, e.Name)
	}
	return buf.String()
}

// ParseSurface parses a text surface (produced by FormatSurface) back into entries.
func ParseSurface(text string) []APIEntry {
	var entries []APIEntry
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		entries = append(entries, APIEntry{
			Kind: parts[0],
			Name: parts[1],
		})
	}
	return entries
}

// DiffSurfaces compares old and new API surfaces, returning additions and removals.
func DiffSurfaces(old, new []APIEntry) []APIDiff {
	oldSet := make(map[string]APIEntry, len(old))
	for _, e := range old {
		oldSet[apiEntryKey(e)] = e
	}

	newSet := make(map[string]APIEntry, len(new))
	for _, e := range new {
		newSet[apiEntryKey(e)] = e
	}

	var diffs []APIDiff

	// Find additions (in new but not old)
	for key, entry := range newSet {
		if _, exists := oldSet[key]; !exists {
			diffs = append(diffs, APIDiff{Change: ChangeAdded, Entry: entry})
		}
	}

	// Find removals (in old but not new)
	for key, entry := range oldSet {
		if _, exists := newSet[key]; !exists {
			diffs = append(diffs, APIDiff{Change: ChangeRemoved, Entry: entry})
		}
	}

	// Sort for determinism
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Change != diffs[j].Change {
			return diffs[i].Change < diffs[j].Change
		}
		if diffs[i].Entry.Kind != diffs[j].Entry.Kind {
			return diffs[i].Entry.Kind < diffs[j].Entry.Kind
		}
		return diffs[i].Entry.Name < diffs[j].Entry.Name
	})

	return diffs
}
