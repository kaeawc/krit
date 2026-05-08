package suggestreviewers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCodeowners_ParsesAndAppliesLastMatchWins(t *testing.T) {
	co := ParseCodeowners(`
# Default owner
* @org/everyone

# Frontend
apps/web/ @org/web @org/design

# Most specific wins
apps/web/login/ @org/auth
`)

	cases := map[string][]string{
		"README.md":                         {"@org/everyone"},
		"apps/web/index.tsx":                {"@org/web", "@org/design"},
		"apps/web/login/LoginScreen.tsx":    {"@org/auth"},
		"apps/native/ios/AppDelegate.swift": {"@org/everyone"},
	}
	for path, want := range cases {
		got := co.Owners(path)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Owners(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestParseCodeowners_BasenamePatternMatchesAnywhere(t *testing.T) {
	co := ParseCodeowners(`
*.kt @org/kotlin
`)
	if got := co.Owners("apps/feature/foo/Bar.kt"); !reflect.DeepEqual(got, []string{"@org/kotlin"}) {
		t.Errorf("nested .kt match = %v", got)
	}
	if got := co.Owners("Top.kt"); !reflect.DeepEqual(got, []string{"@org/kotlin"}) {
		t.Errorf("root .kt match = %v", got)
	}
	if got := co.Owners("README.md"); got != nil {
		t.Errorf("non-match = %v, want nil", got)
	}
}

func TestParseCodeowners_DoubleStarMatchesAcrossSegments(t *testing.T) {
	co := ParseCodeowners(`
apps/**/feature.kt @org/feature
`)
	if got := co.Owners("apps/web/x/y/feature.kt"); !reflect.DeepEqual(got, []string{"@org/feature"}) {
		t.Errorf("nested ** match = %v", got)
	}
	if got := co.Owners("apps/feature.kt"); !reflect.DeepEqual(got, []string{"@org/feature"}) {
		t.Errorf("zero-segment ** match = %v, want @org/feature", got)
	}
}

func TestParseCodeowners_IgnoresCommentsAndBlankLines(t *testing.T) {
	co := ParseCodeowners(`
# top comment
   # indented comment

src/ @org/src    # trailing comment is stripped
`)
	if got := co.Owners("src/x.kt"); !reflect.DeepEqual(got, []string{"@org/src"}) {
		t.Errorf("Owners = %v", got)
	}
}

func TestFindCodeownersFile_PrefersRootThenGithubThenDocs(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "CODEOWNERS"), "* @docs\n")
	if got := FindCodeownersFile(dir); got != filepath.Join(dir, "docs", "CODEOWNERS") {
		t.Errorf("found = %s", got)
	}
	mustWrite(t, filepath.Join(dir, ".github", "CODEOWNERS"), "* @gh\n")
	if got := FindCodeownersFile(dir); got != filepath.Join(dir, ".github", "CODEOWNERS") {
		t.Errorf("found = %s", got)
	}
	mustWrite(t, filepath.Join(dir, "CODEOWNERS"), "* @root\n")
	if got := FindCodeownersFile(dir); got != filepath.Join(dir, "CODEOWNERS") {
		t.Errorf("found = %s", got)
	}
}

func TestFindCodeownersFile_NoneFound(t *testing.T) {
	dir := t.TempDir()
	if got := FindCodeownersFile(dir); got != "" {
		t.Errorf("found unexpected file: %s", got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
