package rules

import (
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// projectPath returns path as a path heuristic should read it: relative to
// the scanned project's root, slash-separated with a leading slash
// ("/app/src/main/Foo.kt", "/" for the root itself), when the run recorded a
// root and path lies under it; otherwise path unchanged. A heuristic that
// reads directory names then cannot see the folders above the checkout,
// which differ between machines and CI agents, so a verdict, and the
// per-file cache entry holding it, stays valid when the checkout moves
// (#1814).
func projectPath(ctx *api.Context, path string) string {
	if ctx == nil {
		return path
	}
	root := ctx.Facts.ProjectRoot()
	if root == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}
