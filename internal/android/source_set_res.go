package android

import (
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
)

// SiblingSourceSetResDirs lists the res directories of the other source sets of the module that owns resDir, such as
// src/debug/res next to src/main/res. Test source sets are left out because their resources are not merged into the
// app. It returns nil when resDir is not a <module>/src/<set>/res directory.
func SiblingSourceSetResDirs(resDir string) []string {
	setDir := filepath.Dir(resDir)
	srcDir := filepath.Dir(setDir)
	if filepath.Base(resDir) != "res" || filepath.Base(srcDir) != "src" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(srcDir, "*", "res"))
	var out []string
	for _, dir := range matches {
		if dir == resDir || scanner.IsTestSourceSetName(filepath.Base(filepath.Dir(dir))) {
			continue
		}
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
