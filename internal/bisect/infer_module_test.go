package bisect

import (
	"testing"

	"github.com/kaeawc/krit/internal/snapshot"
)

// TestInferModuleFromTestPathMatchesAbsolutePrefix exercises the happy
// path: a test file under a module's src/test directory should
// resolve to that module via the prefix-suffix match.
func TestInferModuleFromTestPathMatchesAbsolutePrefix(t *testing.T) {
	blob := &snapshot.Blob{
		Modules: []snapshot.Module{
			{Path: ":app", Dir: "/repo/app"},
		},
	}
	got := inferModuleFromTestPath(blob, "/repo/app/src/test/kotlin/foo/Bar.kt")
	if got != ":app" {
		t.Errorf("expected :app, got %q", got)
	}
}

func TestInferModuleFromTestPathMatchesRelativeDir(t *testing.T) {
	blob := &snapshot.Blob{
		Modules: []snapshot.Module{
			{Path: ":app", Dir: "./app"},
		},
	}
	got := inferModuleFromTestPath(blob, "/repo/app/src/test/kotlin/Bar.kt")
	if got != ":app" {
		t.Errorf("expected :app, got %q", got)
	}
}

// TestInferModuleFromTestPathRejectsBackwardsSuffixMatch is the
// regression for the bug: the previous middle clause
// `HasSuffix(m.Dir, prefix)` would attribute a test file under one
// module to any unrelated module whose Dir happened to end with the
// test-file's parent prefix string. With that clause removed, only
// the actually-owning module wins.
func TestInferModuleFromTestPathRejectsBackwardsSuffixMatch(t *testing.T) {
	// Two modules: :owner whose dir matches the test-file prefix,
	// and :unrelated whose Dir is exactly equal to the test-file
	// prefix string (so the buggy clause `HasSuffix(m.Dir, prefix)`
	// also matched). The buggy code would return whichever module
	// came first in the slice; the fixed code returns only :owner.
	blob := &snapshot.Blob{
		Modules: []snapshot.Module{
			// Put the lookalike first so the buggy iteration would
			// have returned it. The prefix equals m.Dir, which
			// trivially matches both `m.Dir == prefix` and the
			// backwards `HasSuffix(m.Dir, prefix)` clause. We must
			// also keep the third clause from matching: trimming
			// "./" gives "lookalike/repo/owner", which is not a
			// suffix of the prefix.
			{Path: ":lookalike", Dir: "./lookalike/repo/owner"},
			{Path: ":owner", Dir: "/repo/owner"},
		},
	}
	got := inferModuleFromTestPath(blob, "/repo/owner/src/test/kotlin/Bar.kt")
	if got != ":owner" {
		t.Errorf("expected :owner (path-suffix match), got %q (backwards-suffix bug)", got)
	}
}

func TestInferModuleFromTestPathReturnsEmptyWhenNoModuleMatches(t *testing.T) {
	blob := &snapshot.Blob{
		Modules: []snapshot.Module{
			{Path: ":other", Dir: "/repo/other"},
		},
	}
	got := inferModuleFromTestPath(blob, "/repo/app/src/test/kotlin/Bar.kt")
	if got != "" {
		t.Errorf("expected empty (no match), got %q", got)
	}
}

func TestInferModuleFromTestPathRespectsAndroidAndIntegrationMarkers(t *testing.T) {
	blob := &snapshot.Blob{
		Modules: []snapshot.Module{
			{Path: ":app", Dir: "/repo/app"},
		},
	}
	cases := []string{
		"/repo/app/src/test/kotlin/Bar.kt",
		"/repo/app/src/androidTest/kotlin/Bar.kt",
		"/repo/app/src/integrationTest/kotlin/Bar.kt",
	}
	for _, p := range cases {
		if got := inferModuleFromTestPath(blob, p); got != ":app" {
			t.Errorf("expected :app for %q, got %q", p, got)
		}
	}
}
