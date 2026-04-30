package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"gopkg.in/yaml.v3"
)

type javaReadinessMatrix struct {
	Version        int                      `yaml:"version"`
	Closure        []string                 `yaml:"closureCriteria"`
	Infrastructure map[string]javaReadiness `yaml:"infrastructure"`
	RuleSetDefault map[string]javaReadiness `yaml:"ruleSetDefaults"`
	Rules          map[string]javaReadiness `yaml:"rules"`
}

type javaReadiness struct {
	Status   v2.LanguageSupportStatus `yaml:"status"`
	Reason   string                   `yaml:"reason"`
	Issue    int                      `yaml:"issue"`
	Evidence []string                 `yaml:"evidence"`
	Fixtures []string                 `yaml:"fixtures"`
}

func TestJavaSupportReadinessCodeCoversDefaultActiveRules(t *testing.T) {
	matrix := JavaSupportReadiness()
	if matrix.Version != 1 {
		t.Fatalf("readiness matrix version = %d, want 1", matrix.Version)
	}
	if len(matrix.ClosureCriteria) == 0 {
		t.Fatal("readiness matrix must define closure criteria")
	}
	if len(matrix.Infrastructure) == 0 {
		t.Fatal("readiness matrix must define infrastructure readiness")
	}

	var missing []string
	for _, r := range v2.Registry {
		if r == nil || !IsDefaultActive(r.ID) {
			continue
		}
		entry, ok := JavaSupportForRule(r)
		if !ok {
			missing = append(missing, r.ID+" (ruleset "+r.Category+")")
			continue
		}
		validateJavaSupportEntry(t, "rule "+r.ID, entry)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("default-active rules missing Java readiness status:\n  %s", strings.Join(missing, "\n  "))
	}
}

func TestJavaSupportReadinessCodeReferencesKnownRules(t *testing.T) {
	matrix := JavaSupportReadiness()
	known := make(map[string]bool, len(v2.Registry))
	for _, r := range v2.Registry {
		if r != nil {
			known[r.ID] = true
		}
	}
	var unknown []string
	for id, entry := range matrix.Rules {
		validateJavaSupportEntry(t, "rule "+id, entry)
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		t.Fatalf("readiness matrix references unknown rules:\n  %s", strings.Join(unknown, "\n  "))
	}
}

func TestMetaForV2RuleIncludesJavaSupportClassification(t *testing.T) {
	explicit := mustV2RuleByID(t, "AddJavascriptInterface")
	meta, ok := MetaForV2Rule(explicit)
	if !ok {
		t.Fatal("AddJavascriptInterface metadata not found")
	}
	support, ok := meta.LanguageSupport[JavaLanguageSupportKey]
	if !ok {
		t.Fatal("AddJavascriptInterface metadata missing Java support classification")
	}
	if support.Status != v2.LanguageSupportSupported {
		t.Fatalf("AddJavascriptInterface Java support = %s, want %s", support.Status, v2.LanguageSupportSupported)
	}

	fallback := mustV2RuleByID(t, "LongMethod")
	meta, ok = MetaForV2Rule(fallback)
	if !ok {
		t.Fatal("LongMethod metadata not found")
	}
	support, ok = meta.LanguageSupport[JavaLanguageSupportKey]
	if !ok {
		t.Fatal("LongMethod metadata missing ruleset-default Java support classification")
	}
	if support.Status != v2.LanguageSupportPending {
		t.Fatalf("LongMethod Java support = %s, want %s", support.Status, v2.LanguageSupportPending)
	}
	if support.Issue != 700 {
		t.Fatalf("LongMethod Java support issue = %d, want 700", support.Issue)
	}
}

func TestJavaSupportReadinessDocsMirrorCode(t *testing.T) {
	doc := loadJavaReadinessMatrix(t)
	code := JavaSupportReadiness()

	if doc.Version != code.Version {
		t.Fatalf("docs readiness matrix version = %d, want %d", doc.Version, code.Version)
	}
	if !reflect.DeepEqual(doc.Closure, code.ClosureCriteria) {
		t.Fatalf("docs closure criteria do not match code source of truth")
	}
	assertReadinessMapsMirrorCode(t, "infrastructure", doc.Infrastructure, code.Infrastructure)
	assertReadinessMapsMirrorCode(t, "ruleset default", doc.RuleSetDefault, code.RuleSetDefaults)
	assertReadinessMapsMirrorCode(t, "rule", doc.Rules, code.Rules)
}

func loadJavaReadinessMatrix(t *testing.T) javaReadinessMatrix {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "java-support-readiness.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var matrix javaReadinessMatrix
	if err := yaml.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return matrix
}

func validateJavaSupportEntry(t *testing.T, label string, entry v2.LanguageSupport) {
	t.Helper()
	if !entry.Status.Valid() {
		t.Fatalf("%s has invalid Java readiness status %q", label, entry.Status)
	}
	switch entry.Status {
	case v2.LanguageSupportSupported:
		if len(entry.Evidence) == 0 && len(entry.Fixtures) == 0 {
			t.Fatalf("%s is supported but has no evidence or fixtures", label)
		}
	case v2.LanguageSupportPartial, v2.LanguageSupportPending, v2.LanguageSupportNeedsDesign:
		if entry.Issue == 0 && entry.Reason == "" {
			t.Fatalf("%s is %s but has no issue or reason", label, entry.Status)
		}
	case v2.LanguageSupportNotApplicable:
		if entry.Reason == "" {
			t.Fatalf("%s is not-applicable but has no reason", label)
		}
	}
}

func assertReadinessMapsMirrorCode(t *testing.T, label string, doc map[string]javaReadiness, code map[string]v2.LanguageSupport) {
	t.Helper()
	var missing, extra, mismatched []string
	for key, support := range code {
		entry, ok := doc[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		docSupport := entry.toLanguageSupport()
		validateJavaSupportEntry(t, label+" "+key, support)
		validateJavaSupportEntry(t, label+" "+key+" docs", docSupport)
		if !reflect.DeepEqual(canonicalLanguageSupport(docSupport), canonicalLanguageSupport(support)) {
			mismatched = append(mismatched, key)
		}
	}
	for key := range doc {
		if _, ok := code[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(mismatched)
	var failures []string
	if len(missing) > 0 {
		failures = append(failures, "missing in docs: "+strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		failures = append(failures, "extra in docs: "+strings.Join(extra, ", "))
	}
	if len(mismatched) > 0 {
		failures = append(failures, "different from code: "+strings.Join(mismatched, ", "))
	}
	if len(failures) > 0 {
		t.Fatalf("%s Java readiness docs do not mirror code source of truth:\n  %s", label, strings.Join(failures, "\n  "))
	}
}

func (entry javaReadiness) toLanguageSupport() v2.LanguageSupport {
	return v2.LanguageSupport{
		Status:   entry.Status,
		Reason:   entry.Reason,
		Issue:    entry.Issue,
		Evidence: entry.Evidence,
		Fixtures: entry.Fixtures,
	}
}

func canonicalLanguageSupport(entry v2.LanguageSupport) v2.LanguageSupport {
	if len(entry.Evidence) == 0 {
		entry.Evidence = nil
	}
	if len(entry.Fixtures) == 0 {
		entry.Fixtures = nil
	}
	return entry
}

func mustV2RuleByID(t *testing.T, id string) *v2.Rule {
	t.Helper()
	for _, r := range v2.Registry {
		if r != nil && r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %s not found", id)
	return nil
}
