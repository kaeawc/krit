package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterComposeRules_AllRulesPresent guards against silently dropping
// a rule when registerComposeRules() splits its inlined blocks across
// per-rule helper functions.
func TestRegisterComposeRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"ComposeColumnRowInScrollable",
		"ComposeDerivedStateMisuse",
		"ComposeLambdaCapturesUnstableState",
		"ComposeModifierFillAfterSize",
		"ComposeModifierBackgroundAfterClip",
		"ComposeModifierClickableBeforePadding",
		"ComposePreviewAnnotationMissing",
		"ComposeMutableDefaultArgument",
		"ComposeStringResourceInsideLambda",
		"ComposeRememberWithoutKey",
		"ComposeLaunchedEffectWithoutKeys",
		"ComposeMutableStateInComposition",
		"ComposeStatefulDefaultParameter",
		"ComposePreviewWithBackingState",
		"ComposeDisposableEffectMissingDispose",
		"ComposeModifierPassedThenChained",
		"ComposeSideEffectInComposition",
		"ComposeUnstableParameter",
		"ComposeRememberSaveableNonParcelable",
	}
	want := make(map[string]struct{}, len(expected))
	for _, id := range expected {
		want[id] = struct{}{}
	}
	registered := make(map[string]*api.Rule, len(expected))
	for _, r := range api.Registry {
		if _, ok := want[r.ID]; ok {
			registered[r.ID] = r
		}
	}
	for _, id := range expected {
		rule, ok := registered[id]
		if !ok {
			t.Errorf("expected compose rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("compose rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("compose rule %q has no Check function", id)
		}
	}
}
