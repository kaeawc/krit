package rules_test

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// applyLintXMLOptions applies a case's lint.xml issue options, which the Lint run reads, to the krit rules with the
// same ID: a kebab-case Lint option (`import-aliases`) sets the camelCase string-list option of the rule's
// descriptor (`importAliases`). The options are cleared again when the test ends.
func applyLintXMLOptions(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var lint struct {
		Issues []struct {
			ID      string `xml:"id,attr"`
			Options []struct {
				Name  string `xml:"name,attr"`
				Value string `xml:"value,attr"`
			} `xml:"option"`
		} `xml:"issue"`
	}
	if err := xml.Unmarshal(data, &lint); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for _, issue := range lint.Issues {
		var rule *api.Rule
		for _, r := range api.Registry {
			if r.ID == issue.ID {
				rule = r
			}
		}
		if rule == nil {
			continue
		}
		meta, ok := rules.MetaForRule(rule)
		if !ok {
			t.Fatalf("%s: rule %s has no options", path, issue.ID)
		}
		for _, opt := range issue.Options {
			name := kebabToCamel(opt.Name)
			found := false
			for _, o := range meta.Options {
				if o.Name != name {
					continue
				}
				if o.Type != api.OptStringList {
					t.Fatalf("%s: %s.%s is not a string list", path, issue.ID, name)
				}
				o.Apply(rule.Implementation, []string{opt.Value})
				apply := o.Apply
				impl := rule.Implementation
				t.Cleanup(func() { apply(impl, []string(nil)) })
				found = true
			}
			if !found {
				t.Fatalf("%s: rule %s has no option %s", path, issue.ID, name)
			}
		}
	}
}

func kebabToCamel(s string) string {
	parts := strings.Split(s, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
