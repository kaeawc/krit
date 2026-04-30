package rules

import (
	"os"
	"path/filepath"
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
	Status   string   `yaml:"status"`
	Reason   string   `yaml:"reason"`
	Issue    int      `yaml:"issue"`
	Evidence []string `yaml:"evidence"`
	Fixtures []string `yaml:"fixtures"`
}

var validJavaReadinessStatuses = map[string]bool{
	"supported":      true,
	"partial":        true,
	"pending":        true,
	"not-applicable": true,
}

func TestJavaSupportReadinessMatrixCoversDefaultActiveRules(t *testing.T) {
	matrix := loadJavaReadinessMatrix(t)
	if matrix.Version != 1 {
		t.Fatalf("readiness matrix version = %d, want 1", matrix.Version)
	}
	if len(matrix.Closure) == 0 {
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
		entry, ok := matrix.Rules[r.ID]
		if !ok {
			entry, ok = matrix.RuleSetDefault[r.Category]
		}
		if !ok {
			missing = append(missing, r.ID+" (ruleset "+r.Category+")")
			continue
		}
		validateJavaReadinessEntry(t, "rule "+r.ID, entry)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("default-active rules missing Java readiness status:\n  %s", strings.Join(missing, "\n  "))
	}
}

func TestJavaSupportReadinessMatrixReferencesKnownRules(t *testing.T) {
	matrix := loadJavaReadinessMatrix(t)
	known := make(map[string]bool, len(v2.Registry))
	for _, r := range v2.Registry {
		if r != nil {
			known[r.ID] = true
		}
	}
	var unknown []string
	for id, entry := range matrix.Rules {
		validateJavaReadinessEntry(t, "rule "+id, entry)
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		t.Fatalf("readiness matrix references unknown rules:\n  %s", strings.Join(unknown, "\n  "))
	}
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

func validateJavaReadinessEntry(t *testing.T, label string, entry javaReadiness) {
	t.Helper()
	if !validJavaReadinessStatuses[entry.Status] {
		t.Fatalf("%s has invalid Java readiness status %q", label, entry.Status)
	}
	switch entry.Status {
	case "supported":
		if len(entry.Evidence) == 0 && len(entry.Fixtures) == 0 {
			t.Fatalf("%s is supported but has no evidence or fixtures", label)
		}
	case "partial", "pending":
		if entry.Issue == 0 && entry.Reason == "" {
			t.Fatalf("%s is %s but has no issue or reason", label, entry.Status)
		}
	case "not-applicable":
		if entry.Reason == "" {
			t.Fatalf("%s is not-applicable but has no reason", label)
		}
	}
}
