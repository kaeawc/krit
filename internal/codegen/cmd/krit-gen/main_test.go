package main

import (
	"bytes"
	"encoding/json"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureInventory is a tiny synthetic inventory covering the feature
// matrix the generator needs to handle:
//
//   - int option with alias (CognitiveFake)
//   - bool option (BoolFake)
//   - regex option with string default (RegexFake)
//   - opt-in rule (default_active = false)
//   - oracle filter, string-list option (OracleFake)
//
// All three rules share a source file so the generator emits a single
// output file and covers the file-grouping path.
const fixtureInventory = `{
  "generated_at": "test",
  "source_commit": "deadbeef",
  "rules": [
    {
      "struct_type": "CognitiveFakeRule",
      "file": "internal/rules/fake.go",
      "id": "CognitiveFake",
      "ruleset": "fake",
      "severity": "warning",
      "description": "int option with alias.",
      "default_active": true,
      "fix_level": "",
      "confidence": 0.9,
      "node_types": [],
      "oracle": null,
      "options": [
        {
          "field": "AllowedComplexity",
          "go_type": "int",
          "yaml_key": "allowedComplexity",
          "aliases": ["threshold"],
          "default": 15,
          "description": "Maximum allowed cognitive complexity."
        }
      ],
      "warnings": []
    },
    {
      "struct_type": "BoolFakeRule",
      "file": "internal/rules/fake.go",
      "id": "BoolFake",
      "ruleset": "fake",
      "severity": "info",
      "description": "Bool option.",
      "default_active": false,
      "fix_level": "cosmetic",
      "confidence": 0,
      "node_types": [],
      "oracle": null,
      "options": [
        {
          "field": "IncludeElvis",
          "go_type": "bool",
          "yaml_key": "includeElvis",
          "aliases": [],
          "default": false,
          "description": "Also flag elvis expressions."
        }
      ],
      "warnings": []
    },
    {
      "struct_type": "RegexFakeRule",
      "file": "internal/rules/fake.go",
      "id": "RegexFake",
      "ruleset": "fake",
      "severity": "warning",
      "description": "Regex option.",
      "default_active": true,
      "fix_level": "",
      "confidence": 0.75,
      "node_types": [],
      "oracle": "treeSitterOnlyFilter",
      "options": [
        {
          "field": "ClassPattern",
          "go_type": "regex",
          "yaml_key": "classPattern",
          "aliases": [],
          "default": "[A-Z][a-zA-Z0-9]*",
          "description": "Allowed class name pattern."
        }
      ],
      "warnings": ["schema missing for FakeField"]
    },
    {
      "struct_type": "OracleFakeRule",
      "file": "internal/rules/other.go",
      "id": "OracleFake",
      "ruleset": "fake",
      "severity": "warning",
      "description": "Oracle-filtered rule with string-list option.",
      "default_active": false,
      "fix_level": "",
      "confidence": 0,
      "node_types": [],
      "oracle": "allFilesAuditedFilter",
      "options": [
        {
          "field": "ForbiddenTypes",
          "go_type": "[]string",
          "yaml_key": "forbiddenTypes",
          "aliases": [],
          "default": null,
          "description": "Forbidden types list."
        }
      ],
      "warnings": []
    }
  ],
  "warnings": [],
  "stats": {}
}`

// writeFixture materializes the fixture inventory plus a stub source
// file (so the hash is stable) into a temp directory rooted at dir.
// Returns the absolute inventory path and the root dir.
func writeFixture(t *testing.T, dir string) (invPath, root string) {
	t.Helper()
	root = dir
	invPath = filepath.Join(dir, "inventory.json")
	if err := os.WriteFile(invPath, []byte(fixtureInventory), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	// Make the referenced source files exist so fileHash doesn't fail.
	if err := os.MkdirAll(filepath.Join(dir, "internal", "rules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"fake.go", "other.go"} {
		p := filepath.Join(dir, "internal", "rules", name)
		if err := os.WriteFile(p, []byte("package rules // "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return invPath, root
}

func TestRun_EmitsFixtureInventory(t *testing.T) {
	tmp := t.TempDir()
	inv, root := writeFixture(t, tmp)
	outDir := filepath.Join(tmp, "out")

	var stdout, stderr bytes.Buffer
	err := Run([]string{
		"-inventory", inv,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run: %v\nstderr: %s", err, stderr.String())
	}

	// Expect two files: fake.go and other.go.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read outDir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	wantNames := []string{"zz_meta_fake_gen.go", "zz_meta_other_gen.go"}
	for _, want := range wantNames {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing emitted file %q (got %v)", want, names)
		}
	}

	// Inspect the multi-rule file.
	fakePath := filepath.Join(outDir, "zz_meta_fake_gen.go")
	data, err := os.ReadFile(fakePath)
	if err != nil {
		t.Fatalf("read %s: %v", fakePath, err)
	}
	src := string(data)

	// gofmt round-trip: reformatting yields the same bytes.
	formatted, err := format.Source(data)
	if err != nil {
		t.Fatalf("emitted source does not gofmt: %v\n---\n%s", err, src)
	}
	if !bytes.Equal(formatted, data) {
		t.Errorf("emitted source is not canonically gofmt'd")
	}

	// Must include the Meta() signatures for all three fake.go rules,
	// sorted alphabetically by struct type.
	wantSnippets := []string{
		`func (r *BoolFakeRule) Meta() registry.RuleDescriptor`,
		`func (r *CognitiveFakeRule) Meta() registry.RuleDescriptor`,
		`func (r *RegexFakeRule) Meta() registry.RuleDescriptor`,
		`[]string{"threshold"}`,
		`registry.OptInt`,
		`registry.OptBool`,
		`registry.OptRegex`,
		`"[A-Z][a-zA-Z0-9]*"`,
		`&registry.OracleFilter{}`, // treeSitterOnlyFilter
		`// TODO(krit-gen): schema missing for FakeField`,
		`value.(*regexp.Regexp)`,
		`value.(int)`,
		`value.(bool)`,
	}
	for _, s := range wantSnippets {
		if !strings.Contains(src, s) {
			t.Errorf("emitted source missing snippet %q\n---\n%s", s, src)
		}
	}

	// Deterministic ordering: Bool < Cognitive < Regex.
	boolIdx := strings.Index(src, "BoolFakeRule")
	cogIdx := strings.Index(src, "CognitiveFakeRule")
	regIdx := strings.Index(src, "RegexFakeRule")
	if !(boolIdx < cogIdx && cogIdx < regIdx) {
		t.Errorf("rules not sorted by struct type: bool=%d cog=%d reg=%d", boolIdx, cogIdx, regIdx)
	}

	// Other file carries the oracle + string-list option.
	otherPath := filepath.Join(outDir, "zz_meta_other_gen.go")
	otherData, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatalf("read %s: %v", otherPath, err)
	}
	other := string(otherData)
	for _, s := range []string{
		`func (r *OracleFakeRule) Meta() registry.RuleDescriptor`,
		`&registry.OracleFilter{AllFiles: true}`,
		`registry.OptStringList`,
		`[]string(nil)`,
		`value.([]string)`,
	} {
		if !strings.Contains(other, s) {
			t.Errorf("other.go emitted source missing snippet %q\n---\n%s", s, other)
		}
	}
}

func TestRun_EmitsMetaIndex(t *testing.T) {
	tmp := t.TempDir()
	inv, root := writeFixture(t, tmp)
	outDir := filepath.Join(tmp, "out")

	var stdout, stderr bytes.Buffer
	err := Run([]string{
		"-inventory", inv,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run: %v\nstderr: %s", err, stderr.String())
	}

	indexPath := filepath.Join(outDir, "zz_meta_index_gen.go")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}
	src := string(data)

	// gofmt round-trip: reformatting yields the same bytes.
	formatted, err := format.Source(data)
	if err != nil {
		t.Fatalf("index does not gofmt: %v\n---\n%s", err, src)
	}
	if !bytes.Equal(formatted, data) {
		t.Errorf("index source is not canonically gofmt'd")
	}

	// Every rule struct in the fixture should appear, sorted.
	wantSnippets := []string{
		"func AllMetaProviders() []registry.MetaProvider",
		"(*BoolFakeRule)(nil),",
		"(*CognitiveFakeRule)(nil),",
		"(*OracleFakeRule)(nil),",
		"(*RegexFakeRule)(nil),",
		"func metaByName() map[string]registry.RuleDescriptor",
		"metaByNameOnce sync.Once",
	}
	for _, s := range wantSnippets {
		if !strings.Contains(src, s) {
			t.Errorf("index missing snippet %q\n---\n%s", s, src)
		}
	}

	// Deterministic order: Bool < Cognitive < Oracle < Regex.
	order := []string{"BoolFakeRule", "CognitiveFakeRule", "OracleFakeRule", "RegexFakeRule"}
	prev := -1
	for _, name := range order {
		idx := strings.Index(src, "(*"+name+")(nil)")
		if idx < 0 {
			t.Errorf("missing %s in index", name)
			continue
		}
		if idx <= prev {
			t.Errorf("index not sorted: %s appears at %d (prev=%d)", name, idx, prev)
		}
		prev = idx
	}
}

func TestRun_VerifyDetectsStaleOutput(t *testing.T) {
	tmp := t.TempDir()
	inv, root := writeFixture(t, tmp)
	outDir := filepath.Join(tmp, "out")

	// First pass: generate.
	if err := Run([]string{
		"-inventory", inv,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", root,
	}, io.Discard, io.Discard); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	// Clean verify pass must succeed.
	if err := Run([]string{
		"-inventory", inv,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", root,
		"-verify",
	}, io.Discard, io.Discard); err != nil {
		t.Fatalf("clean verify: %v", err)
	}

	// Mutate one of the emitted files and expect verify to fail.
	mutate := filepath.Join(outDir, "zz_meta_fake_gen.go")
	data, err := os.ReadFile(mutate)
	if err != nil {
		t.Fatalf("read %s: %v", mutate, err)
	}
	if err := os.WriteFile(mutate, append(data, []byte("// drift\n")...), 0o644); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if err := Run([]string{
		"-inventory", inv,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", root,
		"-verify",
	}, io.Discard, io.Discard); err == nil {
		t.Fatalf("expected verify to fail on drifted file")
	}
}

func TestRun_FlagsAndErrors(t *testing.T) {
	// Missing -inventory -> error.
	err := Run(nil, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-inventory is required") {
		t.Fatalf("expected -inventory required error, got %v", err)
	}

	tmp := t.TempDir()
	inv, root := writeFixture(t, tmp)

	// Unknown ruleset -> error (filter matches nothing).
	err = Run([]string{
		"-inventory", inv,
		"-out", filepath.Join(tmp, "out"),
		"-rulesets", "does-not-exist",
		"-root", root,
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no rules matched filter") {
		t.Fatalf("expected filter-miss error, got %v", err)
	}
}

func TestParseRulesetsFilter(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"licensing", []string{"licensing"}},
		{"licensing, naming , performance ", []string{"licensing", "naming", "performance"}},
	}
	for _, c := range cases {
		got := parseRulesetsFilter(c.in)
		if c.want == nil {
			if got != nil {
				t.Errorf("parseRulesetsFilter(%q) = %v, want nil", c.in, got)
			}
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseRulesetsFilter(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for _, w := range c.want {
			if !got[w] {
				t.Errorf("parseRulesetsFilter(%q) missing %q (got %v)", c.in, w, got)
			}
		}
	}
}

func TestEmittedFilename(t *testing.T) {
	cases := []struct{ in, out string }{
		{"internal/rules/licensing.go", "zz_meta_licensing_gen.go"},
		{"internal/rules/style_classes.go", "zz_meta_style_classes_gen.go"},
		{"foo.go", "zz_meta_foo_gen.go"},
	}
	for _, c := range cases {
		got := emittedFilename(c.in)
		if got != c.out {
			t.Errorf("emittedFilename(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

// TestRun_ExcludedStructsSkipped verifies that rules whose StructType is
// in the excludedStructs set are NOT emitted by the generator; the output
// file instead contains a NOTE comment pointing at the hand-written
// sibling. When every rule in a file is excluded, the generator deletes
// the stale generated file entirely.
func TestRun_ExcludedStructsSkipped(t *testing.T) {
	// Synthetic fixture covering two shapes:
	//   - keep.go has one regular rule + one excluded rule → kept emits,
	//     excluded becomes a NOTE comment.
	//   - only.go has only an excluded rule → no output file.
	const inventory = `{
	  "generated_at": "test",
	  "source_commit": "deadbeef",
	  "rules": [
	    {
	      "struct_type": "KeepMeRule",
	      "file": "internal/rules/keep.go",
	      "id": "KeepMe",
	      "ruleset": "fake",
	      "severity": "warning",
	      "description": "",
	      "default_active": true,
	      "fix_level": "",
	      "confidence": 0,
	      "node_types": [],
	      "oracle": null,
	      "options": [],
	      "warnings": []
	    },
	    {
	      "struct_type": "ExcludedInlineRule",
	      "file": "internal/rules/keep.go",
	      "id": "ExcludedInline",
	      "ruleset": "fake",
	      "severity": "warning",
	      "description": "",
	      "default_active": true,
	      "fix_level": "",
	      "confidence": 0,
	      "node_types": [],
	      "oracle": null,
	      "options": [],
	      "warnings": []
	    },
	    {
	      "struct_type": "ExcludedSoloRule",
	      "file": "internal/rules/only.go",
	      "id": "ExcludedSolo",
	      "ruleset": "fake",
	      "severity": "warning",
	      "description": "",
	      "default_active": true,
	      "fix_level": "",
	      "confidence": 0,
	      "node_types": [],
	      "oracle": null,
	      "options": [],
	      "warnings": []
	    }
	  ],
	  "warnings": [],
	  "stats": {}
	}`

	tmp := t.TempDir()
	invPath := filepath.Join(tmp, "inventory.json")
	if err := os.WriteFile(invPath, []byte(inventory), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "internal", "rules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"keep.go", "only.go"} {
		p := filepath.Join(tmp, "internal", "rules", name)
		if err := os.WriteFile(p, []byte("package rules // "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// Temporarily extend the exclusion set for this test.
	prev := excludedStructs
	excludedStructs = map[string]bool{
		"ExcludedInlineRule": true,
		"ExcludedSoloRule":   true,
	}
	defer func() { excludedStructs = prev }()

	outDir := filepath.Join(tmp, "out")
	var stdout, stderr bytes.Buffer
	if err := Run([]string{
		"-inventory", invPath,
		"-out", outDir,
		"-rulesets", "fake",
		"-root", tmp,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v\nstderr: %s", err, stderr.String())
	}

	// keep.go should emit a file, with KeepMeRule present and a NOTE
	// comment pointing at the hand-written ExcludedInline meta file.
	keepPath := filepath.Join(outDir, "zz_meta_keep_gen.go")
	keepData, err := os.ReadFile(keepPath)
	if err != nil {
		t.Fatalf("read %s: %v", keepPath, err)
	}
	keep := string(keepData)
	if !strings.Contains(keep, `func (r *KeepMeRule) Meta() registry.RuleDescriptor`) {
		t.Errorf("keep.go emitted file missing KeepMeRule.Meta(): %s", keep)
	}
	if strings.Contains(keep, `func (r *ExcludedInlineRule) Meta()`) {
		t.Errorf("keep.go emitted file still has ExcludedInlineRule.Meta() — should be excluded")
	}
	if !strings.Contains(keep, `// NOTE: Meta() for ExcludedInlineRule is hand-written in meta_excluded_inline.go.`) {
		t.Errorf("keep.go emitted file missing NOTE comment for ExcludedInlineRule: %s", keep)
	}

	// only.go should not emit a file at all (no rules survive exclusion).
	onlyPath := filepath.Join(outDir, "zz_meta_only_gen.go")
	if _, err := os.Stat(onlyPath); err == nil {
		t.Errorf("only.go should not have emitted a file (all rules excluded)")
	}

	// Handwritten filename helper: smoke test.
	got := handWrittenMetaFilename("PublicToInternalLeakyAbstractionRule")
	if got != "meta_public_to_internal_leaky_abstraction.go" {
		t.Errorf("handWrittenMetaFilename = %q, want meta_public_to_internal_leaky_abstraction.go", got)
	}
}

func TestRenderDefault_NumericFromJSON(t *testing.T) {
	// JSON numbers decode as float64 — renderDefault must coerce back
	// to an int literal for OptInt.
	var opt optionEntry
	if err := json.Unmarshal([]byte(`{
		"field": "Threshold", "go_type": "int", "yaml_key": "threshold",
		"aliases": [], "default": 15, "description": ""
	}`), &opt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := renderDefault(opt); got != "15" {
		t.Errorf("renderDefault int = %q, want %q", got, "15")
	}
}
