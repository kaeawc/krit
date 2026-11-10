package fsutil

import "testing"

func TestIsBasicTestPath(t *testing.T) {
	cases := map[string]bool{
		"/app/src/test/kotlin/A.kt":        true,
		"/app/src/androidTest/kotlin/A.kt": true,
		"/app/src/commonTest/A.kt":         true,
		"/app/src/jvmTest/A.kt":            true,
		"/app/src/testFixtures/A.kt":       true,
		"/app/src/main/kotlin/A.kt":        false,
		"/app/src/main/kotlin/test.kt":     false,
	}
	for path, want := range cases {
		if got := IsBasicTestPath(path); got != want {
			t.Errorf("IsBasicTestPath(%q) = %v, want %v", path, got, want)
		}
	}
}
