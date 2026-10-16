package serve

import "testing"

// dirtyIntroducesSourceFile decides whether the daemon may reuse the prior
// manifest's source-path lists. Reusing them after a file was added (or
// renamed to) hides that file from the scan until the daemon restarts; the
// warm-versus-clean mutation matrix (tests/parity, #786) covers the end-to-end
// behavior through a real `krit serve`.
func TestDirtyIntroducesSourceFile(t *testing.T) {
	known := map[string]string{"/r/src/A.kt": "h", "src/B.kt": "h"}
	cases := []struct {
		name  string
		dirty []string
		want  bool
	}{
		{"nothing dirty", nil, false},
		{"known absolute", []string{"/r/src/A.kt"}, false},
		{"known root-relative", []string{"/r/src/B.kt"}, false},
		{"added kotlin", []string{"/r/src/New.kt"}, true},
		{"added script", []string{"/r/build.gradle.kts"}, true},
		{"added java", []string{"/r/src/New.java"}, true},
		{"unrelated extension", []string{"/r/src/readme.md"}, false},
		{"one known one added", []string{"/r/src/A.kt", "/r/src/New.kt"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dirtyIntroducesSourceFile(tc.dirty, known, "/r"); got != tc.want {
				t.Fatalf("dirtyIntroducesSourceFile(%v) = %v, want %v", tc.dirty, got, tc.want)
			}
		})
	}
}
