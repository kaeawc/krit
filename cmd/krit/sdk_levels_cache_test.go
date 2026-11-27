package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const sdkLevelsCacheTile = `package com.example

import android.content.Intent
import android.service.quicksettings.TileService

class MyTile : TileService() {
    fun go(intent: Intent) {
        startActivityAndCollapse(intent)
    }
}
`

const sdkLevelsCacheCheck = `package com.example

import android.os.Build

fun legacy(): Boolean {
    if (Build.VERSION.SDK_INT >= 21) {
        return true
    }
    return false
}
`

func sdkLevelsCacheBuildScript(minSdk, targetSdk string) string {
	return `plugins { id("com.android.library") }
android {
    namespace = "com.example"
    compileSdk = 36
    defaultConfig {
        minSdk = ` + minSdk + `
        targetSdk = ` + targetSdk + `
    }
}
`
}

// A library module needs no AndroidManifest.xml (AGP 8), so its SDK levels come only from the build script. A warm
// run must follow an edit to minSdk or targetSdk there exactly as a cold run does (#1856).
func TestSDKLevelEditsWithoutManifestReachWarmRuns(t *testing.T) {
	edits := []struct {
		name, minSdk, targetSdk, wantBefore, wantAfter string
	}{
		{"targetSdk 34 to 33", "26", "33",
			"ObsoleteSdkInt:warning StartActivityAndCollapseDeprecated:error",
			"ObsoleteSdkInt:warning StartActivityAndCollapseDeprecated:warning"},
		{"minSdk 26 to 19", "19", "34",
			"ObsoleteSdkInt:warning StartActivityAndCollapseDeprecated:error",
			"StartActivityAndCollapseDeprecated:error"},
	}
	for _, edit := range edits {
		t.Run(edit.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			script := filepath.Join(project, "app", "build.gradle.kts")
			for rel, content := range map[string]string{
				"settings.gradle.kts":                       "include(\":app\")\n",
				"app/build.gradle.kts":                      sdkLevelsCacheBuildScript("26", "34"),
				"app/src/main/java/com/example/MyTile.kt":   sdkLevelsCacheTile,
				"app/src/main/java/com/example/Obsolete.kt": sdkLevelsCacheCheck,
			} {
				path := filepath.Join(project, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			findings := func(args ...string) string {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--no-fir", "-f", "json"}, args...), project)...)
				var report struct {
					Findings []struct {
						Rule     string `json:"rule"`
						Severity string `json:"severity"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				var got []string
				for _, f := range report.Findings {
					if f.Rule == "ObsoleteSdkInt" || f.Rule == "StartActivityAndCollapseDeprecated" {
						got = append(got, f.Rule+":"+f.Severity)
					}
				}
				sort.Strings(got)
				return strings.Join(got, " ")
			}
			if got := findings(); got != edit.wantBefore {
				t.Fatalf("cold run = %q, want %q", got, edit.wantBefore)
			}
			if got := findings(); got != edit.wantBefore {
				t.Fatalf("warm run before the edit = %q, want %q", got, edit.wantBefore)
			}
			if err := os.WriteFile(script, []byte(sdkLevelsCacheBuildScript(edit.minSdk, edit.targetSdk)), 0o644); err != nil {
				t.Fatal(err)
			}
			warm, cold := findings(), findings("--no-cache")
			if cold != edit.wantAfter || warm != cold {
				t.Fatalf("after the edit: warm run %q, --no-cache run %q, want %q", warm, cold, edit.wantAfter)
			}
		})
	}
}
