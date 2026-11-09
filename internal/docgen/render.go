package docgen

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
)

const maxFixtureLines = 80

// ruleView is a rule plus the context a page needs.
type ruleView struct {
	rule     *api.Rule
	meta     api.RuleDescriptor
	detekt   detektInfo
	fixtures fixtureIndex
	fsys     fs.FS
}

func newRuleView(r *api.Rule, src Sources, d detektInfo, f fixtureIndex) ruleView {
	meta, _ := rules.MetaForRule(r)
	return ruleView{rule: r, meta: meta, detekt: d, fixtures: f, fsys: src.Fixtures}
}

// renderRule returns the markdown page for one rule.
func (v ruleView) render() string {
	var b strings.Builder
	r := v.rule
	fmt.Fprintf(&b, "# %s\n\n", r.ID)
	if r.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", r.Description)
	}
	v.writeSummary(&b)
	v.writeConfig(&b)
	v.writeCompatibility(&b)
	v.writeExamples(&b)
	v.writeRelated(&b)
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func (v ruleView) writeSummary(b *strings.Builder) {
	r := v.rule
	fix := "no"
	if lvl, ok := rules.GetV2FixLevel(r); ok {
		fix = "yes (" + lvl.String() + ")"
	}
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(b, "| Rule ID | `%s` |\n", r.ID)
	fmt.Fprintf(b, "| Category | `%s` |\n", r.Category)
	fmt.Fprintf(b, "| Severity | %s |\n", r.Sev)
	fmt.Fprintf(b, "| Active by default | %s |\n", yesNo(rules.DeclaredDefaultActive(r)))
	fmt.Fprintf(b, "| Auto-fixable | %s |\n", fix)
	fmt.Fprintf(b, "| Maturity | %s |\n", r.Maturity)
	fmt.Fprintf(b, "| Precision | %s |\n", rules.V2RulePrecision(r))
	if len(r.Aliases) > 0 {
		fmt.Fprintf(b, "| Aliases | %s |\n", codeList(r.Aliases))
	}
	b.WriteString("\n")
}

func codeList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "`" + s + "`"
	}
	return strings.Join(q, ", ")
}

func (v ruleView) writeConfig(b *strings.Builder) {
	b.WriteString("## Configuration\n\n")
	r := v.rule
	fmt.Fprintf(b, "```yaml\n%s:\n  %s:\n    active: %t\n", r.Category, r.ID, rules.DeclaredDefaultActive(r))
	for _, o := range v.meta.Options {
		if o.Default != nil {
			fmt.Fprintf(b, "    %s: %s\n", o.Name, yamlValue(o.Default))
		}
	}
	b.WriteString("```\n\n")
	if len(v.meta.Options) == 0 {
		b.WriteString("This rule has no options besides `active`, `excludes` and `includes`.\n\n")
		return
	}
	b.WriteString("| Option | Type | Default | Description |\n|---|---|---|---|\n")
	for _, o := range v.meta.Options {
		name := "`" + o.Name + "`"
		if len(o.Aliases) > 0 {
			name += " (alias: " + codeList(o.Aliases) + ")"
		}
		def := "none"
		if o.Default != nil {
			def = "`" + yamlValue(o.Default) + "`"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", name, o.Type, def, cell(o.Description))
	}
	b.WriteString("\n")
}

// cell makes s safe inside a markdown table cell.
func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ")
}

// yamlValue renders a default as a YAML flow value.
func yamlValue(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		v = s.String()
	}
	out, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(out)
}

func (v ruleView) writeCompatibility(b *strings.Builder) {
	lines := v.compatLines()
	if len(lines) == 0 {
		return
	}
	b.WriteString("## Compatibility\n\n")
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
	b.WriteString("\n")
}

func (v ruleView) compatLines() []string {
	var lines []string
	id := v.rule.ID
	if set, ok := v.detekt.rulesets[id]; ok {
		lines = append(lines, fmt.Sprintf("Mirrors detekt 1.23.8 [`%s`](https://detekt.dev/docs/rules/%s/#%s) (same rule name and config keys).",
			id, set, strings.ToLower(id)))
	}
	for _, old := range v.detekt.renamed[id] {
		lines = append(lines, fmt.Sprintf("Replaces detekt `%s`; the detekt config importer maps it to this rule.", old))
	}
	if lint := rules.LintSuppressionAliases(v.rule); len(lint) > 0 {
		lines = append(lines, "Mirrors Android Lint "+codeList(lint)+"; `@Suppress` and `lint.xml` use those IDs too.")
	} else if v.rule.Category == "android-lint" {
		lines = append(lines, "Mirrors an Android Lint check of the same name.")
	}
	for _, note := range v.detekt.parity[id] {
		lines = append(lines, "Parity note: "+note)
	}
	return lines
}

func (v ruleView) writeExamples(b *strings.Builder) {
	bad := v.fixtures.find("positive", v.rule.Category, v.rule.ID)
	good := v.fixtures.find("negative", v.rule.Category, v.rule.ID)
	if bad == "" && good == "" {
		return
	}
	b.WriteString("## Examples\n\n")
	v.writeFixture(b, "Non-compliant (reported)", bad)
	v.writeFixture(b, "Compliant (not reported)", good)
}

func (v ruleView) writeFixture(b *strings.Builder, title, p string) {
	if p == "" {
		return
	}
	data, err := fs.ReadFile(v.fsys, p)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	note := ""
	if len(lines) > maxFixtureLines {
		lines = lines[:maxFixtureLines]
		note = " (excerpt)"
	}
	fmt.Fprintf(b, "**%s**%s\n\n```%s\n%s\n```\n\n", title, note, fenceLang(p), strings.Join(lines, "\n"))
}

func fenceLang(p string) string {
	switch path.Ext(p) {
	case ".xml":
		return "xml"
	case ".java":
		return "java"
	default:
		return "kotlin"
	}
}

func (v ruleView) writeRelated(b *strings.Builder) {
	r := v.rule
	if d := r.Deprecated; d != nil {
		fmt.Fprintf(b, "!!! warning \"Deprecated\"\n    Deprecated since %s. %s", d.Since, d.Reason)
		if d.ReplacedBy != "" {
			fmt.Fprintf(b, " Use %s", v.ruleLink(d.ReplacedBy))
		}
		b.WriteString("\n\n")
	}
	if len(r.RelatedRules) == 0 {
		return
	}
	b.WriteString("## Related rules\n\n")
	for _, id := range r.RelatedRules {
		fmt.Fprintf(b, "- %s\n", v.ruleLink(id))
	}
	b.WriteString("\n")
}

// ruleLink links to another rule's page when it is registered.
func (v ruleView) ruleLink(id string) string {
	for _, o := range api.Registry {
		if o.ID == id {
			rel := api.DocsSlug(id) + ".md"
			if o.Category != v.rule.Category {
				rel = "../" + api.DocsSlug(o.Category) + "/" + api.DocsSlug(id) + ".md"
			}
			return "[`" + id + "`](" + rel + ")"
		}
	}
	return "`" + id + "`"
}
