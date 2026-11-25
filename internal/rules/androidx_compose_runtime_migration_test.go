package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// The retired ComposeMutableStateInComposition ID and the retired Lint IDs keep suppressing the rules that replaced
// them, so existing @Suppress / @SuppressLint annotations still apply.
func TestAndroidxComposeRuntimeRetiredIDsSuppress(t *testing.T) {
	cases := []struct {
		rule, suppress, source string
	}{
		{"UnrememberedMutableState", `@Suppress("ComposeMutableStateInComposition")`, `
import androidx.compose.runtime.mutableStateOf

@Composable
fun Counter() {
    val count = mutableStateOf(0)
}`},
		{"UnrememberedMutableState", `@SuppressLint("UnrememberedMutableState")`, `
import androidx.compose.runtime.mutableStateOf

@Composable
fun Counter() {
    val count = mutableStateOf(0)
}`},
		{"RememberInComposition", `@Suppress("UnrememberedMutableInteractionSource")`, `
import androidx.compose.foundation.interaction.MutableInteractionSource

@Composable
fun Card() {
    val source = MutableInteractionSource()
}`},
		{"RememberInComposition", `@Suppress("UnrememberedAnimatable")`, `
import androidx.compose.animation.core.Animatable

@Composable
fun Card() {
    val alpha = Animatable(0f)
}`},
		{"FrequentlyChangingValue", `@Suppress("FrequentlyChangedStateReadInComposition")`, `
import androidx.compose.foundation.lazy.LazyListState

@Composable
fun Card(state: LazyListState) {
    val index = state.firstVisibleItemIndex
}`},
	}
	for _, c := range cases {
		t.Run(c.rule+"/"+c.suppress, func(t *testing.T) {
			header := "package p\n\nimport androidx.compose.runtime.Composable\n"
			plain := parseInline(t, header+c.source)
			findings := runRuleByNameOnFile(t, c.rule, plain)
			if len(findings) != 1 {
				t.Fatalf("expected 1 unsuppressed finding, got %d: %v", len(findings), findings)
			}
			suppressed := parseInline(t, header+insertBeforeComposable(c.source, c.suppress))
			// The dispatcher drops findings it can attribute to an annotated scope; anything left must be dropped by
			// the pipeline's suppression filter, which also maps the aliases.
			filter := scanner.BuildSuppressionFilter(suppressed, nil, nil, "").WithRuleAliases(rules.AllSuppressionAliases())
			for _, f := range runRuleByNameOnFile(t, c.rule, suppressed) {
				if !filter.IsSuppressedAtColumn(c.rule, "compose", f.Line, f.Col) {
					t.Fatalf("%s must suppress %s at %d:%d", c.suppress, c.rule, f.Line, f.Col)
				}
			}
		})
	}
}

func insertBeforeComposable(source, annotation string) string {
	const marker = "@Composable"
	for i := 0; i+len(marker) <= len(source); i++ {
		if source[i:i+len(marker)] == marker {
			return source[:i] + annotation + "\n" + source[i:]
		}
	}
	return source
}
