package rules

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestProjectPath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci", "junit-kit", "app")
	facts := filefacts.NewCache()
	facts.SetProjectRoot(root)
	ctx := &api.Context{Facts: facts}
	cases := []struct{ name, path, want string }{
		{"file under the root", filepath.Join(root, "sub", "Foo.kt"), "/sub/Foo.kt"},
		{"the root itself", root, "/"},
		{"outside the root", filepath.Join(string(filepath.Separator), "elsewhere", "Foo.kt"), filepath.Join(string(filepath.Separator), "elsewhere", "Foo.kt")},
		{"sibling sharing a prefix", root + "-other" + string(filepath.Separator) + "Foo.kt", root + "-other" + string(filepath.Separator) + "Foo.kt"},
		{"relative path", filepath.Join("sub", "Foo.kt"), filepath.Join("sub", "Foo.kt")},
	}
	for _, tc := range cases {
		if got := projectPath(ctx, tc.path); got != tc.want {
			t.Errorf("%s: projectPath(%q) = %q, want %q", tc.name, tc.path, got, tc.want)
		}
	}
	// Without a recorded root (LSP, single-file dispatch), paths stay as given.
	abs := filepath.Join(root, "sub", "Foo.kt")
	for _, c := range []*api.Context{nil, {}, {Facts: filefacts.NewCache()}} {
		if got := projectPath(c, abs); got != abs {
			t.Errorf("no root: projectPath = %q, want %q", got, abs)
		}
	}
}

// The heuristics read only the folders inside the project once it has a root.
func TestPathHeuristicsStopAtProjectRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci1", "junit-kit", "kotlin", "app")
	facts := filefacts.NewCache()
	facts.SetProjectRoot(root)
	ctx := &api.Context{Facts: facts}

	// The root's own name and its parent ("kotlin") no longer satisfy a
	// package: sub/ declaring app.sub, and a root file declaring x.app.
	if dir := projectPath(ctx, filepath.Join(root, "sub")); packageOmitsCommonRoot(dir, "app.sub") {
		t.Errorf("packageOmitsCommonRoot(%q, app.sub) = true; the root's name must not count", dir)
	}
	if dir := projectPath(ctx, root); packageOmitsCommonRoot(dir, "x.app") {
		t.Errorf("packageOmitsCommonRoot(%q, x.app) = true; the parent folder must not count as a source root", dir)
	}
	// A real source root inside the project still does (ktor's common/src/).
	if dir := projectPath(ctx, filepath.Join(root, "common", "src")); !packageOmitsCommonRoot(dir, "io.ktor.client") {
		t.Errorf("packageOmitsCommonRoot(%q, io.ktor.client) = false; common/src/ is a source root", dir)
	}

	// junit-kit above the root is not a test-framework module; one inside is.
	if p := projectPath(ctx, filepath.Join(root, "lib", "src", "main", "kotlin", "p", "A.kt")); isTestFrameworkModulePath(p) {
		t.Errorf("isTestFrameworkModulePath(%q) = true; the checkout's ancestors must not count", p)
	}
	if p := projectPath(ctx, filepath.Join(root, "junit-support", "src", "main", "kotlin", "p", "A.kt")); !isTestFrameworkModulePath(p) {
		t.Errorf("isTestFrameworkModulePath(%q) = false; junit-support is a test-framework module", p)
	}
}
