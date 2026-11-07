package rules_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

// tvManifest mirrors a synthetic manifest that was run through Android Lint 32.4.1.
func tvManifest() *rules.Manifest {
	perms := []struct {
		name   string
		maxSdk int
	}{
		{"android.permission.CAMERA", 0}, {"android.permission.BLUETOOTH", 0}, {"android.permission.CALL_PHONE", 25},
		{"android.permission.RECORD_AUDIO", 0}, {"android.permission.ACCESS_FINE_LOCATION", 0}, {"android.permission.INTERNET", 0},
	}
	m := &rules.Manifest{Path: "AndroidManifest.xml", RootLine: 1}
	for i, p := range perms {
		m.UsesPermissionEntries = append(m.UsesPermissionEntries, rules.ManifestUsesPermission{Name: p.name, Line: i + 3, HasMaxSdk: p.maxSdk > 0, MaxSdk: p.maxSdk})
	}
	m.UsesFeatures = []rules.ManifestUsesFeature{
		{Name: "android.hardware.camera", Line: 9},
		{Name: "android.hardware.bluetooth", Required: "false", Line: 10},
		{Name: "android.hardware.nfc", Required: "true", Line: 11},
		{Name: "android.hardware.microphone", Required: "@bool/x", Line: 12},
		{Name: "android.hardware.location.gps.extra", Line: 13},
	}
	m.Application = &rules.ManifestApplication{Line: 14, Activities: []rules.ManifestComponent{
		{Tag: "activity", Name: ".A", Line: 15, HasIntentFilter: true, IntentFilterCategories: []string{"android.intent.category.LEANBACK_LAUNCHER"}},
		{Tag: "activity", Name: ".B", Line: 18, HasBanner: true, HasIntentFilter: true, IntentFilterCategories: []string{"android.intent.category.LAUNCHER"}},
	}}
	return m
}

func tvFindingLines(t *testing.T, ruleName string, m *rules.Manifest) string {
	t.Helper()
	var out []string
	for _, f := range runManifestRule(findManifestRule(t, ruleName), m) {
		out = append(out, strconv.Itoa(f.Line)+" "+f.Message)
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

// Expected findings were produced by running Android Lint 32.4.1 on the manifest this models.
func TestAndroidTvDetectorLintParity(t *testing.T) {
	m := tvManifest()
	tests := map[string]string{
		"MissingLeanbackSupportManifest":  "1 Expecting `<uses-feature android:name=\"android.software.leanback\" android:required=\"false\" />` tag",
		"MissingLeanbackLauncherManifest": "",
		"PermissionImpliesUnsupportedHardwareManifest": strings.Join([]string{
			"5 Permission exists without corresponding hardware `<uses-feature android:name=\"android.hardware.telephony\" required=\"false\">` tag",
			"7 Permission exists without corresponding hardware `<uses-feature android:name=\"android.hardware.location.gps\" required=\"false\">` tag",
		}, "\n"),
	}
	for rule, want := range tests {
		if got := tvFindingLines(t, rule, m); got != want {
			t.Errorf("%s:\ngot  %q\nwant %q", rule, got, want)
		}
	}
}

func TestAndroidTvDetectorSkipsNonTvApps(t *testing.T) {
	m := tvManifest()
	m.UsesFeatures = m.UsesFeatures[:4]
	m.Application.Activities = m.Application.Activities[1:]
	for _, rule := range []string{"MissingLeanbackSupportManifest", "MissingLeanbackLauncherManifest", "PermissionImpliesUnsupportedHardwareManifest"} {
		if got := tvFindingLines(t, rule, m); got != "" {
			t.Errorf("%s reported on a manifest that is not a TV app: %q", rule, got)
		}
	}
}

func TestMissingLeanbackLauncherForLeanbackFeatureWithoutLauncher(t *testing.T) {
	m := tvManifest()
	m.UsesFeatures = append(m.UsesFeatures, rules.ManifestUsesFeature{Name: "android.software.leanback", Required: "false", Line: 14})
	m.Application.Activities = m.Application.Activities[1:]
	want := "1 Expecting an activity to have `android.intent.category.LEANBACK_LAUNCHER` intent filter"
	if got := tvFindingLines(t, "MissingLeanbackLauncherManifest", m); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
