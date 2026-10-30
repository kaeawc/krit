package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadyLineLabelsModuleKotlinFileCount(t *testing.T) {
	for _, conventional := range []bool{false, true} {
		t.Run(map[bool]string{false: "custom-layout", true: "conventional-layout"}[conventional], func(t *testing.T) {
			root := t.TempDir()
			for _, relative := range []string{"app/custom", "app/src/main/kotlin", "app/src/test/kotlin", "app/src/main/java"} {
				if err := os.MkdirAll(filepath.Join(root, relative), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("include(\":app\")\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			writeKotlinFile(t, root, "app/custom/Outside.kt", "class Outside\n")
			want := "0 module Kotlin files, 1 modules"
			if conventional {
				writeKotlinFile(t, root, "app/src/main/kotlin/Main.kt", "class Main\n")
				writeKotlinFile(t, root, "app/src/test/kotlin/Test.kt", "class Test\n")
				writeKotlinFile(t, root, "app/src/main/java/Main.java", "class JavaMain {}\n")
				want = "2 module Kotlin files, 1 modules"
			}
			state := newDaemonState(root)
			if err := state.warm(); err != nil {
				t.Fatal(err)
			}
			line := state.readyLine(", watcher=off")
			if !strings.Contains(line, want) || !strings.HasSuffix(line, ", watcher=off)") {
				t.Fatalf("ready line=%q, want scoped count %q and watcher suffix", line, want)
			}
		})
	}
}
