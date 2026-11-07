package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// A cross-file rule outside the script policy (DeadCode) must not report in a
// script, while the same finding in a .kt file still passes.
func TestApplySuppression_ScriptPolicyDropsCrossFileRules(t *testing.T) {
	t.Cleanup(func() { rules.SetScriptPolicy(rules.ScriptPolicy{}) })
	rules.SetScriptPolicy(rules.ScriptPolicy{})
	dir := t.TempDir()
	script := filepath.Join(dir, "tool.main.kts")
	source := filepath.Join(dir, "Tool.kt")
	for path, body := range map[string]string{script: "fun helper() {}\n", source: "fun helper() {}\n"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := ParsePhase{}.Run(context.Background(), ParseInput{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.KotlinFiles) != 2 {
		t.Fatalf("parsed %d files, want 2", len(out.KotlinFiles))
	}
	findings := []scanner.Finding{
		{File: script, Line: 1, Rule: "DeadCode", RuleSet: "dead-code", Message: "unused"},
		{File: script, Line: 1, Rule: "TooGenericExceptionCaught", RuleSet: "exceptions", Message: "generic"},
		{File: source, Line: 1, Rule: "DeadCode", RuleSet: "dead-code", Message: "unused"},
	}
	kept := ApplySuppression(findings, out.KotlinFiles)
	if len(kept) != 2 {
		t.Fatalf("kept %d findings, want 2: %+v", len(kept), kept)
	}
	for _, f := range kept {
		if f.File == script && f.Rule == "DeadCode" {
			t.Errorf("DeadCode in a script must be dropped: %+v", f)
		}
	}
}
