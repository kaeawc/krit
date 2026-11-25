package scan

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestGradleProjectOutputsForceInProcessScan(t *testing.T) {
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--gradle-project-outputs", "outputs.json"}); err != nil {
		t.Fatal(err)
	}
	if daemonCompatibleFlags(f) {
		t.Fatal("--gradle-project-outputs must run in-process: the daemon wire does not carry it")
	}
}

func TestFirDaemonPoolForcesInProcessScan(t *testing.T) {
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	f := registerScanFlags(fs)
	t.Setenv(firchecks.FirDaemonPoolEnv, "build-0")
	if daemonCompatibleFlags(f) {
		t.Fatal("KRIT_FIR_DAEMON_POOL must run in-process: a delegated scan would not see it")
	}
}

// A built project dependency (--gradle-project-outputs) is left out of the compile
// context; the scanned project's own roots stay.
func TestLoadGradleCompileContextSkipsBuiltProjects(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	appMain := filepath.Join(root, "app", "src", "main", "kotlin")
	libMain := filepath.Join(root, "lib", "src", "main", "kotlin")
	for _, dir := range []string{modelDir, appMain, libMain} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(struct {
		Schema   int                   `json:"schema"`
		Projects []gradlemodel.Project `json:"projects"`
	}{Schema: 2, Projects: []gradlemodel.Project{
		{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []gradlemodel.SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{appMain}, ProjectDeps: []string{":lib"}},
		}},
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []gradlemodel.SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{libMain}},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "model.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if dirs, _, _ := loadGradleCompileContextBuilt([]string{appMain}, "", false, nil); !reflect.DeepEqual(dirs, []string{appMain, libMain}) {
		t.Fatalf("without outputs: %v", dirs)
	}
	if dirs, _, _ := loadGradleCompileContextBuilt([]string{appMain}, "", false, map[string]bool{":lib": true}); !reflect.DeepEqual(dirs, []string{appMain}) {
		t.Fatalf("with :lib built: %v", dirs)
	}
}
