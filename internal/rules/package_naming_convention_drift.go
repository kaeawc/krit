package rules

import (
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

const packageNamingConventionDriftSourceRoot = "/src/main/kotlin/"

// PackageNamingConventionDriftRule reports Kotlin source files whose package
// declaration no longer follows the directory path under src/main/kotlin.
type PackageNamingConventionDriftRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence holds the 0.95 dispatch default — package-header drift
// is a purely structural comparison against the expected prefix
// derived from the source root path. No heuristic path.
func (r *PackageNamingConventionDriftRule) Confidence() float64 { return 0.95 }

func (r *PackageNamingConventionDriftRule) NodeTypes() []string { return []string{"package_header"} }

func (r *PackageNamingConventionDriftRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	pkg := packageHeaderNameFlat(file, idx)
	if pkg == "" {
		return nil
	}

	expectedPrefix := packageNamingConventionExpectedPrefix(file.Path)
	if expectedPrefix == "" {
		return nil
	}

	if pkg == expectedPrefix || strings.HasPrefix(pkg, expectedPrefix+".") {
		return nil
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Package declaration '%s' drifts from source path; expected prefix '%s'.", pkg, expectedPrefix))}
}

func packageHeaderNameFlat(file *scanner.File, idx uint32) string {
	if idNode := file.FlatFindChild(idx, "identifier"); idNode != 0 {
		return strings.TrimSpace(file.FlatNodeText(idNode))
	}

	text := strings.TrimSpace(strings.TrimPrefix(file.FlatNodeText(idx), "package "))
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return text
}

func packageNamingConventionExpectedPrefix(filePath string) string {
	path := filepath.ToSlash(filePath)
	idx := strings.LastIndex(path, packageNamingConventionDriftSourceRoot)
	if idx < 0 {
		return ""
	}

	relative := path[idx+len(packageNamingConventionDriftSourceRoot):]
	dir := pathpkg.Dir(relative)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}

	return strings.ReplaceAll(dir, "/", ".")
}
