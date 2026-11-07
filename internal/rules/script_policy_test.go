package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// lintAt parses code at a path with the given file name and runs one rule.
func lintAt(t *testing.T, ruleID, name, code string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return runRuleByNameOnFile(t, ruleID, file)
}

func resetScriptPolicy(t *testing.T, p rules.ScriptPolicy) {
	t.Helper()
	rules.SetScriptPolicy(p)
	t.Cleanup(func() { rules.SetScriptPolicy(rules.ScriptPolicy{}) })
}

const scriptCatchSrc = `fun risky() {}
try {
  risky()
} catch (e: Exception) {
  println("failed")
}
`

func manyFunctions(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "fun f%d() {}\n", i)
	}
	return sb.String()
}

func TestScriptPolicyAutoKeepsDetektStyleRules(t *testing.T) {
	for _, name := range []string{"tool.main.kts", "other.kts", "build.gradle.kts", "Tool.kt"} {
		if got := lintAt(t, "TooGenericExceptionCaught", name, scriptCatchSrc); len(got) == 0 {
			t.Errorf("TooGenericExceptionCaught should fire in %s", name)
		}
	}
}

func TestScriptPolicyAutoSkipsScriptIrrelevantRules(t *testing.T) {
	tests := []struct {
		rule, code string
	}{
		{"MagicNumber", "val a = 1234 + 5678\n"},
		{"ThrowingExceptionInMain", "fun main() {\n  throw IllegalStateException(\"x\")\n}\n"},
		{"NestedClassesVisibility", "internal class Outer {\n  public class Nested\n}\n"},
		{"TopLevelPropertyNaming", "val Bad_Name = 1\n"},
	}
	for _, tc := range tests {
		if got := lintAt(t, tc.rule, "Sample.kt", "package p\n"+tc.code); len(got) == 0 && tc.rule != "TopLevelPropertyNaming" {
			t.Fatalf("%s must fire on a .kt file for the script comparison to mean anything", tc.rule)
		}
		for _, name := range []string{"tool.main.kts", "build.gradle.kts"} {
			if got := lintAt(t, tc.rule, name, tc.code); len(got) != 0 {
				t.Errorf("%s fired in %s: %v", tc.rule, name, got)
			}
		}
	}
}

func TestScriptTopLevelPropertiesAreLocalVariables(t *testing.T) {
	const code = "val Bad_Name = 1\nval goodName = 2\n"
	if got := lintAt(t, "VariableNaming", "Sample.kt", "package p\n"+code); len(got) != 0 {
		t.Fatalf("top-level property in a .kt file is not a variable: %v", got)
	}
	got := lintAt(t, "VariableNaming", "tool.main.kts", code)
	if len(got) != 1 || got[0].Line != 1 {
		t.Fatalf("want one VariableNaming finding on line 1 of the script, got %v", got)
	}
}

func TestScriptFunctionsDoNotCountTowardFileThreshold(t *testing.T) {
	src := manyFunctions(14)
	if got := lintAt(t, "TooManyFunctions", "Sample.kt", "package p\n"+src); len(got) == 0 {
		t.Fatal("14 functions in a .kt file must trip TooManyFunctions")
	}
	if got := lintAt(t, "TooManyFunctions", "tool.main.kts", src); len(got) != 0 {
		t.Fatalf("functions of a script do not count toward the file threshold: %v", got)
	}
	class := "class Big {\n" + strings.ReplaceAll(src, "fun ", "  fun ") + "}\n"
	if got := lintAt(t, "TooManyFunctions", "tool.main.kts", class); len(got) == 0 {
		t.Fatal("a class declared in a script keeps its own function threshold")
	}
}

func TestScriptTopLevelPropertyNeedsNoDocumentation(t *testing.T) {
	const code = "val topLevel = 1\nclass Holder {\n  val member = 2\n}\n"
	if got := lintAt(t, "UndocumentedPublicProperty", "Sample.kt", "package p\n"+code); len(got) != 2 {
		t.Fatalf(".kt file: want 2 findings, got %v", got)
	}
	got := lintAt(t, "UndocumentedPublicProperty", "tool.main.kts", code)
	if len(got) != 1 || !strings.Contains(got[0].Message, "member") {
		t.Fatalf("script: want only the class member flagged, got %v", got)
	}
}

