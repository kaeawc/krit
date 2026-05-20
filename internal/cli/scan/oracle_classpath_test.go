package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestResolveOracleClasspath_ConfigOnly(t *testing.T) {
	t.Setenv("CLASSPATH", "")
	cfg := config.NewConfigFromData(map[string]interface{}{
		"oracle": map[string]interface{}{
			"classpath": []interface{}{"/lib/a.jar", "/lib/b.jar"},
		},
	})
	got := resolveOracleClasspath(cfg)
	want := []string{"/lib/a.jar", "/lib/b.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveOracleClasspath_EnvOnly(t *testing.T) {
	// Synthesize a CLASSPATH using the platform's path separator so
	// the test runs on Linux and macOS without branching.
	envValue := "/env/x.jar" + string(os.PathListSeparator) + "/env/y.jar"
	t.Setenv("CLASSPATH", envValue)
	got := resolveOracleClasspath(nil)
	want := []string{"/env/x.jar", "/env/y.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveOracleClasspath_ConfigBeforeEnv(t *testing.T) {
	envValue := "/env/x.jar" + string(os.PathListSeparator) + "/env/y.jar"
	t.Setenv("CLASSPATH", envValue)
	cfg := config.NewConfigFromData(map[string]interface{}{
		"oracle": map[string]interface{}{
			"classpath": []interface{}{"/cfg/a.jar"},
		},
	})
	got := resolveOracleClasspath(cfg)
	// Config entries come first so an explicit krit.yml override
	// wins over an ambient env value if the daemon honors order.
	want := []string{"/cfg/a.jar", "/env/x.jar", "/env/y.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveOracleClasspath_DedupesAcrossSources(t *testing.T) {
	envValue := "/shared.jar" + string(os.PathListSeparator) + "/env/y.jar"
	t.Setenv("CLASSPATH", envValue)
	cfg := config.NewConfigFromData(map[string]interface{}{
		"oracle": map[string]interface{}{
			"classpath": []interface{}{"/cfg/a.jar", "/shared.jar"},
		},
	})
	got := resolveOracleClasspath(cfg)
	want := []string{"/cfg/a.jar", "/shared.jar", "/env/y.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveOracleClasspath_EmptyByDefault(t *testing.T) {
	t.Setenv("CLASSPATH", "")
	if got := resolveOracleClasspath(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	emptyCfg := config.NewConfigFromData(nil)
	if got := resolveOracleClasspath(emptyCfg); got != nil {
		t.Errorf("expected nil for empty cfg, got %v", got)
	}
}

func TestResolveOracleClasspath_IgnoresBlankEntries(t *testing.T) {
	// A trailing separator on CLASSPATH inserts an empty entry on
	// most platforms — `filepath.SplitList` returns "" for it. We
	// drop those so the daemon's classpath array stays clean.
	envValue := "/x.jar" + string(os.PathListSeparator) + "" + string(os.PathListSeparator) + "/y.jar"
	t.Setenv("CLASSPATH", envValue)
	got := resolveOracleClasspath(nil)
	for _, p := range got {
		if p == "" {
			t.Fatalf("blank entry leaked: %v", got)
		}
	}
	// Use filepath to avoid platform-specific assertions on path
	// formatting.
	if filepath.Clean(got[0]) != filepath.Clean("/x.jar") {
		t.Errorf("first entry = %q, want %q", got[0], "/x.jar")
	}
}
