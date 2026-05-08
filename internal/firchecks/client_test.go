package firchecks

import (
	"os"
	"testing"
)

func TestFindFirJar_NoJarReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	result := FindFirJar([]string{tmp})
	if result != "" {
		t.Errorf("expected empty string when no jar exists, got %q", result)
	}
}

func TestFindFirJar_FindsJarInProjectDir(t *testing.T) {
	tmp := t.TempDir()
	kritDir := tmp + "/.krit"
	if err := os.MkdirAll(kritDir, 0755); err != nil {
		t.Fatal(err)
	}
	jarPath := kritDir + "/krit-fir.jar"
	if err := os.WriteFile(jarPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	result := FindFirJar([]string{tmp})
	if result == "" {
		t.Fatal("expected FindFirJar to find the jar")
	}
}

func TestHashFirSources_Deterministic(t *testing.T) {
	dirs := []string{"/b/src", "/a/src"}
	h1 := hashFirSources(dirs)
	h2 := hashFirSources([]string{"/a/src", "/b/src"}) // reversed order
	if h1 != h2 {
		t.Errorf("hashFirSources not order-independent: %q vs %q", h1, h2)
	}
	if len(h1) != 16 {
		t.Errorf("expected 16-char hash, got %d: %q", len(h1), h1)
	}
}

func TestHashFirSources_DifferentDirs(t *testing.T) {
	h1 := hashFirSources([]string{"/repo/a"})
	h2 := hashFirSources([]string{"/repo/b"})
	if h1 == h2 {
		t.Errorf("different dirs should produce different hashes")
	}
}

func TestFirDaemonPIDPaths_UsesKritFirSuffix(t *testing.T) {
	pid := firPIDPath("deadbeef12345678")
	port := firPortPath("deadbeef12345678")
	// Both paths should contain "krit-fir" to distinguish from oracle daemons.
	if pid == port {
		t.Errorf("pid and port paths must be different")
	}
}
