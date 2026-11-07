package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

// tvUnsupportedFeatures are the hardware features (matched by prefix) that Lint says a TV may not provide.
var tvUnsupportedFeatures = []string{
	"android.hardware.touchscreen", "android.hardware.faketouch", "android.hardware.telephony", "android.hardware.camera",
	"android.hardware.bluetooth", "android.hardware.nfc", "android.hardware.location.gps", "android.hardware.microphone",
	"android.hardware.sensors",
}

const leanbackLauncherCategory = "android.intent.category.LEANBACK_LAUNCHER"

// MissingTvBannerRule mirrors Lint's MissingTvBanner: a Leanback launcher activity without android:banner.
type MissingTvBannerRule struct {
	ManifestBase
	AndroidRule
}

// UnsupportedTvHardwareRule mirrors Lint's UnsupportedTvHardware: a TV-unsupported hardware feature not marked
// android:required="false".
type UnsupportedTvHardwareRule struct {
	ManifestBase
	AndroidRule
}

// ImpliedTouchscreenHardwareRule mirrors Lint's ImpliedTouchscreenHardware: a TV app that never declares the
// touchscreen feature.
type ImpliedTouchscreenHardwareRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *MissingTvBannerRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *UnsupportedTvHardwareRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *ImpliedTouchscreenHardwareRule) Confidence() float64 { return api.ConfidenceHigh }

// tvManifestFacts is what Lint's AndroidTvDetector collects while visiting a manifest.
type tvManifestFacts struct {
	app                         *manifest.Node
	hasLeanbackFeature          bool
	hasLeanbackLauncher         bool
	hasApplicationBanner        bool
	launcherActivitiesNoBanner  int
	allUnsupported              map[string]bool
	unsupportedNotOptionalOrder []*manifest.Node
}

func tvIsUnsupportedFeature(name string) bool {
	for _, prefix := range tvUnsupportedFeatures {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func tvCollect(m *manifest.Manifest) (tvManifestFacts, bool) {
	facts := tvManifestFacts{allUnsupported: map[string]bool{}}
	if m.Root == nil {
		return facts, false
	}
	var walk func(n *manifest.Node)
	walk = func(n *manifest.Node) {
		switch n.Tag {
		case "application":
			facts.app = n
			_, facts.hasApplicationBanner = n.Attr("android:banner")
		case "uses-feature":
			if name, ok := n.Attr("android:name"); ok {
				if tvIsUnsupportedFeature(name.Value) {
					facts.allUnsupported[name.Value] = true
					if req, has := n.Attr("android:required"); !has || strings.EqualFold(req.Value, "true") {
						facts.unsupportedNotOptionalOrder = append(facts.unsupportedNotOptionalOrder, n)
					}
				}
				if name.Value == "android.software.leanback" {
					facts.hasLeanbackFeature = true
				}
			}
		case "activity", "activity-alias":
			if tvHasLeanbackIntentFilter(n) {
				facts.hasLeanbackLauncher = true
				if _, has := n.Attr("android:banner"); !has {
					facts.launcherActivitiesNoBanner++
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(m.Root)
	return facts, facts.hasLeanbackFeature || facts.hasLeanbackLauncher
}

func tvHasLeanbackIntentFilter(n *manifest.Node) bool {
	for _, filter := range n.ChildrenByTag("intent-filter") {
		for _, cat := range filter.ChildrenByTag("category") {
			if cat.AttrValue("android:name") == leanbackLauncherCategory {
				return true
			}
		}
	}
	return false
}

func (r *MissingTvBannerRule) check(ctx *api.Context) {
	facts, isTv := tvCollect(ctx.Manifest)
	if !isTv || facts.hasApplicationBanner || facts.launcherActivitiesNoBanner == 0 || facts.app == nil {
		return
	}
	emitManifestAtName(ctx, facts.app, "Expecting `android:banner` with the `<application>` tag or each Leanback launcher activity")
}

func (r *UnsupportedTvHardwareRule) check(ctx *api.Context) {
	facts, isTv := tvCollect(ctx.Manifest)
	if !isTv {
		return
	}
	const msg = "Expecting `android:required=\"false\"` for this hardware feature that may not be supported by all Android TVs"
	for _, n := range facts.unsupportedNotOptionalOrder {
		if req, ok := n.Attr("android:required"); ok {
			emitManifestAtAttr(ctx, req, msg)
		} else {
			emitManifestAtName(ctx, n, msg)
		}
	}
}

func (r *ImpliedTouchscreenHardwareRule) check(ctx *api.Context) {
	facts, isTv := tvCollect(ctx.Manifest)
	if !isTv || facts.allUnsupported["android.hardware.touchscreen"] {
		return
	}
	emitManifestAtName(ctx, ctx.Manifest.Root, "Hardware feature `android.hardware.touchscreen` not explicitly marked as optional")
}
