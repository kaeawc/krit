package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

// makeYAMLConfig parses inline YAML into a Config. Used to vary configs in
// fingerprint tests without depending on disk layout.
func makeYAMLConfig(t *testing.T, dir string, body string) *config.Config {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// TestConfigHash_RuleSetSensitivity verifies that adding, removing, or
// reordering rules in a non-sorted way still produces a stable hash for the
// same set, and a different hash for a different set.
func TestConfigHash_RuleSetSensitivity(t *testing.T) {
	cfg := config.NewConfig()

	a := ComputeConfigHash([]string{"R1", "R2", "R3"}, cfg, false)
	b := ComputeConfigHash([]string{"R3", "R1", "R2"}, cfg, false)
	if a != b {
		t.Errorf("hash should be order-insensitive: %s vs %s", a, b)
	}

	added := ComputeConfigHash([]string{"R1", "R2", "R3", "R4"}, cfg, false)
	if added == a {
		t.Error("adding a rule must change the hash")
	}
	removed := ComputeConfigHash([]string{"R1", "R2"}, cfg, false)
	if removed == a {
		t.Error("removing a rule must change the hash")
	}
	renamed := ComputeConfigHash([]string{"R1", "R2", "R3X"}, cfg, false)
	if renamed == a {
		t.Error("renaming a rule must change the hash")
	}
}

// TestConfigHash_EditorConfigToggle verifies that toggling editorconfig
// changes the hash. Editor-config-derived thresholds (e.g. MaxLineLength)
// affect findings, so the cache must invalidate when the toggle flips.
func TestConfigHash_EditorConfigToggle(t *testing.T) {
	cfg := config.NewConfig()
	off := ComputeConfigHash([]string{"R1"}, cfg, false)
	on := ComputeConfigHash([]string{"R1"}, cfg, true)
	if off == on {
		t.Errorf("editorconfig toggle must change hash: off=%s on=%s", off, on)
	}
}

// TestConfigHash_ConfigBodySensitivity verifies that mutating the resolved
// config body (a threshold, a rule toggle, an arbitrary value) changes the
// hash. This is the cache's only protection against silently reusing
// findings produced under a different config.
func TestConfigHash_ConfigBodySensitivity(t *testing.T) {
	dir := t.TempDir()

	base := makeYAMLConfig(t, dir+"/base", `
style:
  MaxLineLength:
    max: 120
`)
	mutated := makeYAMLConfig(t, dir+"/mut", `
style:
  MaxLineLength:
    max: 100
`)
	disabled := makeYAMLConfig(t, dir+"/dis", `
style:
  MaxLineLength:
    enabled: false
    max: 120
`)

	rules := []string{"style.MaxLineLength"}
	hBase := ComputeConfigHash(rules, base, false)
	hMut := ComputeConfigHash(rules, mutated, false)
	hDis := ComputeConfigHash(rules, disabled, false)

	if hBase == hMut {
		t.Error("changing a numeric threshold must change the hash")
	}
	if hBase == hDis {
		t.Error("disabling a rule via config must change the hash")
	}
	if hMut == hDis {
		t.Error("different config mutations must produce distinct hashes")
	}
}

// TestConfigHash_NilConfigStable verifies that ComputeConfigHash is stable
// across repeated calls with a nil config. Nil is a valid input (e.g. before
// config is loaded) and must not produce non-deterministic output.
func TestConfigHash_NilConfigStable(t *testing.T) {
	rules := []string{"R1", "R2"}
	first := ComputeConfigHash(rules, nil, false)
	for i := 0; i < 5; i++ {
		got := ComputeConfigHash(rules, nil, false)
		if got != first {
			t.Fatalf("nil-config hash changed on iter %d: %s != %s", i, first, got)
		}
	}
}

// TestRuleHash_OrderInsensitive verifies the legacy rule-name-only hash is
// stable across input orderings. This is what older caches were keyed by.
func TestRuleHash_OrderInsensitive(t *testing.T) {
	a := ComputeRuleHash([]string{"R1", "R2", "R3"})
	b := ComputeRuleHash([]string{"R3", "R2", "R1"})
	c := ComputeRuleHash([]string{"R2", "R1", "R3"})
	if a != b || b != c {
		t.Errorf("ComputeRuleHash must be order-insensitive: %s %s %s", a, b, c)
	}
}

// TestRuleHash_UniquenessForSubstrings verifies that rule names with shared
// prefixes don't collide due to a naive separator-free join.
func TestRuleHash_UniquenessForSubstrings(t *testing.T) {
	a := ComputeRuleHash([]string{"AB", "CD"})
	b := ComputeRuleHash([]string{"ABCD"})
	if a == b {
		t.Errorf("rule joining must use a separator; got collision %s", a)
	}
	// Defensive: confirm comma is the separator and not present in rule names.
	if strings.Contains("AB", ",") {
		t.Skip("rule name contains separator; test invariant invalid")
	}
}
