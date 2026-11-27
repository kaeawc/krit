package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// #1885: RunProject indexes Java classes into the source resolver, so Kotlin
// rules see a Kotlin class or receiver that reaches a framework type through
// a Java source class; and a warm run follows an edit to only that Java
// class's supertype exactly as --no-cache does.
func TestRunProject_KotlinRulesSeeJavaSourceSupertypes(t *testing.T) {
	ids := map[string]bool{"DenyListedApi": true, "DoNotCallViewToString": true}
	var active []*api.Rule
	for _, r := range api.Registry {
		if ids[r.ID] {
			active = append(active, r)
		}
	}
	if len(active) != len(ids) {
		t.Fatalf("found %d of the rules %v", len(active), ids)
	}
	const (
		jBaseView  = "package com.ex;\n\nimport android.content.Context;\nimport android.widget.LinearLayout;\n\npublic class JBase extends LinearLayout {\n  public JBase(Context c) { super(c); }\n}\n"
		jBasePlain = "package com.ex;\n\nimport android.content.Context;\n\npublic class JBase {\n  public JBase(Context c) {}\n  public void setOnClickListener(Object l) {}\n}\n"
		use        = "package com.ex\n\nfun clear(b: JBase) {\n  b.setOnClickListener(null)\n}\n"
		kView      = "package com.ex\n\nimport android.content.Context\n\nclass KView(c: Context) : JBase(c) {\n  override fun toString(): String = \"k\"\n}\n\nfun f(k: KView) = k.toString()\n"
	)
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("JBase.java", jBaseView)
	write("Use.kt", use)
	write("KView.kt", kView)
	cachePath := filepath.Join(dir, ".krit-test", "krit.cache")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(useCache bool) string {
		t.Helper()
		host := ProjectHostState{}
		if useCache {
			host.AnalysisCache = cache.Load(cachePath)
			host.AnalysisCacheFilePath = cachePath
			host.AnalysisCacheLookup = true
		}
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}, ActiveRules: active, Format: "json", Version: "test"},
			Host: host,
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		var out []string
		for _, f := range res.FinalFindings.Findings() {
			if ids[f.Rule] {
				out = append(out, fmt.Sprintf("%s:%d %s", filepath.Base(f.File), f.Line, f.Rule))
			}
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}
	const want = "KView.kt:9 DoNotCallViewToString, Use.kt:4 DenyListedApi"
	if got := run(false); got != want {
		t.Fatalf("--no-cache findings = %q, want %q", got, want)
	}
	if got := run(true); got != want {
		t.Fatalf("cold findings = %q, want %q", got, want)
	}
	write("JBase.java", jBasePlain)
	if cold, warm := run(false), run(true); cold != "" || warm != cold {
		t.Fatalf("after JBase stops extending LinearLayout: warm %q, --no-cache %q, want both empty", warm, cold)
	}
	write("JBase.java", jBaseView)
	if cold, warm := run(false), run(true); cold != want || warm != cold {
		t.Fatalf("after JBase extends LinearLayout again: warm %q, --no-cache %q, want %q", warm, cold, want)
	}
}

// testdata/java_resolver holds one project per directory: Kotlin and Java sources and an expected.txt listing the
// findings ("File.kt:line Rule", sorted) RunProject reports with CanvasSize, the rule the #1950 review's repros
// misfired. Each runs from a copy, so no cache is written into the tree.
func TestRunProject_JavaResolverFixtures(t *testing.T) {
	var active []*api.Rule
	for _, r := range api.Registry {
		if r.ID == "CanvasSize" {
			active = append(active, r)
		}
	}
	if len(active) != 1 {
		t.Fatal("CanvasSize not registered")
	}
	cases, err := os.ReadDir(filepath.Join("testdata", "java_resolver"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		t.Run(c.Name(), func(t *testing.T) {
			src := filepath.Join("testdata", "java_resolver", c.Name())
			dir := t.TempDir()
			if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(src, "expected.txt"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := RunProject(context.Background(), ProjectInput{
				Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}, ActiveRules: active, Format: "json", Version: "test"},
			})
			if err != nil {
				t.Fatalf("RunProject: %v", err)
			}
			var got []string
			for _, f := range res.FinalFindings.Findings() {
				rel, _ := filepath.Rel(dir, f.File)
				got = append(got, fmt.Sprintf("%s:%d %s", filepath.ToSlash(rel), f.Line, f.Rule))
			}
			sort.Strings(got)
			if g, w := strings.Join(got, "\n"), strings.TrimSpace(string(want)); g != w {
				t.Errorf("findings:\n%s\nwant:\n%s", g, w)
			}
		})
	}
}