func TestScriptFileSuppressWorksInScripts(t *testing.T) {
	src := "@file:Suppress(\"TooGenericExceptionCaught\")\n" + scriptCatchSrc
	if got := lintAt(t, "TooGenericExceptionCaught", "tool.main.kts", src); len(got) != 0 {
		t.Fatalf("@file:Suppress must silence the rule in a script: %v", got)
	}
	named := "@Suppress(\"TooGenericExceptionCaught\")\nfun run() {\n  try {\n    risky()\n  } catch (e: Exception) {\n    println(\"x\")\n  }\n}\n"
	if got := lintAt(t, "TooGenericExceptionCaught", "tool.main.kts", named); len(got) != 0 {
		t.Fatalf("@Suppress on a script function must silence the rule: %v", got)
	}
}

func TestScriptConfigExcludesApplyToScripts(t *testing.T) {
	saved := rules.GetRuleExcludes("TooGenericExceptionCaught")
	t.Cleanup(func() { rules.SetRuleExcludes("TooGenericExceptionCaught", saved) })
	for _, tc := range []struct {
		glob, name string
		want       bool
	}{
		{"**/*.kts", "tool.main.kts", true},
		{"**/*.main.kts", "tool.main.kts", true},
		{"**/*.main.kts", "build.gradle.kts", false},
		{"**/*.gradle.kts", "scripts/build.gradle.kts", true},
		{"**/scripts/**", "scripts/tool.main.kts", true},
		{"**/scripts/**", "tool.main.kts", false},
	} {
		rules.SetRuleExcludes("TooGenericExceptionCaught", []string{tc.glob})
		got := lintAt(t, "TooGenericExceptionCaught", tc.name, scriptCatchSrc)
		if excluded := len(got) == 0; excluded != tc.want {
			t.Errorf("exclude %q on %s: excluded=%v, want %v", tc.glob, tc.name, excluded, tc.want)
		}
	}
}

func TestScriptPolicyAllRunsEveryRule(t *testing.T) {
	resetScriptPolicy(t, rules.ScriptPolicy{Mode: rules.ScriptPolicyAll})
	if got := lintAt(t, "MagicNumber", "tool.main.kts", "val a = 1234 + 5678\n"); len(got) == 0 {
		t.Fatal("policy all must run MagicNumber on a script")
	}
}

func TestScriptPolicyNoneRunsNoRule(t *testing.T) {
	resetScriptPolicy(t, rules.ScriptPolicy{Mode: rules.ScriptPolicyNone})
	if got := lintAt(t, "TooGenericExceptionCaught", "tool.main.kts", scriptCatchSrc); len(got) != 0 {
		t.Fatalf("policy none must skip source rules on scripts: %v", got)
	}
	if got := lintAt(t, "TooGenericExceptionCaught", "Tool.kt", scriptCatchSrc); len(got) == 0 {
		t.Fatal("policy none must not touch .kt files")
	}
}

func TestScriptPolicyIncludeExclude(t *testing.T) {
	resetScriptPolicy(t, rules.ScriptPolicy{
		Mode:    rules.ScriptPolicyNone,
		Include: map[string]bool{"TooGenericExceptionCaught": true, "SwallowedException": true},
		Exclude: map[string]bool{"SwallowedException": true},
	})
	if got := lintAt(t, "TooGenericExceptionCaught", "tool.main.kts", scriptCatchSrc); len(got) == 0 {
		t.Fatal("include must re-enable a rule under policy none")
	}
	if got := lintAt(t, "SwallowedException", "tool.main.kts", scriptCatchSrc); len(got) != 0 {
		t.Fatalf("exclude must win over include: %v", got)
	}
}

