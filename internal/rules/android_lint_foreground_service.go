package rules

import (
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

// ForegroundServicePermissionRule mirrors Lint's ForegroundServicePermission: a <service> whose
// foregroundServiceType needs permissions the manifest does not declare.
type ForegroundServicePermissionRule struct {
	ManifestBase
	AndroidRule
}

// QueryAllPackagesPermissionRule mirrors Lint's QueryAllPackagesPermission: a uses-permission for
// QUERY_ALL_PACKAGES in an app that targets Android 11 or later.
type QueryAllPackagesPermissionRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *ForegroundServicePermissionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *QueryAllPackagesPermissionRule) Confidence() float64 { return api.ConfidenceHigh }

const (
	foregroundServiceMinTargetSdk = 34
	queryAllPackagesMinTargetSdk  = 30
	foregroundServicePermission   = "android.permission.FOREGROUND_SERVICE"
	queryAllPackagesPermission    = "android.permission.QUERY_ALL_PACKAGES"
)

// foregroundServiceNeed is what one foregroundServiceType value requires: a specific permission and, for some
// types, any one permission out of a list.
type foregroundServiceNeed struct {
	permission string
	anyOf      []string
}

// foregroundServiceNeeds maps each foregroundServiceType value Lint checks to its required permissions.
var foregroundServiceNeeds = map[string]foregroundServiceNeed{
	"dataSync":        {permission: "android.permission.FOREGROUND_SERVICE_DATA_SYNC"},
	"mediaPlayback":   {permission: "android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK"},
	"phoneCall":       {permission: "android.permission.FOREGROUND_SERVICE_PHONE_CALL"},
	"connectedDevice": {permission: "android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE"},
	"mediaProjection": {permission: "android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION"},
	"remoteMessaging": {permission: "android.permission.FOREGROUND_SERVICE_REMOTE_MESSAGING"},
	"fileManagement":  {permission: "android.permission.FOREGROUND_SERVICE_FILE_MANAGEMENT"},
	"specialUse":      {permission: "android.permission.FOREGROUND_SERVICE_SPECIAL_USE"},
	"location": {
		permission: "android.permission.FOREGROUND_SERVICE_LOCATION",
		anyOf:      []string{"android.permission.ACCESS_COARSE_LOCATION", "android.permission.ACCESS_FINE_LOCATION"},
	},
	"camera": {
		permission: "android.permission.FOREGROUND_SERVICE_CAMERA",
		anyOf:      []string{"android.permission.CAMERA", "android.permission.SYSTEM_CAMERA"},
	},
	"microphone": {
		permission: "android.permission.FOREGROUND_SERVICE_MICROPHONE",
		anyOf: []string{
			"android.permission.CAPTURE_AUDIO_HOTWORD", "android.permission.CAPTURE_AUDIO_OUTPUT",
			"android.permission.CAPTURE_MEDIA_OUTPUT", "android.permission.CAPTURE_TUNER_AUDIO_INPUT",
			"android.permission.CAPTURE_VOICE_COMMUNICATION_OUTPUT", "android.permission.RECORD_AUDIO",
		},
	},
	"health": {
		permission: "android.permission.FOREGROUND_SERVICE_HEALTH",
		anyOf: []string{
			"android.permission.ACTIVITY_RECOGNITION", "android.permission.BODY_SENSORS",
			"android.permission.HIGH_SAMPLING_RATE_SENSORS",
		},
	},
	"systemExempted": {
		permission: "android.permission.FOREGROUND_SERVICE_SYSTEM_EXEMPTED",
		anyOf:      []string{"android.permission.SCHEDULE_EXACT_ALARM", "android.permission.USE_EXACT_ALARM"},
	},
}

// declaredPermissions lists the android:name of every uses-permission child of the node, in document order.
func declaredPermissions(n *manifest.Node) []string {
	var names []string
	for _, p := range n.ChildrenByTag("uses-permission") {
		if a, ok := p.Attr("android:name"); ok {
			names = append(names, a.Value)
		}
	}
	return names
}

