package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// A manifest added to a module that had none changes the project's shape. A warm daemon run must pick it up as a
// one-shot --no-cache run does, without waiting for a build-script edit (#1903). The manifest arrives either as a
// file in an existing directory or inside a directory moved into the tree, which delivers no event for its files.
func TestDaemonPicksUpANewManifest(t *testing.T) {
	buildDaemonBinary(t)
	for _, arrival := range []string{"file", "directory"} {
		t.Run(arrival, func(t *testing.T) {
			repo := shortTempDir(t)
			for rel, content := range map[string]string{
				"settings.gradle.kts": "include(\":app\")\n",
				"app/build.gradle.kts": "plugins { id(\"com.android.application\") }\nandroid {\n    namespace = \"com.example\"\n" +
					"    compileSdk = 36\n    defaultConfig {\n        minSdk = 24\n        targetSdk = 35\n    }\n}\n",
			} {
				path := filepath.Join(repo, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// The directory case moves a whole src/main in, so the module's source lives in another source set.
			sourceSet := "main"
			if arrival == "directory" {
				sourceSet = "debug"
			}
			source := filepath.Join(repo, "app", "src", sourceSet, "java", "com", "example", "A.kt")
			if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte("package com.example\n\nclass A\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s"); code != 0 {
				log, _ := os.ReadFile(filepath.Join(repo, ".krit", "daemon.log"))
				t.Fatalf("daemon start failed: exit=%d log=%q", code, log)
			}
			t.Cleanup(func() { _, _, _ = runKrit(t, "daemon", "stop", "--repo", repo) })

			manifestFindings := func(args ...string) string {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-type-oracle", "-f", "json"}, args...), repo)...)
				var report struct {
					Findings []struct {
						File string `json:"file"`
						Rule string `json:"rule"`
						Line int    `json:"line"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				var got []string
				for _, f := range report.Findings {
					if filepath.Base(f.File) == "AndroidManifest.xml" {
						got = append(got, f.Rule)
					}
				}
				sort.Strings(got)
				return strings.Join(got, " ")
			}
			if got := manifestFindings(); got != "" {
				t.Fatalf("daemon run before the manifest exists reported manifest findings: %q", got)
			}

			const manifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <activity android:name=".Main" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
			if arrival == "file" {
				if err := os.WriteFile(filepath.Join(repo, "app", "src", "main", "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				staged := filepath.Join(shortTempDir(t), "main")
				if err := os.MkdirAll(staged, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staged, "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(staged, filepath.Join(repo, "app", "src", "main")); err != nil {
					t.Fatal(err)
				}
			}
			want := manifestFindings("--no-daemon", "--no-cache")
			if want == "" {
				t.Fatal("--no-daemon --no-cache reported no manifest findings; the test project no longer exercises the manifest rules")
			}
			// The watcher reports asynchronously; give it a moment, as an editor's next save would.
			var got string
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(200 * time.Millisecond) {
				if got = manifestFindings(); got == want || time.Now().After(deadline) {
					break
				}
			}
			if got != want {
				t.Fatalf("daemon run after adding the manifest = %q, --no-daemon --no-cache = %q", got, want)
			}
		})
	}
}
