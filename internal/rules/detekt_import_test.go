package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/schema"
)

// detektProject writes files under a fresh project root marked as a VCS root.
func detektProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// snapshotRegistry captures every rule's settings so a test that applies a
// config can put the shared registry back afterwards.
func snapshotRegistry(t *testing.T) {
	t.Helper()
	type saved struct{ dst, copy reflect.Value }
	var all []saved
	for _, r := range api.Registry {
		v := reflect.ValueOf(r.Implementation)
		if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
			continue
		}
		c := reflect.New(v.Elem().Type()).Elem()
		c.Set(v.Elem())
		all = append(all, saved{v.Elem(), c})
	}
	inactive := map[string]bool{}
	for k, v := range rules.DefaultInactive {
		inactive[k] = v
	}
	excludes := rules.GetAllRuleExcludes()
	t.Cleanup(func() {
		for _, s := range all {
			s.dst.Set(s.copy)
		}
		for k := range rules.DefaultInactive {
			delete(rules.DefaultInactive, k)
		}
		for k, v := range inactive {
			rules.DefaultInactive[k] = v
		}
		for _, r := range api.Registry {
			rules.SetRuleExcludes(r.ID, excludes[r.ID])
		}
		scanner.SetRuleIgnoreAnnotated(nil)
	})
}

// applyProjectConfig loads defaults + detekt + krit.yml for root and applies
// them to the shared rule registry, restoring it when the test ends.
func applyProjectConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	snapshotRegistry(t)
	cfg, err := config.LoadAndMergeDefaultsWith(config.LoadOptions{}, "", root)
	if err != nil {
		t.Fatal(err)
	}
	rules.ApplyConfig(cfg)
	return cfg
}

// fnWithLines returns a Kotlin function spanning exactly total lines.
func fnWithLines(annotation, name string, total int) string {
	var b strings.Builder
	if annotation != "" {
		b.WriteString(annotation + "\n")
	}
	fmt.Fprintf(&b, "fun %s() {\n", name)
	for i := 0; i < total-2; i++ {
		fmt.Fprintf(&b, "    val x%d = %d\n", i, i)
	}
	b.WriteString("}\n")
	return b.String()
}

func TestDetektLongMethodThresholdAppliedToRuleRun(t *testing.T) {
	src := "package test\n" + fnWithLines("", "process", 80)

	if got := runRuleByName(t, "LongMethod", src); len(got) == 0 {
		t.Fatal("precondition: default threshold should flag an 80-line function")
	}

	root := detektProject(t, map[string]string{
		"config/detekt/detekt.yml": "complexity:\n  LongMethod:\n    threshold: 120\n",
	})
	applyProjectConfig(t, root)

	if got := runRuleByName(t, "LongMethod", src); len(got) != 0 {
		t.Fatalf("detekt threshold 120 should not flag an 80-line function, got %d", len(got))
	}
	// detekt reports once the line count reaches the threshold.
	if got := runRuleByName(t, "LongMethod", "package test\n"+fnWithLines("", "edge", 119)); len(got) != 0 {
		t.Errorf("119-line function is below threshold 120, got %d findings", len(got))
	}
	if got := runRuleByName(t, "LongMethod", "package test\n"+fnWithLines("", "edge", 120)); len(got) == 0 {
		t.Error("120-line function reaches threshold 120 and should be flagged")
	}
}

func TestKritYmlThresholdBeatsDetektOnRuleRun(t *testing.T) {
	root := detektProject(t, map[string]string{
		"detekt.yml": "complexity:\n  LongMethod:\n    threshold: 120\n",
		"krit.yml":   "complexity:\n  LongMethod:\n    allowedLines: 70\n",
	})
	applyProjectConfig(t, root)
	if got := runRuleByName(t, "LongMethod", "package test\n"+fnWithLines("", "process", 80)); len(got) == 0 {
		t.Fatal("krit.yml allowedLines: 70 should override detekt threshold 120")
	}
}

func TestDetektNestedBlockDepthThreshold(t *testing.T) {
	src := `package test
fun nest(a: Boolean) {
    if (a) {
        if (a) {
            if (a) {
                if (a) {
                    if (a) {
                        println("x")
                    }
                }
            }
        }
    }
}
`
	if got := runRuleByName(t, "NestedBlockDepth", src); len(got) == 0 {
		t.Fatal("precondition: default depth should flag five nested ifs")
	}
	root := detektProject(t, map[string]string{"detekt.yml": "complexity:\n  NestedBlockDepth:\n    threshold: 6\n"})
	applyProjectConfig(t, root)
	if got := runRuleByName(t, "NestedBlockDepth", src); len(got) != 0 {
		t.Fatalf("threshold 6 should allow five nested ifs, got %d findings", len(got))
	}
}

