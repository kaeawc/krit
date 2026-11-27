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
	p := newOnClickProject(t)
	for _, s := range []struct {
		name, content, want string
	}{
		{"handler missing", onClickActivityWithout, "activity_main.xml:3"},
		{"handler added", onClickActivityWith, ""},
		{"handler removed again", onClickActivityWithout, "activity_main.xml:3"},
	} {
		p.write(onClickActivity, s.content)
		warm := p.scan()
		fresh := p.scan("--no-cache")
		if fresh != s.want {
			t.Fatalf("%s: fresh OnClick findings [%s], want [%s]", s.name, fresh, s.want)
		}
		if warm != fresh {
			t.Fatalf("%s: warm OnClick findings [%s], fresh [%s]", s.name, warm, fresh)
		}
	}
}

// A warm run whose sources are all cache hits skips the parse when the
// cross-file findings are reusable. A res edit still re-runs OnClick, which
// must then see every source, parsed on demand, not the empty parse (#1860).
func TestOnClickFollowsResourceEditsAfterAWarmParseSkip(t *testing.T) {
	p := newOnClickProject(t)
	p.extra = []string{"--config", allCategoriesOffConfig(t), "--enable-rules", "OnClick,DeadCode", "-v"}
	p.write(onClickActivity, onClickActivityWithout)
	p.write("app/src/main/res/values/strings.xml", "<resources>\n    <string name=\"title\">One</string>\n</resources>\n")
	if got := p.scan(); got != "activity_main.xml:3" {
		t.Fatalf("cold OnClick findings [%s], want [activity_main.xml:3]", got)
	}
	for _, s := range []struct{ name, path, content string }{
		{"values edit", "app/src/main/res/values/strings.xml", "<resources>\n    <string name=\"title\">Two</string>\n</resources>\n"},
		{"drawable added", "app/src/main/res/drawable/dot.xml", "<shape xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n"},
	} {
		p.write(s.path, s.content)
		warm := p.scan()
		if !strings.Contains(p.stderr, "Skipped parse") {
			t.Fatalf("%s: the warm run parsed, so it no longer covers the parse skip:\n%s", s.name, p.stderr)
		}
		if fresh := p.scan("--no-cache"); warm != fresh {
			t.Fatalf("%s: warm OnClick findings [%s], fresh [%s]", s.name, warm, fresh)
		}
	}
}

const onClickActivity = "app/src/main/kotlin/p/MainActivity.kt"

const onClickActivityHead = "package p\n\nimport android.app.Activity\nimport android.os.Bundle\nimport android.view.View\n\nclass MainActivity : Activity() {\n    override fun onCreate(savedInstanceState: Bundle?) {\n        super.onCreate(savedInstanceState)\n        setContentView(R.layout.activity_main)\n    }\n"

const (
	onClickActivityWithout = onClickActivityHead + "}\n"
	onClickActivityWith    = onClickActivityHead + "\n    fun onSubmitClicked(v: View) {}\n}\n"
)

// onClickProject is an Android app whose layout names an onClick handler.
type onClickProject struct {
	t      *testing.T
	dir    string
	extra  []string
	stderr string
}

func newOnClickProject(t *testing.T) *onClickProject {
	p := &onClickProject{t: t, dir: t.TempDir()}
	p.write("settings.gradle.kts", "include(\":app\")\n")
	p.write("app/build.gradle.kts", "plugins { id(\"com.android.application\") }\n\nandroid {\n    namespace = \"p\"\n    compileSdk = 34\n}\n")
	p.write("app/src/main/AndroidManifest.xml", "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"p\"><application/></manifest>\n")
	p.write("app/src/main/res/layout/activity_main.xml", "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <Button android:id=\"@+id/submit\" android:onClick=\"onSubmitClicked\" />\n</LinearLayout>\n")
	return p
}

func (p *onClickProject) write(name, content string) {
	p.t.Helper()
	path := filepath.Join(p.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

// scan returns the OnClick findings as sorted file:line keys.
func (p *onClickProject) scan(extra ...string) string {
	p.t.Helper()
	rules := p.extra
	if rules == nil {
		rules = []string{"--enable-rules", "OnClick"}
	}
	args := append(append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "-f", "json", "-q"}, rules...), extra...)
	stdout, stderr, code := runKrit(p.t, append(args, p.dir)...)
	if code > 1 {
		p.t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
	}
	p.stderr = stderr
	var report struct {
		Findings []scopeFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		p.t.Fatalf("invalid JSON for %v: %v", extra, err)
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
