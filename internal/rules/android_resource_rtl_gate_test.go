package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func rtlLayout() *android.ResourceIndex {
	return indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "TextView",
		Line: 2,
		Col:  5,
		Attributes: map[string]string{
			"android:layout_marginRight": "4dp",
			"android:paddingRight":       "4dp",
			"android:textStyle":          "subscript",
		},
	})
}

// rtlLayoutRuleIDs are the layout rules gated like Lint's RtlDetector. RtlCompatResource has its own tests.
var rtlLayoutRuleIDs = []string{"RtlHardcodedResource", "RtlSymmetryResource", "RtlSuperscriptResource"}

func runWithRtlMains(t *testing.T, ruleID string, mains []api.RtlMainProject, known bool) []scanner.Finding {
	t.Helper()
	r := findResourceRule(t, ruleID)
	ctx := &api.Context{ResourceIndex: rtlLayout(), Rule: r, Collector: scanner.NewFindingCollector(0)}
	if known {
		ctx.RtlMainsFor = func(string) ([]api.RtlMainProject, bool) { return mains, true }
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

// TestRtlLayoutRulesFollowLintsRtlApplies encodes RtlDetector.rtlApplies, checked against Lint 32.4.1: the
// including app needs an <application> that does not set supportsRtl="false" and a targetSdk of 17 or more,
// where an unset supportsRtl counts as on.
func TestRtlLayoutRulesFollowLintsRtlApplies(t *testing.T) {
	app := func(minSdk, target int, off bool) api.RtlMainProject {
		return api.RtlMainProject{HasApplication: true, RtlOff: off, MinSdk: minSdk, TargetSdk: target}
	}
	cases := []struct {
		name  string
		mains []api.RtlMainProject
		known bool
		want  bool
		// hardcodedAdvice marks cases where RtlHardcoded still gives its "Consider adding" advice for a minSdk below 17.
		hardcodedAdvice bool
	}{
		{"supportsRtl unset", []api.RtlMainProject{app(21, 21, false)}, true, true, false},
		{"supportsRtl false", []api.RtlMainProject{app(21, 21, true)}, true, false, false},
		{"target 16", []api.RtlMainProject{app(14, 16, false)}, true, false, true},
		{"target 17", []api.RtlMainProject{app(14, 17, false)}, true, true, false},
		{"target unset follows minSdk 21", []api.RtlMainProject{app(21, 0, false)}, true, true, false},
		{"target unset follows minSdk 16", []api.RtlMainProject{app(16, 0, false)}, true, false, true},
		{"levels undeclared default to API 1", []api.RtlMainProject{app(0, 0, false)}, true, false, true},
		{"main without an application", []api.RtlMainProject{{MinSdk: 21, TargetSdk: 21}}, true, false, false},
		{"no app includes the module", nil, true, false, false},
		{"one including app is enough", []api.RtlMainProject{app(21, 21, true), app(21, 21, false)}, true, true, false},
		{"unknown apps keep reporting", nil, false, true, false},
	}
	for _, id := range rtlLayoutRuleIDs {
		for _, tc := range cases {
			t.Run(id+"/"+tc.name, func(t *testing.T) {
				got := runWithRtlMains(t, id, tc.mains, tc.known)
				if id == "RtlHardcodedResource" && tc.hardcodedAdvice {
					return
				}
				if tc.want && len(got) == 0 {
					t.Fatal("want a finding")
				}
				if !tc.want && len(got) != 0 {
					t.Fatalf("got %d findings, want none", len(got))
				}
			})
		}
	}
}

// TestRtlHardcodedWordingFollowsMinSdk checks the Left/Right advice: "replacing" once the including app's minSdk
// is 17, "adding" while it is below 17 (reported even when RTL is off), as in Lint.
func TestRtlHardcodedWordingFollowsMinSdk(t *testing.T) {
	cases := []struct {
		name     string
		main     api.RtlMainProject
		wantWord string
	}{
		{"modern app", api.RtlMainProject{HasApplication: true, MinSdk: 21, TargetSdk: 21}, "Consider replacing"},
		{"old minSdk", api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 21}, "Consider adding"},
		{"old minSdk with RTL off", api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 21, RtlOff: true}, "Consider adding"},
		{"old minSdk and target", api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 14}, "Consider adding"},
		{"modern app with RTL off", api.RtlMainProject{HasApplication: true, MinSdk: 21, TargetSdk: 21, RtlOff: true}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runWithRtlMains(t, "RtlHardcodedResource", []api.RtlMainProject{tc.main}, true)
			if tc.wantWord == "" {
				if len(got) != 0 {
					t.Fatalf("got %d findings, want none", len(got))
				}
				return
			}
			if len(got) == 0 {
				t.Fatal("want findings")
			}
			for _, f := range got {
				if !strings.HasPrefix(f.Message, tc.wantWord) {
					t.Errorf("message %q does not start with %q", f.Message, tc.wantWord)
				}
			}
		})
	}
}

func TestRtlGravityNeedsRtlButNotMinSdk(t *testing.T) {
	r := findResourceRule(t, "RtlHardcodedResource")
	layout := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "TextView", Line: 2, Col: 5,
		Attributes: map[string]string{"android:gravity": "left"},
	})
	run := func(main api.RtlMainProject) int {
		ctx := &api.Context{
			ResourceIndex: layout, Rule: r, Collector: scanner.NewFindingCollector(0),
			RtlMainsFor: func(string) ([]api.RtlMainProject, bool) { return []api.RtlMainProject{main}, true },
		}
		r.Check(ctx)
		return len(api.ContextFindings(ctx))
	}
	if got := run(api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 21}); got != 1 {
		t.Errorf("old minSdk, RTL on: got %d findings, want 1", got)
	}
	if got := run(api.RtlMainProject{HasApplication: true, MinSdk: 14, TargetSdk: 21, RtlOff: true}); got != 0 {
		t.Errorf("old minSdk, RTL off: got %d findings, want 0", got)
	}
}
