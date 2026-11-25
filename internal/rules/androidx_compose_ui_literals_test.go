package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func acuiRunRule(t *testing.T, id string, file *scanner.File) []scanner.Finding {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			var out []scanner.Finding
			filter := scanner.BuildSuppressionFilter(file, nil, nil, "")
			cols := NewDispatcher([]*api.Rule{r}, nil).Run(file)
			for _, f := range cols.Findings() {
				if !filter.IsSuppressedAtColumn(f.Rule, f.RuleSet, f.Line, f.Col) {
					out = append(out, f)
				}
			}
			return out
		}
	}
	t.Fatalf("rule %q not registered", id)
	return nil
}

// The Lint issue IDs are the rule IDs, so @SuppressLint written for Android Lint silences krit too.
func TestAcuiLiteralRulesHonorSuppressLint(t *testing.T) {
	file := parseInlineKt(t, `package p

import android.annotation.SuppressLint
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.intl.Locale

@SuppressLint("MissingColorAlphaChannel", "InvalidColorHexValue", "InvalidLanguageTagDelimiter")
fun suppressed() {
    Color(0x123456)
    Color(0x12345)
    Locale("en_US")
}

fun reported() {
    Color(0x123456)
    Color(0x12345)
    Locale("en_US")
}
`)
	for _, id := range []string{"MissingColorAlphaChannel", "InvalidColorHexValue", "InvalidLanguageTagDelimiter"} {
		findings := acuiRunRule(t, id, file)
		if len(findings) != 1 || findings[0].Line < 14 {
			t.Errorf("%s: want one finding in reported(), got %+v", id, findings)
		}
	}
}

// The alpha fix keeps the digits and only prepends the channel, cased like the first letter digit (Lint looks at the
// first non-digit character, so `_` and `L` count as uppercase).
func TestAcuiMissingColorAlphaFixCasing(t *testing.T) {
	file := parseInlineKt(t, `package p

import androidx.compose.ui.graphics.Color

val a = Color(0xab12cd)
val b = Color(0x12_ab_cd)
val c = Color(0x123456L)
val d = Color(0X12abCD)
`)
	want := map[int]string{5: "0xffab12cd", 6: "0xFF12_ab_cd", 7: "0xFF123456L", 8: "0xff12abCD"}
	findings := acuiRunRule(t, "MissingColorAlphaChannel", file)
	if len(findings) != len(want) {
		t.Fatalf("want %d findings, got %+v", len(want), findings)
	}
	for _, f := range findings {
		if f.Fix == nil || f.Fix.Replacement != want[f.Line] {
			t.Errorf("line %d: want fix %q, got %+v", f.Line, want[f.Line], f.Fix)
		}
	}
}

// A tag held in a constant or built by concatenation is reported, but only a literal tag gets Lint's text fix.
func TestAcuiLanguageTagFixOnlyRewritesLiterals(t *testing.T) {
	file := parseInlineKt(t, `package p

import androidx.compose.ui.text.intl.Locale

const val TAG = "en_US"

val a = Locale(TAG)
val b = Locale("en" + "_GB")
val c = Locale("en_AU")
`)
	findings := acuiRunRule(t, "InvalidLanguageTagDelimiter", file)
	if len(findings) != 3 {
		t.Fatalf("want 3 findings, got %+v", findings)
	}
	for _, f := range findings {
		hasFix := f.Fix != nil
		if hasFix != (f.Line == 9) {
			t.Errorf("line %d: fix presence %v", f.Line, hasFix)
		}
		if f.Line == 9 && f.Fix.Replacement != `"en-AU"` {
			t.Errorf("line 9: fix %q", f.Fix.Replacement)
		}
	}
}

// Pinned gap: isKnownLintIssueID only counts rules in the android-lint rule set, so lint.xml and Gradle
// `lint { disable }` entries naming these compose-rule-set Lint IDs are not applied yet. The per-rule opt-in marker
// being added for Lint-ID rules outside android-lint will close it; flip this assertion then.
func TestAcuiLintIDsNotYetKnownToLintXML(t *testing.T) {
	for _, id := range []string{"MissingColorAlphaChannel", "InvalidColorHexValue", "InvalidLanguageTagDelimiter"} {
		if isKnownLintIssueID(id) {
			t.Errorf("%s is now a known Lint issue ID; drop this pinned gap test and note it in the docs", id)
		}
	}
}
