package onboarding

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadRegistry(t *testing.T) {
	path := filepath.Join("..", "..", "config", "onboarding", "controversial-rules.json")
	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if reg.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", reg.SchemaVersion)
	}
	if len(reg.Questions) == 0 {
		t.Fatal("no questions parsed")
	}

	// Every question must have defaults for all four profiles;
	// LoadRegistry validates this but double-check that at least
	// one question has non-trivial content we rely on downstream.
	found := false
	for _, q := range reg.Questions {
		if q.ID == "allow-bang-operator" {
			found = true
			if !q.Invert() {
				t.Error("allow-bang-operator must be inverted")
			}
			if q.RulesetForYAML() != "potential-bugs" {
				t.Errorf("allow-bang-operator ruleset = %q, want potential-bugs", q.RulesetForYAML())
			}
		}
	}
	if !found {
		t.Error("registry missing expected question allow-bang-operator")
	}
}

func TestResolveAnswersCascadeYes(t *testing.T) {
	reg := &Registry{
		SchemaVersion: 1,
		Questions: []Question{
			{
				ID:         "parent",
				Kind:       "parent",
				Rules:      nil,
				CascadesTo: []string{"child"},
				Defaults:   map[string]bool{"strict": true, "balanced": true, "relaxed": false, "detekt-compat": false},
			},
			{
				ID:          "child",
				Kind:        "rule",
				Rules:       []string{"ChildRule"},
				CascadeFrom: strPtr("parent"),
				Defaults:    map[string]bool{"strict": true, "balanced": false, "relaxed": false, "detekt-compat": false},
			},
		},
	}

	// User answers parent "yes" → child should derive from its strict default = true.
	answers, err := ResolveAnswers(reg, "balanced", func(q *Question, def bool) bool {
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(answers))
	}
	if answers[1].QuestionID != "child" || !answers[1].Cascaded || answers[1].Parent != "parent" {
		t.Errorf("child answer not cascaded correctly: %+v", answers[1])
	}
	if !answers[1].Value {
		t.Error("child value should be true (strict default) when parent answered yes")
	}
}

func TestResolveAnswersCascadeNo(t *testing.T) {
	reg := &Registry{
		SchemaVersion: 1,
		Questions: []Question{
			{
				ID:         "parent",
				Kind:       "parent",
				CascadesTo: []string{"child"},
				Defaults:   map[string]bool{"strict": true, "balanced": true, "relaxed": false, "detekt-compat": false},
			},
			{
				ID:          "child",
				Kind:        "rule",
				Rules:       []string{"ChildRule"},
				CascadeFrom: strPtr("parent"),
				Defaults:    map[string]bool{"strict": true, "balanced": false, "relaxed": false, "detekt-compat": true},
			},
		},
	}

	// User answers parent "no" → child uses relaxed default = false.
	answers, err := ResolveAnswers(reg, "strict", func(q *Question, def bool) bool {
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(answers))
	}
	if answers[1].Value {
		t.Error("child value should be false (relaxed default) when parent answered no")
	}
}

func TestBuildOverridesInversion(t *testing.T) {
	reg := &Registry{
		SchemaVersion: 1,
		Questions: []Question{
			{
				ID:              "allow-bang-operator",
				Kind:            "rule",
				Rules:           []string{"UnsafeCallOnNullableType"},
				PositiveFixture: strPtr("tests/fixtures/positive/potential-bugs/UnsafeCallOnNullableType.kt"),
				Defaults:        map[string]bool{"strict": false, "balanced": true, "relaxed": true, "detekt-compat": true},
			},
			{
				ID:              "flag-magic-numbers",
				Kind:            "rule",
				Rules:           []string{"MagicNumber"},
				PositiveFixture: strPtr("tests/fixtures/positive/style/MagicNumber.kt"),
				Defaults:        map[string]bool{"strict": true, "balanced": true, "relaxed": false, "detekt-compat": true},
			},
		},
	}

	answers := []Answer{
		{QuestionID: "allow-bang-operator", Value: true},  // yes = allow = disable
		{QuestionID: "flag-magic-numbers", Value: true},   // yes = flag = enable
	}
	overrides := BuildOverrides(reg, answers)
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}

	byRule := make(map[string]Override)
	for _, o := range overrides {
		byRule[o.Rule] = o
	}
	if byRule["UnsafeCallOnNullableType"].Active {
		t.Error("UnsafeCallOnNullableType should be inverted to active=false when user allows !!")
	}
	if !byRule["MagicNumber"].Active {
		t.Error("MagicNumber should stay active=true when user flags magic numbers")
	}
	if byRule["UnsafeCallOnNullableType"].Ruleset != "potential-bugs" {
		t.Errorf("UnsafeCallOnNullableType ruleset = %q, want potential-bugs", byRule["UnsafeCallOnNullableType"].Ruleset)
	}
	if byRule["MagicNumber"].Ruleset != "style" {
		t.Errorf("MagicNumber ruleset = %q, want style", byRule["MagicNumber"].Ruleset)
	}
}

