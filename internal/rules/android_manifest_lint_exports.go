package rules

// Manifest checks of Lint's SecurityDetector: ExportedReceiver,
// ExportedService and ExportedContentProvider. GrantAllUris is in android_manifest_security.go.

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ExportedReceiverManifestRule reports exported receivers without a permission.
type ExportedReceiverManifestRule struct {
	ManifestBase
	AndroidRule
}

// ExportedContentProviderManifestRule reports exported providers without any permission.
type ExportedContentProviderManifestRule struct {
	ManifestBase
	AndroidRule
}

// explicitExported returns the parsed android:exported value, or nil when unset.
func explicitExported(n *android.XMLNode) *bool {
	raw := xmlAttrValue(n, "android:exported")
	if raw == "" {
		return nil
	}
	v := strings.EqualFold(raw, "true")
	return &v
}

func hasIntentFilterChild(n *android.XMLNode) bool {
	return len(xmlChildrenNamed(n, "intent-filter")) > 0
}

// unprotectedByPermission mirrors SecurityDetector.isUnprotectedByPermission.
func unprotectedByPermission(n *android.XMLNode, app *android.XMLNode) bool {
	perm := xmlAttrValue(n, "android:permission")
	if perm == "TODO" {
		return true
	}
	return perm == "" && app != nil && xmlAttrValue(app, "android:permission") == ""
}

func firstActionOf(n *android.XMLNode) (string, bool) {
	for _, filter := range xmlChildrenNamed(n, "intent-filter") {
		for _, action := range xmlChildrenNamed(filter, "action") {
			return xmlAttrValue(action, "android:name"), true
		}
	}
	return "", false
}

func isStandardReceiver(n *android.XMLNode) bool {
	if xmlAttrValue(n, "android:name") == "com.google.android.gms.tagmanager.InstallReferrerReceiver" {
		return true
	}
	action, ok := firstActionOf(n)
	return ok && strings.HasPrefix(action, "android.")
}

var wearableListenerActions = map[string]bool{
	"com.google.android.gms.wearable.BIND_LISTENER":      true,
	"com.google.android.gms.wearable.DATA_CHANGED":       true,
	"com.google.android.gms.wearable.MESSAGE_RECEIVED":   true,
	"com.google.android.gms.wearable.CAPABILITY_CHANGED": true,
	"com.google.android.gms.wearable.CHANNEL_EVENT":      true,
}

func isWearableListenerService(n *android.XMLNode) bool {
	for _, filter := range xmlChildrenNamed(n, "intent-filter") {
		for _, action := range xmlChildrenNamed(filter, "action") {
			if wearableListenerActions[xmlAttrValue(action, "android:name")] {
				return true
			}
		}
	}
	return false
}

// isSliceProvider mirrors SliceDetector.isSliceProvider.
func isSliceProvider(n *android.XMLNode) bool {
	for _, filter := range xmlChildrenNamed(n, "intent-filter") {
		hasView, hasSlice := false, false
		for _, action := range xmlChildrenNamed(filter, "action") {
			if xmlAttrValue(action, "android:name") == "android.intent.action.VIEW" {
				hasView = true
			}
		}
		for _, cat := range xmlChildrenNamed(filter, "category") {
			if xmlAttrValue(cat, "android:name") == "android.app.slice.category.SLICE" {
				hasSlice = true
			}
		}
		if hasView && hasSlice {
			return true
		}
	}
	return false
}

// checkExportedComponents reports exported components of one tag that no permission protects.
func checkExportedComponents(ctx *api.Context, base BaseRule, tag, message string, skip func(*android.XMLNode) bool) {
	m := ctx.Manifest
	app := manifestApplicationNode(manifestRoot(m))
	if app == nil {
		return
	}
	for _, n := range xmlChildrenNamed(app, tag) {
		explicit := explicitExported(n)
		exported := (explicit != nil && *explicit) || (explicit == nil && hasIntentFilterChild(n))
		if !exported || !unprotectedByPermission(n, app) || skip(n) {
			continue
		}
		// Without android:exported the component is only exported before Android 12.
		if explicit == nil && m.TargetSDK >= 31 {
			continue
		}
		ctx.Emit(baseFindingAt(m.Path, n.Line, n.Col+1, base, message))
	}
}

