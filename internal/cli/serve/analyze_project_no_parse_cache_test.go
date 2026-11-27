package serve

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

// The tests in this file talk to the daemon in wire JSON rather than through
// daemon.AnalyzeProjectArgs: the field names are the contract an older or
// newer peer sees (#1919).

// noParseCacheWireStats is the part of AnalyzeProjectStats these tests read.
type noParseCacheWireStats struct {
	ParseHits         int64 `json:"parse_hits"`
	ParseMisses       int64 `json:"parse_misses"`
	FindingsBundleHit bool  `json:"findings_bundle_hit"`
	// ParseCacheBypassed is the daemon's confirmation that it honoured
	// no_parse_cache.
	ParseCacheBypassed bool `json:"parse_cache_bypassed"`
}

type noParseCacheWireResult struct {
	Findings json.RawMessage       `json:"findings"`
	Stats    noParseCacheWireStats `json:"stats"`
}

// staleTreeRule fires on the seeded stale tree and not on the real source. It
// reads only the tree's top-level node types, so the stale tree alone decides.
const staleTreeRule = "EmptyKotlinFile"

// staleParseFixture returns a Kotlin source over the parse cache's size floor
// that declares a class, and a same-length variant whose declarations are one
// block comment. A tree parsed from the variant and stored under the real
// source's key is a stale entry the cache key cannot detect, as after a
// tree-repair change that did not bump kotlinTreeRepairRevision: it makes the
// real file look empty.
func staleParseFixture() (real, variant string) {
	real = kotlinFixture("A")
	header := "package demo\n\n"
	variant = header + "/*" + strings.Repeat(" ", len(real)-len(header)-5) + "*/\n"
	return real, variant
}

// seedStaleParse stores the variant's tree under path's real content in the
// daemon's resident parse cache and returns that cache.
func seedStaleParse(t *testing.T, state *daemonState, path, real, variant string) *scanner.ParseCache {
	t.Helper()
	pc, err := state.parseCacheFor(state.repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil || pc == nil {
		t.Fatalf("parse cache: %v", err)
	}
	parser := scanner.GetKotlinParser()
	defer scanner.PutKotlinParser(parser)
	stale, err := scanner.ParseKotlinFlat(context.Background(), parser, []byte(variant))
	if err != nil {
		t.Fatal(err)
	}
	if err := pc.Save(path, []byte(real), stale); err != nil {
		t.Fatal(err)
	}
	if _, ok := pc.Load(path, []byte(real)); !ok {
		t.Fatal("seeded entry is not served")
	}
	return pc
}

func callAnalyzeWire(t *testing.T, socket string, args map[string]any) noParseCacheWireResult {
	t.Helper()
	var res noParseCacheWireResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &res); err != nil {
		t.Fatalf("analyze-project %v: %v", args, err)
	}
	return res
}

