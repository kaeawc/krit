package android

import (
	"os"
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

// SourceSetAssetDirs lists the assets directories UnsafeNativeCodeLocation reads for resDir: for a
// <module>/src/<set>/res directory, the assets directory of every source set of the module (src/main/assets,
// src/debug/assets, src/<flavor>/assets, ...), test source sets excepted; for any other layout, the assets directory
// beside resDir. Directories that do not exist are left out.
func SourceSetAssetDirs(resDir string) []string {
	setDir := filepath.Dir(resDir)
	srcDir := filepath.Dir(setDir)
	if filepath.Base(resDir) != "res" || filepath.Base(srcDir) != "src" {
		own := filepath.Join(setDir, "assets")
		if info, err := os.Stat(own); err == nil && info.IsDir() {
			return []string{own}
		}
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(srcDir, "*", "assets"))
	var out []string
	for _, dir := range matches {
		if scanner.IsTestSourceSetName(filepath.Base(filepath.Dir(dir))) {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, dir)
		}
	}
	sort.Strings(out)
	return out
}
