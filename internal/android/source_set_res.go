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

// ManifestModuleResDirs lists the res directories merged into the module of an AndroidManifest.xml: the res of every
// non-test source set for <module>/src/<set>/AndroidManifest.xml, or <dir>/res for a manifest outside src/ (a flat
// layout). The directories need not exist, so a caller that fingerprints them notices one being created.
func ManifestModuleResDirs(manifestPath string) []string {
	dir := filepath.Dir(manifestPath)
	src := filepath.Dir(dir)
	if filepath.Base(src) != "src" {
		return []string{filepath.Join(dir, "res")}
	}
	sets, err := os.ReadDir(src)
	if err != nil {
		return []string{filepath.Join(dir, "res")}
	}
	var out []string
	for _, set := range sets {
		if set.IsDir() && !scanner.IsTestSourceSetName(set.Name()) {
			out = append(out, filepath.Join(src, set.Name(), "res"))
		}
	}
	sort.Strings(out)
	return out
}