func TestParseScriptPolicyFromConfig(t *testing.T) {
	cfg := config.NewConfigFromData(map[string]interface{}{
		"scripts": map[string]interface{}{
			"policy":  "ALL",
			"include": []interface{}{"MagicNumber"},
			"exclude": []interface{}{"SwallowedException"},
		},
	})
	p := rules.ParseScriptPolicy(cfg)
	if p.Mode != rules.ScriptPolicyAll || !p.Include["MagicNumber"] || !p.Exclude["SwallowedException"] {
		t.Fatalf("unexpected policy %+v", p)
	}
	if got := rules.ParseScriptPolicy(nil).Mode; got != rules.ScriptPolicyAuto {
		t.Fatalf("nil config mode = %q, want auto", got)
	}
	bad := config.NewConfigFromData(map[string]interface{}{"scripts": map[string]interface{}{"policy": "sometimes"}})
	if got := rules.ParseScriptPolicy(bad).Mode; got != rules.ScriptPolicyAuto {
		t.Fatalf("unknown mode = %q, want auto", got)
	}
}

func TestApplyConfigInstallsScriptPolicy(t *testing.T) {
	t.Cleanup(func() { rules.SetScriptPolicy(rules.ScriptPolicy{}) })
	rules.ApplyConfig(config.NewConfigFromData(map[string]interface{}{
		"scripts": map[string]interface{}{"policy": "none"},
	}))
	if got := lintAt(t, "TooGenericExceptionCaught", "tool.main.kts", scriptCatchSrc); len(got) != 0 {
		t.Fatalf("scripts.policy none from config must skip source rules: %v", got)
	}
}

func TestScriptExcludedRulesByCategory(t *testing.T) {
	standalone := rules.ScriptExcludedRules(scanner.ScriptStandalone)
	gradle := rules.ScriptExcludedRules(scanner.ScriptGradle)
	for _, id := range []string{"DeadCode", "ComposeClickableWithoutMinTouchTarget", "UntestedPublicApi", "MagicNumber", "LibraryEntitiesShouldNotBePublic"} {
		if !standalone[id] || !gradle[id] {
			t.Errorf("%s should be excluded for scripts (standalone=%v gradle=%v)", id, standalone[id], gradle[id])
		}
	}
	for _, id := range []string{"TooGenericExceptionCaught", "SwallowedException", "NestedBlockDepth", "CyclomaticComplexMethod", "VariableNaming", "ForbiddenMethodCall"} {
		if standalone[id] || gradle[id] {
			t.Errorf("%s should run on scripts (standalone excluded=%v gradle excluded=%v)", id, standalone[id], gradle[id])
		}
	}
	if rules.ScriptExcludedRules(scanner.ScriptNone) != nil {
		t.Error("regular Kotlin files have no script exclusions")
	}
}

func TestScriptScopeTablesNameRegisteredRules(t *testing.T) {
	ids := map[string]bool{}
	categories := map[string]bool{}
	for _, r := range api.Registry {
		ids[r.ID] = true
		categories[r.Category] = true
	}
	for id := range api.ScriptScopeOverrides() {
		if !ids[id] {
			t.Errorf("script scope override names unknown rule %q", id)
		}
	}
	for category := range api.ScriptScopeCategoryDefaults() {
		if !categories[category] {
			t.Errorf("script scope default names unknown category %q", category)
		}
	}
}

// Rules that read Gradle build scripts run through their own phase. Their
// category must never switch them off for build scripts.
func TestScriptPolicyLeavesGradleLanguageRulesAlone(t *testing.T) {
	gradle := &scanner.File{Path: "build.gradle.kts", Language: scanner.LangGradle}
	if got := rules.ScriptPolicyExcludes(gradle); got != nil {
		t.Fatalf("Gradle-language file must not get script exclusions, got %d rules", len(got))
	}
}

func TestScriptPolicyExcludesSuppressCrossFileFindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.main.kts")
	if err := os.WriteFile(path, []byte("fun helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	filter := scanner.BuildSuppressionFilter(file, nil, nil, "").WithExcludedRules(rules.ScriptPolicyExcludes(file))
	if !filter.IsSuppressed("DeadCode", "dead-code", 1) {
		t.Error("DeadCode findings in a script must be suppressed")
	}
	if filter.IsSuppressed("TooGenericExceptionCaught", "exceptions", 1) {
		t.Error("detekt-style rules must still report in a script")
	}
}
