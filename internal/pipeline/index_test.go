package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestIndexPhase_Name(t *testing.T) {
	if (IndexPhase{}).Name() != "index" {
		t.Fatalf("Name = %q, want index", (IndexPhase{}).Name())
	}
}

func TestCallTargetFilterPerfAttrsIncludesDisabledBy(t *testing.T) {
	attrs := callTargetFilterPerfAttrs(oracle.CallTargetFilterSummary{
		Fingerprint: "abc123",
		DisabledBy:  []string{"Deprecation", "IgnoredReturnValue"},
	})

	if attrs["fingerprint"] != "abc123" {
		t.Fatalf("fingerprint attr = %q, want abc123", attrs["fingerprint"])
	}
	if attrs["disabledBy"] != "Deprecation,IgnoredReturnValue" {
		t.Fatalf("disabledBy attr = %q", attrs["disabledBy"])
	}
	if _, ok := attrs["disabledByTruncated"]; ok {
		t.Fatalf("disabledByTruncated present for short list: %v", attrs)
	}
}

func TestCallTargetFilterPerfAttrsCapsDisabledBy(t *testing.T) {
	disabledBy := make([]string, 30)
	for i := range disabledBy {
		disabledBy[i] = "Rule"
	}
	attrs := callTargetFilterPerfAttrs(oracle.CallTargetFilterSummary{DisabledBy: disabledBy})

	if attrs["disabledByTruncated"] != "1" {
		t.Fatalf("disabledByTruncated = %q, want 1", attrs["disabledByTruncated"])
	}
	if got := strings.Count(attrs["disabledBy"], "Rule"); got != 25 {
		t.Fatalf("disabledBy attr has %d entries, want 25: %q", got, attrs["disabledBy"])
	}
}

