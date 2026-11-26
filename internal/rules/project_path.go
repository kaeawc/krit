package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
	return scanner.RootRelativePath(projectRoot(ctx), path)
}

// projectRoot returns the project root the run recorded, or "".
func projectRoot(ctx *api.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.Facts.ProjectRoot()
}

// isProjectTestFile is scanner.IsTestFile(projectPath(ctx, path)), except
// that a configured test marker spelled as an absolute path inside the
// project root still matches (scanner.IsTestFileIn).
func isProjectTestFile(ctx *api.Context, path string) bool {
	return scanner.IsTestFileIn(projectRoot(ctx), path)
}

// projectPathMarkers returns configured path markers ready to match a
// projectPath value: a marker spelled as an absolute path inside the
// project root becomes root-relative ("/work/repo/tools/" is "/tools/"),
// so it keeps matching under a marked root; the others are unchanged and
// match only folders inside the project. markers itself is returned when
// nothing changes.
func projectPathMarkers(ctx *api.Context, markers []string) []string {
	root := projectRoot(ctx)
	if root == "" {
		return markers
	}
	var out []string
	for i, m := range markers {
		rel := scanner.RootRelativeMarker(root, m)
		if rel == m && out == nil {
			continue
		}
		if out == nil {
			out = append(make([]string, 0, len(markers)), markers[:i]...)
		}
		out = append(out, rel)
	}
	if out == nil {
		return markers
	}
	return out
}
