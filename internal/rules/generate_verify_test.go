package rules_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGeneratedFilesUpToDate runs krit-gen in -verify mode against the
// committed zz_meta_*_gen.go files. If the generator would produce
// different output from what's on disk, this test fails.
//
// To fix a failure: run `go generate ./internal/rules/...`.
//
// Bootstrap: build/rule_inventory.json is a gitignored build artifact
// (see #295). On a fresh clone there is no inventory to verify against,
// so we regenerate it here when missing — otherwise `go test ./...`
// would fail before `make test` ever ran tools/rule_inventory.py. If
// the bootstrap itself can't run (no python3 in PATH, or tools/ missing
// in a release tarball) the test skips with a pointer to `make test`.
//
// Note: this test requires source access to the generator package
// (internal/codegen/cmd/krit-gen) and the inventory file
// (build/rule_inventory.json). In a release-tarball environment that
// strips those paths the test would fail to exec `go run` — not a
// concern for CI, which always has the full source tree.
func TestGeneratedFilesUpToDate(t *testing.T) {
	const inventoryRel = "../../build/rule_inventory.json"
	if _, err := os.Stat(inventoryRel); os.IsNotExist(err) {
		script := filepath.Join("..", "..", "tools", "rule_inventory.py")
		if _, err := os.Stat(script); os.IsNotExist(err) {
			t.Skipf("skipping: %s missing and tools/rule_inventory.py not available to bootstrap it (run `make test`)", inventoryRel)
		}
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Skipf("skipping: %s missing and python3 not in PATH to bootstrap it (run `make test`)", inventoryRel)
		}
		out, err := exec.Command(python, script).CombinedOutput()
		if err != nil {
			t.Fatalf("bootstrap tools/rule_inventory.py failed:\n%s\nerror: %v", out, err)
		}
	}

	cmd := exec.Command("go", "run",
		"../codegen/cmd/krit-gen",
		"-inventory", inventoryRel,
		"-out", ".",
		"-root", "../..",
		"-verify",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated files out of date (run `go generate ./internal/rules/...` to fix):\n%s\nerror: %v", out, err)
	}
}
