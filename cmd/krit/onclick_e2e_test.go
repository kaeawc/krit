package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// OnClick checks a layout's android:onClick handlers against the Activity
// that inflates it. As a resource rule it never received the parsed sources,
// so it reported nothing in a real scan (#1860). Its verdict follows the
// Activity, so a Kotlin-only edit must change a warm run's result too.
func TestOnClickReportsEndToEndAndFollowsSourceEdits(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.gradle.kts", "include(\":app\")\n")
	write("app/build.gradle.kts", "plugins { id(\"com.android.application\") }\n\nandroid {\n    namespace = \"p\"\n    compileSdk = 34\n}\n")
	write("app/src/main/AndroidManifest.xml", "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"p\"><application/></manifest>\n")
	write("app/src/main/res/layout/activity_main.xml", "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <Button android:id=\"@+id/submit\" android:onClick=\"onSubmitClicked\" />\n</LinearLayout>\n")
	const activity = "app/src/main/kotlin/p/MainActivity.kt"
	const head = "package p\n\nimport android.app.Activity\nimport android.os.Bundle\nimport android.view.View\n\nclass MainActivity : Activity() {\n    override fun onCreate(savedInstanceState: Bundle?) {\n        super.onCreate(savedInstanceState)\n        setContentView(R.layout.activity_main)\n    }\n"
	without := head + "}\n"
	with := head + "\n    fun onSubmitClicked(v: View) {}\n}\n"

	scan := func(extra ...string) string {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "--enable-rules", "OnClick", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []scopeFinding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		var keys []string
		for _, f := range report.Findings {
			if f.Rule == "OnClick" {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " ")
	}
	for _, s := range []struct {
		name, content, want string
	}{
		{"handler missing", without, "activity_main.xml:3"},
		{"handler added", with, ""},
		{"handler removed again", without, "activity_main.xml:3"},
	} {
		write(activity, s.content)
		warm := scan()
		fresh := scan("--no-cache")
		if fresh != s.want {
			t.Fatalf("%s: fresh OnClick findings [%s], want [%s]", s.name, fresh, s.want)
		}
		if warm != fresh {
			t.Fatalf("%s: warm OnClick findings [%s], fresh [%s]", s.name, warm, fresh)
		}
	}
}
