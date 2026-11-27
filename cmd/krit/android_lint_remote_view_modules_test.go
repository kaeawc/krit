package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// RemoteViewLayout reads the layout the calling module sees: its own resource overrides a library's layout of the
// same name, and a layout only a library declares still comes from that library.
func TestRemoteViewLayoutReadsTheCallingModulesLayout(t *testing.T) {
	const editText = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="match_parent">
    <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" />
</LinearLayout>
`
	const textView = `<TextView xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="wrap_content" android:layout_height="wrap_content" />
`
	const widget = `package com.example

import android.content.Context
import android.widget.RemoteViews

fun own(context: Context) = RemoteViews(context.packageName, R.layout.widget)

fun fromLibrary(context: Context) = RemoteViews(context.packageName, R.layout.lib_only)
`
	// The app is either an app/ module next to lib/, or the root project itself.
	for _, layout := range []struct{ name, app, settings string }{
		{"app module", "app/", "include(\":app\", \":lib\")\n"},
		{"root module", "", "include(\":lib\")\n"},
	} {
		t.Run(layout.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			for rel, content := range map[string]string{
				"settings.gradle.kts": layout.settings,
				layout.app + "build.gradle.kts": "plugins { id(\"com.android.application\") }\n" +
					"android { namespace = \"com.example\" }\ndependencies { implementation(project(\":lib\")) }\n",
				layout.app + "src/main/AndroidManifest.xml":        "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n",
				layout.app + "src/main/res/layout/widget.xml":      textView,
				layout.app + "src/main/java/com/example/Widget.kt": widget,
				"lib/build.gradle.kts":                             "plugins { id(\"com.android.library\") }\nandroid { namespace = \"com.example.lib\" }\n",
				"lib/src/main/AndroidManifest.xml":                 "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n",
				"lib/src/main/res/layout/widget.xml":               editText,
				"lib/src/main/res/layout/lib_only.xml":             editText,
			} {
				path := filepath.Join(project, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stdout, stderr, _ := runKrit(t, "--no-daemon", "--no-fir", "--no-cache", "-f", "json", project)
			var report struct {
				Findings []struct {
					Rule    string `json:"rule"`
					Message string `json:"message"`
				} `json:"findings"`
			}
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
			}
			var got []string
			for _, f := range report.Findings {
				if f.Rule == "RemoteViewLayout" {
					got = append(got, f.Message)
				}
			}
			sort.Strings(got)
			want := []string{"`@layout/lib_only` includes views not allowed in a `RemoteView`: EditText"}
			if len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("RemoteViewLayout findings = %q, want %q", got, want)
			}
		})
	}
}
