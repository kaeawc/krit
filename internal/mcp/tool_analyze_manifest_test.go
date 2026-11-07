package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProjectFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func analyzeAndroidManifest(t *testing.T, root string) string {
	t.Helper()
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: root, Scope: "manifest"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	return string(data)
}

const preferenceManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">
  <application>
    <activity android:name=".%s" android:exported="true"/>
  </application>
</manifest>`

func TestAnalyzeAndroidManifestResolvesPreferenceActivitySuperclass(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "src/main/AndroidManifest.xml", strings.Replace(preferenceManifest, "%s", "SettingsScreen", 1))
	writeProjectFile(t, root, "src/main/kotlin/com/example/app/SettingsScreen.kt", `package com.example.app
import android.preference.PreferenceActivity
open class BaseSettings : PreferenceActivity()
class SettingsScreen : BaseSettings()
`)
	if out := analyzeAndroidManifest(t, root); !strings.Contains(out, "ExportedPreferenceActivityManifest") {
		t.Fatalf("expected the source superclass chain to flag SettingsScreen: %s", out)
	}
}

func TestAnalyzeAndroidManifestIgnoresLookalikePreferenceActivityName(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "src/main/AndroidManifest.xml", strings.Replace(preferenceManifest, "%s", "ProfilePreferenceActivity", 1))
	writeProjectFile(t, root, "src/main/kotlin/com/example/app/ProfilePreferenceActivity.kt", `package com.example.app
import androidx.appcompat.app.AppCompatActivity
class ProfilePreferenceActivity : AppCompatActivity()
`)
	if out := analyzeAndroidManifest(t, root); strings.Contains(out, "ExportedPreferenceActivityManifest") {
		t.Fatalf("a class that only has the name must not be flagged: %s", out)
	}
}
