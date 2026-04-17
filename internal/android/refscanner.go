package android

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FileReference records a location where a resource file is referenced by its
// full name (including extension). Such references would break if the file is
// renamed or converted to a different format (e.g. PNG -> WebP).
type FileReference struct {
	Path string // file containing the reference
	Line int    // 1-based line number
	Text string // the line text containing the reference
}

// ScanFileReferences searches Kotlin, Java, and XML files under searchDirs for
// literal occurrences of fileName (e.g. "icon.png"). It returns every location
// where fileName appears as a substring of a source line.
//
// Android resolves drawable references by resource name (without extension), so
// "@drawable/icon" is safe for both icon.png and icon.webp. This function
// specifically targets direct file-name references that would break after a
// format conversion.
func ScanFileReferences(searchDirs []string, fileName string) []FileReference {
	if fileName == "" {
		return nil
	}

	var refs []FileReference

	for _, dir := range searchDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".kt", ".java", ".xml":
				// scan this file
			default:
				return nil
			}

			found := scanFileForString(path, fileName)
			refs = append(refs, found...)
			return nil
		})
	}

	return refs
}

// scanFileForString reads a file line by line and returns FileReference entries
// for every line containing the target string.
func scanFileForString(path, target string) []FileReference {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var refs []FileReference
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, target) {
			refs = append(refs, FileReference{
				Path: path,
				Line: lineNum,
				Text: line,
			})
		}
	}
	return refs
}
