package rules

import "testing"

func TestRemoteViewModuleDir(t *testing.T) {
	for path, want := range map[string]string{
		"app/src/main/java/com/example/Widget.kt":      "app",
		"app/src/debug/kotlin/com/example/Widget.kt":   "app",
		"app/src/main/res/layout/widget.xml":           "app",
		"feature/lib/src/main/res/layout/widget.xml":   "feature/lib",
		"src/main/java/com/example/Widget.kt":          ".",
		"src/main/res/layout/widget.xml":               ".",
		"app/src/main/java/com/src/main/java/Deep.kt":  "app",
		"flat/res/layout/widget.xml":                   "flat",
		"res/layout/widget.xml":                        ".",
		"scripts/Widget.kt":                            "",
		"app/src/main/assets/layout/widget.xml":        "",
		"app/src/main/java/com/example/res/x/Weird.kt": "app",
	} {
		if got := remoteViewModuleDir(path); got != want {
			t.Errorf("remoteViewModuleDir(%q) = %q, want %q", path, got, want)
		}
	}
}