func TestWriteConfigDeepMerge(t *testing.T) {
	profileYAML := []byte(`
style:
  MagicNumber:
    active: true
    ignoreEnums: false
  WildcardImport:
    active: true
potential-bugs:
  UnsafeCast:
    active: true
`)
	overrides := []Override{
		{Ruleset: "potential-bugs", Rule: "UnsafeCallOnNullableType", Active: false},
		{Ruleset: "style", Rule: "MagicNumber", Active: false}, // overrides existing
	}
	body, err := WriteConfig(WriteConfigOptions{
		ProfileYAML: profileYAML,
		ProfileName: "balanced",
		Overrides:   overrides,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse the body to confirm the merged tree is well-formed.
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("generated YAML does not parse: %v\n---\n%s", err, body)
	}

	style, _ := parsed["style"].(map[string]interface{})
	if style == nil {
		t.Fatal("merged YAML missing style block")
	}
	magic, _ := style["MagicNumber"].(map[string]interface{})
	if magic == nil {
		t.Fatal("merged YAML missing style.MagicNumber")
	}
	if magic["active"] != false {
		t.Errorf("MagicNumber.active = %v, want false (override)", magic["active"])
	}
	// Existing keys preserved.
	if magic["ignoreEnums"] != false {
		t.Errorf("MagicNumber.ignoreEnums = %v, want false (preserved from profile)", magic["ignoreEnums"])
	}

	pb, _ := parsed["potential-bugs"].(map[string]interface{})
	if pb == nil {
		t.Fatal("merged YAML missing potential-bugs block")
	}
	if _, ok := pb["UnsafeCallOnNullableType"]; !ok {
		t.Error("potential-bugs.UnsafeCallOnNullableType not added by override")
	}
	if _, ok := pb["UnsafeCast"]; !ok {
		t.Error("potential-bugs.UnsafeCast was dropped instead of preserved")
	}

	// Header comment should appear.
	if !strings.HasPrefix(string(body), "# Generated by krit init") {
		t.Error("body missing generated header comment")
	}
}

func TestWriteConfigThresholdOverrides(t *testing.T) {
	profileYAML := []byte(`
complexity:
  LongMethod:
    active: true
    allowedLines: 60
  CyclomaticComplexMethod:
    active: true
    allowedComplexity: 14
`)
	body, err := WriteConfig(WriteConfigOptions{
		ProfileYAML: profileYAML,
		ProfileName: "balanced",
		ThresholdOverrides: []ThresholdOverride{
			{Ruleset: "complexity", Rule: "LongMethod", Field: "allowedLines", Value: 40},
			{Ruleset: "complexity", Rule: "CyclomaticComplexMethod", Field: "allowedComplexity", Value: 10},
			{Ruleset: "style", Rule: "MaxLineLength", Field: "maxLineLength", Value: 100},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := YAMLUnmarshalMap(body, &parsed); err != nil {
		t.Fatalf("generated YAML does not parse: %v\n---\n%s", err, body)
	}

	comp, _ := parsed["complexity"].(map[string]interface{})
	if comp == nil {
		t.Fatal("missing complexity block")
	}
	lm, _ := comp["LongMethod"].(map[string]interface{})
	if lm["allowedLines"] != 40 {
		t.Errorf("LongMethod.allowedLines = %v, want 40", lm["allowedLines"])
	}
	// Existing active state is preserved.
	if lm["active"] != true {
		t.Errorf("LongMethod.active = %v, want true", lm["active"])
	}
	cc, _ := comp["CyclomaticComplexMethod"].(map[string]interface{})
	if cc["allowedComplexity"] != 10 {
		t.Errorf("CyclomaticComplexMethod.allowedComplexity = %v, want 10", cc["allowedComplexity"])
	}

	// style block was created from scratch for MaxLineLength.
	st, _ := parsed["style"].(map[string]interface{})
	if st == nil {
		t.Fatal("style block missing after override created it")
	}
	mll, _ := st["MaxLineLength"].(map[string]interface{})
	if mll["maxLineLength"] != 100 {
		t.Errorf("MaxLineLength.maxLineLength = %v, want 100", mll["maxLineLength"])
	}
}

func TestTopRules(t *testing.T) {
	res := &ScanResult{
		ByRule: map[string]int{
			"A": 10,
			"B": 5,
			"C": 10,
			"D": 1,
		},
	}
	top := res.TopRules(2)
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	// A and C tie at 10; A wins on name.
	if top[0].Name != "A" || top[0].Count != 10 {
		t.Errorf("top[0] = %+v, want A(10)", top[0])
	}
	if top[1].Name != "C" || top[1].Count != 10 {
		t.Errorf("top[1] = %+v, want C(10)", top[1])
	}
}

func TestBuildFindingsMapCapsAt3(t *testing.T) {
	findings := []rawFinding{
		{File: "a.kt", Line: 1, Rule: "R1", Message: "m1"},
		{File: "b.kt", Line: 2, Rule: "R1", Message: "m2"},
		{File: "c.kt", Line: 3, Rule: "R1", Message: "m3"},
		{File: "d.kt", Line: 4, Rule: "R1", Message: "m4"}, // should be dropped
		{File: "e.kt", Line: 5, Rule: "R2", Message: "m5"},
	}
	m := buildFindingsMap(findings)
	if len(m["R1"]) != 3 {
		t.Errorf("R1: expected 3 findings, got %d", len(m["R1"]))
	}
	if len(m["R2"]) != 1 {
		t.Errorf("R2: expected 1 finding, got %d", len(m["R2"]))
	}
	// Verify the 4th was dropped.
	for _, f := range m["R1"] {
		if f.Message == "m4" {
			t.Error("R1 should not contain m4 (4th finding should be capped)")
		}
	}
}

func TestStrictStagesOrdering(t *testing.T) {
	if len(StrictStages) == 0 {
		t.Fatal("StrictStages is empty")
	}
	seen := make(map[string]bool)
	for _, s := range StrictStages {
		if s.Prefix == "" {
			t.Error("empty prefix in StrictStages")
		}
		if s.Label == "" {
			t.Error("empty label in StrictStages")
		}
		if seen[s.Prefix] {
			t.Errorf("duplicate prefix %q", s.Prefix)
		}
		seen[s.Prefix] = true
	}
}

func strPtr(s string) *string { return &s }
