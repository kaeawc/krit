package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A released binary has no config/ directory next to it, so defaults must
// come from the embedded copy and match the checked-in file exactly.
func TestLoadAndMergeDefaults_EmbeddedMatchesCheckedInFile(t *testing.T) {
	onDisk, err := filepath.Abs(filepath.Join("..", "..", "config", "default-krit.yml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if path := FindDefaultConfig(); path != "" {
		t.Fatalf("test setup: expected no on-disk default config, found %s", path)
	}
	if got := DefaultConfigSource(); got != "embedded" {
		t.Errorf("DefaultConfigSource() = %q, want embedded", got)
	}

	embedded, err := LoadAndMergeDefaults("")
	if err != nil {
		t.Fatalf("LoadAndMergeDefaults: %v", err)
	}
	want, err := loadFile(onDisk)
	if err != nil {
		t.Fatal(err)
	}
	if len(embedded.data) == 0 {
		t.Fatal("embedded defaults are empty")
	}
	if !reflect.DeepEqual(embedded.data, want.data) {
		t.Error("embedded default-krit.yml differs from config/default-krit.yml")
	}
}

func TestLoadAndMergeDefaults_UserConfigOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	base, err := LoadAndMergeDefaults("")
	if err != nil {
		t.Fatal(err)
	}
	if base.GetTopLevelKeyString("krit-test-marker", "") != "" {
		t.Fatal("test setup: marker key already present in defaults")
	}
	user := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(user, []byte("krit-test-marker: set\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		userPath string
		roots    []string
	}{
		{"explicit path", user, nil},
		{"auto-detected from root", "", []string{dir}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadAndMergeDefaults(tc.userPath, tc.roots...)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.GetTopLevelKeyString("krit-test-marker", ""); got != "set" {
				t.Errorf("user key not merged: %q", got)
			}
			if len(cfg.data) <= 1 {
				t.Error("embedded defaults were dropped by the merge")
			}
		})
	}
}
