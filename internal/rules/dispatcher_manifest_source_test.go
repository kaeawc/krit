package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/manifest"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// manifestSourceProbe reports once in every source file and once at line 1 of
// every manifest, so a test can see which side's suppression applied.
func manifestSourceProbe(seen *[]string) *api.Rule {
	r := api.FakeRule("ManifestSourceProbe", api.WithNeeds(api.NeedsManifest|api.NeedsParsedFiles), api.WithCheck(func(ctx *api.Context) {
		for _, m := range ctx.Manifests {
			*seen = append(*seen, "manifest:"+filepath.Base(filepath.Dir(m.Path)))
			ctx.Emit(scanner.Finding{File: m.Path, Line: 3, Col: 5, Rule: "ManifestSourceProbe", Message: "manifest"})
		}
		for _, f := range ctx.ParsedFiles {
			*seen = append(*seen, "source:"+filepath.Base(f.Path))
			ctx.Emit(scanner.Finding{File: f.Path, Line: 3, Col: 1, Rule: "ManifestSourceProbe", Message: "source"})
		}
	}))
	r.Languages = []scanner.Language{scanner.LangKotlin, scanner.LangJava}
	return r
}

func writeManifestSourceFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunManifestSource_SeesEveryManifestAndSourceFile(t *testing.T) {
	dir := t.TempDir()
	ktPath := filepath.Join(dir, "A.kt")
	writeManifestSourceFile(t, ktPath, "package a\n\nclass A\n")
	kt, err := scanner.ParseFile(context.Background(), ktPath)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	d := NewDispatcher([]*api.Rule{manifestSourceProbe(&seen)}, nil)
	if got := len(d.ManifestSourceRules()); got != 1 {
		t.Fatalf("ManifestSourceRules = %d, want 1", got)
	}
	manifests := []*manifest.Manifest{
		{Path: filepath.Join(dir, "lib", "AndroidManifest.xml")},
		{Path: filepath.Join(dir, "app", "AndroidManifest.xml")},
	}
	cols, ok := d.RunManifestSource(manifests, []*scanner.File{kt})
	if !ok {
		t.Fatal("RunManifestSource reported a recovered panic")
	}
	want := []string{"manifest:app", "manifest:lib", "source:A.kt"}
	if len(seen) != len(want) {
		t.Fatalf("rule saw %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("rule saw %v, want %v (manifests sorted by path)", seen, want)
		}
	}
	if cols.Len() != 3 {
		t.Fatalf("findings = %d, want 3", cols.Len())
	}
}

func TestRunManifestSource_AppliesEachTargetFilesSuppression(t *testing.T) {
	dir := t.TempDir()
	ktPath := filepath.Join(dir, "A.kt")
	writeManifestSourceFile(t, ktPath, "package a\n\n@Suppress(\"ManifestSourceProbe\")\nclass A\n")
	plainPath := filepath.Join(dir, "B.kt")
	writeManifestSourceFile(t, plainPath, "package a\n\nclass B\n")
	ignored := filepath.Join(dir, "app", "AndroidManifest.xml")
	writeManifestSourceFile(t, ignored, `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">
    <application tools:ignore="ManifestSourceProbe" />
</manifest>
`)
	plain := filepath.Join(dir, "lib", "AndroidManifest.xml")
	writeManifestSourceFile(t, plain, `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="a" />
    <application />
</manifest>
`)
	var files []*scanner.File
	for _, p := range []string{ktPath, plainPath} {
		f, err := scanner.ParseFile(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	var seen []string
	d := NewDispatcher([]*api.Rule{manifestSourceProbe(&seen)}, nil)
	cols, _ := d.RunManifestSource([]*manifest.Manifest{{Path: ignored}, {Path: plain}}, files)
	got := map[string]bool{}
	for _, f := range cols.Findings() {
		got[f.File] = true
	}
	if got[ktPath] {
		t.Error("@Suppress on the source declaration did not silence the source finding")
	}
	if !got[plainPath] {
		t.Error("unsuppressed source finding was dropped")
	}
	if got[ignored] {
		t.Error("tools:ignore on <application> did not silence the manifest finding")
	}
	if !got[plain] {
		t.Error("unsuppressed manifest finding was dropped")
	}
}

func TestRunManifestSource_NoManifestsSkipsRules(t *testing.T) {
	var seen []string
	d := NewDispatcher([]*api.Rule{manifestSourceProbe(&seen)}, nil)
	cols, ok := d.RunManifestSource(nil, nil)
	if !ok || cols.Len() != 0 || len(seen) != 0 {
		t.Fatalf("expected no run without manifests, got ok=%v findings=%d seen=%v", ok, cols.Len(), seen)
	}
}
