package parity_test

import "testing"

// DenyListedApi reports View.setOnClickListener(null) on a View subclass declared in another file; editing only that
// file's supertype must not leave a stale finding (or a stale miss) in the warm run.
func TestDenyListedApiFollowsOtherFilesAcrossWarmRuns(t *testing.T) {
	const (
		badge    = "package p\n\nimport android.content.Context\nimport android.view.View\n\nopen class BadgeView(c: Context) : View(c)\n"
		notBadge = "package p\n\nimport android.content.Context\n\nopen class BadgeView(c: Context) {\n  fun setOnClickListener(l: Any?) {}\n}\n"
	)
	checkSlackWarmMatchesNoCache(t, map[string]string{
		"BadgeView.kt": badge,
		"Use.kt":       "package p\n\nfun clear(badge: BadgeView) {\n  badge.setOnClickListener(null)\n}\n",
	}, []string{"DenyListedApi"}, []slackCacheEdit{
		{"BadgeView stops extending View", "BadgeView.kt", notBadge},
		{"BadgeView extends View again", "BadgeView.kt", badge},
	})
}
