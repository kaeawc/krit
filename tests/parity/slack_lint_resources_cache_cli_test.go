package parity_test

import "testing"

// ArgInFormattedQuantityStringRes proves a Resources receiver through a subclass declared in another file, resolving
// its supertype through that file's imports (#1857). Editing only that file, including only its import, must keep
// the warm run equal to --no-cache.
func TestArgInFormattedQuantityStringResFollowsOtherFilesAcrossWarmRuns(t *testing.T) {
	const (
		android   = "package p\n\nimport android.content.res.AssetManager\nimport android.content.res.Configuration\nimport android.content.res.Resources\nimport android.util.DisplayMetrics\n\nopen class MyRes(a: AssetManager, d: DisplayMetrics, c: Configuration) : Resources(a, d, c)\n"
		lookalike = "package p\n\nimport android.content.res.AssetManager\nimport android.content.res.Configuration\nimport q.Resources\nimport android.util.DisplayMetrics\n\nopen class MyRes(a: AssetManager, d: DisplayMetrics, c: Configuration) : Resources(a, d, c)\n"
		plain     = "package p\n\nopen class MyRes\n"
	)
	checkSlackWarmMatchesNoCache(t, map[string]string{
		"MyRes.kt":      android,
		"QResources.kt": "package q\n\nopen class Resources(a: Any, d: Any, c: Any) {\n  fun getQuantityString(id: Int, n: Int, vararg args: Any): String = \"\"\n}\n",
		"Use.kt":        "package p\n\nfun count(r: MyRes, n: Int) = r.getQuantityString(1, n, n)\n",
	}, []string{"ArgInFormattedQuantityStringRes"}, []slackCacheEdit{
		{"MyRes imports a lookalike Resources", "MyRes.kt", lookalike},
		{"MyRes imports Android's Resources again", "MyRes.kt", android},
		{"MyRes stops extending Resources", "MyRes.kt", plain},
		{"MyRes extends Resources again", "MyRes.kt", android},
	})
}
