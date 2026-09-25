package oracle

import "testing"

func TestInJVMCompilableSourceSet(t *testing.T) {
	cases := map[string]bool{
		"/p/src/main/kotlin/a/A.kt":            true,
		"/p/src/jvmMain/kotlin/a/A.kt":         true,
		"/p/src/commonMain/kotlin/a/A.kt":      true,
		"/p/src/androidMain/kotlin/a/A.kt":     true,
		"/p/src/jsMain/kotlin/a/A.kt":          false,
		"/p/src/iosArm64Main/kotlin/A.kt":      false,
		"/p/src/wasmJsTest/kotlin/A.kt":        false,
		"/p/src/jsMain/kotlin/a/kotlin/b/A.kt": false, // innermost src/<set>/kotlin root wins
		"/p/lib/Loose.kt":                      true,  // outside the source-set layout
		"relative/src/nativeMain/kotlin/A.kt":  false,
	}
	for path, want := range cases {
		if got := InJVMCompilableSourceSet(path); got != want {
			t.Errorf("InJVMCompilableSourceSet(%q) = %v, want %v", path, got, want)
		}
	}
}
