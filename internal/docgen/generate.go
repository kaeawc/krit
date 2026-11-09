package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
)

// Generate writes docs/rules/<category>/index.md, one page per rule, and
// categories.md under outDir (the docs "rules" directory).
func Generate(rs []*api.Rule, src Sources, outDir string) error {
	if err := checkSlugs(rs); err != nil {
		return err
	}
	detekt := loadDetektInfo(src)
	fixtures := indexFixtures(src.Fixtures)
	byCat := groupByCategory(rs)
	for _, cat := range sortedKeys(byCat) {
		dir := filepath.Join(outDir, api.DocsSlug(cat))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, "index.md"), renderCategory(cat, byCat[cat])); err != nil {
			return err
		}
		for _, r := range byCat[cat] {
			page := newRuleView(r, src, detekt, fixtures).render()
			if err := writeFile(filepath.Join(dir, api.DocsSlug(r.ID)+".md"), page); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return writeFile(filepath.Join(outDir, "categories.md"), renderCategories(byCat))
}

func writeFile(p, content string) error {
	return os.WriteFile(p, []byte(content), 0o644)
}

func groupByCategory(rs []*api.Rule) map[string][]*api.Rule {
	out := map[string][]*api.Rule{}
	for _, r := range rs {
		out[r.Category] = append(out[r.Category], r)
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	}
	return out
}

// checkSlugs fails when two rules would share a page, or a rule would
// overwrite its category index.
func checkSlugs(rs []*api.Rule) error {
	seen := map[string]string{}
	for _, r := range rs {
		key := strings.ToLower(api.RuleDocsPath(r.Category, r.ID))
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("rules %s and %s map to the same page %s", prev, r.ID, key)
		}
		if api.DocsSlug(r.ID) == "index" {
			return fmt.Errorf("rule %s collides with the category index page", r.ID)
		}
		seen[key] = r.ID
	}
	return nil
}

func renderCategory(cat string, rs []*api.Rule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s rules\n\n%d rules.\n\n", cat, len(rs))
	b.WriteString("| Rule | Default | Fix | Description |\n|---|---|---|---|\n")
	for _, r := range rs {
		_, fixable := rules.GetV2FixLevel(r)
		fmt.Fprintf(&b, "| [`%s`](%s.md) | %s | %s | %s |\n",
			r.ID, api.DocsSlug(r.ID), yesNo(rules.DeclaredDefaultActive(r)), yesNo(fixable), cell(r.Description))
	}
	return b.String()
}

func renderCategories(byCat map[string][]*api.Rule) string {
	var b strings.Builder
	b.WriteString("# Rule reference\n\nEvery rule has its own page, generated from the rule registry.\n\n")
	b.WriteString("| Category | Rules |\n|---|---|\n")
	for _, cat := range sortedKeys(byCat) {
		fmt.Fprintf(&b, "| [%s](%s/index.md) | %d |\n", cat, api.DocsSlug(cat), len(byCat[cat]))
	}
	return b.String()
}
