package scan

import (
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
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

// filepathIsAbs lets the test assert absolute-path-ness without importing
// path/filepath at the top (and without colliding if other tests in the
// package shadow it).
func filepathIsAbs(p string) bool {
	return len(p) > 0 && (p[0] == '/' || (len(p) > 2 && p[1] == ':'))
}
