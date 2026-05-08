package arch

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// BroadFile represents a file with high import breadth.
type BroadFile struct {
	Path         string
	PackageCount int
}

// ImportBreadth counts the number of distinct packages imported by a file.
// It parses the file's Lines for "import" declarations and extracts parent packages.
func ImportBreadth(file *scanner.File) int {
	if file == nil {
		return 0
	}
	packages := make(map[string]struct{})

	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}

		imp := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))

		// Handle "import ... as ..." aliases
		if idx := strings.Index(imp, " as "); idx >= 0 {
			imp = strings.TrimSpace(imp[:idx])
		}

		// Extract the package portion (everything before the last segment)
		if strings.HasSuffix(imp, ".*") {
			// Wildcard import: package is everything before .*
			imp = strings.TrimSuffix(imp, ".*")
		} else if lastDot := strings.LastIndex(imp, "."); lastDot > 0 {
			imp = imp[:lastDot]
		} else {
			continue
		}

		if imp != "" {
			packages[imp] = struct{}{}
		}
	}

	return len(packages)
}

// FindBroadFiles returns files whose import breadth exceeds the threshold,
// sorted by package count descending.
func FindBroadFiles(files []*scanner.File, threshold int) []BroadFile {
	var broad []BroadFile

	for _, f := range files {
		count := ImportBreadth(f)
		if count > threshold {
			broad = append(broad, BroadFile{
				Path:         f.Path,
				PackageCount: count,
			})
		}
	}

	sort.Slice(broad, func(i, j int) bool {
		if broad[i].PackageCount != broad[j].PackageCount {
			return broad[i].PackageCount > broad[j].PackageCount
		}
		return broad[i].Path < broad[j].Path
	})
	return broad
}
