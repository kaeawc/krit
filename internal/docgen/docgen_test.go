package docgen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kaeawc/krit/config"
	_ "github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
)

const repoRoot = "../.."

func realSources(t *testing.T) Sources {
	t.Helper()
	migration, err := os.ReadFile(filepath.Join(repoRoot, "docs", "detekt-migration.md"))
	if err != nil {
		t.Fatal(err)
	}
	return Sources{
		Fixtures:       os.DirFS(filepath.Join(repoRoot, "tests", "fixtures")),
		DetektDefaults: config.DetektDefaults(),
		Migration:      string(migration),
	}
}

func generate(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "rules")
	if err := Generate(api.Registry, realSources(t), out); err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGeneratePagesMatchDocsURLs(t *testing.T) {
	out := generate(t)
	for _, r := range api.Registry {
		// RuleDocsPath is "rules/<cat>/<slug>/"; the page is <cat>/<slug>.md.
		rel := strings.TrimSuffix(strings.TrimPrefix(api.RuleDocsPath(r.Category, r.ID), "rules/"), "/") + ".md"
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("no page for %s at %s: %v", r.ID, rel, err)
		}
	}
}

func TestGenerateMagicNumberPage(t *testing.T) {
	page := read(t, filepath.Join(generate(t), "style", "magic-number.md"))
	for _, want := range []string{
		"# MagicNumber",
		"| Active by default | yes |",
		"style:\n  MagicNumber:\n    active: true",
		"`ignoreNumbers`",
		"Mirrors detekt 1.23.8 [`MagicNumber`](https://detekt.dev/docs/rules/style/#magicnumber)",
		"**Non-compliant (reported)**",
		"**Compliant (not reported)**",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("MagicNumber page lacks %q", want)
		}
	}
}

func TestGenerateRenamedRuleAndFixable(t *testing.T) {
	out := generate(t)
	page := read(t, filepath.Join(out, "style", "braces-on-if-statements.md"))
	if !strings.Contains(page, "Replaces detekt `MandatoryBracesIfStatements`") {
		t.Errorf("rename note missing:\n%s", page)
	}
	page = read(t, filepath.Join(out, "style", "unused-import.md"))
	if !strings.Contains(page, "| Auto-fixable | yes (") {
		t.Errorf("fixable rule not marked:\n%s", page)
	}
}

func TestGenerateParityNoteAndAndroidLint(t *testing.T) {
	out := generate(t)
	page := read(t, filepath.Join(out, "style", "serial-version-uid-in-serializable-class.md"))
	if !strings.Contains(page, "Parity note:") || !strings.Contains(page, "does not report enum classes") {
		t.Errorf("parity note missing:\n%s", page)
	}
	page = read(t, filepath.Join(out, "android-lint", "accepts-user-certificates.md"))
	if !strings.Contains(page, "Android Lint") {
		t.Errorf("android lint note missing:\n%s", page)
	}
}

func TestGenerateWithoutFixturesOmitsExamples(t *testing.T) {
	src := realSources(t)
	src.Fixtures = nil
	out := t.TempDir()
	if err := Generate(api.Registry, src, out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, filepath.Join(out, "style", "magic-number.md")), "## Examples") {
		t.Error("examples written without fixtures")
	}
}

func TestGeneratedLinksResolve(t *testing.T) {
	out := generate(t)
	link := regexp.MustCompile(`\]\(([^)#:]+\.md)\)`)
	err := filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		for _, m := range link.FindAllStringSubmatch(read(t, p), -1) {
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(p), m[1])); statErr != nil {
				t.Errorf("%s: broken link %s", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMkdocsNavCoversEveryCategory(t *testing.T) {
	nav := read(t, filepath.Join(repoRoot, "mkdocs.yml"))
	for _, r := range api.Registry {
		want := "rules/" + api.DocsSlug(r.Category) + "/index.md"
		if !strings.Contains(nav, want) {
			t.Errorf("mkdocs.yml nav lacks %s (category %s)", want, r.Category)
		}
	}
	if !strings.Contains(nav, "rules/categories.md") {
		t.Error("mkdocs.yml nav lacks rules/categories.md")
	}
}

func TestCheckSlugsRejectsCollisions(t *testing.T) {
	rs := []*api.Rule{{ID: "FooBar", Category: "x"}, {ID: "FOOBar", Category: "x"}}
	if err := checkSlugs(rs); err == nil {
		t.Error("expected collision error")
	}
	if err := checkSlugs([]*api.Rule{{ID: "Index", Category: "x"}}); err == nil {
		t.Error("expected index collision error")
	}
}

func TestBulletsJoinsContinuations(t *testing.T) {
	got := bullets([]string{"- `A` does x", "  and y", "", "- `B` z"})
	if len(got) != 2 || got[0] != "`A` does x and y" {
		t.Errorf("got %q", got)
	}
}
