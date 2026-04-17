package rules_test

import (
	"os/exec"
	"testing"
)

// TestGeneratedFilesUpToDate runs krit-gen in -verify mode against the
// committed zz_meta_*_gen.go files. If the generator would produce
// different output from what's on disk, this test fails.
//
// To fix a failure: run `go generate ./internal/rules/...`.
//
// Note: this test requires source access to the generator package
// (internal/codegen/cmd/krit-gen) and the inventory file
// (build/rule_inventory.json). In a release-tarball environment that
// strips those paths the test would fail to exec `go run` — not a
// concern for CI, which always has the full source tree.
func TestGeneratedFilesUpToDate(t *testing.T) {
	cmd := exec.Command("go", "run",
		"../codegen/cmd/krit-gen",
		"-inventory", "../../build/rule_inventory.json",
		"-out", ".",
		"-root", "../..",
		"-verify",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated files out of date (run `go generate ./internal/rules/...` to fix):\n%s\nerror: %v", out, err)
	}
}
