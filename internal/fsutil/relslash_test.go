package fsutil

import (
	"path/filepath"
	"testing"
)

func TestRelSlash(t *testing.T) {
	cases := []struct{ root, path, want string }{
		{"/r", "/r/a/b.kt", "a/b.kt"},
		{"/r", "/other/b.kt", "/other/b.kt"},
		{"/r", "/r", "."},
		{"/r", "/rx/a.kt", "/rx/a.kt"},
		{"/r", "/r/..generated/Foo.kt", "..generated/Foo.kt"},
		{"/r/a", "/r", "/r"},
	}
	for _, c := range cases {
		if got := RelSlash(c.root, c.path); got != c.want {
			t.Errorf("RelSlash(%q,%q)=%q want %q", c.root, c.path, got, c.want)
		}
	}
}

func TestCommonDir(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		{nil, ""},
		{[]string{"/r/a/build.gradle"}, "/r/a"},
		{[]string{"/r/a/build.gradle", "/r/b/build.gradle", "/r/a/x/y.gradle"}, "/r"},
		{[]string{"/r/ab/x", "/r/a/y"}, "/r"},
	} {
		var paths []string
		for _, p := range tc.paths {
			paths = append(paths, filepath.FromSlash(p))
		}
		if got := CommonDir(paths); got != filepath.FromSlash(tc.want) {
			t.Errorf("CommonDir(%v) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}