func TestIndexPhase_Run_NoResolver_WhenNoRuleNeedsIt(t *testing.T) {
	in := IndexInput{
		ParseResult: ParseResult{
			Paths: []string{t.TempDir()},
			ActiveRules: []*api.Rule{
				{ID: "R", Description: "d", NodeTypes: nil, Check: func(*api.Context) {}},
			},
		},
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver != nil {
		t.Errorf("Resolver = %v, want nil (no rule needs it)", out.Resolver)
	}
}

func TestIndexPhase_Run_BuildsResolver_WhenRuleNeedsIt(t *testing.T) {
	dir := t.TempDir()
	kt := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(kt, []byte("class A { val x: Int = 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), kt)
	if err != nil {
		t.Fatal(err)
	}

	in := IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{file},
			ActiveRules: []*api.Rule{
				{ID: "T", Description: "d", Needs: api.NeedsResolver, Check: func(*api.Context) {}},
			},
		},
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver == nil {
		t.Fatal("Resolver is nil; expected a resolver since a rule declares NeedsResolver")
	}
}

func TestIndexPhase_Run_SkipResolverIndexDoesNotBuildResolver(t *testing.T) {
	tracker := perf.New(true)
	in := IndexInput{
		ParseResult: ParseResult{
			Paths: []string{t.TempDir()},
			ActiveRules: []*api.Rule{
				{ID: "T", Description: "d", Needs: api.NeedsResolver, Check: func(*api.Context) {}},
			},
		},
		Tracker: tracker,
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true, SkipResolverIndex: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver != nil {
		t.Fatalf("Resolver = %v, want nil when SkipResolverIndex is set", out.Resolver)
	}
	if _, ok := findTiming(tracker.GetTimings(), "typeIndex"); ok {
		t.Fatalf("typeIndex timing should not be emitted when SkipResolverIndex is set: %#v", tracker.GetTimings())
	}
}

func TestIndexPhase_Run_UsesPrebuiltResolver(t *testing.T) {
	prebuilt := typeinfer.NewResolver()
	in := IndexInput{
		ParseResult: ParseResult{
			Paths: []string{t.TempDir()},
			ActiveRules: []*api.Rule{
				{ID: "T", Description: "d", Needs: api.NeedsResolver, Check: func(*api.Context) {}},
			},
		},
		PrebuiltResolver: prebuilt,
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver == nil {
		t.Fatal("Resolver unexpectedly nil despite PrebuiltResolver being non-nil")
	}
	// Identity — IndexPhase must pass through the pre-built resolver
	// unchanged, not construct a fresh one.
	if interface{}(out.Resolver) != interface{}(prebuilt) {
		t.Errorf("Resolver identity mismatch: got %v, want %v", out.Resolver, prebuilt)
	}
}

func TestIndexPhase_Run_InputTypesUsesLazyOracle(t *testing.T) {
	dir := t.TempDir()
	oraclePath := filepath.Join(dir, "types.json")
	if err := os.WriteFile(oraclePath, []byte(`{"version":1,"files":{},"dependencies":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tracker := perf.New(true)
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true, SkipResolverIndex: true}.Run(context.Background(), IndexInput{
		ParseResult: ParseResult{
			Paths: []string{dir},
			ActiveRules: []*api.Rule{
				{ID: "OracleRule", Description: "d", Needs: api.NeedsOracle, Check: func(*api.Context) {}},
			},
		},
		Tracker:        tracker,
		OracleEnabled:  true,
		BaseResolver:   typeinfer.NewResolver(),
		InputTypesPath: oraclePath,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Oracle != nil {
		t.Fatal("IndexResult.Oracle should stay nil until lazy lookup loads")
	}
	cr, ok := out.Resolver.(*oracle.CompositeResolver)
	if !ok {
		t.Fatalf("Resolver = %T, want *oracle.CompositeResolver", out.Resolver)
	}
	lazy, ok := cr.Oracle().(*oracle.LazyLookup)
	if !ok {
		t.Fatalf("Composite oracle = %T, want *oracle.LazyLookup", cr.Oracle())
	}
	if lazy.Loaded() {
		t.Fatal("lazy oracle loaded during IndexPhase")
	}
	typeOracle, ok := findTiming(tracker.GetTimings(), "typeOracle")
	if !ok {
		t.Fatalf("missing typeOracle timing: %#v", tracker.GetTimings())
	}
	if _, ok := findTiming(typeOracle.Children, "jsonLoadDeferred"); !ok {
		t.Fatalf("missing jsonLoadDeferred timing: %#v", typeOracle.Children)
	}
}

func TestIndexPhase_Run_GraphSkipped(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	out, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Graph != nil {
		t.Errorf("Graph = %v, want nil when SkipModules=true", out.Graph)
	}
}

func TestIndexPhase_Run_AndroidSkipped(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	out, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.AndroidProject != nil {
		t.Errorf("AndroidProject = %v, want nil when SkipAndroid=true", out.AndroidProject)
	}
}

func TestIndexPhase_Run_ContextCancel(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runPhase[IndexInput, IndexResult](ctx, IndexPhase{SkipModules: true, SkipAndroid: true}, in)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var pe *PhaseError
	if !errors.As(err, &pe) || pe.Phase != "index" {
		t.Fatalf("want PhaseError phase=index, got %v", err)
	}
}

func TestIndexPhase_Run_JavaIndexingChildTimings(t *testing.T) {
	dir := t.TempDir()
	kt := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(kt, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	java := filepath.Join(dir, "B.java")
	if err := os.WriteFile(java, []byte("package a;\npublic class B { A a; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kotlinFile, err := scanner.ParseFile(context.Background(), kt)
	if err != nil {
		t.Fatal(err)
	}

	tracker := perf.New(true)
	crossTracker := tracker.Serial("crossFileAnalysis")
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{kotlinFile},
		},
		BuildCodeIndex:         true,
		CrossFileParentTracker: crossTracker,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	crossTracker.End()
	if len(out.JavaFiles) != 1 {
		t.Fatalf("JavaFiles = %d, want 1", len(out.JavaFiles))
	}

	cross, ok := findTiming(tracker.GetTimings(), "crossFileAnalysis")
	if !ok {
		t.Fatalf("missing crossFileAnalysis timing: %#v", tracker.GetTimings())
	}
	javaIndexing, ok := findTiming(cross.Children, "javaIndexing")
	if !ok {
		t.Fatalf("missing javaIndexing child: %#v", cross.Children)
	}
	for _, name := range []string{
		"collectJavaFiles",
		"fileRead",
		"parseCacheLoad",
		"parseCacheHitSummary",
		"treeSitterParse",
		"flattenTree",
		"queueParseCacheSave",
		"referenceExtraction",
		"filesSummary",
	} {
		if _, ok := findTiming(javaIndexing.Children, name); !ok {
			t.Fatalf("missing javaIndexing child %q: %#v", name, javaIndexing.Children)
		}
	}
	summary, _ := findTiming(javaIndexing.Children, "parseCacheHitSummary")
	if summary.Metrics["hits"] != 0 || summary.Metrics["misses"] != 1 {
		t.Fatalf("cache metrics = %#v, want hits=0 misses=1", summary.Metrics)
	}
	filesSummary, _ := findTiming(javaIndexing.Children, "filesSummary")
	if filesSummary.Metrics["files"] != 1 || filesSummary.Metrics["bytes"] == 0 {
		t.Fatalf("files metrics = %#v, want one non-empty Java file", filesSummary.Metrics)
	}
}

func TestIndexPhase_Run_ReusesCollectedCrossFileJavaPaths(t *testing.T) {
	dir := t.TempDir()
	kt := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(kt, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	java := filepath.Join(dir, "B.java")
	if err := os.WriteFile(java, []byte("package a;\npublic class B { A a; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kotlinFile, err := scanner.ParseFile(context.Background(), kt)
	if err != nil {
		t.Fatal(err)
	}

	tracker := perf.New(true)
	crossTracker := tracker.Serial("crossFileAnalysis")
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{kotlinFile},
		},
		BuildCodeIndex:         true,
		CrossFileParentTracker: crossTracker,
		CrossFileJavaPaths:     []string{},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	crossTracker.End()
	if len(out.JavaFiles) != 0 {
		t.Fatalf("JavaFiles = %d, want 0 when caller supplies an empty Java path list", len(out.JavaFiles))
	}
}

func findTiming(entries []perf.TimingEntry, name string) (perf.TimingEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return perf.TimingEntry{}, false
}
