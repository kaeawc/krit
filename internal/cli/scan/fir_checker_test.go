package scan

import (
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestActiveRuleIDs(t *testing.T) {
	cases := []struct {
		name string
		in   []*api.Rule
		want []string
	}{
		{"empty", nil, []string{}},
		{"all nil entries skipped", []*api.Rule{nil, nil}, []string{}},
		{"single", []*api.Rule{{ID: "Foo"}}, []string{"Foo"}},
		{"mixed nil and real", []*api.Rule{nil, {ID: "Foo"}, nil, {ID: "Bar"}}, []string{"Foo", "Bar"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := activeRuleIDs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("activeRuleIDs = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestResolveFIRTargetFilesNotAllFilesUsesSummaryPaths(t *testing.T) {
	summary := firchecks.FirFilterSummary{
		AllFiles: false,
		Paths:    []string{"/a", "/b"},
	}
	got := resolveFIRTargetFiles(summary, nil)
	if !reflect.DeepEqual(got, summary.Paths) {
		t.Fatalf("got %v; want %v (verbatim from summary)", got, summary.Paths)
	}
}

func TestResolveFIRTargetFilesAllFilesAbsolutizesAndSorts(t *testing.T) {
	summary := firchecks.FirFilterSummary{AllFiles: true}
	parsed := []*scanner.File{
		nil,
		{Path: "z.kt"},
		{Path: "a.kt"},
		nil,
		{Path: "m.kt"},
	}
	got := resolveFIRTargetFiles(summary, parsed)

	if len(got) != 3 {
		t.Fatalf("got %d files (after dropping nils); want 3", len(got))
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("expected sorted output, got %v", got)
	}
	for _, p := range got {
		if !filepathIsAbs(p) {
			t.Fatalf("expected absolute path, got %q", p)
		}
	}
}

func TestRunFIRCheckerPassDisabledIsNoOp(t *testing.T) {
	base := []scanner.Finding{{Rule: "X"}}
	got := runFIRCheckerPass(firCheckerOpts{Enabled: false}, base)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v; want %v (base unchanged when disabled)", got, base)
	}
}

// Guard against paying for JVM startup when the active rule set contains
// no FIR-eligible rules. Important now that --depth=thorough defaults
// FIR on regardless of which rules the project actually has enabled.
func TestRunFIRCheckerPassUnknownRuleInvokesChecker(t *testing.T) {
	checker := firchecks.NewFakeFirChecker()
	base := []scanner.Finding{{Rule: "X"}}
	got := runFIRCheckerPass(firCheckerOpts{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: "NotAFirRule"}},
		Tracker:     perf.New(false),
	}, base)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v; want %v (base unchanged when no FIR rules active)", got, base)
	}
	if len(checker.Called) != 1 {
		t.Fatalf("checker.Check should receive unknown rule ID; got %d invocations", len(checker.Called))
	}
}

// Active rules' configured options reach the FIR checker as ruleConfigs;
// krit-interpreted keys (active, excludes) and option-less rules do not.
func TestRunFIRCheckerPassSendsActiveRuleOptions(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("coroutines", "InjectDispatcher", "active", true)
	cfg.Set("coroutines", "InjectDispatcher", "excludes", []interface{}{"**/gen/**"})
	cfg.Set("coroutines", "InjectDispatcher", "dispatcherNames", []interface{}{"IO"})
	cfg.Set("style", "MagicNumber", "active", true)
	cfg.Set("style", "Inactive", "threshold", 9) // not an active rule
	checker := firchecks.NewFakeFirChecker()
	runFIRCheckerPass(firCheckerOpts{
		Enabled: true,
		Checker: checker,
		Config:  cfg,
		ActiveRules: []*api.Rule{
			{ID: "InjectDispatcher", Category: "coroutines"},
			{ID: "MagicNumber", Category: "style"},
		},
		Tracker: perf.New(false),
	}, nil)
	if len(checker.CalledRuleConfigs) != 1 {
		t.Fatalf("expected one Check call, got %d", len(checker.CalledRuleConfigs))
	}
	want := firchecks.RuleConfigs{"InjectDispatcher": {"dispatcherNames": []interface{}{"IO"}}}
	if got := checker.CalledRuleConfigs[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ruleConfigs = %#v; want %#v", got, want)
	}
}

// filepathIsAbs lets the test assert absolute-path-ness without importing
// path/filepath at the top (and without colliding if other tests in the
// package shadow it).
func filepathIsAbs(p string) bool {
	return len(p) > 0 && (p[0] == '/' || (len(p) > 2 && p[1] == ':'))
}
