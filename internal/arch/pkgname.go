package arch

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Drift represents a mismatch between a file's declared package and its expected package.
type Drift struct {
	FilePath string
	Declared string
	Expected string
	Line     int
}

// ExpectedPackage derives the expected Kotlin/Java package name from a file's
// path relative to a source root. For example:
//
//	filePath="/project/app/src/main/kotlin/com/example/feature/Foo.kt"
//	sourceRoot="/project/app/src/main/kotlin"
//	returns "com.example.feature"
func ExpectedPackage(filePath string, sourceRoot string) string {
	rel, err := filepath.Rel(sourceRoot, filePath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)

	// Strip the filename
	dir := filepath.Dir(rel)
	if dir == "." || dir == "" {
		return ""
	}

	// Replace / with .
	return strings.ReplaceAll(filepath.ToSlash(dir), "/", ".")
}

// PackageNameDrift checks if a file's declared package matches the expected
// package derived from its path. Returns nil if they match or if the file
// has no package declaration.
func PackageNameDrift(file *scanner.File, sourceRoot string) *Drift {
	if file == nil {
		return nil
	}

	declared := ""
	declLine := 0

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if strings.HasPrefix(trimmed, "package ") {
			declared = strings.TrimSpace(strings.TrimPrefix(trimmed, "package "))
			declLine = i + 1
			break
		}
		// If we hit an import or other code first, there's no package declaration
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "fun ") || strings.HasPrefix(trimmed, "object ") {
			break
		}
	}

	// No package declaration — skip
	if declared == "" {
		return nil
	}

	expected := ExpectedPackage(file.Path, sourceRoot)
	if declared == expected {
		return nil
	}

	return &Drift{
		FilePath: file.Path,
		Declared: declared,
		Expected: expected,
		Line:     declLine,
	}
}