// lintAppTargetSdk returns the target SDK Lint applies to constraint-guarded reports, or 0 when it is unknown or the
// module is a library: Lint judges a library by the app that includes it, which krit cannot see.
func lintAppTargetSdk(ctx *api.Context, m *manifest.Manifest) int {
	if !lintPermissionManifestNamespace(m) {
		return 0
	}
	if m.Path != "" {
		if isProjectTestFile(ctx, m.Path) {
			return 0
		}
		info := lookupManifestGradleInfo(m.Path)
		if info.found {
			if !info.isApplication || info.isLibrary || info.isTest {
				return 0
			}
			if info.hasTargetSdk {
				if !info.permissionTargetSdkProven {
					return 0
				}
				return info.targetSdk
			}
			return 0
		}
	}
	return m.TargetSDK
}

var lintPermissionTargetSdkLine = lazyre.New(`(?m)^\s*targetSdk(?:Version)?\b\s*(?:=\s*[0-9]+|\(\s*[0-9]+\s*\)|[0-9]+)\s*;?\s*(?://[^\n]*)?$`)

func lintPermissionLiteralTargetSdk(content string) bool {
	if strings.Count(content, "targetSdk") != 1 {
		return false
	}
	if strings.Contains(content, "/*") || strings.Contains(content, `"""`) || strings.Contains(content, "'''") {
		return false
	}
	return lintPermissionTargetSdkLine.MatchString(content)
}

func lintPermissionManifestNamespace(parsedManifest *manifest.Manifest) bool {
	namespace, ok := parsedManifest.Root.Attr("xmlns:android")
	if !ok || namespace.Value != "http://schemas.android.com/apk/res/android" {
		return false
	}
	valid := true
	manifestWalk(parsedManifest.Root, func(node, _ *manifest.Node) {
		for _, attribute := range node.Attrs {
			if attribute.Name == "xmlns:android" && attribute.Value != namespace.Value {
				valid = false
			}
			if strings.HasPrefix(attribute.Name, "xmlns:") && attribute.Name != "xmlns:android" && attribute.Value == namespace.Value {
				valid = false
			}
		}
	})
	return valid
}

func (r *ForegroundServicePermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || lintAppTargetSdk(ctx, m) < foregroundServiceMinTargetSdk {
		return
	}
	declared := declaredPermissions(m.Root)
	if !slices.Contains(declared, foregroundServicePermission) {
		return
	}
	for _, container := range m.Root.Children {
		for _, service := range container.ChildrenByTag("service") {
			checkForegroundServiceTypes(ctx, service, declared)
		}
	}
}

// checkForegroundServiceTypes reports each type listed in the service's foregroundServiceType whose permissions are
// not all declared.
func checkForegroundServiceTypes(ctx *api.Context, service *manifest.Node, declared []string) {
	types, ok := service.Attr("android:foregroundServiceType")
	if !ok {
		return
	}
	for _, serviceType := range strings.Split(types.Value, "|") {
		need, known := foregroundServiceNeeds[serviceType]
		if !known {
			continue
		}
		hasAny := need.anyOf == nil
		for _, name := range declared {
			if slices.Contains(need.anyOf, name) {
				hasAny = true
			}
		}
		if slices.Contains(declared, need.permission) && hasAny {
			continue
		}
		msg := "foregroundServiceType:" + serviceType + " requires permission:[" + need.permission + "]"
		if need.anyOf != nil {
			msg += " AND any permission in list:[" + strings.Join(need.anyOf, ", ") + "]"
		}
		emitManifestAtNode(ctx, service, msg)
	}
}

func (r *QueryAllPackagesPermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || lintAppTargetSdk(ctx, m) < queryAllPackagesMinTargetSdk {
		return
	}
	manifestWalk(m.Root, func(n, _ *manifest.Node) {
		if n.Tag != "uses-permission" {
			return
		}
		if a, ok := n.Attr("android:name"); ok && a.Value == queryAllPackagesPermission {
			emitManifestAtAttr(ctx, a,
				"A `<queries>` declaration should generally be used instead of QUERY_ALL_PACKAGES; see https://g.co/dev/packagevisibility for details")
		}
	})
}
