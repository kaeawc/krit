package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	shippedconfig "github.com/kaeawc/krit/config"
	"github.com/kaeawc/krit/internal/onboarding"
)

// Every file krit init and the default-config loader read must be embedded.
func TestFSCarriesShippedConfig(t *testing.T) {
	want := []string{shippedconfig.DefaultConfigName, "onboarding/controversial-rules.json"}
	for _, name := range onboarding.ProfileNames {
		want = append(want, "profiles/"+name+".yml")
	}
	for _, name := range want {
		embedded, err := shippedconfig.FS.ReadFile(name)
		if err != nil {
			t.Errorf("%s not embedded: %v", name, err)
			continue
		}
		onDisk, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embedded, onDisk) {
			t.Errorf("embedded %s differs from the checked-in file", name)
		}
	}
}

func TestWriteTreeMirrorsConfigLayout(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "config")
	if err := shippedconfig.WriteTree(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range onboarding.ProfileNames {
		if _, err := os.Stat(onboarding.ProfilePath(root, name)); err != nil {
			t.Errorf("profile %s missing: %v", name, err)
		}
	}
	if _, err := onboarding.LoadRegistry(filepath.Join(dir, "onboarding", "controversial-rules.json")); err != nil {
		t.Errorf("registry unreadable: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, shippedconfig.DefaultConfigName))
	if err != nil || !bytes.Equal(got, shippedconfig.DefaultConfig()) {
		t.Errorf("default-krit.yml not written intact: %v", err)
	}
}

func TestContentHashIsStable(t *testing.T) {
	a, b := shippedconfig.ContentHash(), shippedconfig.ContentHash()
	if a != b || len(a) != 64 {
		t.Errorf("ContentHash unstable or malformed: %q vs %q", a, b)
	}
}
