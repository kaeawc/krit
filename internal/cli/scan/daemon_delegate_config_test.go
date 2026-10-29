package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigServedByDaemon pins that an explicit --config only rides the
// daemon when it names the krit.yml the daemon itself loads (the repo
// root's). The analyze-project wire carries no config path, so any other
// file must run in-process or its settings would be silently ignored.
func TestConfigServedByDaemon(t *testing.T) {
	repo := t.TempDir()
	rootCfg := filepath.Join(repo, "krit.yml")
	if err := os.WriteFile(rootCfg, []byte("# root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "custom.yml")
	if err := os.WriteFile(other, []byte("# other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.yml")
	if err := os.Symlink(rootCfg, link); err != nil {
		t.Fatal(err)
	}
	bare := t.TempDir() // no krit.yml at its root

	tests := []struct {
		name   string
		config string
		repo   string
		want   bool
	}{
		{"no --config", "", repo, true},
		{"--config is the root krit.yml", rootCfg, repo, true},
		{"--config is a symlink to the root krit.yml", link, repo, true},
		{"--config names another file", other, repo, false},
		{"--config set, repo has no krit.yml", other, bare, false},
		{"--config names a missing file", filepath.Join(repo, "missing.yml"), repo, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.Config = tt.config
			if got := configServedByDaemon(f, tt.repo); got != tt.want {
				t.Errorf("configServedByDaemon(%q, %q) = %v, want %v", tt.config, tt.repo, got, tt.want)
			}
		})
	}
}

// TestConfigServedByDaemon_RelativePathResolvesAgainstCwd matches the
// in-process path, which opens --config relative to the working directory.
func TestConfigServedByDaemon_RelativePathResolvesAgainstCwd(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".krit.yml"), []byte("# root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	f := freshScanFlags(t)
	*f.Config = ".krit.yml"
	if !configServedByDaemon(f, repo) {
		t.Fatal("relative --config naming the root .krit.yml should be served by the daemon")
	}
}
