package rules

import (
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

const (
	permUseExactAlarm           = "android.permission.USE_EXACT_ALARM"
	permReadMediaImages         = "android.permission.READ_MEDIA_IMAGES"
	permReadMediaVideo          = "android.permission.READ_MEDIA_VIDEO"
	permReadMediaVisualSelected = "android.permission.READ_MEDIA_VISUAL_USER_SELECTED"
)

// ExactAlarmRule mirrors Lint's ExactAlarm: USE_EXACT_ALARM declared while targeting below API 33.
type ExactAlarmRule struct {
	ManifestBase
	AndroidRule
}

// SelectedPhotoAccessRule mirrors Lint's SelectedPhotoAccess: READ_MEDIA_IMAGES or READ_MEDIA_VIDEO without
// READ_MEDIA_VISUAL_USER_SELECTED in an app targeting API 33 or higher.
type SelectedPhotoAccessRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *ExactAlarmRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *SelectedPhotoAccessRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *ExactAlarmRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestTargetSdk(m) >= 33 {
		return
	}
	var walk func(n *manifest.Node)
	walk = func(n *manifest.Node) {
		if n.Tag == "uses-permission" {
			if a, ok := n.Attr("android:name"); ok && a.Value == permUseExactAlarm {
				emitManifestAtValue(ctx, a, "`USE_EXACT_ALARM` can only be used when targeting API level 33 or higher")
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(m.Root)
}

func (r *SelectedPhotoAccessRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestIsLibrary(m) || manifestTargetSdk(m) < 33 {
		return
	}
	var found []manifest.Attr
	for _, n := range m.Root.ChildrenByTag("uses-permission") {
		a, ok := n.Attr("android:name")
		if !ok {
			continue
		}
		switch a.Value {
		case permReadMediaVisualSelected:
			return
		case permReadMediaImages, permReadMediaVideo:
			found = append(found, a)
		}
	}
	for _, a := range found {
		emitManifestAtValue(ctx, a, "Your app is currently not handling Selected Photos Access introduced in Android 14+")
	}
}
