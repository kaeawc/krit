package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runRuleWithIndexedResolver runs one rule with a resolver that indexed the inline file, so
// declared parameter and property types resolve.
func runRuleWithIndexedResolver(t *testing.T, ruleID, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %s not found", ruleID)
	return nil
}

func TestTextViewEdits(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"cast of TextView text to Editable", "import android.widget.TextView\nimport android.text.Editable\nfun f(tv: TextView) { val e = tv.text as Editable }", 1},
		{"cast of getText call", "import android.widget.TextView\nimport android.text.Editable\nfun f(tv: TextView) { val e = tv.getText() as? Editable }", 1},
		{"EditText is editable", "import android.widget.EditText\nimport android.text.Editable\nfun f(et: EditText) { val e = et.text as Editable }", 0},
		{"unrelated receiver", "import android.text.Editable\nfun f(box: Box) { val e = box.text as Editable }", 0},
		{"setText on EditText is normal", "import android.widget.EditText\nfun f(editText: EditText) { editText.setText(\"hello\") }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleWithIndexedResolver(t, "TextViewEdits", "package test\n"+tc.code+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}
