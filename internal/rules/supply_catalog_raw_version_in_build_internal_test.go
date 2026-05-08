package rules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestVersionCatalogRawVersionInBuild(t *testing.T) {
	root := internalFixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "version-catalog-raw-version-in-build")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "version-catalog-raw-version-in-build")

	t.Run("positive fixture flags every catalog-shadowed coordinate", func(t *testing.T) {
		findings := runVersionCatalogRawVersionInBuild(t, positiveDir)
		expected := map[string]bool{
			"com.squareup.okhttp3:okhttp": false,
			"androidx.core:core-ktx":      false,
			"com.google.code.gson:gson":   false,
		}
		for _, f := range findings {
			for coord := range expected {
				if strings.Contains(f.Message, "'"+coord+":") {
					expected[coord] = true
				}
			}
		}
		for coord, seen := range expected {
			if !seen {
				t.Errorf("expected finding for coordinate %q; got %d findings: %v", coord, len(findings), findings)
			}
		}
		if len(findings) != len(expected) {
			t.Errorf("expected exactly %d findings, got %d: %v", len(expected), len(findings), findings)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runVersionCatalogRawVersionInBuild(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestSplitCoordinate(t *testing.T) {
	cases := []struct {
		in          string
		wantCoord   string
		wantVersion string
		wantOK      bool
	}{
		{"com.squareup.okhttp3:okhttp:4.12.0", "com.squareup.okhttp3:okhttp", "4.12.0", true},
		{"androidx.core:core-ktx:1.12.0", "androidx.core:core-ktx", "1.12.0", true},
		// Two-part — ambiguous, skip.
		{"only:two", "", "", false},
		// Version without a digit — likely a placeholder/property; skip.
		{"a:b:RELEASE", "", "", false},
		// Empty version.
		{"a:b:", "", "", false},
		// Bad characters in name.
		{"a:b/c:1.0", "", "", false},
	}
	for _, c := range cases {
		coord, version, ok := splitCoordinate(c.in)
		if ok != c.wantOK || coord != c.wantCoord || version != c.wantVersion {
			t.Errorf("splitCoordinate(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, coord, version, ok, c.wantCoord, c.wantVersion, c.wantOK)
		}
	}
}

func TestMaskGradleComments(t *testing.T) {
	t.Run("line comment is masked", func(t *testing.T) {
		var inBlock bool
		got := maskGradleComments(`x // "a:b:1.0"`, &inBlock)
		if strings.Contains(got, `"a:b:1.0"`) {
			t.Errorf("expected literal in line-comment to be masked, got %q", got)
		}
	})
	t.Run("block comment spans lines", func(t *testing.T) {
		var inBlock bool
		l1 := maskGradleComments(`/* "a:b:1.0"`, &inBlock)
		if !inBlock {
			t.Fatalf("expected to remain inside block comment after %q", l1)
		}
		if strings.Contains(l1, `"a:b:1.0"`) {
			t.Errorf("expected block-comment content masked on opening line, got %q", l1)
		}
		l2 := maskGradleComments(`"a:b:1.0" */ x`, &inBlock)
		if inBlock {
			t.Fatalf("expected to exit block comment after %q", l2)
		}
		if strings.Contains(l2, `"a:b:1.0"`) {
			t.Errorf("expected pre-close content masked, got %q", l2)
		}
	})
	t.Run("string literal preserved", func(t *testing.T) {
		var inBlock bool
		got := maskGradleComments(`implementation("com.example:foo:1.0")`, &inBlock)
		if !strings.Contains(got, `"com.example:foo:1.0"`) {
			t.Errorf("expected literal preserved, got %q", got)
		}
	})
}

func runVersionCatalogRawVersionInBuild(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()
	graph, err := module.DiscoverModules(projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}
	rule := &VersionCatalogRawVersionInBuildRule{
		BaseRule: BaseRule{RuleName: "VersionCatalogRawVersionInBuild", RuleSetName: supplyChainRuleSet, Sev: "warning"},
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}
