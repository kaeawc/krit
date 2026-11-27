package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const daemonNewManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
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

// A manifest or res directory added to a module changes the project's shape. A warm daemon run must pick it up as a
// one-shot --no-cache run does, without waiting for a build-script edit (#1903). It arrives as a manifest file in an
// existing directory, inside a src/main moved into the tree (which delivers no event for its files), or as a new res
// directory holding only a drawable.
func TestDaemonPicksUpANewManifest(t *testing.T) {
	buildDaemonBinary(t)
	for _, arrival := range []struct {
		name string
		// sourceSet holds the module's Kotlin source; watched is the base name of the arriving file whose findings
		// are compared; manifestFirst writes the manifest before the daemon starts; rules are extra flags, such as
		// opt-in rules that report on the arriving file.
		sourceSet, watched string
		manifestFirst      bool
		rules              []string
	}{
		{name: "file", sourceSet: "main", watched: "AndroidManifest.xml"},
		{name: "directory", sourceSet: "debug", watched: "AndroidManifest.xml"},
		{name: "drawable", sourceSet: "main", watched: "ic.png", manifestFirst: true,
			rules: []string{"--enable-rules", "ConvertToWebp,IconLocation"}},
	} {
		t.Run(arrival.name, func(t *testing.T) {
			repo := shortTempDir(t)
			files := map[string]string{
				"settings.gradle.kts": "include(\":app\")\n",
				"app/build.gradle.kts": "plugins { id(\"com.android.application\") }\nandroid {\n    namespace = \"com.example\"\n" +
					"    compileSdk = 36\n    defaultConfig {\n        minSdk = 24\n        targetSdk = 35\n    }\n}\n",
				"app/src/" + arrival.sourceSet + "/java/com/example/A.kt": "package com.example\n\nclass A\n",
			}
			if arrival.manifestFirst {
				files["app/src/main/AndroidManifest.xml"] = daemonNewManifest
			}
			for rel, content := range files {
				path := filepath.Join(repo, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s"); code != 0 {
				log, _ := os.ReadFile(filepath.Join(repo, ".krit", "daemon.log"))
				t.Fatalf("daemon start failed: exit=%d log=%q", code, log)
			}
			t.Cleanup(func() { _, _, _ = runKrit(t, "daemon", "stop", "--repo", repo) })

			findings := func(daemon bool, args ...string) string {
				t.Helper()
				// Without --no-fir a box with no JDK runs the scan in-process instead of on the daemon.
				flags := append([]string{"--no-type-oracle", "--no-fir", "-f", "json"}, arrival.rules...)
				stdout, stderr, _ := runKrit(t, append(append(flags, args...), repo)...)
				if daemon && !strings.Contains(stderr, "info: using daemon") {
					t.Fatalf("the scan did not run on the daemon:\n%s", stderr)
				}
				var report struct {
					Findings []struct {
						File string `json:"file"`
						Rule string `json:"rule"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				var got []string
				for _, f := range report.Findings {
					if filepath.Base(f.File) == arrival.watched {
						got = append(got, f.Rule)
					}
				}
				sort.Strings(got)
				return strings.Join(got, " ")
			}
			if got := findings(true); got != "" {
				t.Fatalf("daemon run before %s exists reported findings on it: %q", arrival.watched, got)
			}

			switch arrival.name {
			case "file":
				if err := os.WriteFile(filepath.Join(repo, "app", "src", "main", "AndroidManifest.xml"), []byte(daemonNewManifest), 0o644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				staged := filepath.Join(shortTempDir(t), "main")
				if err := os.MkdirAll(staged, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staged, "AndroidManifest.xml"), []byte(daemonNewManifest), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(staged, filepath.Join(repo, "app", "src", "main")); err != nil {
					t.Fatal(err)
				}
			case "drawable":
				icon := filepath.Join(repo, "app", "src", "main", "res", "drawable", "ic.png")
				if err := os.MkdirAll(filepath.Dir(icon), 0o755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(icon)
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 96, 96))); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			want := findings(false, "--no-daemon", "--no-cache")
			if want == "" {
				t.Fatalf("--no-daemon --no-cache reported no findings on %s; the test project no longer exercises it", arrival.watched)
			}
			// The watcher reports asynchronously; give it a moment, as an editor's next save would.
			var got string
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(200 * time.Millisecond) {
				if got = findings(true); got == want || time.Now().After(deadline) {
					break
				}
			}
			if got != want {
				t.Fatalf("daemon run after %s arrived = %q, --no-daemon --no-cache = %q", arrival.watched, got, want)
			}
		})
	}
}