func TestDetektTooManyFunctionsThresholds(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "fun f%d() {}\n", i)
	}
	if got := runRuleByName(t, "TooManyFunctions", b.String()); len(got) == 0 {
		t.Fatal("precondition: default threshold should flag 30 top-level functions")
	}
	root := detektProject(t, map[string]string{"detekt.yml": "complexity:\n  TooManyFunctions:\n    thresholdInFiles: 40\n    thresholdInClasses: 40\n"})
	applyProjectConfig(t, root)
	if got := runRuleByName(t, "TooManyFunctions", b.String()); len(got) != 0 {
		t.Fatalf("thresholdInFiles 40 should allow 30 functions, got %d", len(got))
	}
}

func TestDetektInactiveRulesAreDisabled(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": `
style:
  MaxLineLength:
    active: false
  DestructuringDeclarationWithTooManyEntries:
    active: false
`})
	applyProjectConfig(t, root)
	for _, id := range []string{"MaxLineLength", "DestructuringDeclarationWithTooManyEntries"} {
		if rules.IsDefaultActive(id) {
			t.Errorf("%s should be inactive after importing active: false", id)
		}
	}
}

func TestDetektRulesetInactiveDisablesRules(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": "complexity:\n  active: false\n"})
	applyProjectConfig(t, root)
	if rules.IsDefaultActive("LongMethod") {
		t.Error("complexity: active false should disable LongMethod")
	}
}

func TestDetektRuleOptionsReachRules(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": `
style:
  ReturnCount:
    max: 7
    excludeGuardClauses: false
naming:
  TopLevelPropertyNaming:
    constantPattern: '[A-Z]{3}'
complexity:
  ComplexMethod:
    threshold: 31
  LongParameterList:
    functionThreshold: 9
    constructorThreshold: 12
`})
	cfg := applyProjectConfig(t, root)
	checks := []struct {
		set, rule, key string
		want           int
	}{
		{"style", "ReturnCount", "max", 7},
		{"complexity", "CyclomaticComplexMethod", "allowedComplexity", 30},
		{"complexity", "LongParameterList", "allowedFunctionParameters", 8},
		{"complexity", "LongParameterList", "allowedConstructorParameters", 11},
	}
	for _, c := range checks {
		if got := cfg.GetInt(c.set, c.rule, c.key, -1); got != c.want {
			t.Errorf("%s.%s.%s = %d, want %d", c.set, c.rule, c.key, got, c.want)
		}
	}
	if got := cfg.GetString("naming", "TopLevelPropertyNaming", "constantPattern", ""); got != "[A-Z]{3}" {
		t.Errorf("constantPattern = %q", got)
	}
	// 9 parameters: allowed 8 from detekt functionThreshold 9 (reports at >= 9).
	nine := "package test\nfun f(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int, i: Int) {}\n"
	if got := runRuleByName(t, "LongParameterList", nine); len(got) == 0 {
		t.Error("9 parameters reach functionThreshold 9 and should be flagged")
	}
	eight := "package test\nfun f(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int) {}\n"
	if got := runRuleByName(t, "LongParameterList", eight); len(got) != 0 {
		t.Errorf("8 parameters are below functionThreshold 9, got %d findings", len(got))
	}
}

func TestDetektIgnoreAnnotatedExemptsDeclarations(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": `
complexity:
  LongMethod:
    ignoreAnnotated: ['Composable']
`})
	applyProjectConfig(t, root)
	annotated := "package test\n" + fnWithLines("@Composable", "screen", 80)
	if got := runRuleByName(t, "LongMethod", annotated); len(got) != 0 {
		t.Errorf("@Composable function should be exempt, got %d findings", len(got))
	}
	plain := "package test\n" + fnWithLines("", "screen", 80)
	if got := runRuleByName(t, "LongMethod", plain); len(got) == 0 {
		t.Error("unannotated function should still be flagged")
	}
	other := "package test\n" + fnWithLines("@Deprecated(\"x\")", "screen", 80)
	if got := runRuleByName(t, "LongMethod", other); len(got) == 0 {
		t.Error("a different annotation must not exempt the function")
	}
}

func fileAt(t *testing.T, rel, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDetektExcludesAndIncludesGlobs(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": `
complexity:
  LongMethod:
    excludes: ['**/generated/**']
    includes: ['**/main/**']
`})
	applyProjectConfig(t, root)
	src := "package test\n" + fnWithLines("", "process", 80)
	cases := []struct {
		rel  string
		want bool
	}{
		{"src/main/Foo.kt", true},
		{"src/main/generated/Foo.kt", false},
		{"src/test/Foo.kt", false},
	}
	for _, c := range cases {
		got := runRuleByNameOnFile(t, "LongMethod", fileAt(t, c.rel, src))
		if (len(got) > 0) != c.want {
			t.Errorf("%s: flagged=%v, want %v", c.rel, len(got) > 0, c.want)
		}
	}
}

func TestDetektRulesetExcludesApplyToRules(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": "complexity:\n  excludes: ['**/legacy/**']\n"})
	applyProjectConfig(t, root)
	src := "package test\n" + fnWithLines("", "process", 80)
	if got := runRuleByNameOnFile(t, "LongMethod", fileAt(t, "src/legacy/Foo.kt", src)); len(got) != 0 {
		t.Errorf("ruleset excludes should skip legacy files, got %d", len(got))
	}
	if got := runRuleByNameOnFile(t, "LongMethod", fileAt(t, "src/app/Foo.kt", src)); len(got) == 0 {
		t.Error("files outside the excluded glob should still be checked")
	}
}

func TestDetektOptOutRestoresKritDefaults(t *testing.T) {
	root := detektProject(t, map[string]string{
		"detekt.yml": "complexity:\n  LongMethod:\n    threshold: 120\n",
		"krit.yml":   "detekt:\n  enabled: false\n",
	})
	applyProjectConfig(t, root)
	if got := runRuleByName(t, "LongMethod", "package test\n"+fnWithLines("", "process", 80)); len(got) == 0 {
		t.Fatal("with detekt.enabled: false the krit default threshold applies")
	}
}

func TestDetektImportedConfigValidates(t *testing.T) {
	root := detektProject(t, map[string]string{"detekt.yml": `
build:
  maxIssues: 0
config:
  validation: true
complexity:
  active: true
  excludes: ['**/test/**']
  ComplexMethod:
    threshold: 15
    ignoreSingleWhenExpression: true
  LongMethod:
    threshold: 60
  LongParameterList:
    functionThreshold: 6
    constructorThreshold: 7
    ignoreDefaultParameters: false
    ignoreAnnotated: ['Composable']
  TooManyFunctions:
    thresholdInFiles: 11
    thresholdInClasses: 11
    thresholdInInterfaces: 11
    thresholdInObjects: 11
    thresholdInEnums: 11
style:
  MagicNumber:
    ignoreNumbers: ['-1', '0', '1', '2']
    includes: ['**/main/**']
  MaxLineLength:
    active: false
naming:
  FunctionName:
    functionPattern: '[a-z][a-zA-Z0-9]*'
`})
	cfg := applyProjectConfig(t, root)
	for _, e := range schema.ValidateConfig(cfg) {
		if e.Level == "error" {
			t.Errorf("imported detekt config fails validation: %s", e)
		}
	}
}

func TestDetektMappingTablesMatchRegistry(t *testing.T) {
	byID := map[string]*api.Rule{}
	for _, r := range api.Registry {
		if _, dup := byID[r.ID]; !dup {
			byID[r.ID] = r
		}
	}
	for detekt, targets := range config.DetektRuleRenames() {
		for _, target := range targets {
			if _, ok := byID[target]; !ok {
				t.Errorf("rename %s -> %s: krit has no rule %s", detekt, target, target)
			}
		}
	}
	for rule, opts := range config.DetektThresholdOptions() {
		r, ok := byID[rule]
		if !ok {
			t.Errorf("threshold table names unknown rule %s", rule)
			continue
		}
		meta, _ := rules.MetaForRule(r)
		primary := map[string]bool{}
		for _, opt := range meta.Options {
			primary[opt.Name] = true
		}
		for detektKey, target := range opts {
			if !primary[target] {
				t.Errorf("%s.%s -> %s: not an option of the krit rule", rule, detektKey, target)
			}
		}
	}
}
