package fsutil

import "testing"

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
