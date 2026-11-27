package scanner

import (
	"path/filepath"
	"testing"
)

// The fixtures sit under t.TempDir() so they are absolute on Windows too,
// where a path without a volume is not.
func TestRootRelativePath(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ci", "vendor", "repo")
	cases := []struct{ name, root, path, want string }{
		{"under the root", root, filepath.Join(root, "app", "A.kt"), "/app/A.kt"},
		{"the root itself", root, root, "/"},
		{"outside the root", root, filepath.Join(base, "x", "A.kt"), filepath.Join(base, "x", "A.kt")},
		{"no root", "", filepath.Join(root, "app", "A.kt"), filepath.Join(root, "app", "A.kt")},
		{"relative path", root, filepath.Join("app", "A.kt"), filepath.Join("app", "A.kt")},
	}
	for _, tc := range cases {
		if got := RootRelativePath(tc.root, tc.path); got != tc.want {
			t.Errorf("%s: RootRelativePath = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A rule's exclude glob matches only folders inside the project, so a
// checkout under vendor/ is not excluded wholesale and the verdict does not
// change when the checkout moves (#1834).
func TestSuppressionFilterExcludesMatchRootRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ci", "vendor", "repo")
	excludes := map[string][]string{"R": {"**/vendor/**"}}
	outside := &File{Path: filepath.Join(root, "app", "A.kt")}
	if !BuildSuppressionFilterAt(outside, nil, excludes, "", "").IsSuppressedAt("R", "s", 1, -1) {
		t.Fatal("without a root the absolute path still matches, as before")
	}
	if BuildSuppressionFilterAt(outside, nil, excludes, "", root).IsSuppressedAt("R", "s", 1, -1) {
		t.Fatal("a vendor/ folder above the project root excluded the file")
	}
	inside := &File{Path: filepath.Join(root, "vendor", "lib", "B.kt")}
	if !BuildSuppressionFilterAt(inside, nil, excludes, "", root).IsSuppressedAt("R", "s", 1, -1) {
		t.Fatal("a vendor/ folder inside the project must still exclude")
	}
}

// A configured marker spelled as an absolute path inside the root takes the
// root-relative form; any other marker is left as given.
func TestRootRelativeMarker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	slashRoot := filepath.ToSlash(root)
	cases := []struct{ name, marker, want string }{
		{"absolute inside the root", filepath.Join(root, "tools") + string(filepath.Separator), "/tools/"},
		{"slash-separated inside the root", slashRoot + "/spec/", "/spec/"},
		{"the root itself", root, "/"},
		{"relative marker", "/tools/", "/tools/"},
		{"sibling sharing a prefix", slashRoot + "-other/tools/", slashRoot + "-other/tools/"},
		{"no marker", "", ""},
	}
	for _, tc := range cases {
		if got := RootRelativeMarker(root, tc.marker); got != tc.want {
			t.Errorf("%s: RootRelativeMarker(%q) = %q, want %q", tc.name, tc.marker, got, tc.want)
		}
	}
	if got := RootRelativeMarker("", slashRoot+"/tools/"); got != slashRoot+"/tools/" {
		t.Errorf("without a root the marker changed to %q", got)
	}
}
