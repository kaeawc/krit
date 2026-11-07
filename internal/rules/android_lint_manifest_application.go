package rules

import (
	"strconv"

	"github.com/kaeawc/krit/internal/rules/api"
)

// DataExtractionRulesRule mirrors Lint's DataExtractionRules: backup attributes of <application> that need
// android:dataExtractionRules or android:fullBackupContent counterparts around Android 12.
type DataExtractionRulesRule struct {
	ManifestBase
	AndroidRule
}

// RedundantLabelRule mirrors Lint's RedundantLabel: an activity label equal to the application label.
type RedundantLabelRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *DataExtractionRulesRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *RedundantLabelRule) Confidence() float64 { return api.ConfidenceHigh }

const (
	cloudBackupDeprecated = "` is deprecated from Android 12 and higher and may be removed in future versions. " +
		"Consider adding the attribute `android:dataExtractionRules` specifying an `@xml` resource which configures cloud backups and device transfers on Android 12 and higher."
	backupDeprecated = "` is deprecated from Android 12 and may be removed in future versions. " +
		"Consider adding the attribute `android:dataExtractionRules` specifying an `@xml` resource which configures backups and device transfers on Android 12 and higher."
	backupDeprecatedHigher = "` is deprecated from Android 12 and higher and may be removed in future versions. " +
		"Consider adding the attribute `android:dataExtractionRules` specifying an `@xml` resource which configures backups and device transfers on Android 12 and higher."
)

func (r *DataExtractionRulesRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil || manifestIsLibrary(m) {
		return
	}
	app := m.Root.Find("application")
	if app == nil || (len(app.Children) == 0 && len(app.Attrs) == 0) {
		return
	}
	allowBackup, hasAllow := app.Attr("android:allowBackup")
	rules, hasRules := app.Attr("android:dataExtractionRules")
	full, hasFull := app.Attr("android:fullBackupContent")
	minSdk, targetSdk := manifestMinSdk(m), manifestTargetSdk(m)
	switch {
	case minSdk < 31 && targetSdk >= 31:
		switch {
		case hasAllow && allowBackup.Value == "false" && !hasRules:
			emitManifestAtValue(ctx, allowBackup, "The attribute `android:allowBackup"+cloudBackupDeprecated)
		case hasFull && !hasRules:
			emitManifestAtValue(ctx, full, "The attribute `android:fullBackupContent"+cloudBackupDeprecated)
		case hasRules && !hasFull:
			emitManifestAtValue(ctx, rules, "The attribute `android:dataExtractionRules` only applies for Android 12 and higher; "+
				"since `minSdkVersion` is API "+strconv.Itoa(minSdk)+" you should also set `android:fullBackupContent`")
		}
	case minSdk >= 31 && !hasRules:
		switch {
		case hasAllow && !hasFull && allowBackup.Value == "true":
			emitManifestAtValue(ctx, allowBackup, "The attribute `android:allowBackup` is deprecated from Android 12 and the default allows backup")
		case hasAllow:
			emitManifestAtValue(ctx, allowBackup, "The attribute `android:allowBackup"+backupDeprecated)
		}
		if hasFull {
			emitManifestAtValue(ctx, full, "The attribute `android:fullBackupContent"+backupDeprecatedHigher)
		}
	}
}

func (r *RedundantLabelRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Root == nil {
		return
	}
	app := m.Root.Find("application")
	if app == nil {
		return
	}
	appLabel, ok := app.Attr("android:label")
	if !ok {
		return
	}
	for _, activity := range app.ChildrenByTag("activity") {
		if label, ok := activity.Attr("android:label"); ok && label.Value == appLabel.Value {
			emitManifestAtAttr(ctx, label, "Redundant label can be removed")
		}
	}
}
