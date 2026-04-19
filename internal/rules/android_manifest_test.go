package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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

	t.Run("allowBackup missing triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: nil,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("debuggable false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:       5,
				Debuggable: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

func TestExportedWithoutPermission(t *testing.T) {
	r := findManifestRule(t, "ExportedWithoutPermission")

	t.Run("exported without permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{Tag: "service", Name: ".MyService", Line: 10, Exported: boolPtr(true)},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	t.Run("duplicate triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10},
					{Tag: "activity", Name: ".MainActivity", Line: 20},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unique activities is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10},
					{Tag: "activity", Name: ".DetailActivity", Line: 20},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWrongManifestParentManifest(t *testing.T) {
	r := findManifestRule(t, "WrongManifestParentManifest")

	t.Run("activity under manifest triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Elements: []rules.ManifestElement{
				{Tag: "activity", Line: 10, ParentTag: "manifest"},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("uses-sdk under manifest is clean", func(t *testing.T) {
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
}

func TestGradleOverridesManifest(t *testing.T) {
	r := findManifestRule(t, "GradleOverridesManifest")

	t.Run("minSdk in manifest triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:    "AndroidManifest.xml",
			MinSDK:  21,
			UsesSdk: &rules.ManifestElement{Tag: "uses-sdk", Line: 3},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("both minSdk and targetSdk triggers twice", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			MinSDK:    21,
			TargetSDK: 34,
			UsesSdk:   &rules.ManifestElement{Tag: "uses-sdk", Line: 3},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
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

	t.Run("drawable icon triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line: 5,
				Icon: "@drawable/ic_launcher",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("mipmap icon is clean", func(t *testing.T) {
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

	t.Run("no icon is clean", func(t *testing.T) {
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

func TestUniquePermission(t *testing.T) {
	r := findManifestRule(t, "UniquePermission")

	t.Run("system permission collision triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:        "AndroidManifest.xml",
			Permissions: []string{"android.permission.CAMERA"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("custom permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:        "AndroidManifest.xml",
			Permissions: []string{"com.example.MY_PERMISSION"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no permissions is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSystemPermission(t *testing.T) {
	r := findManifestRule(t, "SystemPermission")

	t.Run("dangerous permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiple dangerous permissions trigger multiple", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA", "android.permission.RECORD_AUDIO"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("safe permission is clean", func(t *testing.T) {
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

func TestExportedServiceManifest(t *testing.T) {
	r := findManifestRule(t, "ExportedServiceManifest")

	t.Run("exported service without permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Services: []rules.ManifestComponent{
					{Tag: "service", Name: ".MyService", Line: 10, Exported: boolPtr(true)},
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
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("duplicate uses-feature triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "true", Line: 5},
				{Name: "android.hardware.camera", Required: "false", Line: 10},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unique features is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "true", Line: 5},
				{Name: "android.hardware.bluetooth", Required: "true", Line: 10},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no features is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
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

	t.Run("both missing triggers single combined finding", func(t *testing.T) {
		findings := runManifestRule(r, appManifest("", ""))
		if len(findings) != 1 {
			t.Fatalf("expected 1 combined finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "versionCode") ||
			!strings.Contains(findings[0].Message, "versionName") {
			t.Fatalf("expected combined message mentioning both attributes, got %q", findings[0].Message)
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

	t.Run("missing supportsRtl triggers", func(t *testing.T) {
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

	t.Run("supportsRtl=false triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				SupportsRtl: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("supportsRtl=true is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Line:        5,
				SupportsRtl: boolPtr(true),
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
}

func TestInvalidUsesTagAttributeManifest(t *testing.T) {
	r := findManifestRule(t, "InvalidUsesTagAttributeManifest")

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
					{Tag: "activity", Name: ".SettingsActivity", Line: 10, HasIntentFilter: true},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	t.Run("API 28+ missing networkSecurityConfig triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 28,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

func TestRtlCompatManifest(t *testing.T) {
	r := findManifestRule(t, "RtlCompatManifest")

	t.Run("API 17+ without supportsRtl triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 17,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("API 17+ with supportsRtl=false triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 21,
			Application: &rules.ManifestApplication{
				Line:        5,
				SupportsRtl: boolPtr(false),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("API 17+ with supportsRtl=true is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 33,
			Application: &rules.ManifestApplication{
				Line:        5,
				SupportsRtl: boolPtr(true),
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("API below 17 is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 16,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no targetSDK is clean", func(t *testing.T) {
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

func TestAppIndexingErrorManifest(t *testing.T) {
	r := findManifestRule(t, "AppIndexingErrorManifest")

	t.Run("VIEW with http only triggers (missing https)", func(t *testing.T) {
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("VIEW with both http and https is clean", func(t *testing.T) {
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
						IntentFilterDataSchemes: []string{"http", "https"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("VIEW with custom scheme is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".CustomSchemeActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.VIEW"},
						IntentFilterDataSchemes: []string{"myapp"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-VIEW activity is clean", func(t *testing.T) {
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
			t.Fatalf("expected 0 findings for library manifest, got %d", len(findings))
		}
	})
}

func TestMissingLeanbackLauncherManifest(t *testing.T) {
	r := findManifestRule(t, "MissingLeanbackLauncherManifest")

	t.Run("leanback feature without LEANBACK_LAUNCHER triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
			},
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                    "activity",
						Name:                   ".MainActivity",
						Line:                   10,
						HasIntentFilter:        true,
						IntentFilterCategories: []string{"android.intent.category.LAUNCHER"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("leanback feature with LEANBACK_LAUNCHER is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
			},
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                    "activity",
						Name:                   ".TvActivity",
						Line:                   10,
						HasIntentFilter:        true,
						IntentFilterCategories: []string{"android.intent.category.LEANBACK_LAUNCHER"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no leanback feature is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{Tag: "activity", Name: ".MainActivity", Line: 10},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("leanback feature no application triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

func TestPermissionImpliesUnsupportedHardwareManifest(t *testing.T) {
	r := findManifestRule(t, "PermissionImpliesUnsupportedHardwareManifest")

	t.Run("CAMERA permission without optional feature triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("CAMERA with required=false feature is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA"},
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "false", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("CAMERA with required=true feature triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.CAMERA"},
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no mapped permissions is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.INTERNET"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple permissions same feature triggers once", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.BLUETOOTH", "android.permission.BLUETOOTH_ADMIN"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

func TestAppIndexingWarningManifest(t *testing.T) {
	r := findManifestRule(t, "AppIndexingWarningManifest")

	t.Run("browsable without VIEW action triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                    "activity",
						Name:                   ".DeepLinkActivity",
						Line:                   10,
						HasIntentFilter:        true,
						IntentFilterActions:    []string{"android.intent.action.MAIN"},
						IntentFilterCategories: []string{"android.intent.category.BROWSABLE"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("browsable with VIEW action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                    "activity",
						Name:                   ".DeepLinkActivity",
						Line:                   10,
						HasIntentFilter:        true,
						IntentFilterActions:    []string{"android.intent.action.VIEW"},
						IntentFilterCategories: []string{"android.intent.category.BROWSABLE"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no browsable category is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                    "activity",
						Name:                   ".MainActivity",
						Line:                   10,
						HasIntentFilter:        true,
						IntentFilterActions:    []string{"android.intent.action.MAIN"},
						IntentFilterCategories: []string{"android.intent.category.LAUNCHER"},
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
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestGoogleAppIndexingDeepLinkErrorManifest(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingDeepLinkErrorManifest")

	t.Run("http scheme without host triggers", func(t *testing.T) {
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("custom scheme without host is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".CustomSchemeActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.VIEW"},
						IntentFilterDataSchemes: []string{"myapp"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("scheme with host is clean", func(t *testing.T) {
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
						IntentFilterDataHosts:   []string{"example.com"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no scheme is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                 "activity",
						Name:                ".DeepLinkActivity",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.intent.action.VIEW"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-VIEW activity is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Activities: []rules.ManifestComponent{
					{
						Tag:                     "activity",
						Name:                    ".MainActivity",
						Line:                    10,
						HasIntentFilter:         true,
						IntentFilterActions:     []string{"android.intent.action.MAIN"},
						IntentFilterDataSchemes: []string{"myapp"},
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
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

func TestMissingLeanbackSupportManifest(t *testing.T) {
	r := findManifestRule(t, "MissingLeanbackSupportManifest")

	t.Run("leanback without touchscreen opt-out triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("leanback with touchscreen required=true triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
				{Name: "android.hardware.touchscreen", Required: "true", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("leanback with touchscreen required=false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.software.leanback", Line: 3},
				{Name: "android.hardware.touchscreen", Required: "false", Line: 5},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no leanback feature is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesFeatures: []rules.ManifestUsesFeature{
				{Name: "android.hardware.camera", Line: 3},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("receiver with DEVICE_ADMIN action but no meta-data triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".MyAdminReceiver",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.app.action.DEVICE_ADMIN_ENABLED"},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !testing.Short() {
			t.Logf("finding: %s", findings[0].Message)
		}
	})

	t.Run("receiver with DEVICE_ADMIN action and meta-data is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".MyAdminReceiver",
						Line:                10,
						HasIntentFilter:     true,
						IntentFilterActions: []string{"android.app.action.DEVICE_ADMIN_ENABLED"},
						MetaDataEntries: []rules.ManifestMetaData{
							{
								Name:     "android.app.device_admin",
								Resource: "@xml/device_admin",
							},
						},
					},
				},
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("receiver without DEVICE_ADMIN action is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			Application: &rules.ManifestApplication{
				Receivers: []rules.ManifestComponent{
					{
						Tag:                 "receiver",
						Name:                ".MyReceiver",
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

	t.Run("nil application is clean", func(t *testing.T) {
		m := &rules.Manifest{Path: "AndroidManifest.xml"}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// FullBackupContentManifest
// ---------------------------------------------------------------------------

func TestFullBackupContentManifest(t *testing.T) {
	r := findManifestRule(t, "FullBackupContentManifest")

	t.Run("allowBackup true targetSdk 23 missing fullBackupContent triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 23,
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

	t.Run("allowBackup nil targetSdk 23 missing fullBackupContent triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 23,
			Application: &rules.ManifestApplication{
				Line: 5,
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("allowBackup false is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 23,
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

	t.Run("fullBackupContent set is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 23,
			Application: &rules.ManifestApplication{
				Line:              5,
				AllowBackup:       boolPtr(true),
				FullBackupContent: "@xml/backup_rules",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("targetSdk below 23 is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:      "AndroidManifest.xml",
			TargetSDK: 22,
			Application: &rules.ManifestApplication{
				Line:        5,
				AllowBackup: boolPtr(true),
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
// ProtectedPermissionsManifest
// ---------------------------------------------------------------------------

func TestProtectedPermissionsManifest(t *testing.T) {
	r := findManifestRule(t, "ProtectedPermissionsManifest")

	t.Run("system permission triggers", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.BRICK"},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiple system permissions trigger", func(t *testing.T) {
		m := &rules.Manifest{
			Path: "AndroidManifest.xml",
			UsesPermissions: []string{
				"android.permission.INSTALL_PACKAGES",
				"android.permission.SET_TIME",
			},
		}
		findings := runManifestRule(r, m)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("normal permission is clean", func(t *testing.T) {
		m := &rules.Manifest{
			Path:            "AndroidManifest.xml",
			UsesPermissions: []string{"android.permission.INTERNET"},
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

// findManifestRule looks up a manifest rule by name from the v2 registry.
func findManifestRule(t *testing.T, name string) *v2rules.Rule {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.Needs.Has(v2rules.NeedsManifest) && r.ID == name {
			return r
		}
	}
	t.Fatalf("manifest rule %q not found in v2 Registry (NeedsManifest)", name)
	return nil
}

// runManifestRule invokes a v2 manifest rule and returns findings.
func runManifestRule(r *v2rules.Rule, m *rules.Manifest) []scanner.Finding {
	ctx := &v2rules.Context{
		Manifest: m,
		Rule:     r,
	}
	r.Check(ctx)
	return ctx.Findings
}
