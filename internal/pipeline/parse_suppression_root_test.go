package pipeline

import (
	"context"
	"path/filepath"
	"testing"
)

// A resident file keeps its suppression filter across daemon requests, but
// exclude globs match the path relative to the project root, so a request
// under another root rebuilds the filter instead of reusing the first
// root's verdict (#1834).
func TestInstallSourceSuppressionRebuildsForAnotherRoot(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	module := filepath.Join(repo, "module")
	path := filepath.Join(module, "src", "main", "kotlin", "Foo.kt")
	f, err := ParseSingle(context.Background(), path, []byte("class Foo\n"))
	if err != nil {
		t.Fatal(err)
	}
	excludes := map[string][]string{"R": {"**/module/**"}}

	installSourceSuppression(f, excludes, nil, repo)
	if !f.Suppression.IsSuppressedAt("R", "s", 1, -1) {
		t.Fatal("under the repository root, module/ is inside the project and must exclude")
	}
	installSourceSuppression(f, excludes, nil, module)
	if f.Suppression.IsSuppressedAt("R", "s", 1, -1) {
		t.Fatal("under the module root, the filter built for the repository root was reused")
	}
	first := f.Suppression
	installSourceSuppression(f, excludes, nil, module)
	if f.Suppression != first {
		t.Fatal("a filter built under the same root was rebuilt")
	}
}
