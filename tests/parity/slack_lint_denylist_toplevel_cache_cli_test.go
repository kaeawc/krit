package parity_test

import "testing"

// DenyListedApi lets a top-level runCatching of the file's package shadow kotlin's, but not a member function of the
// same name (#1888). Editing only the other file between the two must keep the warm run equal to --no-cache.
func TestDenyListedApiPackageShadowFollowsOtherFilesAcrossWarmRuns(t *testing.T) {
	const (
		topLevel = "package p\n\nfun <T> runCatching(block: () -> T): T = block()\n\nclass Svc\n"
		member   = "package p\n\nclass Svc {\n  fun <T> runCatching(block: () -> T): T = block()\n}\n"
	)
	checkSlackWarmMatchesNoCache(t, map[string]string{
		"Fns.kt": member,
		"Use.kt": "package p\n\nfun f() = runCatching { 1 }\n",
	}, []string{"DenyListedApi"}, []slackCacheEdit{
		{"runCatching becomes top-level", "Fns.kt", topLevel},
		{"runCatching becomes a member again", "Fns.kt", member},
	})
}
