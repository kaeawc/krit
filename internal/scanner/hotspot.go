package scanner

import "sort"

// ClassFanInStat captures how many external files reference a class-like symbol.
type ClassFanInStat struct {
	Symbol           Symbol
	ReferencingFiles []string
	FanIn            int
}

// ClassLikeFanInStats returns class-like declarations with their distinct
// external referencing files, sorted by descending fan-in.
func (idx *CodeIndex) ClassLikeFanInStats(ignoreCommentRefs bool) []ClassFanInStat {
	if idx == nil {
		return nil
	}

	stats := make([]ClassFanInStat, 0, len(idx.Symbols))
	for _, sym := range idx.Symbols {
		if !isClassLikeSymbol(sym.Kind) {
			continue
		}

		files := idx.externalReferenceFiles(sym.Name, sym.File, ignoreCommentRefs)
		stats = append(stats, ClassFanInStat{
			Symbol:           sym,
			ReferencingFiles: files,
			FanIn:            len(files),
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].FanIn != stats[j].FanIn {
			return stats[i].FanIn > stats[j].FanIn
		}
		if stats[i].Symbol.Name != stats[j].Symbol.Name {
			return stats[i].Symbol.Name < stats[j].Symbol.Name
		}
		return stats[i].Symbol.File < stats[j].Symbol.File
	})

	return stats
}

func isClassLikeSymbol(kind string) bool {
	switch kind {
	case "class", "object", "interface":
		return true
	default:
		return false
	}
}

func (idx *CodeIndex) externalReferenceFiles(name, declaringFile string, ignoreCommentRefs bool) []string {
	var source map[string]bool
	if ignoreCommentRefs {
		source = idx.nonCommentRefFilesByName[name]
	} else {
		source = idx.refFilesByName[name]
	}
	if len(source) == 0 {
		return nil
	}

	files := make([]string, 0, len(source))
	for file := range source {
		if file == declaringFile {
			continue
		}
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}