func ruleFired(t *testing.T, findings json.RawMessage, rule string) bool {
	t.Helper()
	var report output.JSONReport
	if err := json.Unmarshal(findings, &report); err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	for _, f := range report.Findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// A no_parse_cache request parses every file afresh: it neither reads a
// seeded stale parse-cache entry nor the resident trees, and writes neither,
// as an in-process --no-parse-cache scan neither reads nor writes the parse
// cache (#1919).
func TestAnalyzeProject_NoParseCacheSkipsSeededParseCache(t *testing.T) {
	socket, state := startServerForTest(t)
	real, variant := staleParseFixture()
	pathA := writeKotlinFile(t, state.root, "A.kt", real)
	contentB := kotlinFixture("B")
	pathB := writeKotlinFile(t, state.root, "B.kt", contentB)
	pc := seedStaleParse(t, state, pathA, real, variant)
	hitsBefore := pc.Stats().Hits

	res := callAnalyzeWire(t, socket, map[string]any{"no_fir": true, "no_parse_cache": true})

	if ruleFired(t, res.Findings, staleTreeRule) {
		t.Errorf("no_parse_cache request read the seeded stale tree: %s fired on a file that declares a class", staleTreeRule)
	}
	if got := pc.Stats().Hits; got != hitsBefore {
		t.Errorf("parse cache hits moved %d -> %d during a no_parse_cache request", hitsBefore, got)
	}
	if !res.Stats.ParseCacheBypassed {
		t.Errorf("stats.parse_cache_bypassed = false, want the daemon to confirm the bypass; stats=%+v", res.Stats)
	}
	if _, ok := pc.Load(pathB, []byte(contentB)); ok {
		t.Errorf("no_parse_cache request wrote B.kt's tree to the parse cache")
	}
	for _, p := range []string{pathA, pathB} {
		if _, ok := state.workspace.LookupParsedByPath(p); ok {
			t.Errorf("no_parse_cache request stored %s in the resident trees", filepath.Base(p))
		}
	}
}

// The bypass is per request: a later request without the flag still parses
// through the daemon's resident parse cache, whose entries the no_parse_cache
// request left in place.
func TestAnalyzeProject_NoParseCacheLeavesResidentCacheForLaterRequests(t *testing.T) {
	socket, state := startServerForTest(t)
	real, variant := staleParseFixture()
	pathA := writeKotlinFile(t, state.root, "A.kt", real)
	writeKotlinFile(t, state.root, "B.kt", kotlinFixture("B"))
	pc := seedStaleParse(t, state, pathA, real, variant)

	callAnalyzeWire(t, socket, map[string]any{"no_fir": true, "no_parse_cache": true})

	// A different rule selection misses the first request's findings bundle,
	// so this request parses.
	later := callAnalyzeWire(t, socket, map[string]any{"no_fir": true, "disable_rules": "MagicNumber"})
	if later.Stats.FindingsBundleHit {
		t.Fatalf("later request was served by the findings bundle and did not parse; stats=%+v", later.Stats)
	}
	if later.Stats.ParseHits == 0 {
		t.Errorf("later request did not read the resident parse cache; stats=%+v", later.Stats)
	}
	if later.Stats.ParseCacheBypassed {
		t.Errorf("later request reported parse_cache_bypassed without asking for it")
	}
	if _, ok := pc.Load(pathA, []byte(real)); !ok {
		t.Errorf("seeded entry is gone after the no_parse_cache request")
	}
	if !ruleFired(t, later.Findings, staleTreeRule) {
		t.Errorf("later request did not use the seeded parse-cache entry")
	}
}

// A request from a client that predates no_parse_cache omits the field and
// keeps the parse cache, and the daemon does not claim a bypass.
func TestAnalyzeProject_OldClientKeepsParseCache(t *testing.T) {
	socket, state := startServerForTest(t)
	real, variant := staleParseFixture()
	pathA := writeKotlinFile(t, state.root, "A.kt", real)
	seedStaleParse(t, state, pathA, real, variant)

	res := callAnalyzeWire(t, socket, map[string]any{"no_fir": true})
	if res.Stats.ParseHits == 0 {
		t.Errorf("request without no_parse_cache did not read the parse cache; stats=%+v", res.Stats)
	}
	if res.Stats.ParseCacheBypassed {
		t.Errorf("request without no_parse_cache reported parse_cache_bypassed")
	}
	if !ruleFired(t, res.Findings, staleTreeRule) {
		t.Errorf("request without no_parse_cache did not use the seeded parse-cache entry")
	}
}

// The daemon also keeps parsed trees resident per path, filled from the parse
// cache. A no_parse_cache request must not take a tree from there either, or a
// stale parse-cache entry read by an earlier request would still be served.
func TestAnalyzeProject_NoParseCacheSkipsResidentTrees(t *testing.T) {
	socket, state := startServerForTest(t)
	real, variant := staleParseFixture()
	pathA := writeKotlinFile(t, state.root, "A.kt", real)
	parser := scanner.GetKotlinParser()
	stale, err := scanner.ParseKotlinFlat(context.Background(), parser, []byte(variant))
	scanner.PutKotlinParser(parser)
	if err != nil {
		t.Fatal(err)
	}
	state.workspace.StoreParsed(pathA, []byte(real), &scanner.File{
		Path: pathA, Language: scanner.LangKotlin, Content: []byte(real),
		Lines: strings.Split(real, "\n"), FlatTree: stale,
	})

	res := callAnalyzeWire(t, socket, map[string]any{"no_fir": true, "no_parse_cache": true})
	if ruleFired(t, res.Findings, staleTreeRule) {
		t.Errorf("no_parse_cache request used the stale resident tree: %s fired on a file that declares a class", staleTreeRule)
	}
}

// buildProjectInput wiring per request: --no-parse-cache drops both the parse
// cache and the resident trees, --no-cache only the parse cache, and a plain
// request keeps the daemon's one parse cache.
func TestBuildProjectInput_ParseCacheWiring(t *testing.T) {
	root := t.TempDir()
	state := newDaemonState(root)
	state.cachedConfig = config.NewConfig()
	t.Cleanup(state.closeParseCache)
	for _, tc := range []struct {
		name         string
		wire         string
		wantCache    bool
		wantResident bool
	}{
		{"plain", `{}`, true, true},
		{"no-cache", `{"no_cache":true}`, false, true},
		{"no-parse-cache", `{"no_parse_cache":true}`, false, false},
		{"both", `{"no_cache":true,"no_parse_cache":true}`, false, false},
	} {
		var args daemon.AnalyzeProjectArgs
		if err := json.Unmarshal([]byte(tc.wire), &args); err != nil {
			t.Fatal(err)
		}
		args.Paths, args.NoFir, args.NoTypeOracle = []string{root}, true, true
		in, err := state.buildProjectInput(args, oracle.BackendFIR)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := in.Host.ParseCache != nil; got != tc.wantCache {
			t.Errorf("%s: parse cache attached = %v, want %v", tc.name, got, tc.wantCache)
		}
		if tc.wantCache && in.Host.ParseCache != state.parseCache {
			t.Errorf("%s: request got a parse cache other than the daemon's resident one", tc.name)
		}
		if got := in.Host.ResidentFiles != nil; got != tc.wantResident {
			t.Errorf("%s: resident trees attached = %v, want %v", tc.name, got, tc.wantResident)
		}
	}
}

// Under --strict-verify the in-process baseline the daemon compares against
// honours no_parse_cache too, so neither run reads the seeded stale entry.
func TestAnalyzeProject_StrictVerifyNoParseCache(t *testing.T) {
	socket, state := startServerForTest(t)
	state.strictVerify = true
	real, variant := staleParseFixture()
	pathA := writeKotlinFile(t, state.root, "A.kt", real)
	pc := seedStaleParse(t, state, pathA, real, variant)
	hitsBefore := pc.Stats().Hits

	res := callAnalyzeWire(t, socket, map[string]any{"no_fir": true, "no_parse_cache": true})
	if ruleFired(t, res.Findings, staleTreeRule) {
		t.Errorf("strict-verify no_parse_cache request read the seeded stale tree")
	}
	if !res.Stats.ParseCacheBypassed {
		t.Errorf("strict-verify response did not confirm the bypass; stats=%+v", res.Stats)
	}
	if got := pc.Stats().Hits; got != hitsBefore {
		t.Errorf("daemon parse cache hits moved %d -> %d", hitsBefore, got)
	}
}
