package parity_test

import "testing"

// DoNotCallViewToString and InflationInItemDecoration prove their classes through hierarchies declared in other
// files; editing only those files must keep the warm run equal to --no-cache.
func TestSlackViewChecksFollowOtherFilesAcrossWarmRuns(t *testing.T) {
	const (
		badge         = "package p\n\nimport android.content.Context\nimport android.view.View\n\nopen class BadgeView(c: Context) : View(c) {\n  override fun toString(): String = \"badge\"\n}\n"
		badgePlain    = "package p\n\nimport android.content.Context\n\nopen class BadgeView(c: Context) {\n  override fun toString(): String = \"badge\"\n}\n"
		decoration    = "package p\n\nimport androidx.recyclerview.widget.RecyclerView\n\nopen class BaseDecoration : RecyclerView.ItemDecoration()\n"
		notDecoration = "package p\n\nopen class BaseDecoration\n"
	)
	checkSlackWarmMatchesNoCache(t, map[string]string{
		"BadgeView.kt":      badge,
		"BaseDecoration.kt": decoration,
		"Use.kt":            "package p\n\nfun describe(badge: BadgeView): String = badge.toString()\n",
		"Header.kt": "package p\n\nimport android.view.LayoutInflater\nimport android.view.ViewGroup\n\n" +
			"class Header : BaseDecoration() {\n  fun draw(parent: ViewGroup) = LayoutInflater.from(parent.context).inflate(1, parent, false)\n}\n",
	}, []string{"DoNotCallViewToString", "InflationInItemDecoration"}, []slackCacheEdit{
		{"BadgeView stops extending View", "BadgeView.kt", badgePlain},
		{"BadgeView extends View again", "BadgeView.kt", badge},
		{"BaseDecoration stops extending ItemDecoration", "BaseDecoration.kt", notDecoration},
		{"BaseDecoration extends ItemDecoration again", "BaseDecoration.kt", decoration},
	})
}