func (r *ExportedReceiverManifestRule) check(ctx *api.Context) {
	checkExportedComponents(ctx, r.BaseRule, "receiver", "Exported receiver does not require permission", isStandardReceiver)
}

func (r *ExportedServiceManifestRule) check(ctx *api.Context) {
	checkExportedComponents(ctx, r.BaseRule, "service", "Exported service does not require permission", isWearableListenerService)
}

func (r *ExportedContentProviderManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	app := manifestApplicationNode(manifestRoot(m))
	if app == nil {
		return
	}
	for _, n := range xmlChildrenNamed(app, "provider") {
		exported := true
		if raw := xmlAttrValue(n, "android:exported"); raw != "" {
			exported = strings.EqualFold(raw, "true")
		}
		if !exported || xmlAttrValue(n, "android:readPermission") != "" || xmlAttrValue(n, "android:writePermission") != "" ||
			xmlAttrValue(n, "android:permission") != "" || len(xmlChildrenNamed(n, "path-permission")) > 0 {
			continue
		}
		if isSliceProvider(n) || xmlAttrValue(n, "tools:node") == "remove" {
			continue
		}
		ctx.Emit(baseFindingAt(m.Path, n.Line, n.Col+1, r.BaseRule,
			"Exported content providers can provide access to potentially sensitive data"))
	}
}

// IllegalResourceRefManifestRule mirrors Lint's ManifestDetector ILLEGAL_REFERENCE:
// versionCode, minSdkVersion and targetSdkVersion must be literals.
type IllegalResourceRefManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *IllegalResourceRefManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	root := manifestRoot(m)
	if root == nil {
		return
	}
	if a, ok := xmlAttrOf(root, "android:versionCode"); ok && strings.HasPrefix(a.Value, "@") {
		ctx.Emit(baseFindingAt(m.Path, a.Line, a.Col, r.BaseRule,
			"The `android:versionCode` cannot be a resource url, it must be a literal integer"))
	}
	for _, sdk := range xmlChildrenNamed(root, "uses-sdk") {
		for _, name := range []string{"android:minSdkVersion", "android:targetSdkVersion"} {
			if a, ok := xmlAttrOf(sdk, name); ok && strings.HasPrefix(a.Value, "@") {
				ctx.Emit(baseFindingAt(m.Path, a.Line, a.Col, r.BaseRule,
					"The `"+name+"` cannot be a resource url, it must be a literal integer (or string if a preview codename)"))
			}
		}
	}
}

// DeprecatedManifestRule mirrors the manifest part of Lint's DeprecationDetector.
type DeprecatedManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DeprecatedManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	root := manifestRoot(m)
	if root == nil {
		return
	}
	if a, ok := xmlAttrOf(root, "android:sharedUserId"); ok {
		if _, hasMax := xmlAttrOf(root, "android:sharedUserMaxSdkVersion"); !hasMax {
			ctx.Emit(baseFindingAt(m.Path, a.Line, a.Col, r.BaseRule,
				"Consider removing `sharedUserId` for new users by adding `android:sharedUserMaxSdkVersion=\"32\"` to your manifest. See https://developer.android.com/guide/topics/manifest/manifest-element for details."))
		}
	}
	for _, perm := range xmlChildrenNamed(root, "uses-permission-sdk-m") {
		ctx.Emit(baseFindingAt(m.Path, perm.Line, perm.Col+1, r.BaseRule,
			"`uses-permission-sdk-m` is deprecated: Use `uses-permission-sdk-23 instead"))
	}
	if app := manifestApplicationNode(root); app != nil {
		for _, svc := range xmlChildrenNamed(app, "service") {
			if a, ok := xmlAttrOf(svc, "android:permission"); ok && a.Value == "android.permission.BIND_CHOOSER_TARGET_SERVICE" {
				ctx.Emit(baseFindingAt(m.Path, a.Line, a.Col, r.BaseRule,
					"ChooserTargetService` is deprecated: Please see https://developer.android.com/training/sharing/receive.html?source=studio#providing-direct-share-targets"))
			}
		}
	}
}
