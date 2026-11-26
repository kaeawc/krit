package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/kaeawc/krit/internal/rules" // registers CompileTimeConstant
	api "github.com/kaeawc/krit/internal/rules/api"
)

// CompileTimeConstant's verdict for App.kt depends on the annotation on a callee in Lib.kt (#1610). Editing only
// Lib.kt must change the warm result exactly as a --no-cache run does (#1769 review, #1778).
func TestRunProject_CompileTimeConstantWarmRunSeesEditsToOtherFiles(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CompileTimeConstant" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("CompileTimeConstant not registered")
	}
	const annotated = "package lib\n\nimport com.google.errorprone.annotations.CompileTimeConstant\n\n" +
		"fun log(@CompileTimeConstant msg: String) {}\n"
	const plain = "package lib\n\nfun log(msg: String) {}\n"
	const caller = "package app\n\nimport lib.log\n\nfun use(dynamic: String) {\n    log(dynamic)\n}\n"

	h := &warmCrossHarness{t: t, dir: t.TempDir(), rule: rule, crossCalls: new(int64)}
	h.write("lib/Lib.kt", annotated)
	h.write("app/App.kt", caller)
	noCache := func(label string, want int) {
		t.Helper()
		fresh := &warmCrossHarness{t: t, dir: h.dir, rule: rule, crossCalls: new(int64)}
		cacheDir := filepath.Join(h.dir, ".krit-test")
		saved := cacheDir + ".saved"
		if err := os.Rename(cacheDir, saved); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.RemoveAll(cacheDir)
			if err := os.Rename(saved, cacheDir); err != nil {
				t.Fatal(err)
			}
		}()
		if got := fresh.run().FindingsCount; got != want {
			t.Fatalf("%s with no cache: findings=%d, want %d", label, got, want)
		}
	}
	check := func(label string, want int) {
		t.Helper()
		if got := h.run().FindingsCount; got != want {
			t.Fatalf("%s: findings=%d, want %d", label, got, want)
		}
		noCache(label, want)
	}

	check("cold", 1)
	check("warm", 1)
	h.write("lib/Lib.kt", plain)
	check("warm after removing the annotation in Lib.kt", 0)
	h.write("lib/Lib.kt", annotated)
	check("warm after restoring it", 1)
}
