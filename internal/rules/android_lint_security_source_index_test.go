package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/scanner"
)

func lintSourceIndexForTest(t *testing.T, sources map[string]string) *lintSourceIndex {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	for name, body := range sources {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return lintSourceClasses(files)
}

func TestLintManifestClassWithoutPackage(t *testing.T) {
	idx := lintSourceIndexForTest(t, map[string]string{
		"A.kt": "package com.a\n\nclass Main\nclass Shared\nclass Outer {\n    class Inner\n}\n",
		"B.kt": "package com.b\n\nclass Shared\nclass Other\n",
	})
	noPackage := &manifest.Manifest{}
	for _, tc := range []struct{ name, wantFQN string }{
		{".Main", "com.a.Main"},
		{"Main", "com.a.Main"},
		{".Outer$Inner", "com.a.Outer.Inner"},
		{".Inner", "com.a.Outer.Inner"},
		// Two classes end with .Shared: no match, as before the index.
		{".Shared", ""},
		{".Missing", ""},
		// A qualified name is not matched by suffix.
		{"com.c.Other", ""},
	} {
		fqn, cls := lintManifestClass(noPackage, tc.name, idx)
		got := ""
		if cls != nil {
			got = fqn
		}
		if got != tc.wantFQN {
			t.Errorf("lintManifestClass(%q) = %q, want %q", tc.name, got, tc.wantFQN)
		}
	}
	if _, cls := lintManifestClass(&manifest.Manifest{Package: "com.b"}, ".Other", idx); cls == nil {
		t.Error("lintManifestClass(.Other) with package com.b found nothing")
	}
}

func TestLintSourceIndexExtendsTarget(t *testing.T) {
	idx := lintSourceIndexForTest(t, map[string]string{
		"Base.kt": "package com.a\n\nimport androidx.appcompat.app.AppCompatActivity\n\nopen class Base : AppCompatActivity()\n",
		"Screen.kt": "package com.a\n\nclass Screen : Base()\n" +
			// A supertype cycle through sources must terminate.
			"open class Loop1 : Loop2()\nopen class Loop2 : Loop1()\n",
		"Svc.kt": "package com.b\n\nimport com.a.Base\n\nclass Leaf : Base()\nclass Plain\n",
	})
	for _, tc := range []struct {
		fqn     string
		targets map[string]bool
		want    bool
	}{
		{"com.a.Screen", activityBaseClasses, true},
		{"com.b.Leaf", activityBaseClasses, true},
		{"com.b.Leaf", serviceBaseClasses, false},
		{"com.b.Plain", activityBaseClasses, false},
		{"com.a.Loop1", activityBaseClasses, false},
	} {
		c := idx.classes[tc.fqn]
		if c == nil {
			t.Fatalf("class %s not indexed", tc.fqn)
		}
		// The second call reads the memo; both must agree.
		for range 2 {
			if got := idx.extendsTarget(c, tc.targets); got != tc.want {
				t.Errorf("extendsTarget(%s) = %v, want %v", tc.fqn, got, tc.want)
			}
		}
	}
	if len(idx.extends) != 5 {
		t.Errorf("memo holds %d entries, want one per (class, target set) = 5", len(idx.extends))
	}
}

func TestLintSourceFileClassesAreSharedPerRun(t *testing.T) {
	prev := fileFactsCache()
	t.Cleanup(func() { setSharedFileFacts(prev) })
	setSharedFileFacts(filefacts.NewCache())
	sources := map[string]string{
		"A.kt": "package com.a\n\nimport android.preference.PreferenceActivity\n\nopen class Settings : PreferenceActivity()\n",
		"B.kt": "package com.a\n\nclass Child : Settings()\nclass Plain\n",
	}
	first := lintSourceIndexForTest(t, sources)
	var files []*scanner.File
	for _, c := range first.classes {
		files = append(files, c.file)
	}
	// Another rule in the same run reuses each file's parsed scope and declarations.
	second := lintSourceClasses(files)
	for fqn, c := range first.classes {
		if second.classes[fqn] != c {
			t.Errorf("class %s was parsed again for the second index", fqn)
		}
	}
	facts := ComputeActivityPreferenceFacts(files)
	for fqn, want := range map[string]bool{"com.a.Settings": true, "com.a.Child": true, "com.a.Plain": false} {
		if got, ok := facts[fqn]; !ok || got != want {
			t.Errorf("preference fact for %s = %v (present %v), want %v", fqn, got, ok, want)
		}
	}
}
