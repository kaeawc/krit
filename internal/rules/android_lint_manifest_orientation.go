package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

// LockedOrientationActivityRule mirrors Lint's LockedOrientationActivity: an activity locked to portrait.
type LockedOrientationActivityRule struct {
	ManifestBase
	AndroidRule
}

// NonResizeableActivityRule mirrors Lint's NonResizeableActivity: an activity with android:resizeableActivity not true.
type NonResizeableActivityRule struct {
	ManifestBase
	AndroidRule
}

// IntentFilterUniqueDataAttributesRule mirrors Lint's IntentFilterUniqueDataAttributes: a <data> tag that mixes
// scheme, host and path attributes inside an intent filter with several <data> tags.
type IntentFilterUniqueDataAttributesRule struct {
	ManifestBase
	AndroidRule
}

// DiscouragedAPIRule mirrors the manifest attribute part of Lint's DiscouragedApi: orientation, resizability and
// aspect-ratio attributes that Android 16 ignores on large screens.
type DiscouragedAPIRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *LockedOrientationActivityRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *NonResizeableActivityRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *IntentFilterUniqueDataAttributesRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *DiscouragedAPIRule) Confidence() float64 { return api.ConfidenceHigh }

// lockedPortraitOrientations are the screenOrientation values Lint treats as locked.
var lockedPortraitOrientations = map[string]bool{
	"portrait": true, "reversePortrait": true, "sensorPortrait": true, "userPortrait": true,
}

// lastApplication returns the last <application> child of the manifest root, which Lint uses.
func lastApplication(root *manifest.Node) *manifest.Node {
	var app *manifest.Node
	for _, n := range root.ChildrenByTag("application") {
		app = n
	}
	return app
}

// emitActivityAttrOrName reports at the attribute when present and at the tag name otherwise.
func emitActivityAttrOrName(ctx *api.Context, activity *manifest.Node, attr, msg string) {
	if a, ok := activity.Attr(attr); ok {
		emitManifestAtAttr(ctx, a, msg)
		return
	}
	emitManifestAtName(ctx, activity, msg)
}

// reportActivitiesNamed reports every activity under the last <application> whose name was collected by a
// qualifying activity, as Lint's findActivityElements does.
func reportActivitiesNamed(ctx *api.Context, names map[string]bool, attr, msg string) {
	if len(names) == 0 {
		return
	}
	app := lastApplication(ctx.Manifest.Root)
	for _, activity := range app.ChildrenByTag("activity") {
		if names[activity.AttrValue("android:name")] {
			emitActivityAttrOrName(ctx, activity, attr, msg)
		}
	}
}

// activityNamesWhere collects the names of activities anywhere in the manifest that satisfy pred.
func activityNamesWhere(root *manifest.Node, pred func(*manifest.Node) bool) map[string]bool {
	names := map[string]bool{}
	manifestWalk(root, func(n, _ *manifest.Node) {
		if n.Tag == "activity" && pred(n) {
			names[n.AttrValue("android:name")] = true
		}
	})
	return names
}

func (r *LockedOrientationActivityRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestIsLibrary(m) {
		return
	}
	names := activityNamesWhere(m.Root, func(n *manifest.Node) bool {
		a, ok := n.Attr("android:screenOrientation")
		return ok && lockedPortraitOrientations[a.Value]
	})
	reportActivitiesNamed(ctx, names, "android:screenOrientation",
		"Expecting `android:screenOrientation=\"unspecified\"` or `\"fullSensor\"` for this activity so the user can use the application in any orientation and provide a great experience on Chrome OS devices")
}

func (r *NonResizeableActivityRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestIsLibrary(m) {
		return
	}
	names := activityNamesWhere(m.Root, func(n *manifest.Node) bool {
		a, ok := n.Attr("android:resizeableActivity")
		return ok && !strings.EqualFold(a.Value, "true")
	})
	reportActivitiesNamed(ctx, names, "android:resizeableActivity",
		"Expecting `android:resizeableActivity=\"true\"` for this activity so the user can take advantage of the multi-window environment on all devices. NOTE: beginning with Android 12, all apps run in multi-window mode on large screen devices.")
}

// dataTagPieces counts the independent URL parts a <data> tag declares: scheme, host with its port, and each path form.
func dataTagPieces(data *manifest.Node) int {
	pieces := 0
	for _, attr := range []string{"scheme", "host"} {
		if _, ok := data.Attr("android:" + attr); ok {
			pieces++
		}
	}
	for _, attr := range []string{"path", "pathPrefix", "pathPattern", "pathAdvancedPattern", "pathSuffix"} {
		if _, ok := data.Attr("android:" + attr); ok {
			pieces++
		}
	}
	return pieces
}

func (r *IntentFilterUniqueDataAttributesRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "intent-filter" {
			return
		}
		data := n.ChildrenByTag("data")
		if len(data) <= 1 {
			return
		}
		for _, d := range data {
			if dataTagPieces(d) > 1 {
				emitManifestAtNode(ctx, d, "Consider splitting data tag into multiple tags with individual attributes to avoid confusion")
			}
		}
	})
}

const discouragedAdaptiveSuffix = " will be ignored in most cases, starting from Android 16. " +
	"Android is moving toward a model where apps are expected to adapt to various orientations, display sizes, and aspect ratios."

func (r *DiscouragedAPIRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		for _, a := range n.Attrs {
			switch a.Name {
			case "android:minAspectRatio", "android:maxAspectRatio":
				emitManifestAtAttr(ctx, a, "Minimum and maximum aspect ratios"+discouragedAdaptiveSuffix)
			case "android:screenOrientation":
				emitManifestAtAttr(ctx, a, "Fixed screen orientations"+discouragedAdaptiveSuffix)
			case "android:resizeableActivity":
				if a.Value == "false" {
					emitManifestAtAttr(ctx, a, "Setting `resizeableActivity` to `false`"+discouragedAdaptiveSuffix)
				}
			}
		}
	})
}
