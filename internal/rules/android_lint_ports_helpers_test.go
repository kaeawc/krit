package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func findingLines(findings []scanner.Finding) string {
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, fmt.Sprint(f.Line))
	}
	return strings.Join(lines, ",")
}

// findingPositions joins findings as "line:col" sorted by position.
func findingPositions(findings []scanner.Finding) string {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Col < findings[j].Col
	})
	positions := make([]string, 0, len(findings))
	for _, f := range findings {
		positions = append(positions, fmt.Sprintf("%d:%d", f.Line, f.Col))
	}
	return strings.Join(positions, ",")
}

// parseManifestForTest parses manifest XML into the rule-facing model, as the pipeline does.
func parseManifestForTest(t *testing.T, xml string) *rules.Manifest {
	t.Helper()
	parsed, err := android.ParseManifestBytes([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "AndroidManifest.xml")
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	return pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, path))
}

// manifestRulePositions runs a manifest rule on XML and returns its findings as sorted "line:col" positions.
func manifestRulePositions(t *testing.T, ruleID, xml string) string {
	t.Helper()
	return findingPositions(runManifestRule(findManifestRule(t, ruleID), parseManifestForTest(t, xml)))
}

// resourcePositions runs a resource rule over the given res/ files and returns "path:line:col" entries, sorted.
func resourcePositions(t *testing.T, ruleID string, files map[string]string) []string {
	t.Helper()
	resDir := filepath.Join(t.TempDir(), "res")
	for name, body := range files {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	findings := runResourceRule(findResourceRule(t, ruleID), idx)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Col < findings[j].Col
	})
	var out []string
	for _, f := range findings {
		rel, _ := filepath.Rel(resDir, f.File)
		out = append(out, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), f.Line, f.Col))
	}
	return out
}

// projectManifestFindings writes files (including AndroidManifest.xml) into a temp directory and runs a manifest rule
// on that manifest, returning "path:line:col" entries relative to the directory, sorted.
func projectManifestFindings(t *testing.T, ruleID string, files map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	parsed, err := android.ParseManifestBytes([]byte(files["AndroidManifest.xml"]))
	if err != nil {
		t.Fatal(err)
	}
	m := pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, manifestPath))
	findings := runManifestRule(findManifestRule(t, ruleID), m)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Col < findings[j].Col
	})
	var out []string
	for _, f := range findings {
		rel, _ := filepath.Rel(dir, f.File)
		out = append(out, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), f.Line, f.Col))
	}
	return out
}

// sourcePositionsWithResources runs a source rule over a Java or Kotlin file with the resources under resFiles.
func sourcePositionsWithResources(t *testing.T, ruleID, filename, code string, resFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	resDir := filepath.Join(dir, "res")
	for name, body := range resFiles {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "src", filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	var file *scanner.File
	if strings.HasSuffix(filename, ".java") {
		file, err = scanner.ParseJavaFile(context.Background(), path)
	} else {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if err != nil {
		t.Fatal(err)
	}
	rule := findRegistryRule(t, ruleID)
	wants := map[string]bool{}
	for _, typ := range rule.NodeTypes {
		wants[typ] = true
	}
	collector := scanner.NewFindingCollector(0)
	for i := range file.FlatTree.Types {
		flatIdx := uint32(i)
		if !wants[file.FlatType(flatIdx)] {
			continue
		}
		node := file.FlatTree.Node(flatIdx)
		rule.Check(&api.Context{File: file, Idx: flatIdx, Node: &node, Rule: rule, ResourceIndex: idx, DefaultConfidence: rule.Confidence, Collector: collector})
	}
	return findingPositions(collector.Columns().Findings())
}

func findRegistryRule(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %q not registered", id)
	return nil
}
