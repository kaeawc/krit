package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func boolPtr(b bool) *bool { return &b }

func writeAndroidModuleManifestFixture(t *testing.T, plugin string) string {
	t.Helper()
	dir := t.TempDir()
	buildPath := filepath.Join(dir, "build.gradle.kts")
	manifestPath := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := "plugins {\n    id(\"" + plugin + "\")\n}\n"
	if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("<manifest package=\"test\" />\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}

func TestAllowBackupManifest(t *testing.T) {
	r := findManifestRule(t, "AllowBackupManifest")

	t.Run("allowBackup true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("allowBackup missing with backup rules is clean", func(t *testing.T) {
		for _, app := range []*rules.ManifestApplication{
			{Line: 5, FullBackupContent: "@xml/rules"},
			{Line: 5, DataExtractionRules: "@xml/extraction"},
		} {
			m := &rules.Manifest{Path: "AndroidManifest.xml", Application: app}
			if findings := runManifestRule(r, m); len(findings) != 0 {
				t.Fatalf("expected 0 findings, got %d", len(findings))
			}
		}
	})

	activity := []rules.ManifestComponent{{Tag: "activity", Name: ".Main", Line: 6}}
	t.Run("allowBackup missing triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: nil,
				Activities:  activity,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("overlay manifests are not judged for a missing allowBackup", func(t *testing.T) {
		cases := map[string]*rules.Manifest{
			"no components":     {Path: "AndroidManifest.xml", Application: &rules.ManifestApplication{Line: 5}},
			"tools:node marker": {Path: "AndroidManifest.xml", Application: &rules.ManifestApplication{Line: 5, ToolsNode: "merge", Activities: activity}},
			"bazel directory":   {Path: "repo/bazel/AndroidManifest.xml", Application: &rules.ManifestApplication{Line: 5, Activities: activity}},
			"services only app": {Path: "AndroidManifest.xml", Application: &rules.ManifestApplication{Line: 5, Services: activity}},
		}
		for name, m := range cases {
			got := runManifestRule(r, m)
			if name == "services only app" {
				if len(got) != 1 {
					t.Errorf("%s: a manifest with a component stands alone, got %d findings", name, len(got))
				}
				continue
			}
			if len(got) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", name, len(got))
			}
		}
	})

	t.Run("explicit allowBackup true in an overlay still reports", func(t *testing.T) {
		m := &rules.Manifest{Path: "repo/bazel/AndroidManifest.xml", Application: &rules.ManifestApplication{Line: 5, AllowBackup: boolPtr(true)}}
		if got := runManifestRule(r, m); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})

	t.Run("library module manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: writeAndroidModuleManifestFixture(t, "com.android.library"),
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
		}
	})

	t.Run("allowBackup false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDebuggableManifest(t *testing.T) {
	r := findManifestRule(t, "DebuggableManifest")

	t.Run("debuggable true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:       5,
				Debuggable: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 || findings[0].Message != "Avoid hardcoding the debug mode; leaving it out allows debug and release builds to automatically assign one" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("debuggable false is hardcoded too", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:       5,
				Debuggable: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("debuggable not set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDeepLinkMissingAutoVerify(t *testing.T) {
	r := findManifestRule(t, "DeepLinkMissingAutoVerify")

	run := func(t *testing.T, body string) []scanner.Finding {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return runManifestRule(r, &rules.Manifest{Path: path})
	}

	t.Run("https browsable default filter without autoVerify triggers", func(t *testing.T) {
		findings := run(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <activity android:name=".MainActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" android:host="app.example.com" />
            </intent-filter>
        </activity>
    </application>
</manifest>`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("autoVerify true and non web filters are clean", func(t *testing.T) {
		findings := run(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <activity android:name=".VerifiedActivity">
            <intent-filter android:autoVerify="true">
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" android:host="app.example.com" />
            </intent-filter>
        </activity>
        <activity android:name=".CustomSchemeActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="myapp" android:host="open" />
            </intent-filter>
        </activity>
        <activity android:name=".NotBrowsableActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <data android:scheme="https" android:host="app.example.com" />
            </intent-filter>
        </activity>
    </application>
</manifest>`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("activity alias web filter without autoVerify triggers", func(t *testing.T) {
		findings := run(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <activity-alias android:name=".AliasActivity" android:targetActivity=".MainActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="http" android:host="app.example.com" />
            </intent-filter>
        </activity-alias>
    </application>
</manifest>`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

func TestExportedWithoutPermission(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")

	t.Run("exported without permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MyActivity", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("services receivers and providers are left to the Lint-aligned rules", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services:  []rules.ManifestComponent{{Tag: "service", Name: ".S", Line: 10, Exported: boolPtr(true)}},
				Receivers: []rules.ManifestComponent{{Tag: "receiver", Name: ".R", Line: 11, Exported: boolPtr(true)}},
				Providers: []rules.ManifestComponent{{Tag: "provider", Name: ".P", Line: 12, Exported: boolPtr(true)}},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("exported with permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{Tag: "service", Name: ".MyService", Line: 10, Exported: boolPtr(true), Permission: "android.permission.BIND_JOB_SERVICE"},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("not exported is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{Tag: "service", Name: ".MyService", Line: 10, Exported: boolPtr(false)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMissingExportedFlag(t *testing.T) {
	r := findManifestRule(t, "MissingExportedFlag")

	t.Run("intent-filter without exported triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, HasIntentFilter: true, Exported: nil},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("intent-filter with exported set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, HasIntentFilter: true, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no intent-filter without exported is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".DetailActivity", Line: 15, HasIntentFilter: false, Exported: nil},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDuplicateActivityManifest(t *testing.T) {
	r := findManifestRule(t, "DuplicateActivityManifest")
	const head = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
  <application>
`
	const tail = "  </application>\n</manifest>\n"

	t.Run("relative and qualified names of one class are duplicates", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.t", head+
			`    <activity android:name=".A"/>
    <activity android:name="com.t.A"/>
    <activity android:name="A"/>
    <activity android:name="org.other.A"/>
`+tail)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Message != "Duplicate registration for activity `com.t.A`" || findings[0].Line != 4 || findings[0].Col != 15 {
			t.Fatalf("unexpected finding %+v", findings[0])
		}
	})

	t.Run("services receivers and aliases are not activities", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.t", head+
			`    <service android:name=".S"/>
    <service android:name=".S"/>
    <activity-alias android:name=".AL" android:targetActivity=".A"/>
    <activity-alias android:name=".AL" android:targetActivity=".A"/>
`+tail)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("tools node on the first declaration does not hide the duplicate", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.t", head+
			`    <activity android:name=".D" tools:node="remove"/>
    <activity android:name=".D"/>
`+tail)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unique activities is clean", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.t", head+
			`    <activity android:name=".MainActivity"/>
    <activity android:name=".DetailActivity"/>
`+tail)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWrongManifestParentManifest(t *testing.T) {
	r := findManifestRule(t, "WrongManifestParentManifest")
	const header = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">` + "\n"

	t.Run("elements under the wrong parent report Lint's messages", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.example", header+
			`  <application>
    <uses-sdk android:minSdkVersion="21"/>
    <activity android:name=".A"/>
    <provider android:name=".P"/>
  </application>
  <activity android:name=".B"/>
  <provider android:name=".Q"/>
  <uses-library android:name="lib"/>
</manifest>
`)
		want := map[int]string{
			3: "The <uses-sdk> element must be a direct child of the <manifest> root element",
			7: "The <activity> element must be a direct child of the <application> element",
			8: "The <provider> element must be a direct child of the <application> element or the <queries> element",
			9: "The <uses-library> element must be a direct child of the <application> element",
		}
		got := map[int]string{}
		for _, f := range findings {
			got[f.Line] = f.Message
		}
		if len(got) != len(want) || len(findings) != len(want) {
			t.Fatalf("expected %d findings, got %v", len(want), got)
		}
		for line, msg := range want {
			if got[line] != msg {
				t.Errorf("line %d: got %q, want %q", line, got[line], msg)
			}
		}
	})

	t.Run("correct nesting and unlisted elements are clean", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.example", header+
			`  <uses-sdk android:minSdkVersion="21"/>
  <instrumentation android:name=".I"/>
  <queries><provider android:authorities="x"/></queries>
  <application>
    <activity android:name=".A"><meta-data android:name="m" android:value="v"/></activity>
    <activity-alias android:name=".Alias" android:targetActivity=".A"/>
    <uses-library android:name="lib"/>
  </application>
</manifest>
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestGradleOverridesManifest(t *testing.T) {
	r := findManifestRule(t, "GradleOverridesManifest")

	// run writes <module>/build.gradle.kts and the main manifest, then runs the rule on the manifest.
	run := func(t *testing.T, build, manifest string) []scanner.Finding {
		t.Helper()
		dir := t.TempDir()
		if build != "" {
			if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(build), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return runManifestRule(r, &rules.Manifest{Path: path})
	}
	const gradleBoth = "android { defaultConfig {\n minSdk = 24\n targetSdk = 34\n versionCode = 7\n } }\n"
	const gradleMinOnly = "android {\n defaultConfig {\n minSdk = 24\n }\n}\n"
	const sdk = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:versionCode="1">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="33"/>
</manifest>
`

	t.Run("minSdk in manifest triggers when gradle sets it", func(t *testing.T) {
		findings := run(t, gradleMinOnly, sdk)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		want := "This `minSdkVersion` value (`21`) is not used; it is always overridden by the value specified in the Gradle build script (`24`)"
		if findings[0].Message != want || findings[0].Line != 2 {
			t.Fatalf("unexpected finding %+v", findings[0])
		}
	})

	t.Run("min, target and version code are all overridden", func(t *testing.T) {
		findings := run(t, gradleBoth, sdk)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("overrideLibrary keeps the minSdk value meaningful", func(t *testing.T) {
		manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
  <uses-sdk android:minSdkVersion="21" tools:overrideLibrary="x.y"/>
</manifest>
`
		if findings := run(t, gradleMinOnly, manifest); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("module without a build script is clean", func(t *testing.T) {
		if findings := run(t, "", sdk); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no uses-sdk is clean", func(t *testing.T) {
		if findings := run(t, gradleMinOnly, "<manifest/>\n"); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUsesSdkManifest(t *testing.T) {
	r := findManifestRule(t, "UsesSdkManifest")

	t.Run("missing uses-sdk triggers when application present", func(t *testing.T) {
		m := &rules.Manifest{
			Path:        "AndroidManifest.xml",
			Application: &rules.ManifestApplication{Line: 5},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("skipped when no application element (library stub)", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for library stub manifest, got %d", len(findings))
		}
	})

	t.Run("present uses-sdk is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:        "AndroidManifest.xml",
			Application: &rules.ManifestApplication{Line: 5},
			UsesSdk:     &rules.ManifestElement{Tag: "uses-sdk", Line: 3},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMipmapLauncher(t *testing.T) {
	r := findManifestRule(t, "MipmapLauncher")

	// run writes a module build script and a manifest, then runs the rule on the manifest.
	run := func(t *testing.T, build, manifest string) []scanner.Finding {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(build), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return runManifestRule(r, &rules.Manifest{Path: path})
	}
	const densities = "android { defaultConfig { resourceConfigurations += listOf(\"xhdpi\", \"en\") } }\n"
	const plain = "android { defaultConfig { minSdk = 24 } }\n"
	const manifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <application android:icon="@drawable/ic_launcher">
    <activity android:name=".Main" android:icon="@drawable/ic_main">
      <intent-filter><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
    </activity>
    <activity android:name=".Other" android:icon="@drawable/ic_other"/>
  </application>
</manifest>
`

	t.Run("drawable icons on the application and launchable activities trigger", func(t *testing.T) {
		findings := run(t, densities, manifest)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Line != 2 || findings[1].Line != 3 {
			t.Errorf("expected findings on lines 2 and 3, got %d and %d", findings[0].Line, findings[1].Line)
		}
		if findings[0].Message != "Should use `@mipmap` instead of `@drawable` for launcher icons" {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("modules without density resConfigs are clean like Lint", func(t *testing.T) {
		if findings := run(t, plain, manifest); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mipmap icon is clean", func(t *testing.T) {
		clean := `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <application android:icon="@mipmap/ic_launcher"/>
</manifest>
`
		if findings := run(t, densities, clean); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUniquePermission(t *testing.T) {
	r := findManifestRule(t, "UniquePermission")
	const header = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">` + "\n"

	t.Run("same unqualified name in two permissions triggers on the later one", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.example", header+
			`  <permission android:name="com.example.perm.READ"/>
  <permission android:name="org.other.perm.READ"/>
</manifest>
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		const want = "Permission name `READ` is not unique (appears in both `com.example.perm.READ` and `org.other.perm.READ`)"
		if findings[0].Line != 3 || findings[0].Message != want {
			t.Errorf("got line %d %q, want line 3 %q", findings[0].Line, findings[0].Message, want)
		}
	})

	t.Run("permission groups use their own label", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.example", header+
			`  <permission-group android:name="a.GROUP"/>
  <permission-group android:name="b.GROUP"/>
</manifest>
`)
		if len(findings) != 1 || !strings.HasPrefix(findings[0].Message, "Permission group name `GROUP` is not unique") {
			t.Fatalf("unexpected findings: %+v", findings)
		}
	})

	t.Run("distinct names, repeated identical names and a system-like name are clean", func(t *testing.T) {
		findings := runManifestXML(t, r, "com.example", header+
			`  <permission android:name="com.example.perm.READ"/>
  <permission android:name="com.example.perm.WRITE"/>
  <permission android:name="com.example.perm.READ"/>
  <permission android:name="android.permission.CAMERA"/>
</manifest>
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSystemPermission(t *testing.T) {
	r := findManifestRule(t, "SystemPermission")

	t.Run("system only permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.INSTALL_PACKAGES"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiple system only permissions trigger multiple", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.REBOOT", "android.permission.READ_LOGS"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("runtime dangerous and normal permissions are clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA", "android.permission.READ_SMS", "android.permission.INTERNET"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestManifestTypoManifest(t *testing.T) {
	r := findManifestRule(t, "ManifestTypoManifest")

	t.Run("typo triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "aplication", Line: 5, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("correct tag is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "application", Line: 5, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMissingApplicationIconManifest(t *testing.T) {
	r := findManifestRule(t, "MissingApplicationIconManifest")

	t.Run("missing icon triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("icon present is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
				Icon: "@mipmap/ic_launcher",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no application is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("library module manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: writeAndroidModuleManifestFixture(t, "com.android.library"),
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
		}
	})
}

func TestTargetNewer(t *testing.T) {
	r := findManifestRule(t, "TargetNewer")

	t.Run("old target triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 28,
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("recent target is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 34,
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no target set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestExportedLintComponentRules(t *testing.T) {
	wrap := func(target, body string) string {
		return `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
  <uses-sdk android:targetSdkVersion="` + target + `"/>
  <application>
` + body + `  </application>
</manifest>
`
	}
	run := func(t *testing.T, rule, target, body string) []scanner.Finding {
		r := findManifestRule(t, rule)
		path := filepath.Join(t.TempDir(), "AndroidManifest.xml")
		if err := os.WriteFile(path, []byte(wrap(target, body)), 0o644); err != nil {
			t.Fatal(err)
		}
		tsdk := 34
		if target == "30" {
			tsdk = 30
		}
		return runManifestRule(r, &rules.Manifest{Path: path, TargetSDK: tsdk, Application: &rules.ManifestApplication{}})
	}

	t.Run("exported service and receiver without permission", func(t *testing.T) {
		got := run(t, "ExportedServiceManifest", "34", `    <service android:name=".S" android:exported="true"/>
`)
		if len(got) != 1 || got[0].Message != "Exported service does not require permission" || got[0].Line != 4 || got[0].Col != 6 {
			t.Fatalf("unexpected service findings: %+v", got)
		}
		got = run(t, "ExportedReceiverManifest", "34", `    <receiver android:name=".R" android:exported="true"/>
`)
		if len(got) != 1 || got[0].Message != "Exported receiver does not require permission" {
			t.Fatalf("unexpected receiver findings: %+v", got)
		}
	})

	t.Run("permissions protect components, application permission counts", func(t *testing.T) {
		if got := run(t, "ExportedServiceManifest", "34", `    <service android:name=".S" android:exported="true" android:permission="p.PERM"/>
`); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %+v", got)
		}
		if got := run(t, "ExportedServiceManifest", "34", `    <service android:name=".S" android:exported="false"/>
`); len(got) != 0 {
			t.Fatalf("not exported: expected 0 findings, got %+v", got)
		}
	})

	t.Run("implicit export only counts before Android 12", func(t *testing.T) {
		body := `    <service android:name=".S"><intent-filter><action android:name="com.example.ACT"/></intent-filter></service>
`
		if got := run(t, "ExportedServiceManifest", "30", body); len(got) != 1 {
			t.Fatalf("targetSdk 30: expected 1 finding, got %+v", got)
		}
		if got := run(t, "ExportedServiceManifest", "34", body); len(got) != 0 {
			t.Fatalf("targetSdk 34: expected 0 findings, got %+v", got)
		}
	})

	t.Run("standard receivers and wearable services are skipped", func(t *testing.T) {
		if got := run(t, "ExportedReceiverManifest", "34", `    <receiver android:name=".R" android:exported="true"><intent-filter><action android:name="android.intent.action.BOOT_COMPLETED"/></intent-filter></receiver>
`); len(got) != 0 {
			t.Fatalf("standard receiver: expected 0 findings, got %+v", got)
		}
		if got := run(t, "ExportedServiceManifest", "34", `    <service android:name=".S" android:exported="true"><intent-filter><action android:name="com.google.android.gms.wearable.DATA_CHANGED"/></intent-filter></service>
`); len(got) != 0 {
			t.Fatalf("wearable service: expected 0 findings, got %+v", got)
		}
	})

	t.Run("providers are exported unless said otherwise", func(t *testing.T) {
		msg := "Exported content providers can provide access to potentially sensitive data"
		got := run(t, "ExportedContentProviderManifest", "34", `    <provider android:name=".P" android:authorities="a"/>
    <provider android:name=".P2" android:authorities="b" android:exported="false"/>
    <provider android:name=".P3" android:authorities="c" android:readPermission="r"/>
    <provider android:name=".P4" android:authorities="d"><path-permission android:pathPrefix="/x" android:readPermission="r"/></provider>
    <provider android:name=".P5" android:authorities="e" tools:node="remove"/>
`)
		if len(got) != 1 || got[0].Message != msg || got[0].Line != 4 {
			t.Fatalf("unexpected provider findings: %+v", got)
		}
	})

	t.Run("grant-uri-permission for the root path", func(t *testing.T) {
		got := run(t, "GrantAllUris", "34", `    <provider android:name=".P" android:authorities="a" android:exported="false"><grant-uri-permission android:pathPrefix="/"/><grant-uri-permission android:pathPrefix="/files"/></provider>
`)
		if len(got) != 1 || got[0].Message != "Content provider shares everything; this is potentially dangerous" {
			t.Fatalf("unexpected findings: %+v", got)
		}
	})
}

func TestIntentFilterExportRequired(t *testing.T) {
	r := findManifestRule(t, "IntentFilterExportRequired")

	t.Run("intent-filter without exported on API 31+ triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 31,
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, HasIntentFilter: true, Exported: nil},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("intent-filter with exported set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, HasIntentFilter: true, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("API below 31 is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 30,
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, HasIntentFilter: true, Exported: nil},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 || findings[0].Severity != "warning" {
			t.Fatalf("below API 31 Lint still reports, as a warning: %+v", findings)
		}
	})

	t.Run("no target SDK set still triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{Tag: "service", Name: ".MyService", Line: 15, HasIntentFilter: true, Exported: nil},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

func TestCleartextTraffic(t *testing.T) {
	r := findManifestRule(t, "CleartextTraffic")

	t.Run("cleartext true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:                 5,
				UsesCleartextTraffic: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("cleartext false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:                 5,
				UsesCleartextTraffic: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cleartext not set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestBackupRules(t *testing.T) {
	r := findManifestRule(t, "BackupRules")

	t.Run("missing backup config triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("fullBackupContent set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:              5,
				FullBackupContent: "@xml/backup_rules",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("dataExtractionRules set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:                5,
				DataExtractionRules: "@xml/data_extraction_rules",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("allowBackup false skips check", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDuplicateUsesFeatureManifest(t *testing.T) {
	r := findManifestRule(t, "DuplicateUsesFeatureManifest")
	manifest := func(features string) string {
		return "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n" + features + "</manifest>\n"
	}

	t.Run("every later duplicate by name reports at the name attribute", func(t *testing.T) {
		findings := runManifestXML(t, r, "p", manifest(
			`  <uses-feature android:name="android.hardware.camera"/>
  <uses-feature android:name="android.hardware.camera" android:required="false"/>
  <uses-feature android:name="android.hardware.camera"/>
`))
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Message != "Duplicate declaration of uses-feature `android.hardware.camera`" || findings[0].Line != 3 || findings[0].Col != 17 {
			t.Fatalf("unexpected finding %+v", findings[0])
		}
	})

	t.Run("duplicate glEsVersion declarations are not reported", func(t *testing.T) {
		findings := runManifestXML(t, r, "p", manifest(
			`  <uses-feature android:glEsVersion="0x00020000"/>
  <uses-feature android:glEsVersion="0x00020000"/>
`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unique features is clean", func(t *testing.T) {
		findings := runManifestXML(t, r, "p", manifest(
			`  <uses-feature android:name="android.hardware.camera"/>
  <uses-feature android:name="android.hardware.bluetooth"/>
`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no features is clean", func(t *testing.T) {
		if findings := runManifestXML(t, r, "p", manifest("")); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMultipleUsesSdkManifest(t *testing.T) {
	r := findManifestRule(t, "MultipleUsesSdkManifest")

	t.Run("two uses-sdk triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "uses-sdk", Line: 3, ParentTag: "manifest"},
				{Tag: "uses-sdk", Line: 8, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("single uses-sdk is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "uses-sdk", Line: 3, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no uses-sdk is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestManifestOrderManifest(t *testing.T) {
	r := findManifestRule(t, "ManifestOrderManifest")

	t.Run("application before uses-permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "application", Line: 3, ParentTag: "manifest"},
				{Tag: "uses-permission", Line: 10, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("application before uses-sdk triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "application", Line: 3, ParentTag: "manifest"},
				{Tag: "uses-sdk", Line: 10, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("uses-feature and supports-screens after application use Lint's message", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "application", Line: 3, ParentTag: "manifest"},
				{Tag: "uses-feature", Line: 10, ParentTag: "manifest"},
				{Tag: "supports-screens", Line: 11, ParentTag: "manifest"},
				{Tag: "uses-library", Line: 4, ParentTag: "application"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 || findings[0].Message != "`<uses-feature>` tag appears after `<application>` tag" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("correct order is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "uses-sdk", Line: 3, ParentTag: "manifest"},
				{Tag: "uses-permission", Line: 5, ParentTag: "manifest"},
				{Tag: "application", Line: 10, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no application is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "uses-sdk", Line: 3, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("library module manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      writeAndroidModuleManifestFixture(t, "com.android.library"),
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
		}
	})
}

func TestMissingVersionManifest(t *testing.T) {
	r := findManifestRule(t, "MissingVersionManifest")

	// App manifest fixture with an Application element containing activities
	// (required for the library-stub heuristic to not skip).
	appManifest := func(versionCode, versionName string) *rules.Manifest {
		return &rules.Manifest{
			Path:        "src/main/AndroidManifest.xml",
			VersionCode: versionCode,
			VersionName: versionName,
			Application: &rules.ManifestApplication{
				Line: 1,
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 2},
				},
			},
		}
	}

	t.Run("both missing reports one finding per attribute in Lint wording", func(t *testing.T) {
		findings := runManifestRule(r, appManifest("", ""))
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Message != "Should set `android:versionCode` to specify the application version" ||
			findings[1].Message != "Should set `android:versionName` to specify the application version" {
			t.Fatalf("unexpected messages %q / %q", findings[0].Message, findings[1].Message)
		}
	})

	t.Run("missing versionCode triggers once", func(t *testing.T) {
		findings := runManifestRule(r, appManifest("", "1.0"))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("missing versionName triggers once", func(t *testing.T) {
		findings := runManifestRule(r, appManifest("1", ""))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("both present is clean", func(t *testing.T) {
		findings := runManifestRule(r, appManifest("1", "1.0"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMockLocationManifest(t *testing.T) {
	r := findManifestRule(t, "MockLocationManifest")

	t.Run("mock location in non-debug triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.ACCESS_MOCK_LOCATION"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("mock location in debug manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			IsDebugManifest: true,
			UsesPermissions: []string{"android.permission.ACCESS_MOCK_LOCATION"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no mock location is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.INTERNET"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMockLocationManifestSkipsDebugSourceSets(t *testing.T) {
	r := findManifestRule(t, "MockLocationManifest")
	for path, want := range map[string]int{
		"app/src/main/AndroidManifest.xml":         1,
		"app/src/release/AndroidManifest.xml":      1,
		"app/src/debug/AndroidManifest.xml":        0,
		"app/src/stagingDebug/AndroidManifest.xml": 0,
		"app/src/androidTest/AndroidManifest.xml":  0,
	} {
		m := &rules.Manifest{Path: path, UsesPermissions: []string{"android.permission.ACCESS_MOCK_LOCATION"}}
		if got := len(runManifestRule(r, m)); got != want {
			t.Errorf("%s: expected %d findings, got %d", path, want, got)
		}
	}
}

func TestUnpackedNativeCodeManifest(t *testing.T) {
	r := findManifestRule(t, "UnpackedNativeCodeManifest")

	t.Run("native libs without extractNativeLibs=false triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:          "AndroidManifest.xml",
			HasNativeLibs: true,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("native libs with extractNativeLibs=true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:          "AndroidManifest.xml",
			HasNativeLibs: true,
			Application: &rules.ManifestApplication{
				Line:              5,
				ExtractNativeLibs: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("native libs with extractNativeLibs=false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:          "AndroidManifest.xml",
			HasNativeLibs: true,
			Application: &rules.ManifestApplication{
				Line:              5,
				ExtractNativeLibs: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no native libs is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestRtlEnabledManifest(t *testing.T) {
	r := findManifestRule(t, "RtlEnabledManifest")
	const notExplicit = "The project references RTL attributes, but does not explicitly enable or disable RTL support with `android:supportsRtl` in the manifest"
	modern := api.RtlMainProject{HasApplication: true, MinSdk: 21, TargetSdk: 34}

	// runWith runs the rule with the given apps deciding for the manifest. A nil list means the project's apps are unknown.
	runWith := func(m *rules.Manifest, mains []api.RtlMainProject) []scanner.Finding {
		ctx := &api.Context{Manifest: m, Rule: r, Collector: scanner.NewFindingCollector(0)}
		if mains != nil {
			ctx.RtlMainsFor = func(string) ([]api.RtlMainProject, bool) { return mains, true }
		}
		r.Check(ctx)
		return api.ContextFindings(ctx)
	}

	// manifestWithLayout writes src/main/res/layout/main.xml with the given attributes and returns a manifest next to it.
	manifestWithLayout := func(t *testing.T, layoutAttrs string, rtl *bool) *rules.Manifest {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "src", "main")
		if err := os.MkdirAll(filepath.Join(dir, "res", "layout"), 0o755); err != nil {
			t.Fatal(err)
		}
		layout := "<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"><TextView " + layoutAttrs + "/></LinearLayout>"
		if err := os.WriteFile(filepath.Join(dir, "res", "layout", "main.xml"), []byte(layout), 0o644); err != nil {
			t.Fatal(err)
		}
		return &rules.Manifest{
			Path: filepath.Join(dir, "AndroidManifest.xml"),
			Application: &rules.ManifestApplication{
				Line:        5,
				SupportsRtl: rtl,
			},
		}
	}

	t.Run("textAlignment or a start/end gravity with no supportsRtl triggers", func(t *testing.T) {
		for _, attrs := range []string{`android:textAlignment="viewStart"`, `android:gravity="start"`, `android:layout_gravity="center|end"`} {
			findings := runWith(manifestWithLayout(t, attrs, nil), []api.RtlMainProject{modern})
			if len(findings) != 1 || findings[0].Message != notExplicit {
				t.Fatalf("%s: got %+v", attrs, findings)
			}
		}
	})

	t.Run("start/end padding and margin attributes alone do not trigger", func(t *testing.T) {
		for _, attrs := range []string{`android:paddingStart="4dp"`, `android:layout_marginEnd="4dp"`, `android:gravity="left"`, `android:gravity="center"`} {
			if findings := runWith(manifestWithLayout(t, attrs, nil), []api.RtlMainProject{modern}); len(findings) != 0 {
				t.Fatalf("%s: expected 0 findings, got %+v", attrs, findings)
			}
		}
	})

	t.Run("an explicit supportsRtl is a decision, not a finding", func(t *testing.T) {
		for _, rtl := range []bool{true, false} {
			m := manifestWithLayout(t, `android:textAlignment="viewStart"`, boolPtr(rtl))
			if findings := runWith(m, []api.RtlMainProject{modern}); len(findings) != 0 {
				t.Fatalf("supportsRtl=%v: expected 0 findings, got %d", rtl, len(findings))
			}
		}
	})

	t.Run("a library module is judged by the app that includes it", func(t *testing.T) {
		lib := manifestWithLayout(t, `android:textAlignment="viewStart"`, nil)
		lib.Application = nil
		if findings := runWith(lib, []api.RtlMainProject{modern}); len(findings) != 1 || findings[0].Message != notExplicit {
			t.Fatalf("library of an RTL-enabled app: got %+v", findings)
		}
		rtlOff := api.RtlMainProject{HasApplication: true, RtlOff: true, MinSdk: 21, TargetSdk: 34}
		if findings := runWith(lib, []api.RtlMainProject{rtlOff}); len(findings) != 0 {
			t.Fatalf("library of an app with supportsRtl=false: got %+v", findings)
		}
		oldTarget := api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 16}
		if findings := runWith(lib, []api.RtlMainProject{oldTarget}); len(findings) != 0 {
			t.Fatalf("library of an app targeting 16: got %+v", findings)
		}
		if findings := runWith(lib, []api.RtlMainProject{{}}); len(findings) != 0 {
			t.Fatalf("library with no application anywhere: got %+v", findings)
		}
		if findings := runWith(lib, nil); len(findings) != 1 {
			t.Fatalf("unknown apps keep reporting, got %+v", findings)
		}
	})

	t.Run("a manifest outside the main source set is skipped", func(t *testing.T) {
		m := manifestWithLayout(t, `android:textAlignment="viewStart"`, nil)
		m.Path = filepath.Join(filepath.Dir(filepath.Dir(m.Path)), "debug", "AndroidManifest.xml")
		if findings := runWith(m, []api.RtlMainProject{modern}); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("supportsRtl=true with a target below 17 triggers", func(t *testing.T) {
		m := manifestWithLayout(t, `android:paddingLeft="4dp"`, boolPtr(true))
		old := api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 16}
		findings := runWith(m, []api.RtlMainProject{old})
		if len(findings) != 1 || findings[0].Message != "You must set `android:targetSdkVersion` to at least 17 when enabling RTL support" {
			t.Fatalf("got %+v", findings)
		}
		// An unset target is the minSdk, so it counts too.
		unsetTarget := api.RtlMainProject{HasApplication: true, MinSdk: 14}
		if findings := runWith(m, []api.RtlMainProject{unsetTarget}); len(findings) != 1 {
			t.Fatalf("unset target: got %+v", findings)
		}
		if findings := runWith(m, []api.RtlMainProject{modern}); len(findings) != 0 {
			t.Fatalf("modern target: got %+v", findings)
		}
	})

	t.Run("supportsRtl=true with no layouts is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:        "AndroidManifest.xml",
			Application: &rules.ManifestApplication{Line: 5, SupportsRtl: boolPtr(true)},
		}
		if findings := runWith(m, []api.RtlMainProject{modern}); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestInvalidUsesFeatureRequiredManifest(t *testing.T) {
	r := findManifestRule(t, "InvalidUsesFeatureRequiredManifest")

	t.Run("invalid required value triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "yes", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("required=true is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("required=false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "false", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("required not set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestExportedPreferenceActivityManifest(t *testing.T) {
	r := findManifestRule(t, "ExportedPreferenceActivityManifest")

	t.Run("exported PreferenceActivity triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MyPreferenceActivity", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("PreferenceActivity with intent filter triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".AppPreferenceActivity", Line: 10, HasIntentFilter: true},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("SettingsActivity name alone is not enough", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".SettingsActivity", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("explicit exported false wins over intent filter", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".AppPreferenceActivity", Line: 10, Exported: boolPtr(false), HasIntentFilter: true},
				},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-exported PreferenceActivity is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MyPreferenceActivity", Line: 10, Exported: boolPtr(false)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("regular exported activity is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestInsecureBaseConfigurationManifest(t *testing.T) {
	r := findManifestRule(t, "InsecureBaseConfigurationManifest")

	t.Run("API 28+ usesCleartextTraffic true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 28,
			Application: &rules.ManifestApplication{
				Line:                 5,
				UsesCleartextTraffic: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("API 28+ missing networkSecurityConfig is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		manifestPath := filepath.Join(t.TempDir(), "src", "main", "AndroidManifest.xml")
		configPath := filepath.Join(filepath.Dir(manifestPath), "res", "xml", "network_security_config.xml")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return manifestPath
	}
	withConfig := func(path string) *rules.Manifest {
		return &rules.Manifest{
			Path:      path,
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line:                  5,
				NetworkSecurityConfig: "@xml/network_security_config",
			},
		}
	}

	t.Run("config permitting cleartext triggers", func(t *testing.T) {
		path := writeConfig(t, `<network-security-config><base-config cleartextTrafficPermitted="true" /></network-security-config>`)
		if findings := runManifestRule(r, withConfig(path)); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("config trusting user CAs in base-config triggers", func(t *testing.T) {
		path := writeConfig(t, `<network-security-config><base-config><trust-anchors><certificates src="user" /></trust-anchors></base-config></network-security-config>`)
		if findings := runManifestRule(r, withConfig(path)); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("cleartext and user CAs scoped to a domain-config is clean", func(t *testing.T) {
		path := writeConfig(t, `<network-security-config><base-config cleartextTrafficPermitted="false" /><domain-config cleartextTrafficPermitted="true"><domain>dev.example.com</domain><trust-anchors><certificates src="user" /></trust-anchors></domain-config></network-security-config>`)
		if findings := runManifestRule(r, withConfig(path)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("user CAs only in debug-overrides is clean", func(t *testing.T) {
		path := writeConfig(t, `<network-security-config><base-config cleartextTrafficPermitted="false" /><debug-overrides><trust-anchors><certificates src="user" /></trust-anchors></debug-overrides></network-security-config>`)
		if findings := runManifestRule(r, withConfig(path)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("config blocking cleartext is clean", func(t *testing.T) {
		path := writeConfig(t, `<network-security-config><base-config cleartextTrafficPermitted="false" /></network-security-config>`)
		if findings := runManifestRule(r, withConfig(path)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("library module with cleartext is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      writeAndroidModuleManifestFixture(t, "com.android.library"),
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line:                 5,
				UsesCleartextTraffic: boolPtr(true),
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("API 28+ with networkSecurityConfig is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line:                  5,
				NetworkSecurityConfig: "@xml/network_security_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("API below 28 is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 27,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no application is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 33,
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestNetworkSecurityConfigDebugOverrides(t *testing.T) {
	r := findManifestRule(t, "NetworkSecurityConfigDebugOverrides")

	writeConfig := func(t *testing.T, manifestPath, variant, body string) {
		t.Helper()
		configPath := filepath.Join(filepath.Dir(manifestPath), "..", variant, "res", "xml", "network_security_config.xml")
		if variant == "main" {
			configPath = filepath.Join(filepath.Dir(manifestPath), "res", "xml", "network_security_config.xml")
		}
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("main network security config with debug-overrides triggers", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		writeConfig(t, manifestPath, "main", `<network-security-config>
    <debug-overrides>
        <trust-anchors>
            <certificates src="@raw/debug_cas" />
        </trust-anchors>
    </debug-overrides>
</network-security-config>`)
		m := &rules.Manifest{
			Path: manifestPath,
			Application: &rules.ManifestApplication{
				Line:                  2,
				NetworkSecurityConfig: "@xml/network_security_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].File, "src/main/res/xml/network_security_config.xml") {
			t.Fatalf("expected finding on network config file, got %q", findings[0].File)
		}
	})

	t.Run("debug source set network security config is clean", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "src", "debug", "AndroidManifest.xml")
		writeConfig(t, manifestPath, "debug", `<network-security-config>
    <debug-overrides />
</network-security-config>`)
		m := &rules.Manifest{
			Path: manifestPath,
			Application: &rules.ManifestApplication{
				Line:                  2,
				NetworkSecurityConfig: "@xml/network_security_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("main network security config without debug-overrides is clean", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		writeConfig(t, manifestPath, "main", `<network-security-config>
    <base-config cleartextTrafficPermitted="false" />
</network-security-config>`)
		m := &rules.Manifest{
			Path: manifestPath,
			Application: &rules.ManifestApplication{
				Line:                  2,
				NetworkSecurityConfig: "@xml/network_security_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUnprotectedSMSBroadcastReceiverManifest(t *testing.T) {
	r := findManifestRule(t, "UnprotectedSMSBroadcastReceiverManifest")

	t.Run("SMS receiver without permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".SmsReceiver",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.provider.Telephony.SMS_RECEIVED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("SMS receiver with permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".SmsReceiver",
						Line:                10,
						HasIntentFilter:     true,
						Permission:          "android.permission.BROADCAST_SMS",
						IntentFilterActions: []string{"android.provider.Telephony.SMS_RECEIVED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("receiver without SMS action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".BootReceiver",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.intent.action.BOOT_COMPLETED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUnsafeProtectedBroadcastReceiverManifest(t *testing.T) {
	r := findManifestRule(t, "UnsafeProtectedBroadcastReceiverManifest")

	t.Run("exported receiver with BOOT_COMPLETED no permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".BootReceiver",
						Line:                10,
						HasIntentFilter:     true,
						Exported:            boolPtr(true),
						IntentFilterActions: []string{"android.intent.action.BOOT_COMPLETED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("exported receiver with permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".BootReceiver",
						Line:                10,
						HasIntentFilter:     true,
						Exported:            boolPtr(true),
						Permission:          "com.example.BOOT_PERMISSION",
						IntentFilterActions: []string{"android.intent.action.BOOT_COMPLETED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-exported receiver is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".BootReceiver",
						Line:                10,
						HasIntentFilter:     true,
						Exported:            boolPtr(false),
						IntentFilterActions: []string{"android.intent.action.BOOT_COMPLETED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-protected action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".CustomReceiver",
						Line:                10,
						HasIntentFilter:     true,
						Exported:            boolPtr(true),
						IntentFilterActions: []string{"com.example.CUSTOM_ACTION"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUnsupportedChromeOsHardwareManifest(t *testing.T) {
	r := findManifestRule(t, "UnsupportedChromeOsHardwareManifest")

	t.Run("telephony required triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.telephony", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("telephony without required attribute triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.telephony", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("telephony with required=false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.telephony", Required: "false", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("camera required triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("non-problematic feature is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.wifi", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple unsupported features trigger multiple", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.telephony", Required: "true", Line: 5},
				{Name: "android.hardware.camera", Required: "true", Line: 8},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
}

// writeDeepLinkManifest writes a manifest with one activity holding the given intent-filter XML.
func writeDeepLinkManifest(t *testing.T, filters string) *rules.Manifest {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src", "main", "AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.t">
    <application>
        <activity android:name=".Link" android:exported="true">
` + filters + `
        </activity>
    </application>
</manifest>
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return &rules.Manifest{Path: path, Application: &rules.ManifestApplication{}}
}

const wellFormedWebFilter = `<intent-filter>
    <action android:name="android.intent.action.VIEW"/>
    <category android:name="android.intent.category.DEFAULT"/>
    <category android:name="android.intent.category.BROWSABLE"/>
    <data android:scheme="https" android:host="example.com" android:pathPrefix="/docs"/>
</intent-filter>`

func TestGoogleAppIndexingDeepLinkErrorManifest(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingDeepLinkErrorManifest")
	count := func(filters string) int {
		return len(runManifestRule(r, writeDeepLinkManifest(t, filters)))
	}

	t.Run("well formed web filter is clean", func(t *testing.T) {
		if n := count(wellFormedWebFilter); n != 0 {
			t.Fatalf("expected 0 findings, got %d", n)
		}
	})
	t.Run("scheme and host may be split over data elements", func(t *testing.T) {
		if n := count(`<intent-filter>
    <action android:name="android.intent.action.VIEW"/>
    <category android:name="android.intent.category.DEFAULT"/>
    <category android:name="android.intent.category.BROWSABLE"/>
    <data android:scheme="https"/>
    <data android:host="example.com"/>
</intent-filter>`); n != 0 {
			t.Fatalf("expected 0 findings, got %d", n)
		}
	})
	t.Run("custom scheme is not checked", func(t *testing.T) {
		if n := count(`<intent-filter><action android:name="android.intent.action.VIEW"/><data android:scheme="myapp"/></intent-filter>`); n != 0 {
			t.Fatalf("expected 0 findings, got %d", n)
		}
	})
	t.Run("missing host, action and categories are each reported", func(t *testing.T) {
		if n := count(`<intent-filter><data android:scheme="https"/></intent-filter>`); n != 4 {
			t.Fatalf("expected 4 findings, got %d", n)
		}
	})
	t.Run("each filter is checked on its own", func(t *testing.T) {
		// The second filter has no host even though the first one does.
		if n := count(wellFormedWebFilter + `
<intent-filter>
    <action android:name="android.intent.action.VIEW"/>
    <category android:name="android.intent.category.DEFAULT"/>
    <category android:name="android.intent.category.BROWSABLE"/>
    <data android:scheme="https"/>
</intent-filter>`); n != 1 {
			t.Fatalf("expected 1 finding, got %d", n)
		}
	})
	t.Run("path without leading slash is reported", func(t *testing.T) {
		if n := count(`<intent-filter>
    <action android:name="android.intent.action.VIEW"/>
    <category android:name="android.intent.category.DEFAULT"/>
    <category android:name="android.intent.category.BROWSABLE"/>
    <data android:scheme="https" android:host="example.com" android:path="docs"/>
</intent-filter>`); n != 1 {
			t.Fatalf("expected 1 finding, got %d", n)
		}
	})
}

func TestGoogleAppIndexingWarningManifest(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingWarningManifest")

	t.Run("no deep link activity triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                 "activity",
						Name:                ".MainActivity",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.intent.action.MAIN"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("VIEW with custom scheme but no http triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".DeepLinkActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.VIEW"},
						IntentFilterDataSchemes: []string{"myapp"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("VIEW with https scheme is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".DeepLinkActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.VIEW"},
						IntentFilterDataSchemes: []string{"https"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("VIEW with http scheme is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".DeepLinkActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.VIEW"},
						IntentFilterDataSchemes: []string{"http"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("javatests manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: filepath.Join(t.TempDir(), "javatests", "dagger", "hilt", "android", "internal", "managers", "AndroidManifest.xml"),
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                 "activity",
						Name:                ".TestActivity",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.intent.action.MAIN"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for test manifest, got %d", len(findings))
		}
	})

	t.Run("nil application triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		// nil application means no activities, so no deep link support — but rule returns nil for nil app
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("library module manifest is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: writeAndroidModuleManifestFixture(t, "com.android.library"),
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                 "activity",
						Name:                ".MainActivity",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.intent.action.MAIN"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
		}
	})
}

func TestUseCheckPermissionManifest(t *testing.T) {
	r := findManifestRule(t, "UseCheckPermissionManifest")

	t.Run("exported service with sensitive action and no permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:                 "service",
						Name:                ".MyAccessibilityService",
						Line:                10,
						Exported:            boolPtr(true),
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.accessibilityservice.AccessibilityService"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("exported service with sensitive action and permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:                 "service",
						Name:                ".MyAccessibilityService",
						Line:                10,
						Exported:            boolPtr(true),
						Permission:          "android.permission.BIND_ACCESSIBILITY_SERVICE",
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.accessibilityservice.AccessibilityService"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-exported service with sensitive action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:                 "service",
						Name:                ".MyAccessibilityService",
						Line:                10,
						Exported:            boolPtr(false),
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.accessibilityservice.AccessibilityService"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("exported service with non-sensitive action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:                 "service",
						Name:                ".MyService",
						Line:                10,
						Exported:            boolPtr(true),
						HasIntentFilter:     true,
						IntentFilterActions: []string{"com.example.MY_ACTION"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple sensitive services trigger multiple", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:                 "service",
						Name:                ".MyAccessibilityService",
						Line:                10,
						Exported:            boolPtr(true),
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.accessibilityservice.AccessibilityService"},
					},
					{
						Tag:                 "service",
						Name:                ".MyInputMethodService",
						Line:                20,
						Exported:            boolPtr(true),
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.view.InputMethod"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("nil application is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DeviceAdminManifest
// ---------------------------------------------------------------------------

func TestDeviceAdminManifest(t *testing.T) {
	r := findManifestRule(t, "DeviceAdminManifest")
	run := func(receiver string) []scanner.Finding {
		return runManifestXML(t, r, "p", `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <application>
`+receiver+`  </application>
</manifest>
`)
	}
	const meta = `<meta-data android:name="android.app.device_admin" android:resource="@xml/da"/>`
	const filter = `<intent-filter><action android:name="android.app.action.DEVICE_ADMIN_ENABLED"/></intent-filter>`

	t.Run("meta-data without the enabled intent filter reports at the meta-data name", func(t *testing.T) {
		got := run(`    <receiver android:name=".Dev">` + meta + `</receiver>
`)
		if len(got) != 1 || got[0].Message != "You must have an intent filter for action `android.app.action.DEVICE_ADMIN_ENABLED`" || got[0].Line != 3 {
			t.Fatalf("unexpected findings %+v", got)
		}
	})

	t.Run("filter with data does not count", func(t *testing.T) {
		got := run(`    <receiver android:name=".Dev">` + meta + `<intent-filter><action android:name="android.app.action.DEVICE_ADMIN_ENABLED"/><data android:scheme="x"/></intent-filter></receiver>
`)
		if len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})

	t.Run("meta-data with the enabled filter is clean", func(t *testing.T) {
		if got := run(`    <receiver android:name=".Dev">` + meta + filter + `</receiver>
`); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("receivers without the device admin meta-data are not judged", func(t *testing.T) {
		if got := run(`    <receiver android:name=".Dev" android:permission="android.permission.BIND_DEVICE_ADMIN">` + filter + `</receiver>
    <receiver android:name=".Other"/>
`); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// FullBackupContentManifest
// ---------------------------------------------------------------------------

func TestFullBackupContentManifest(t *testing.T) {
	r := findManifestRule(t, "FullBackupContentManifest")
	run := func(t *testing.T, app *rules.ManifestApplication, files map[string]string) []scanner.Finding {
		t.Helper()
		dir := t.TempDir()
		for name, content := range files {
			p := filepath.Join(dir, "res", "xml", name+".xml")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return runManifestRule(r, &rules.Manifest{Path: filepath.Join(dir, "AndroidManifest.xml"), Application: app})
	}

	t.Run("valid rules are clean", func(t *testing.T) {
		findings := run(t, &rules.ManifestApplication{FullBackupContent: "@xml/rules", DataExtractionRules: "@xml/extract"}, map[string]string{
			"rules":   `<full-backup-content><include domain="sharedpref" path="a.xml"/><exclude domain="database"/></full-backup-content>`,
			"extract": `<data-extraction-rules><cloud-backup><include domain="device_file" path="."/></cloud-backup></data-extraction-rules>`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("unknown domain, missing domain and parent paths are reported", func(t *testing.T) {
		findings := run(t, &rules.ManifestApplication{FullBackupContent: "@xml/rules"}, map[string]string{
			"rules": `<full-backup-content>
<include domain="bogus" path="a"/>
<exclude path="b"/>
<include domain="file" path="../c"/>
</full-backup-content>`,
		})
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})
	t.Run("missing attribute, missing resource and nil application are clean", func(t *testing.T) {
		if findings := run(t, &rules.ManifestApplication{}, nil); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
		if findings := run(t, &rules.ManifestApplication{FullBackupContent: "@xml/elsewhere"}, nil); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
		if findings := runManifestRule(r, &rules.Manifest{Path: "AndroidManifest.xml"}); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ProtectedPermissionsManifest
// ---------------------------------------------------------------------------

func TestProtectedPermissionsManifest(t *testing.T) {
	r := findManifestRule(t, "ProtectedPermissionsManifest")
	entries := func(names ...string) []rules.ManifestUsesPermission {
		var out []rules.ManifestUsesPermission
		for i, n := range names {
			out = append(out, rules.ManifestUsesPermission{Name: n, Line: i + 3})
		}
		return out
	}

	t.Run("system permission triggers with Lint's message", func(t *testing.T) {
		m := &rules.Manifest{
			Path:                  "AndroidManifest.xml",
			UsesPermissionEntries: entries("android.permission.BRICK"),
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 || findings[0].Message != "Permission is only granted to system apps" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("multiple system permissions trigger", func(t *testing.T) {
		m := &rules.Manifest{
			Path:                  "AndroidManifest.xml",
			UsesPermissionEntries: entries("android.permission.INSTALL_PACKAGES", "android.permission.SET_TIME", "android.permission.BIND_ACCESSIBILITY_SERVICE"),
		}
		findings := runManifestRule(r, m)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("permissions grantable on old releases depend on minSdk and maxSdkVersion", func(t *testing.T) {
		count := func(minSdk int, e rules.ManifestUsesPermission) int {
			m := &rules.Manifest{Path: "AndroidManifest.xml", MinSDK: minSdk, UsesPermissionEntries: []rules.ManifestUsesPermission{e}}
			return len(runManifestRule(r, m))
		}
		readLogs := rules.ManifestUsesPermission{Name: "android.permission.READ_LOGS", Line: 3}
		if got := count(21, readLogs); got != 0 {
			t.Errorf("READ_LOGS with minSdk 21 (grantable up to 15): got %d", got)
		}
		if got := count(10, readLogs); got != 1 {
			t.Errorf("READ_LOGS with minSdk 10: got %d", got)
		}
		mock := rules.ManifestUsesPermission{Name: "android.permission.ACCESS_MOCK_LOCATION", Line: 3}
		if got := count(21, mock); got != 1 {
			t.Errorf("ACCESS_MOCK_LOCATION with minSdk 21 (grantable up to 22): got %d", got)
		}
		mock.HasMaxSdk, mock.MaxSdk = true, 22
		if got := count(21, mock); got != 0 {
			t.Errorf("maxSdkVersion at the last grantable level: got %d", got)
		}
		reboot := rules.ManifestUsesPermission{Name: "android.permission.REBOOT", Line: 3, HasMaxSdk: true, MaxSdk: 19}
		if got := count(21, reboot); got != 1 {
			t.Errorf("maxSdkVersion does not excuse a never-grantable permission: got %d", got)
		}
	})

	t.Run("normal permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:                  "AndroidManifest.xml",
			UsesPermissionEntries: entries("android.permission.INTERNET"),
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty permissions is clean", func(t *testing.T) {
		m := &rules.Manifest{Path: "AndroidManifest.xml"}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ServiceExportedManifest
// ---------------------------------------------------------------------------

func TestServiceExportedManifest(t *testing.T) {
	r := findManifestRule(t, "ServiceExportedManifest")

	t.Run("exported service without permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:      "service",
						Name:     ".MyService",
						Line:     10,
						Exported: boolPtr(true),
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("exported service with permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:        "service",
						Name:       ".MyService",
						Line:       10,
						Exported:   boolPtr(true),
						Permission: "com.example.MY_PERMISSION",
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-exported service is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:      "service",
						Name:     ".MyService",
						Line:     10,
						Exported: boolPtr(false),
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("nil application is clean", func(t *testing.T) {
		m := &rules.Manifest{Path: "AndroidManifest.xml"}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MissingRegisteredManifest
// ---------------------------------------------------------------------------

func TestMissingRegisteredManifest(t *testing.T) {
	r := findManifestRule(t, "MissingRegisteredManifest")

	t.Run("empty name triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:  "activity",
						Name: "",
						Line: 10,
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("name starting with digit triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:  "activity",
						Name: "1BadName",
						Line: 10,
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("name with invalid character triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{
						Tag:  "service",
						Name: "com.example.My Service",
						Line: 10,
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("valid fully qualified name is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:  "activity",
						Name: "com.example.MainActivity",
						Line: 10,
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("valid relative name is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:  "activity",
						Name: ".MainActivity",
						Line: 10,
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("nil application is clean", func(t *testing.T) {
		m := &rules.Manifest{Path: "AndroidManifest.xml"}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLocaleConfigMissing(t *testing.T) {
	r := findManifestRule(t, "LocaleConfigMissing")
	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "i18n", "locale-config-missing", "src", "main", "AndroidManifest.xml")
	negativePath := filepath.Join(root, "negative", "i18n", "locale-config-missing", "src", "main", "AndroidManifest.xml")

	t.Run("positive fixture triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: positivePath,
			Application: &rules.ManifestApplication{
				Line:         3,
				LocaleConfig: "@xml/locales_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "res/xml/locales_config.xml") {
			t.Fatalf("expected finding to mention missing locales_config.xml, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: negativePath,
			Application: &rules.ManifestApplication{
				Line:         3,
				LocaleConfig: "@xml/locales_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("generated config and other source sets are clean", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		other := filepath.Join(dir, "src", "release", "res", "xml", "locales_config.xml")
		if err := os.MkdirAll(filepath.Dir(other), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(other, []byte("<locale-config/>"), 0644); err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{"@xml/_generated_res_locale_config", "@xml/locales_config"} {
			m := &rules.Manifest{Path: manifestPath, Application: &rules.ManifestApplication{Line: 3, LocaleConfig: ref}}
			if got := runManifestRule(r, m); len(got) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", ref, len(got))
			}
		}
	})

	t.Run("library module is clean", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte("plugins {\n    id(\"com.android.library\")\n}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, []byte("<manifest package=\"test\" />\n"), 0644); err != nil {
			t.Fatal(err)
		}

		m := &rules.Manifest{
			Path: manifestPath,
			Application: &rules.ManifestApplication{
				Line:         1,
				LocaleConfig: "@xml/locales_config",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// findManifestRule looks up a manifest rule by name from the rule registry.
func findManifestRule(t *testing.T, name string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.Needs.Has(api.NeedsManifest) && r.ID == name {
			return r
		}
	}
	t.Fatalf("manifest rule %q not found in rule registry (NeedsManifest)", name)
	return nil
}

// runManifestRule invokes a manifest rule and returns findings.
// runManifestXML writes xml to a temporary AndroidManifest.xml and runs a rule that reads the manifest tree.
func runManifestXML(t *testing.T, r *api.Rule, pkg, xml string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AndroidManifest.xml")
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &rules.Manifest{Path: path, Package: pkg, Application: &rules.ManifestApplication{}}
	return runManifestRule(r, m)
}

func runManifestRule(r *api.Rule, m *rules.Manifest) []scanner.Finding {
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		Manifest:  m,
		Rule:      r,
		Collector: collector,
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestApplicationManifestRulesSkipLibraryModules(t *testing.T) {
	activity := rules.ManifestComponent{
		Tag:                 "activity",
		Name:                ".MainActivity",
		Line:                10,
		HasIntentFilter:     true,
		IntentFilterActions: []string{"android.intent.action.MAIN"},
	}
	for _, name := range []string{
		"AllowBackupManifest", "MissingVersionManifest", "UsesSdkManifest",
		"RtlEnabledManifest", "GoogleAppIndexingWarningManifest", "InsecureBaseConfigurationManifest",
	} {
		t.Run(name+" library module is clean", func(t *testing.T) {
			r := findManifestRule(t, name)
			m := &rules.Manifest{
				Path:      writeAndroidModuleManifestFixture(t, "com.android.library"),
				TargetSDK: 33,
				Application: &rules.ManifestApplication{
					Line:                 5,
					UsesCleartextTraffic: boolPtr(true),
					Activities:           []rules.ManifestComponent{activity},
				},
			}
			if findings := runManifestRule(r, m); len(findings) != 0 {
				t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
			}
		})
	}
}

func TestGoogleAppIndexingWarningManifestScope(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingWarningManifest")

	t.Run("application module with launcher only triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: writeAndroidModuleManifestFixture(t, "com.android.application"),
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{{
					Tag: "activity", Name: ".MainActivity", Line: 10,
					HasIntentFilter:     true,
					IntentFilterActions: []string{"android.intent.action.MAIN"},
				}},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("manifest without any intent filter is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{{Tag: "activity", Name: ".Detail", Line: 10}},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-application gradle module is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: writeAndroidModuleManifestFixture(t, "java-library"),
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{{
					Tag: "activity", Name: ".MainActivity", Line: 10,
					HasIntentFilter:     true,
					IntentFilterActions: []string{"android.intent.action.MAIN"},
				}},
			},
		}
		if findings := runManifestRule(r, m); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestGoogleAppIndexingDeepLinkErrorResolvesSchemeReferences(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingDeepLinkErrorManifest")
	run := func(scheme string, idx *android.ResourceIndex) int {
		m := writeDeepLinkManifest(t, `<intent-filter>
    <action android:name="android.intent.action.VIEW"/>
    <category android:name="android.intent.category.DEFAULT"/>
    <category android:name="android.intent.category.BROWSABLE"/>
    <data android:scheme="`+scheme+`"/>
</intent-filter>`)
		ctx := &api.Context{Manifest: m, ResourceIndex: idx, Rule: r, Collector: scanner.NewFindingCollector(0)}
		r.Check(ctx)
		return len(api.ContextFindings(ctx))
	}
	idx := &android.ResourceIndex{Strings: map[string]string{"web_scheme": "https", "custom_scheme": "myapp"}}

	if run("@string/web_scheme", idx) != 1 {
		t.Errorf("string reference resolving to https without a host should be reported")
	}
	if run("@string/custom_scheme", idx) != 0 {
		t.Errorf("string reference resolving to a custom scheme should be clean")
	}
	if run("@string/missing", idx) != 0 {
		t.Errorf("unresolvable scheme reference is unknown and should be clean")
	}
	if run("@string/web_scheme", nil) != 0 {
		t.Errorf("scheme reference without a string index is unknown and should be clean")
	}
	if run("${appScheme}", idx) != 0 {
		t.Errorf("manifest placeholder scheme should be clean")
	}
}

func TestExportedWithoutPermissionSkipsLauncherAliasTargetsAndInstrumentation(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")
	build := func(aliasTargets []string, elements []rules.ManifestElement) *rules.Manifest {
		return &rules.Manifest{
			Path:     "AndroidManifest.xml",
			Package:  "com.example.app",
			Elements: elements,
			Application: &rules.ManifestApplication{
				LauncherAliasTargets: aliasTargets,
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".EntryActivity", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
	}
	if n := len(runManifestRule(r, build(nil, nil))); n != 1 {
		t.Fatalf("exported activity without permission should be reported, got %d", n)
	}
	if n := len(runManifestRule(r, build([]string{"com.example.app.EntryActivity"}, nil))); n != 0 {
		t.Errorf("launcher alias target should be skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build([]string{".EntryActivity"}, nil))); n != 0 {
		t.Errorf("relative launcher alias target should be skipped, got %d", n)
	}
	inst := []rules.ManifestElement{{Tag: "instrumentation", ParentTag: "manifest", Line: 3}}
	if n := len(runManifestRule(r, build(nil, inst))); n != 0 {
		t.Errorf("test-only manifest with <instrumentation> should be skipped, got %d", n)
	}
	m := build(nil, nil)
	m.Path = "app/src/androidTest/AndroidManifest.xml"
	if n := len(runManifestRule(r, m)); n != 0 {
		t.Errorf("androidTest manifest should be skipped, got %d", n)
	}
}

func TestExportedPreferenceActivityManifest_SourceSuperclassChain(t *testing.T) {
	r := findManifestRule(t, "ExportedPreferenceActivityManifest")
	dir := t.TempDir()
	write := func(name, src string) *scanner.File {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var f *scanner.File
		var err error
		if strings.HasSuffix(name, ".java") {
			f, err = scanner.ParseJavaFile(context.Background(), p)
		} else {
			f, err = scanner.ParseFile(context.Background(), p)
		}
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	files := []*scanner.File{
		write("Base.kt", "package demo.app\nimport android.preference.PreferenceActivity\nopen class BaseSettings : PreferenceActivity()\n"),
		write("Leaf.kt", "package demo.app\nclass LeafSettings : BaseSettings()\n"),
		write("Plain.kt", "package demo.app\nimport android.app.Activity\nclass ProfilePreferenceActivity : Activity()\n"),
		write("Legacy.java", "package demo.app;\nimport android.preference.*;\npublic class LegacyScreen extends PreferenceActivity {}\n"),
	}
	facts := rules.ComputeActivityPreferenceFacts(files)
	run := func(name string) int {
		m := &rules.Manifest{
			Path:                              "AndroidManifest.xml",
			Package:                           "demo.app",
			ActivityExtendsPreferenceActivity: facts,
			Application: &rules.ManifestApplication{Activities: []rules.ManifestComponent{
				{Tag: "activity", Name: name, Line: 3, Exported: boolPtr(true)},
			}},
		}
		return len(runManifestRule(r, m))
	}
	// Positives: the chain reaches PreferenceActivity even though the name does not say so.
	for _, name := range []string{".BaseSettings", ".LeafSettings", "demo.app.LegacyScreen"} {
		if got := run(name); got != 1 {
			t.Errorf("%s: expected 1 finding, got %d", name, got)
		}
	}
	// Negative: the name ends in PreferenceActivity but the source superclass is a plain Activity.
	if got := run(".ProfilePreferenceActivity"); got != 0 {
		t.Errorf("plain Activity subclass: expected 0 findings, got %d", got)
	}
	// Classes outside project sources keep the simple-name fallback.
	if got := run("com.vendor.LibPreferenceActivity"); got != 1 {
		t.Errorf("library class fallback: expected 1 finding, got %d", got)
	}
}

func TestExportedWithoutPermissionFilterlessActivityInLibraryOrPlaygroundModules(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")
	write := func(root, rel, content string) string {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	build := func(path string, withFilter bool) *rules.Manifest {
		activity := rules.ManifestComponent{Tag: "activity", Name: ".DemoActivity", Line: 10, Exported: boolPtr(true), HasIntentFilter: withFilter}
		return &rules.Manifest{Path: path, Package: "com.example.lib",
			Application: &rules.ManifestApplication{Activities: []rules.ManifestComponent{activity}}}
	}
	root := t.TempDir()
	lib := write(root, "lib/src/main/AndroidManifest.xml", "<manifest/>")
	write(root, "lib/build.gradle.kts", "plugins { id(\"com.android.library\") }\n")
	app := write(root, "app/src/main/AndroidManifest.xml", "<manifest/>")
	write(root, "app/build.gradle.kts", "plugins { id(\"com.android.application\") }\n")
	playground := write(root, "playground/src/main/AndroidManifest.xml", "<manifest/>")

	if n := len(runManifestRule(r, build(lib, false))); n != 0 {
		t.Errorf("filterless exported activity in a library module is skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build(playground, false))); n != 0 {
		t.Errorf("filterless exported activity in a playground module is skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build(app, false))); n != 1 {
		t.Errorf("filterless exported activity in an application module is still reported, got %d", n)
	}
	if n := len(runManifestRule(r, build(lib, true))); n != 1 {
		t.Errorf("exported activity with an intent filter in a library module is still reported, got %d", n)
	}
}

func TestExportedWithoutPermissionSkipsPlaygroundAndLibraryActivitiesWithoutFilters(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")
	build := func(path string, hasFilter bool) *rules.Manifest {
		return &rules.Manifest{
			Path: path,
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".Entry", Line: 10, Exported: boolPtr(true), HasIntentFilter: hasFilter},
				},
			},
		}
	}
	if n := len(runManifestRule(r, build("playground/app/src/main/AndroidManifest.xml", false))); n != 0 {
		t.Errorf("playground activity without an intent-filter should be skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build("playground/app/src/main/AndroidManifest.xml", true))); n != 1 {
		t.Errorf("playground activity with an intent-filter should still be reported, got %d", n)
	}
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	if err := os.MkdirAll(filepath.Join(lib, "src", "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "build.gradle.kts"), []byte(`plugins { id("com.android.library") }`), 0o644); err != nil {
		t.Fatal(err)
	}
	libManifest := filepath.Join(lib, "src", "main", "AndroidManifest.xml")
	if n := len(runManifestRule(r, build(libManifest, false))); n != 0 {
		t.Errorf("library activity without an intent-filter should be skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build(libManifest, true))); n != 1 {
		t.Errorf("library activity with an intent-filter should still be reported, got %d", n)
	}
}

func TestExportedWithoutPermissionSkipsActivityEntryPoints(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")
	build := func(path, tag string, actions, categories []string) *rules.Manifest {
		c := rules.ManifestComponent{Tag: tag, Name: ".Entry", Line: 10, Exported: boolPtr(true), HasIntentFilter: true,
			IntentFilterActions: actions, IntentFilterCategories: categories}
		app := &rules.ManifestApplication{}
		switch tag {
		case "activity":
			app.Activities = []rules.ManifestComponent{c}
		default:
			app.Services = []rules.ManifestComponent{c}
		}
		return &rules.Manifest{Path: path, Application: app}
	}
	for _, action := range []string{
		"android.intent.action.SEND", "android.intent.action.SEND_MULTIPLE", "android.intent.action.VIEW",
		"android.intent.action.MAIN", "android.intent.action.NOTIFICATION_PREFERENCES", "android.intent.action.APPLICATION_PREFERENCES",
	} {
		if n := len(runManifestRule(r, build("AndroidManifest.xml", "activity", []string{action}, nil))); n != 0 {
			t.Errorf("activity with %s should be skipped, got %d", action, n)
		}
	}
	for _, category := range []string{"android.intent.category.DEFAULT", "android.intent.category.BROWSABLE"} {
		if n := len(runManifestRule(r, build("AndroidManifest.xml", "activity", []string{"com.example.CUSTOM"}, []string{category}))); n != 0 {
			t.Errorf("activity with category %s should be skipped, got %d", category, n)
		}
	}
	if n := len(runManifestRule(r, build("AndroidManifest.xml", "activity", []string{"com.example.CUSTOM"}, nil))); n != 1 {
		t.Errorf("activity with only a custom action is still reported, got %d", n)
	}
	if n := len(runManifestRule(r, build("test-support/src/main/AndroidManifest.xml", "activity", nil, nil))); n != 0 {
		t.Errorf("a test-support module manifest should be skipped, got %d", n)
	}
	if n := len(runManifestRule(r, build("feature-core/src/main/AndroidManifest.xml", "activity", nil, nil))); n != 1 {
		t.Errorf("a regular module manifest is still reported, got %d", n)
	}
}

func TestDeepLinkMissingAutoVerify_OnlyViewActionAndLiteralFalse(t *testing.T) {
	r := findManifestRule(t, "DeepLinkMissingAutoVerify")
	dir := t.TempDir()
	path := filepath.Join(dir, "src", "main", "AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body := `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <activity android:name=".A">
            <intent-filter android:autoVerify="${verify}">
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" android:host="a.example.com" />
            </intent-filter>
            <intent-filter>
                <action android:name="android.intent.action.SEND" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" android:host="b.example.com" />
            </intent-filter>
            <intent-filter
                android:label="x"
                android:autoVerify="false">
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" android:host="c.example.com" />
            </intent-filter>
        </activity>
    </application>
</manifest>`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	findings := runManifestRule(r, &rules.Manifest{Path: path})
	if len(findings) != 1 || findings[0].Line != 16 {
		t.Fatalf("expected one finding on the intent-filter start line 16, got %v", findings)
	}
}

func TestCleartextTrafficAndBackupRules_SkipVariantManifests(t *testing.T) {
	cleartext := findManifestRule(t, "CleartextTraffic")
	backup := findManifestRule(t, "BackupRules")
	for _, path := range []string{"app/src/debug/AndroidManifest.xml", "app/src/staging/AndroidManifest.xml"} {
		m := &rules.Manifest{
			Path:        path,
			Application: &rules.ManifestApplication{Line: 3, UsesCleartextTraffic: boolPtr(true)},
		}
		if got := len(runManifestRule(cleartext, m)); got != 0 {
			t.Errorf("CleartextTraffic on %s: expected 0 findings, got %d", path, got)
		}
		if got := len(runManifestRule(backup, m)); got != 0 {
			t.Errorf("BackupRules on %s: expected 0 findings, got %d", path, got)
		}
	}
	main := &rules.Manifest{
		Path:        "app/src/main/AndroidManifest.xml",
		Application: &rules.ManifestApplication{Line: 3, UsesCleartextTraffic: boolPtr(true)},
	}
	if got := len(runManifestRule(cleartext, main)); got != 1 {
		t.Errorf("CleartextTraffic on main manifest: expected 1 finding, got %d", got)
	}
	if got := len(runManifestRule(backup, main)); got != 1 {
		t.Errorf("BackupRules on main manifest: expected 1 finding, got %d", got)
	}
}

func TestManifestAudit_ExportedGateAndFeatureReferences(t *testing.T) {
	t.Run("MissingExportedFlag still reports below targetSdk 31, as a warning", func(t *testing.T) {
		r := findManifestRule(t, "MissingExportedFlag")
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 30,
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{{Tag: "activity", Name: ".A", Line: 3, HasIntentFilter: true}},
			},
		}
		if got := runManifestRule(r, m); len(got) != 1 || got[0].Severity != "warning" {
			t.Fatalf("expected 1 warning below API 31, got %+v", got)
		}
		m.TargetSDK = 33
		if got := runManifestRule(r, m); len(got) != 1 || got[0].Severity != "error" {
			t.Fatalf("expected 1 error at API 33, got %+v", got)
		}
	})

	t.Run("InvalidUsesFeatureRequired accepts references and placeholders", func(t *testing.T) {
		r := findManifestRule(t, "InvalidUsesFeatureRequiredManifest")
		m := &rules.Manifest{Path: "AndroidManifest.xml", UsesFeatures: []rules.ManifestUsesFeature{
			{Name: "a", Required: "@bool/needs_a", Line: 2},
			{Name: "b", Required: "${b}", Line: 3},
			{Name: "c", Required: "TRUE", Line: 4},
			{Name: "d", Required: "maybe", Line: 5},
		}}
		got := runManifestRule(r, m)
		if len(got) != 1 || got[0].Line != 5 {
			t.Fatalf("expected a single finding on line 5, got %v", got)
		}
	})

	t.Run("PermissionImpliesUnsupportedHardware treats unresolved required as declared", func(t *testing.T) {
		r := findManifestRule(t, "PermissionImpliesUnsupportedHardwareManifest")
		m := &rules.Manifest{
			Path:                  "AndroidManifest.xml",
			UsesPermissionEntries: []rules.ManifestUsesPermission{{Name: "android.permission.CAMERA", Line: 3}},
			UsesFeatures:          []rules.ManifestUsesFeature{{Name: "android.hardware.camera", Required: "@bool/cam", Line: 2}},
		}
		if n := len(runManifestRule(r, m)); n != 0 {
			t.Fatalf("expected 0 findings, got %d", n)
		}
	})
}

func TestMissingRegisteredManifestAcceptsPlaceholdersAndUnicode(t *testing.T) {
	r := findManifestRule(t, "MissingRegisteredManifest")
	for name, want := range map[string]int{
		"${applicationId}.Receiver": 0,
		"@string/cls":               0,
		".ui.Ünï":                   0,
		"com.example.Outer$Inner":   0,
		"1bad":                      1,
		"com.example.Bad-Name":      1,
		"":                          1,
	} {
		m := &rules.Manifest{Path: "AndroidManifest.xml", Application: &rules.ManifestApplication{
			Activities: []rules.ManifestComponent{{Tag: "activity", Name: name, Line: 3, ParentTag: "application"}},
		}}
		if got := len(runManifestRule(r, m)); got != want {
			t.Errorf("%q: expected %d findings, got %d", name, want, got)
		}
	}
}

func TestExportedFlagRulesUseGradleTargetSdk(t *testing.T) {
	for _, ruleName := range []string{"MissingExportedFlag", "IntentFilterExportRequired"} {
		r := findManifestRule(t, ruleName)
		for target, want := range map[string]string{"30": "warning", "33": "error"} {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte("android {\n  defaultConfig {\n    targetSdk = "+target+"\n  }\n}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			m := &rules.Manifest{
				Path: filepath.Join(dir, "src", "main", "AndroidManifest.xml"),
				Application: &rules.ManifestApplication{Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".Main", Line: 5, HasIntentFilter: true},
				}},
			}
			if got := runManifestRule(r, m); len(got) != 1 || got[0].Severity != want {
				t.Errorf("%s with gradle targetSdk %s: expected one %s, got %+v", ruleName, target, want, got)
			}
		}
	}
}

func TestFullBackupContentManifest_FollowsLint(t *testing.T) {
	r := findManifestRule(t, "FullBackupContentManifest")
	dir := t.TempDir()
	xmlDir := filepath.Join(dir, "res", "xml")
	if err := os.MkdirAll(xmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	backup := `<?xml version="1.0" encoding="utf-8"?>
<full-backup-content>
  <include domain="sharedpref" path="a/b"/>
  <exclude domain="file" path="b"/>
  <exclude domain="bogus"/>
  <include domain="file" path="../x"/>
  <include domain="file" path="f"/>
  <exclude domain="file" path="f"/>
  <include path="nodomain"/>
  <foo/>
</full-backup-content>
`
	if err := os.WriteFile(filepath.Join(xmlDir, "backup.xml"), []byte(backup), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &rules.Manifest{
		Path:        filepath.Join(dir, "AndroidManifest.xml"),
		Application: &rules.ManifestApplication{FullBackupContent: "@xml/backup"},
	}
	var got []string
	for _, f := range runManifestRule(r, m) {
		got = append(got, f.Message)
	}
	want := []string{
		"Subdirectories are not allowed for domain `sharedpref`",
		"`b` is not in an included path",
		"Paths are not allowed to contain `..`",
		"Include `f` is also excluded",
		"Missing domain attribute, expected one of " + "file, database, sharedpref, external, root, device_file, device_database, device_sharedpref, device_root",
		"Unexpected element `<foo>`",
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDeprecatedManifest_FollowsLint(t *testing.T) {
	r := findManifestRule(t, "DeprecatedManifest")
	got := runManifestXML(t, r, "p", `<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:sharedUserId="x.y">
  <uses-permission-sdk-m android:name="android.permission.CAMERA"/>
  <application>
    <service android:name=".S" android:permission="android.permission.BIND_CHOOSER_TARGET_SERVICE"/>
  </application>
</manifest>
`)
	var msgs []string
	for _, f := range got {
		msgs = append(msgs, f.Message)
	}
	sort.Strings(msgs)
	want := []string{
		"ChooserTargetService` is deprecated: Please see https://developer.android.com/training/sharing/receive.html?source=studio#providing-direct-share-targets",
		"Consider removing `sharedUserId` for new users by adding `android:sharedUserMaxSdkVersion=\"32\"` to your manifest. See https://developer.android.com/guide/topics/manifest/manifest-element for details.",
		"`uses-permission-sdk-m` is deprecated: Use `uses-permission-sdk-23 instead",
	}
	if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(msgs, "\n"), strings.Join(want, "\n"))
	}
	clean := runManifestXML(t, r, "p", `<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:sharedUserId="x.y" android:sharedUserMaxSdkVersion="32"/>
`)
	if len(clean) != 0 {
		t.Fatalf("sharedUserMaxSdkVersion must silence the warning, got %+v", clean)
	}
}

func TestUsesSdkManifestIsOffByDefault(t *testing.T) {
	// Lint 32 has no UsesMinSdkAttributes check any more; MultipleUsesSdk still exists and stays on.
	if rules.IsDefaultActive("UsesSdkManifest") {
		t.Error("UsesSdkManifest should be off by default")
	}
	if !rules.IsDefaultActive("MultipleUsesSdkManifest") {
		t.Error("MultipleUsesSdkManifest should stay on by default")
	}
	findManifestRule(t, "UsesSdkManifest")
}

func TestExportedFlagFollowsLint(t *testing.T) {
	const missing = "As of Android 12, `android:exported` must be set; use `true` to make the activity available to other apps, and `false` otherwise."
	launcher := rules.ManifestComponent{Tag: "activity", Name: ".Main", Line: 5, HasIntentFilter: true,
		FirstFilterAction: "android.intent.action.MAIN", FirstFilterCategory: "android.intent.category.LAUNCHER"}
	for _, ruleName := range []string{"MissingExportedFlag", "IntentFilterExportRequired"} {
		r := findManifestRule(t, ruleName)
		run := func(target int, comps ...rules.ManifestComponent) []scanner.Finding {
			m := &rules.Manifest{Path: "AndroidManifest.xml", TargetSDK: target, Application: &rules.ManifestApplication{Activities: comps}}
			return runManifestRule(r, m)
		}
		if got := run(34, launcher); len(got) != 1 || got[0].Message != missing+" For launcher activities, this should be set to true." || got[0].Severity != "error" {
			t.Errorf("%s: launcher without exported at target 34: %+v", ruleName, got)
		}
		if got := run(30, launcher); len(got) != 1 || got[0].Severity != "warning" {
			t.Errorf("%s: below target 31 the finding is a warning: %+v", ruleName, got)
		}
		plain := rules.ManifestComponent{Tag: "activity", Name: ".Plain", Line: 6, HasIntentFilter: true}
		if got := run(34, plain); len(got) != 1 || got[0].Message != missing {
			t.Errorf("%s: plain component message: %+v", ruleName, got)
		}
		navGraph := rules.ManifestComponent{Tag: "activity", Name: ".Nav", Line: 7, HasNavGraph: true}
		if got := run(34, navGraph); len(got) != 1 {
			t.Errorf("%s: a nav-graph counts as an intent filter: %+v", ruleName, got)
		}
		unreadable := rules.ManifestComponent{Tag: "activity", Name: ".Ref", Line: 8, HasIntentFilter: true, ExportedRaw: "@bool/exported"}
		if got := run(34, unreadable); len(got) != 0 {
			t.Errorf("%s: a referenced exported value is set: %+v", ruleName, got)
		}
		hidden := launcher
		hidden.ExportedRaw = "false"
		if got := run(34, hidden); len(got) != 1 || got[0].Message != "A launchable activity must be exported as of Android 12, which also makes it available to other apps." {
			t.Errorf("%s: launchable but not exported: %+v", ruleName, got)
		}
	}
}
