package oracle

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeBuildLogicFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFilterFIRSourceDirsClassifiesOnlyBuildLogic(t *testing.T) {
	root := t.TempDir()
	writeBuildLogicFile(t, filepath.Join(root, ".git"), "gitdir: /tmp/worktree\n")
	writeBuildLogicFile(t, filepath.Join(root, "settings.gradle.kts"), `
// pluginManagement { includeBuild("sample-app") }
val fake = "pluginManagement { includeBuild(\"plugin-module\") }"
includeBuild("sample-app")
pluginManagement {
    includeBuild("gradle/plugins")
}
`)
	for _, dir := range []string{"buildSrc", "build-logic", "gradle/plugins", "plugin-module", "sample-app", "app"} {
		if dir != "app" {
			writeBuildLogicFile(t, filepath.Join(root, dir, "settings.gradle.kts"), "")
		}
	}
	main := filepath.Join(root, "app/src/main/kotlin")
	buildSrc := filepath.Join(root, "buildSrc/src/main/kotlin")
	logic := filepath.Join(root, "build-logic/src/main/kotlin")
	plugins := filepath.Join(root, "gradle/plugins/src/main/kotlin")
	productPlugin := filepath.Join(root, "plugin-module/src/main/kotlin")
	sample := filepath.Join(root, "sample-app/src/main/kotlin")
	for _, path := range []string{main, buildSrc, logic, plugins, productPlugin, sample} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := FilterFIRSourceDirs([]string{main, buildSrc, logic, plugins, productPlugin, sample})
	if want := []string{main, productPlugin, sample}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FIR roots = %v, want %v", got, want)
	}
}

func TestBuildLogicStopsAtGitBoundary(t *testing.T) {
	wrapper := t.TempDir()
	writeBuildLogicFile(t, filepath.Join(wrapper, "settings.gradle.kts"), `pluginManagement { includeBuild("repo") }`)
	for _, gitEntry := range []string{"directory", "file"} {
		t.Run(gitEntry, func(t *testing.T) {
			root := filepath.Join(wrapper, gitEntry, "repo")
			if gitEntry == "directory" {
				writeBuildLogicFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")
			} else {
				writeBuildLogicFile(t, filepath.Join(root, ".git"), "gitdir: /tmp/worktree")
			}
			writeBuildLogicFile(t, filepath.Join(root, "settings.gradle.kts"), "")
			path := filepath.Join(root, "app/src/main/kotlin/App.kt")
			writeBuildLogicFile(t, path, "class App")
			if IsBuildLogicPath(path) {
				t.Fatal("ancestor settings above .git classified product source as build logic")
			}
		})
	}
}

func TestPluginManagementIncludeBuildLexicalScope(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{`pluginManagement { includeBuild("plugins") }`, true},
		{`pluginManagement { /* includeBuild("other") */ includeBuild("plugins") }`, true},
		{`pluginManagement { repositories { includeBuild('plugins') } }`, true},
		{`includeBuild("plugins")`, false},
		{`// pluginManagement { includeBuild("plugins") }`, false},
		{`val s = "pluginManagement { includeBuild(\"plugins\") }"`, false},
		{`def s = /pluginManagement { includeBuild("plugins") }/`, false},
		{`def s = $/pluginManagement { includeBuild("plugins") }/$`, false},
		{`pluginManagement { includeBuild("other") }; includeBuild("plugins")`, false},
	} {
		if got := pluginManagementIncludesBuild([]byte(tc.source), "plugins"); got != tc.want {
			t.Errorf("pluginManagementIncludesBuild(%q) = %t, want %t", tc.source, got, tc.want)
		}
	}
}
