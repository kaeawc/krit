package scan

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	rulespkg "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestClassifyCrossFileNeeds(t *testing.T) {
	type rule = api.Rule
	cases := []struct {
		name                                   string
		rules                                  []*rule
		wantIndex, wantParsed, wantModuleAware bool
	}{
		{
			name:  "empty rule set",
			rules: nil,
		},
		{
			name:            "nil entries skipped",
			rules:           []*rule{nil, nil},
			wantIndex:       false,
			wantParsed:      false,
			wantModuleAware: false,
		},
		{
			name: "parsed-files rule sets parsed only",
			rules: []*rule{
				{ID: "P", Needs: api.NeedsParsedFiles},
			},
			wantParsed: true,
		},
		{
			name: "cross-file rule sets index-backed only",
			rules: []*rule{
				{ID: "C", Needs: api.NeedsCrossFile},
			},
			wantIndex: true,
		},
		{
			name: "module-aware rule sets module-aware only",
			rules: []*rule{
				{ID: "M", Needs: api.NeedsModuleIndex},
			},
			wantModuleAware: true,
		},
		{
			name: "parsed-files takes precedence over cross-file when both bits set",
			rules: []*rule{
				{ID: "PC", Needs: api.NeedsParsedFiles | api.NeedsCrossFile},
			},
			wantParsed: true,
			wantIndex:  false,
		},
		{
			name: "all three flags can be true together",
			rules: []*rule{
				{ID: "P", Needs: api.NeedsParsedFiles},
				{ID: "C", Needs: api.NeedsCrossFile},
				{ID: "M", Needs: api.NeedsModuleIndex},
			},
			wantIndex:       true,
			wantParsed:      true,
			wantModuleAware: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotIndex, gotParsed, gotModuleAware := classifyCrossFileNeeds(tc.rules)
			got := [3]bool{gotIndex, gotParsed, gotModuleAware}
			want := [3]bool{tc.wantIndex, tc.wantParsed, tc.wantModuleAware}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("classifyCrossFileNeeds = (idx=%v parsed=%v module=%v); want (idx=%v parsed=%v module=%v)",
					got[0], got[1], got[2], want[0], want[1], want[2])
			}
		})
	}
}

func TestCanParseOnlyCacheMisses(t *testing.T) {
	result := &cache.Result{
		CachedPaths: map[string]bool{"A.kt": true, "C.kt": true},
		TotalCached: 2,
		TotalFiles:  3,
	}
	if !canParseOnlyCacheMisses([]*api.Rule{{ID: "Node", NodeTypes: []string{"call_expression"}}}, result, true, false, false) {
		t.Fatal("ordinary per-file rules should parse only cache misses")
	}
	if canParseOnlyCacheMisses([]*api.Rule{{ID: "Cross", Needs: api.NeedsCrossFile}}, result, true, false, false) {
		t.Fatal("cross-file rules need the full parsed source set")
	}
	if !canParseOnlyCacheMisses([]*api.Rule{{ID: "Cross", Needs: api.NeedsCrossFile}}, result, true, true, false) {
		t.Fatal("cross-file rules should parse only misses when warm cross findings are available")
	}
	if canParseOnlyCacheMisses([]*api.Rule{{ID: "Parsed", Needs: api.NeedsParsedFiles}}, result, true, false, false) {
		t.Fatal("parsed-files rules need the full parsed source set")
	}
	if !canParseOnlyCacheMisses([]*api.Rule{{ID: "Parsed", Needs: api.NeedsParsedFiles}}, result, true, true, false) {
		t.Fatal("parsed-files rules should parse only misses when warm cross findings are available")
	}
	resourceRule := &api.Rule{ID: "ResourceSource", Needs: api.NeedsResources, NodeTypes: []string{"call_expression"}, Languages: []scanner.Language{scanner.LangKotlin}}
	if canParseOnlyCacheMisses([]*api.Rule{resourceRule}, result, true, false, false) {
		t.Fatal("resource-backed source rules need full parse without a bundle delta")
	}
	if !canParseOnlyCacheMisses([]*api.Rule{resourceRule}, result, true, false, true) {
		t.Fatal("resource-backed source rules should parse only misses when bundle delta is available")
	}
	if canParseOnlyCacheMisses([]*api.Rule{{ID: "Node"}}, result, false, false, false) {
		t.Fatal("disabled cache should parse all files")
	}

	got := cacheMissPaths([]string{"A.kt", "B.kt", "C.kt"}, result)
	want := []string{"B.kt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cacheMissPaths = %v, want %v", got, want)
	}
}

func TestCacheMissPaths_AllowsCachedPathMapWithOtherLanguages(t *testing.T) {
	got := cacheMissPaths([]string{"a.kt", "b.kt"}, &cache.Result{
		CachedPaths: map[string]bool{
			"a.kt":        true,
			"Sample.java": true,
			"Other.java":  true,
		},
	})
	if len(got) != 1 || got[0] != "b.kt" {
		t.Fatalf("cacheMissPaths = %v, want [b.kt]", got)
	}
}

func TestRulesNeedAndroidProject(t *testing.T) {
	cases := []struct {
		name  string
		rules []*api.Rule
		want  bool
	}{
		{name: "empty"},
		{name: "plain source rule", rules: []*api.Rule{{ID: "Plain"}}},
		{name: "manifest", rules: []*api.Rule{{ID: "Manifest", Needs: api.NeedsManifest}}, want: true},
		{name: "resources", rules: []*api.Rule{{ID: "Resources", Needs: api.NeedsResources}}, want: true},
		{name: "gradle", rules: []*api.Rule{{ID: "Gradle", Needs: api.NeedsGradle}}, want: true},
		{name: "icons", rules: []*api.Rule{{ID: "Icons", AndroidDeps: uint32(rulespkg.AndroidDepIcons)}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rulesNeedAndroidProject(tc.rules); got != tc.want {
				t.Fatalf("rulesNeedAndroidProject = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRulesNeedProjectModel(t *testing.T) {
	cases := []struct {
		name  string
		rules []*api.Rule
		want  bool
	}{
		{name: "plain source rule"},
		{name: "old broad category alone does not load model", rules: []*api.Rule{{ID: "Security", Category: "security"}}},
		{name: "android project rule loads model", rules: []*api.Rule{{ID: "Manifest", Needs: api.NeedsManifest}}, want: true},
		{name: "explicit library facts loads model", rules: []*api.Rule{{ID: "Library", NeedsLibraryFacts: true}}, want: true},
		{name: "java facts loads model for semantic classpath", rules: []*api.Rule{{ID: "Java", JavaFacts: &api.JavaFactProfile{ReceiverTypesForCallees: []string{"edit"}}}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rulesNeedProjectModel(tc.rules); got != tc.want {
				t.Fatalf("rulesNeedProjectModel = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldOpenResourceIndexCache(t *testing.T) {
	androidRule := []*api.Rule{{ID: "Resources", Needs: api.NeedsResources}}
	plainRule := []*api.Rule{{ID: "Plain"}}
	cases := []struct {
		name            string
		rules           []*api.Rule
		noResourceCache bool
		want            bool
	}{
		{name: "plain rules never open", rules: plainRule},
		{name: "disabled by flag", rules: androidRule, noResourceCache: true},
		{name: "cold android analysis opens", rules: androidRule, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldOpenResourceIndexCache(tc.rules, tc.noResourceCache)
			if got != tc.want {
				t.Fatalf("shouldOpenResourceIndexCache = %v, want %v", got, tc.want)
			}
		})
	}
}
