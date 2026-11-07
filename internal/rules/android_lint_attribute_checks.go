package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// InvalidManifestAttributeRule mirrors Lint's InvalidManifestAttribute: <activity-alias> attributes the platform ignores.
type InvalidManifestAttributeRule struct {
	ManifestBase
	AndroidRule
}

// EllipsizeMaxLinesRule mirrors Lint's EllipsizeMaxLines: ellipsize combined with a single line below API 23.
type EllipsizeMaxLinesRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal attributes.
func (r *InvalidManifestAttributeRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal attributes.
func (r *EllipsizeMaxLinesRule) Confidence() float64 { return api.ConfidenceHigh }

// aliasIgnoredAttributes are the <activity> attributes that <activity-alias> silently ignores.
var aliasIgnoredAttributes = map[string]bool{
	"enableVrMode": true, "configChanges": true, "uiOptions": true, "directBootAware": true, "forceQueryable": true,
	"maxRecents": true, "preferMinimalPostProcessing": true, "noHistory": true, "alwaysRetainTaskState": true,
	"lockTaskMode": true, "relinquishTaskIdentity": true, "persistableMode": true, "allowEmbedded": true,
	"finishOnTaskLaunch": true, "multiprocess": true, "windowSoftInputMode": true, "canDisplayOnRemoteDevices": true,
	"colorMode": true, "resumeWhilePausing": true, "allowTaskReparenting": true, "showOnLockScreen": true,
	"autoRemoveFromRecents": true, "documentLaunchMode": true, "visibleToInstantApps": true, "requiredDisplayCategory": true,
	"stateNotNeeded": true, "singleUser": true, "recreateOnConfigChanges": true, "rotationAnimation": true,
	"maxAspectRatio": true, "splitName": true, "screenOrientation": true, "hardwareAccelerated": true,
	"supportsPictureInPicture": true, "theme": true, "requireContentUriPermissionFromCaller": true, "resizeableActivity": true,
	"process": true, "clearTaskOnLaunch": true, "minAspectRatio": true, "playHomeTransitionSound": true, "alwaysFocusable": true,
	"enableOnBackInvokedCallback": true, "inheritShowWhenLocked": true, "showWhenLocked": true, "finishOnCloseSystemDialogs": true,
	"immersive": true, "showForAllUsers": true, "systemUserOnly": true, "launchMode": true, "excludeFromRecents": true,
	"turnScreenOn": true, "taskAffinity": true,
}

func (r *InvalidManifestAttributeRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "activity-alias" {
			return
		}
		aliasName := n.AttrValue("android:name")
		if aliasName == "" {
			return
		}
		for _, a := range n.Attrs {
			local, ok := strings.CutPrefix(a.Name, "android:")
			if ok && local != "" && aliasIgnoredAttributes[local] {
				emitManifestAtAttr(ctx, a, "Attribute `"+local+"` on `<activity-alias>` `"+aliasName+
					"` is invalid, and will be silently ignored. This attribute is always ignored on `<activity-alias>`.")
			}
		}
	})
}

func (r *EllipsizeMaxLinesRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	if facts != nil && facts.Profile.MinSdkVersion >= 23 {
		return
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"EllipsizeMaxLines"}, func(v, _ *android.View) {
			ellipsize, ok := v.Attributes["android:ellipsize"]
			if !ok || ellipsize == "end" {
				return
			}
			lines, hasLines := v.Attributes["android:lines"]
			maxLines, hasMax := v.Attributes["android:maxLines"]
			if (!hasLines || lines != "1") && (!hasMax || maxLines != "1") {
				return
			}
			attr, value := "android:maxLines", maxLines
			if hasLines {
				attr, value = "android:lines", lines
			}
			line, col := v.PosOf(attr)
			ctx.Emit(scanner.Finding{File: layout.FilePath, Line: line, Col: col, Message: "Combining `ellipsize=" + ellipsize +
				"` and `" + strings.TrimPrefix(attr, "android:") + "=" + value + "` can lead to crashes. Use `singleLine=true` instead."})
		})
	}
}
