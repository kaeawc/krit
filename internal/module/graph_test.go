package module

import (
	"path/filepath"
	"testing"
)

func TestFileToModule(t *testing.T) {
	root := "/project"
	g := NewModuleGraph(root)
	g.Modules[":app"] = &Module{
		Path: ":app",
		Dir:  filepath.Join(root, "app"),
	}
	g.Modules[":core:util"] = &Module{
		Path: ":core:util",
		Dir:  filepath.Join(root, "core", "util"),
	}
	g.Modules[":core"] = &Module{
		Path: ":core",
		Dir:  filepath.Join(root, "core"),
	}

	tests := []struct {
		file string
		want string
	}{
		{"/project/app/src/main/kotlin/Foo.kt", ":app"},
		{"/project/core/util/src/main/kotlin/Bar.kt", ":core:util"},
		{"/project/core/src/main/kotlin/Baz.kt", ":core"},
		{"/project/unknown/Foo.kt", ""},
		{"/other/project/app/Foo.kt", ""},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			got := g.FileToModule(tt.file)
			if got != tt.want {
				t.Errorf("FileToModule(%q) = %q, want %q", tt.file, got, tt.want)
			}
		})
	}
}

func TestFileToModuleLongestMatch(t *testing.T) {
	root := "/project"
	g := NewModuleGraph(root)
	g.Modules[":samples"] = &Module{
		Path: ":samples",
		Dir:  filepath.Join(root, "samples"),
	}
	g.Modules[":samples:star"] = &Module{
		Path: ":samples:star",
		Dir:  filepath.Join(root, "samples", "star"),
	}
	g.Modules[":samples:star:apk"] = &Module{
		Path: ":samples:star:apk",
		Dir:  filepath.Join(root, "samples", "star", "apk"),
	}

	got := g.FileToModule("/project/samples/star/apk/src/main/kotlin/Main.kt")
	if got != ":samples:star:apk" {
		t.Errorf("expected :samples:star:apk, got %q", got)
	}

	got = g.FileToModule("/project/samples/star/src/main/kotlin/Main.kt")
	if got != ":samples:star" {
		t.Errorf("expected :samples:star, got %q", got)
	}
}
