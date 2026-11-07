package rules_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// runExportedManifest runs a manifest rule over application children and returns finding lines.
func runExportedManifest(t *testing.T, ruleID, target, components string) []scanner.Finding {
	t.Helper()
	body := `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="test">
    <application>
` + components + `    </application>
</manifest>`
	path := filepath.Join(t.TempDir(), "src", "main", "AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &rules.Manifest{Path: path}
	if target != "" {
		m.TargetSDK = map[string]int{"30": 30, "33": 33}[target]
	}
	return runManifestRule(findManifestRule(t, ruleID), m)
}

func exportedLines(findings []scanner.Finding) []int {
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	return lines
}

func TestExportedServiceManifest_FollowsLint(t *testing.T) {
	const components = `        <service android:name=".S1" android:exported="true"/>
        <service android:name=".S2" android:exported="true" android:permission="p.PERM"/>
        <service android:name=".S3"><intent-filter><action android:name="x"/></intent-filter></service>
        <service android:name=".S4" android:exported="false"/>
        <service android:name=".S5"/>
        <service android:name=".S6" android:exported="true"><intent-filter><action android:name="com.google.android.gms.wearable.BIND_LISTENER"/></intent-filter></service>
        <service android:name=".S7" android:exported="true" android:permission="TODO"/>
`
	t.Run("explicit and implicit exports without a permission are reported on android:name", func(t *testing.T) {
		findings := runExportedManifest(t, "ExportedServiceManifest", "", components)
		if got := exportedLines(findings); len(got) != 3 || got[0] != 4 || got[1] != 6 || got[2] != 10 {
			t.Fatalf("expected findings on lines 4, 6 and 10, got %v", got)
		}
		if findings[0].Message != "Exported service does not require permission" {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
	})
	t.Run("an export implied by an intent-filter stops counting at targetSdk 31", func(t *testing.T) {
		if got := exportedLines(runExportedManifest(t, "ExportedServiceManifest", "33", components)); len(got) != 2 || got[0] != 4 || got[1] != 10 {
			t.Fatalf("expected findings on lines 4 and 10 at target 33, got %v", got)
		}
		if got := exportedLines(runExportedManifest(t, "ExportedServiceManifest", "30", components)); len(got) != 3 {
			t.Fatalf("expected 3 findings at target 30, got %v", got)
		}
	})
	t.Run("a permission on the application protects its services", func(t *testing.T) {
		body := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="test">
    <application android:permission="p.APP">
        <service android:name=".S1" android:exported="true"/>
    </application>
</manifest>`
		path := filepath.Join(t.TempDir(), "AndroidManifest.xml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := runManifestRule(findManifestRule(t, "ExportedServiceManifest"), &rules.Manifest{Path: path}); len(got) != 0 {
			t.Fatalf("expected no findings, got %v", got)
		}
	})
}

func TestExportedReceiverManifest_FollowsLint(t *testing.T) {
	findings := runExportedManifest(t, "ExportedReceiverManifest", "", `        <receiver android:name=".R1" android:exported="true"/>
        <receiver android:name=".R2" android:exported="true" android:permission="p.PERM"/>
        <receiver android:name=".R3"><intent-filter><action android:name="x"/></intent-filter></receiver>
        <receiver android:name=".R4"/>
        <receiver android:name=".R5" android:exported="true"><intent-filter><action android:name="android.intent.action.BOOT_COMPLETED"/></intent-filter></receiver>
        <receiver android:name="com.google.android.gms.tagmanager.InstallReferrerReceiver" android:exported="true"/>
`)
	if got := exportedLines(findings); len(got) != 2 || got[0] != 4 || got[1] != 6 {
		t.Fatalf("expected findings on lines 4 and 6, got %v", got)
	}
	if findings[0].Message != "Exported receiver does not require permission" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestExportedContentProviderManifest_FollowsLint(t *testing.T) {
	findings := runExportedManifest(t, "ExportedContentProviderManifest", "", `        <provider android:name=".P1" android:authorities="a1" android:exported="true"/>
        <provider android:name=".P2" android:authorities="a2" android:exported="true" android:readPermission="p.R"/>
        <provider android:name=".P3" android:authorities="a3" android:exported="true" android:permission="p.PERM"/>
        <provider android:name=".P4" android:authorities="a4"/>
        <provider android:name=".P5" android:authorities="a5" android:exported="false"/>
        <provider android:name=".P6" android:authorities="a6"><path-permission android:pathPrefix="/x" android:readPermission="p.R"/></provider>
        <provider android:name=".P7" android:authorities="a7" tools:node="remove"/>
        <provider android:name=".P8" android:authorities="a8"><intent-filter><action android:name="android.intent.action.VIEW"/><category android:name="android.app.slice.category.SLICE"/></intent-filter></provider>
        <provider android:name=".P9" android:authorities="a9"><intent-filter><action android:name="androidx.slice.action.BIND"/></intent-filter></provider>
`)
	if got := exportedLines(findings); len(got) != 3 || got[0] != 4 || got[1] != 7 || got[2] != 12 {
		t.Fatalf("expected findings on lines 4, 7 and 12 (an absent android:exported means exported; only a VIEW action with the slice category exempts a slice provider, as in Lint), got %v", got)
	}
	if findings[0].Message != "Exported content providers can provide access to potentially sensitive data" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}
