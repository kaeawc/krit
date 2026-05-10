package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonState_ConfigCachedAcrossCalls(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "krit.yml"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	state := newDaemonState(root)

	first, err := state.ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig (first): %v", err)
	}
	second, err := state.ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig (second): %v", err)
	}
	if first != second {
		t.Errorf("expected ensureConfig to return the cached pointer; got %p / %p", first, second)
	}
}

func TestDaemonState_ConfigInvalidatedRebuilds(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "krit.yml"), []byte("# v1\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	state := newDaemonState(root)

	first, err := state.ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig (first): %v", err)
	}
	state.InvalidateConfig()
	if err := os.WriteFile(filepath.Join(root, "krit.yml"), []byte("# v2\n"), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	second, err := state.ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig (second): %v", err)
	}
	if first == second {
		t.Errorf("expected post-invalidation ensureConfig to rebuild; got the same pointer back")
	}
}

func TestDaemonState_RepoDirStableAcrossLookups(t *testing.T) {
	root := t.TempDir()
	state := newDaemonState(root)
	first := state.repoDir
	for i := 0; i < 10; i++ {
		if state.repoDir != first {
			t.Fatalf("repoDir flipped at iter %d: %q vs %q", i, state.repoDir, first)
		}
	}
	if first == "" {
		t.Errorf("repoDir should fall back to root; got empty")
	}
}

func TestIsKritConfigPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"krit.yml", true},
		{".krit.yml", true},
		{"a/b/krit.yml", true},
		{"a/.krit.yml", true},
		{"krit.yaml", false},
		{"krit.yml.bak", false},
		{"build.gradle.kts", false},
		{"Foo.kt", false},
	}
	for _, tc := range cases {
		if got := isKritConfigPath(tc.path); got != tc.want {
			t.Errorf("isKritConfigPath(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}
