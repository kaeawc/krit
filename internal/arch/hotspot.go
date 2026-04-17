package arch

import (
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
)

// Hotspot represents a symbol with high fan-in.
type Hotspot struct {
	Name  string
	Kind  string // "class", "function", etc.
	FanIn int    // count of distinct files referencing this symbol
	File  string
	Line  int
}

// SymbolFanIn computes the fan-in (number of distinct files referencing)
// for each declared symbol in the index. The declaring file is excluded
// from the count, so a symbol only referenced in its own file has fan-in 0.
func SymbolFanIn(index *scanner.CodeIndex) map[string]int {
	if index == nil {
		return nil
	}
	fanIn := make(map[string]int, len(index.Symbols))

	// Track which file declares each name (first occurrence wins for dedup).
	declaringFiles := make(map[string]string, len(index.Symbols))
	for _, sym := range index.Symbols {
		if _, seen := declaringFiles[sym.Name]; !seen {
			declaringFiles[sym.Name] = sym.File
		}
	}

	for _, sym := range index.Symbols {
		if _, done := fanIn[sym.Name]; done {
			continue
		}
		refs := index.ReferenceFiles(sym.Name)
		count := 0
		for f := range refs {
			if f != declaringFiles[sym.Name] {
				count++
			}
		}
		fanIn[sym.Name] = count
	}
	return fanIn
}

// FilterHotspots returns symbols whose fan-in exceeds the threshold,
// sorted by fan-in descending. When multiple symbols share a name, only
// the first declaration is included.
func FilterHotspots(index *scanner.CodeIndex, fanIn map[string]int, threshold int) []Hotspot {
	if index == nil {
		return nil
	}
	seen := make(map[string]bool)
	var hotspots []Hotspot

	for _, sym := range index.Symbols {
		if seen[sym.Name] {
			continue
		}
		seen[sym.Name] = true
		fi := fanIn[sym.Name]
		if fi <= threshold {
			continue
		}
		hotspots = append(hotspots, Hotspot{
			Name:  sym.Name,
			Kind:  sym.Kind,
			FanIn: fi,
			File:  sym.File,
			Line:  sym.Line,
		})
	}

	sort.Slice(hotspots, func(i, j int) bool {
		if hotspots[i].FanIn != hotspots[j].FanIn {
			return hotspots[i].FanIn > hotspots[j].FanIn
		}
		return hotspots[i].Name < hotspots[j].Name
	})
	return hotspots
}
