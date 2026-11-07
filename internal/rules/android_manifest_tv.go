package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// This file ports Lint's AndroidTvDetector, which checks a manifest only when it belongs to a TV app:
// it declares the leanback feature or has an activity in the LEANBACK_LAUNCHER category.

const (
	leanbackFeature  = "android.software.leanback"
	leanbackCategory = "android.intent.category.LEANBACK_LAUNCHER"
	touchscreenName  = "android.hardware.touchscreen"
)

// tvUnsupportedHardwarePrefixes is AndroidTvDetector.UNSUPPORTED_HARDWARE_FEATURES, matched by prefix.
var tvUnsupportedHardwarePrefixes = []string{
	"android.hardware.touchscreen", "android.hardware.faketouch", "android.hardware.telephony",
	"android.hardware.camera", "android.hardware.bluetooth", "android.hardware.nfc",
	"android.hardware.location.gps", "android.hardware.microphone", "android.hardware.sensors",
}

// tvImpliedHardware is AndroidTvDetector.getImpliedUnsupportedHardware.
var tvImpliedHardware = map[string]string{
	"android.permission.BLUETOOTH":              "android.hardware.bluetooth",
	"android.permission.BLUETOOTH_ADMIN":        "android.hardware.bluetooth",
	"android.permission.CAMERA":                 "android.hardware.camera",
	"android.permission.RECORD_AUDIO":           "android.hardware.microphone",
	"android.permission.ACCESS_FINE_LOCATION":   "android.hardware.location.gps",
	"android.permission.CALL_PHONE":             "android.hardware.telephony",
	"android.permission.CALL_PRIVILEGED":        "android.hardware.telephony",
	"android.permission.PROCESS_OUTGOING_CALLS": "android.hardware.telephony",
	"android.permission.READ_SMS":               "android.hardware.telephony",
	"android.permission.RECEIVE_SMS":            "android.hardware.telephony",
	"android.permission.RECEIVE_MMS":            "android.hardware.telephony",
	"android.permission.RECEIVE_WAP_PUSH":       "android.hardware.telephony",
	"android.permission.SEND_SMS":               "android.hardware.telephony",
	"android.permission.WRITE_APN_SETTINGS":     "android.hardware.telephony",
	"android.permission.WRITE_SMS":              "android.hardware.telephony",
}

func tvIsUnsupportedHardware(feature string) bool {
	for _, prefix := range tvUnsupportedHardwarePrefixes {
		if strings.HasPrefix(feature, prefix) {
			return true
		}
	}
	return false
}

// tvManifest is the TV-relevant summary of one manifest.
type tvManifest struct {
	isTV                bool
	hasLauncher         bool
	hasLeanbackFeature  bool
	hasAppBanner        bool
	launchersNoBanner   int
	unsupportedRequired []ManifestUsesFeature // unsupported hardware features that are not marked optional
	allUnsupported      map[string]bool       // every declared unsupported hardware feature name
}

func summarizeTV(m *Manifest) tvManifest {
	var tv tvManifest
	tv.allUnsupported = map[string]bool{}
	if m.Application != nil {
		tv.hasAppBanner = m.Application.HasBanner
		for _, c := range m.Application.Activities {
			if c.Tag != "activity" && c.Tag != "activity-alias" {
				continue
			}
			for _, cat := range c.IntentFilterCategories {
				if cat == leanbackCategory {
					tv.hasLauncher = true
					if !c.HasBanner {
						tv.launchersNoBanner++
					}
					break
				}
			}
		}
	}
	for _, f := range m.UsesFeatures {
		if f.Name == leanbackFeature {
			tv.hasLeanbackFeature = true
		}
		if f.Name != "" && tvIsUnsupportedHardware(f.Name) {
			tv.allUnsupported[f.Name] = true
			// Lint parses android:required as a boolean, so references and placeholders count as false.
			if f.Required == "" || strings.EqualFold(f.Required, "true") {
				tv.unsupportedRequired = append(tv.unsupportedRequired, f)
			}
		}
	}
	tv.isTV = tv.hasLeanbackFeature || tv.hasLauncher
	return tv
}

func tvRootLine(m *Manifest) int {
	if m.RootLine > 0 {
		return m.RootLine
	}
	return 1
}

func (r *MissingLeanbackLauncherManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if tv := summarizeTV(m); tv.isTV && !tv.hasLauncher {
		ctx.Emit(baseFinding(m.Path, tvRootLine(m), r.BaseRule,
			"Expecting an activity to have `android.intent.category.LEANBACK_LAUNCHER` intent filter"))
	}
}

func (r *MissingLeanbackSupportManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if tv := summarizeTV(m); tv.isTV && !tv.hasLeanbackFeature {
		ctx.Emit(baseFinding(m.Path, tvRootLine(m), r.BaseRule,
			"Expecting `<uses-feature android:name=\"android.software.leanback\" android:required=\"false\" />` tag"))
	}
}

func (r *PermissionImpliesUnsupportedHardwareManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	tv := summarizeTV(m)
	if !tv.isTV {
		return
	}
	for _, perm := range m.UsesPermissionEntries {
		feature, implied := tvImpliedHardware[perm.Name]
		if !implied || tv.allUnsupported[feature] {
			continue
		}
		ctx.Emit(baseFinding(m.Path, perm.Line, r.BaseRule,
			"Permission exists without corresponding hardware `<uses-feature android:name=\""+feature+"\" required=\"false\">` tag"))
	}
}
