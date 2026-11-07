package rules_test

import (
	"github.com/kaeawc/krit/internal/rules"
	"strings"
	"testing"
)

func TestCalleeAnchor_ReportsOnCalledMethodName(t *testing.T) {
	cases := []struct {
		rule, code, callee string
	}{
		{"BitmapDecodeWithoutOptions", "import android.graphics.BitmapFactory\nfun f(s: java.io.InputStream) = BitmapFactory.decodeStream(s)\n", "decodeStream"},
		{"UpperLowerInvariantMisuse", "fun f(name: String, v: android.widget.TextView) {\n    v.setText(name.uppercase())\n}\n", "uppercase"},
		{"CursorLoopWithColumnIndexInLoop", "fun f(c: android.database.Cursor) {\n    while (c.moveToNext()) {\n        c.getColumnIndex(\"a\")\n    }\n}\n", "getColumnIndex"},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			findings := runRuleByName(t, tc.rule, tc.code)
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1", len(findings))
			}
			f := findings[0]
			line := strings.Split(tc.code, "\n")[f.Line-1]
			if !strings.HasPrefix(line[f.Col-1:], tc.callee) {
				t.Errorf("finding column %d starts %q, want the callee %q", f.Col, line[f.Col-1:], tc.callee)
			}
		})
	}
}

func TestSafeCast_RegisteredAliasesNameWhatItReports(t *testing.T) {
	aliases := rules.AllSuppressionAliases()["SafeCast"]
	for _, want := range []string{"RedundantCast", "UselessCast"} {
		found := false
		for _, a := range aliases {
			found = found || a == want
		}
		if !found {
			t.Errorf("SafeCast aliases %v lack %q", aliases, want)
		}
	}
}

func TestBitmapDecodeWithoutOptions_SkipsSmallImageContexts(t *testing.T) {
	skipped := []string{
		"import android.graphics.BitmapFactory\nfun loadAvatar(p: String) = BitmapFactory.decodeFile(p)\n",
		"import android.graphics.BitmapFactory\nclass TinyDecoder {\n    fun load(p: String) = BitmapFactory.decodeFile(p)\n}\n",
		"import android.graphics.BitmapFactory\nobject IconLoader {\n    fun load(p: java.io.InputStream) = BitmapFactory.decodeStream(p)\n}\n",
	}
	for _, code := range skipped {
		if findings := runRuleByName(t, "BitmapDecodeWithoutOptions", code); len(findings) != 0 {
			t.Errorf("expected no findings for %q, got %d", code, len(findings))
		}
	}
}
