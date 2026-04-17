package rules_test

import (
	"os/exec"
	"testing"
)

// TestRegistryFileUpToDate runs krit-registry-extract in -verify mode
// against the committed zz_registry_gen.go. If the extractor would
// produce a different consolidation, this test fails.
//
// To fix: run `go run ./internal/codegen/cmd/krit-registry-extract
// -rules internal/rules -out internal/rules/zz_registry_gen.go`.
func TestRegistryFileUpToDate(t *testing.T) {
	cmd := exec.Command("go", "run",
		"../codegen/cmd/krit-registry-extract",
		"-rules", ".",
		"-out", "zz_registry_gen.go",
		"-verify",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zz_registry_gen.go out of date (re-run krit-registry-extract):\n%s\nerror: %v", out, err)
	}
}
