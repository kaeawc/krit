package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

// UsingC2DMRule mirrors Lint's UsingC2DM: a C2DM or GCM broadcast receiver registered for both C2DM actions.
type UsingC2DMRule struct {
	ManifestBase
	AndroidRule
}

// InvalidPermissionRule mirrors Lint's InvalidPermission: a permission attribute on an element that ignores it.
type InvalidPermissionRule struct {
	ManifestBase
	AndroidRule
}

// CoarseFineLocationRule mirrors Lint's CoarseFineLocation: ACCESS_FINE_LOCATION without ACCESS_COARSE_LOCATION when
// targeting Android 12 or higher.
type CoarseFineLocationRule struct {
	ManifestBase
	AndroidRule
}

// LeanbackUsesWifiRule mirrors Lint's LeanbackUsesWifi: a TV app that requires Wi-Fi.
type LeanbackUsesWifiRule struct {
	ManifestBase
	AndroidRule
}

// UnnecessaryRequiredFeatureRule mirrors Lint's UnnecessaryRequiredFeature: uses-feature entries for hardware that apps
// usually do not need to require.
type UnnecessaryRequiredFeatureRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *UsingC2DMRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *InvalidPermissionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *CoarseFineLocationRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *LeanbackUsesWifiRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *UnnecessaryRequiredFeatureRule) Confidence() float64 { return api.ConfidenceHigh }

func manifestWalk(root *manifest.Node, fn func(n, parent *manifest.Node)) {
	var walk func(n, parent *manifest.Node)
	walk = func(n, parent *manifest.Node) {
		fn(n, parent)
		for _, c := range n.Children {
			walk(c, n)
		}
	}
	if root != nil {
		walk(root, nil)
	}
}

func (r *UsingC2DMRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "receiver" {
			return
		}
		name, ok := n.Attr("android:name")
		if !ok || (name.Value != "com.google.android.c2dm.C2DMBroadcastReceiver" && name.Value != "com.google.android.gcm.GCMBroadcastReceiver") {
			return
		}
		var receive, registration bool
		for _, filter := range n.ChildrenByTag("intent-filter") {
			for _, action := range filter.ChildrenByTag("action") {
				switch action.AttrValue("android:name") {
				case "com.google.android.c2dm.intent.RECEIVE":
					receive = true
				case "com.google.android.c2dm.intent.REGISTRATION":
					registration = true
				}
			}
		}
		if receive && registration {
			emitManifestAtValue(ctx, name, "The C2DM library does not work on Android P or newer devices; you should migrate to Firebase Cloud Messaging to ensure reliable message delivery")
		}
	})
}

var permissionAttrOwners = map[string]bool{
	"activity": true, "application": true, "provider": true, "service": true, "receiver": true, "activity-alias": true, "path-permission": true,
}

func (r *InvalidPermissionRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if permissionAttrOwners[n.Tag] {
			return
		}
		for _, a := range n.Attrs {
			if a.Name == "permission" || strings.HasSuffix(a.Name, ":permission") {
				emitManifestAtAttr(ctx, a, "Protecting an unsupported element with a permission is a no-op and potentially dangerous")
			}
		}
	})
}

// locationPermissionApplies is true unless the permission is capped below Android 12.
func locationPermissionApplies(n *manifest.Node) bool {
	maxSdk := 1<<31 - 1
	if v, err := strconv.Atoi(n.AttrValue("android:maxSdkVersion")); err == nil {
		maxSdk = v
	}
	return maxSdk >= 31
}

func (r *CoarseFineLocationRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestTargetSdk(m) < 31 {
		return
	}
	var fine, coarse *manifest.Node
	for _, n := range m.Root.ChildrenByTag("uses-permission") {
		switch n.AttrValue("android:name") {
		case "android.permission.ACCESS_FINE_LOCATION":
			if locationPermissionApplies(n) {
				fine = n
			}
		case "android.permission.ACCESS_COARSE_LOCATION":
			if locationPermissionApplies(n) {
				coarse = n
			}
		}
	}
	if fine != nil && coarse == nil {
		emitManifestAtNode(ctx, fine, "If you need access to FINE location, you must request both `ACCESS_FINE_LOCATION` and `ACCESS_COARSE_LOCATION`")
	}
}

func (r *LeanbackUsesWifiRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil {
		return
	}
	var wifiFeature, wifiPermission *manifest.Node
	hasLeanback := false
	for _, n := range m.Root.Children {
		switch n.Tag {
		case "uses-feature":
			switch n.AttrValue("android:name") {
			case "android.software.leanback":
				hasLeanback = true
			case "android.hardware.wifi":
				wifiFeature = n
			}
		case "uses-permission":
			switch n.AttrValue("android:name") {
			case "android.permission.CHANGE_WIFI_STATE", "android.permission.CHANGE_WIFI_MULTICAST_STATE", "android.permission.ACCESS_WIFI_STATE":
				wifiPermission = n
			}
		}
	}
	if !hasLeanback {
		return
	}
	switch {
	case wifiFeature != nil:
		if wifiFeature.AttrValue("android:required") != "false" {
			emitManifestAtNode(ctx, wifiFeature, "Requiring `android.hardware.wifi` limits app availability on TVs that support only Ethernet")
		}
	case wifiPermission != nil:
		emitManifestAtNode(ctx, wifiPermission, "Requiring Wifi permissions limits app availability on TVs that support only Ethernet")
	}
}

var unnecessaryRequiredFeatures = map[string]bool{
	"android.hardware.location.gps": true, "android.hardware.camera.autofocus": true, "android.hardware.nfc.hce": true,
	"android.hardware.touchscreen": true, "android.hardware.screen.portrait": true, "android.hardware.touchscreen.multitouch": true,
	"android.hardware.nfc": true, "android.hardware.telephony": true, "android.hardware.camera.flash": true,
}

func (r *UnnecessaryRequiredFeatureRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "uses-feature" || n.AttrValue("android:required") == "false" {
			return
		}
		name, ok := n.Attr("android:name")
		if !ok || !unnecessaryRequiredFeatures[name.Value] {
			return
		}
		emitManifestAtValue(ctx, name, "Consider whether this feature (`"+name.Value+
			"`) really is required for the app to function; you can set `android:required=\"false\"` to indicate that the feature is used but not required")
	})
}
