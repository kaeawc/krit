package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SignatureOrSystemPermissionsRule mirrors Lint's SignatureOrSystemPermissions: protectionLevel="signatureOrSystem".
type SignatureOrSystemPermissionsRule struct {
	ManifestBase
	AndroidRule
}

// HighSamplingRateRule mirrors Lint's HighSamplingRate: the HIGH_SAMPLING_RATE_SENSORS permission when targeting API 31+.
type HighSamplingRateRule struct {
	ManifestBase
	AndroidRule
}

// KeyboardInaccessibleWidgetRule mirrors Lint's KeyboardInaccessibleWidget: clickable views that are not focusable
// when the minimum SDK is below 26.
type KeyboardInaccessibleWidgetRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal attributes.
func (r *SignatureOrSystemPermissionsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal attributes.
func (r *HighSamplingRateRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal attributes.
func (r *KeyboardInaccessibleWidgetRule) Confidence() float64 { return api.ConfidenceHigh }

// manifestAttrsLocal visits every attribute in the manifest whose name, without prefix, is local.
func manifestAttrsLocal(root *manifest.Node, local string, fn func(*manifest.Node, manifest.Attr)) {
	if root == nil {
		return
	}
	for _, a := range root.Attrs {
		name := a.Name
		if i := strings.LastIndex(name, ":"); i >= 0 {
			name = name[i+1:]
		}
		if name == local {
			fn(root, a)
		}
	}
	for _, c := range root.Children {
		manifestAttrsLocal(c, local, fn)
	}
}

func (r *SignatureOrSystemPermissionsRule) check(ctx *api.Context) {
	manifestAttrsLocal(ctx.Manifest.Root, "protectionLevel", func(_ *manifest.Node, a manifest.Attr) {
		if a.Value == "signatureOrSystem" {
			emitManifestAtAttr(ctx, a, "`protectionLevel` should probably not be set to `signatureOrSystem`")
		}
	})
}

func (r *HighSamplingRateRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestTargetSdk(m) < 31 {
		return
	}
	var walk func(n *manifest.Node)
	walk = func(n *manifest.Node) {
		if n.Tag == "uses-permission" {
			if a, ok := n.Attr("android:name"); ok && a.Value == "android.permission.HIGH_SAMPLING_RATE_SENSORS" {
				emitManifestAtValue(ctx, a, "Most apps don't need access to high sensor sampling rate.")
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(m.Root)
}

func (r *KeyboardInaccessibleWidgetRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	projectMin := 0
	if facts != nil {
		projectMin = facts.Profile.MinSdkVersion
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		// Clickable views are focusable by default from API 26, so Lint reports only when minSdk is lower.
		minSdk := projectMin
		if levels := android.ResolveSDKLevels(layout.FilePath); levels.MinSdk > 0 {
			minSdk = levels.MinSdk
		}
		if minSdk >= 26 {
			continue
		}
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"KeyboardInaccessibleWidget"}, func(v, _ *android.View) {
			if v.Attributes["android:clickable"] != "true" || v.Attributes["android:focusable"] == "true" {
				return
			}
			line, col := v.PosOf("android:clickable")
			ctx.Emit(scanner.Finding{File: layout.FilePath, Line: line, Col: col, Message: "'clickable' attribute found, please also add 'focusable'"})
		})
	}
}
