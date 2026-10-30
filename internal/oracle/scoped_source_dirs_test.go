package oracle

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestScopedSourceUnionDaemonKey(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app", "src", "main", "kotlin")
	lib := filepath.Join(root, "lib", "src", "main", "kotlin")
	libTest := filepath.Join(root, "lib", "src", "test", "kotlin")
	for _, dir := range []string{app, lib, libTest} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	model := &gradlemodel.Model{Projects: []gradlemodel.Project{
		{Path: ":app", SourceSets: []gradlemodel.SourceSet{{Kind: "main", SourceDirs: []string{app}, ProjectDeps: []string{":lib"}}}},
		{Path: ":lib", SourceSets: []gradlemodel.SourceSet{{Kind: "main", SourceDirs: []string{lib}}, {Kind: "test", SourceDirs: []string{libTest}}}},
	}}
	scoped := FilterFIRSourceDirs(UnionSourceDirs(FindSourceDirs([]string{app}), model.CompileSourceDirs([]string{app})))
	if !reflect.DeepEqual(scoped, []string{app, lib}) {
		t.Fatalf("server compilation roots = %v", scoped)
	}
	settings := gradlemodel.KotlinSettings{}
	key := daemonRegistryKeyKT(testJarPath, scoped, settings)
	if key == daemonRegistryKeyKT(testJarPath, []string{app}, settings) {
		t.Fatal("context roots do not affect daemon identity")
	}
	if key == daemonRegistryKeyKT(testJarPath, model.SourceDirs(), settings) {
		t.Fatal("scoped roots reuse whole-model daemon identity")
	}
	if key != daemonRegistryKeyKT(testJarPath, UnionSourceDirs(scoped, []string{lib, app}), settings) {
		t.Fatal("duplicate context roots change daemon identity")
	}
}
