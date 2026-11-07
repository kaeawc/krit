package fileignore

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory without its own .gitignore must give the same answers as the rules it inherits.
func TestIgnoredInheritsParentRulesThroughDirectoriesWithoutAGitignore(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(".gitignore", "build/\n*.log\n!keep.log\n")
	write("mod/.gitignore", "/gen\n")
	write("mod/src/main/kotlin/A.kt", "")
	write("mod/src/main/kotlin/B.log", "")
	write("mod/src/main/kotlin/keep.log", "")
	write("mod/src/main/kotlin/build/C.kt", "")
	write("mod/gen/G.kt", "")
	write("other/src/D.kt", "")
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	m := MatcherForPath(root, info, nil)
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"mod/src/main/kotlin/A.kt", false, false},
		{"mod/src/main/kotlin/B.log", false, true},
		{"mod/src/main/kotlin/keep.log", false, false},
		{"mod/src/main/kotlin/build", true, true},
		{"mod/gen", true, true},
		{"mod/src/gen", true, false},
		{"other/src/D.kt", false, false},
	}
	for round := 0; round < 2; round++ { // the second round answers from the shared caches
		for _, c := range cases {
			if got := m.Ignored(filepath.Join(root, filepath.FromSlash(c.rel)), c.isDir); got != c.want {
				t.Errorf("round %d: Ignored(%s) = %v, want %v", round, c.rel, got, c.want)
			}
		}
	}
}
