package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// refreshedLedger returns an in-memory ledger refreshed from files, which
// live under dir.
func refreshedLedger(dir string, files []string) *declLedger {
	l := newDeclLedger("")
	l.refresh([]string{dir}, files, nil)
	return l
}

// analyzedEntry is the cache entry for path as it would have been written
// when the ledger l was current.
func analyzedEntry(t *testing.T, l *declLedger, path string) *CacheEntry {
	t.Helper()
	digest, ok := l.currentDigest(path)
	if !ok {
		t.Fatalf("cannot digest %s", path)
	}
	hash, err := ContentHash(path)
	if err != nil {
		t.Fatal(err)
	}
	return &CacheEntry{FilePath: path, ContentHash: hash, NameDigest: digest}
}

func TestDemoteStaleNameDepsSkipsWholeCompilationBackend(t *testing.T) {
	dir := t.TempDir()
	use := writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")
	hit := analyzedEntry(t, refreshedLedger(dir, []string{use}), use)
	miss := writeKt(t, dir, "New.kt", "package p\nfun pick(x: Int): Int = 1\n")
	after := refreshedLedger(dir, []string{use, miss})
	hits, misses := demoteStaleNameDeps(InvocationOptions{Backend: BackendFIR}, after, []*CacheEntry{hit}, []string{miss}, nil, false)
	if len(hits) != 1 || len(misses) != 1 {
		t.Fatalf("FIR hits=%d misses=%d, want untouched", len(hits), len(misses))
	}
}

func TestDemoteStaleNameDepsSkipsCrashedAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	crashedPath := writeKt(t, dir, "Crashed.kt", "package p\nfun crashed() = pick(1)\n")
	cleanPath := writeKt(t, dir, "Clean.kt", "package p\nfun clean() = 1\n")
	before := refreshedLedger(dir, []string{crashedPath, cleanPath})
	crashed := analyzedEntry(t, before, crashedPath)
	crashed.Crashed = true
	clean := analyzedEntry(t, before, cleanPath)
	missing := &CacheEntry{FilePath: filepath.Join(dir, "Gone.kt")}
	miss := writeKt(t, dir, "New.kt", "package p\nfun pick(x: Int): Int = 1\n")
	after := refreshedLedger(dir, []string{crashedPath, cleanPath, miss})
	kept, misses := demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, after, []*CacheEntry{crashed, missing, clean}, []string{miss}, nil, false)
	if len(kept) != 2 || kept[0] != crashed || kept[1] != clean {
		t.Fatalf("kept %v, want the crashed and clean hits", kept)
	}
	if len(misses) != 2 || misses[1] != missing.FilePath {
		t.Fatalf("misses %v, want New.kt plus the unreadable hit", misses)
	}
}

// What an edit to the declaring file does to callers that only share a name
// with it. A caller that resolved to the file is covered by the closure check,
// not by this one.
func TestDemoteStaleNameDepsFollowsDeclarationSignatures(t *testing.T) {
	const (
		use    = "package p\nfun usePick() = pick(1)!!\n"
		other  = "package p\nfun unrelated() = 1\n"
		before = "package p\nfun pick(x: Int): Int {\n    return 1\n}\n"
	)
	cases := []struct {
		name  string
		after string // New.kt after the edit; "" deletes it
		want  []string
	}{
		{"unchanged declaration", before, nil},
		{"body-only edit", "package p\nfun pick(x: Int): Int {\n    return 22 + 33\n}\n", nil},
		{"comment inside body", "package p\nfun pick(x: Int): Int {\n    // new\n    return 1\n}\n", nil},
		{"parameter type change", "package p\nfun pick(x: Long): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"return type change", "package p\nfun pick(x: Int): Long {\n    return 1\n}\n", []string{"Use.kt"}},
		{"modifier added", "package p\ninfix fun pick(x: Int): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"annotation added", "package p\n@Deprecated(\"x\")\nfun pick(x: Int): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"made private", "package p\nprivate fun pick(x: Int): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"moved to another package", "package qq\nfun pick(x: Int): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"renamed", "package p\nfun choose(x: Int): Int {\n    return 1\n}\n", []string{"Use.kt"}},
		{"deleted", "", []string{"Use.kt"}},
		{"expression body edit can change the inferred type", "package p\nfun pick(x: Int) = 1L\n", []string{"Use.kt"}},
		{"added overload", before + "fun pick(x: String): Int {\n    return 2\n}\n", []string{"Use.kt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			usePath := writeKt(t, dir, "Use.kt", use)
			otherPath := writeKt(t, dir, "Other.kt", other)
			newPath := writeKt(t, dir, "New.kt", before)
			files := []string{usePath, otherPath, newPath}
			ledger := refreshedLedger(dir, files)
			hits := []*CacheEntry{analyzedEntry(t, ledger, usePath), analyzedEntry(t, ledger, otherPath)}
			if tc.after == "" {
				if err := os.Remove(newPath); err != nil {
					t.Fatal(err)
				}
				files = files[:2]
			} else {
				writeKt(t, dir, "New.kt", tc.after)
			}
			// The next invocation derives its ledger from the edited sources.
			ledger = refreshedLedger(dir, files)
			kept, misses := demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, ledger, hits, nil, nil, false)
			var got []string
			for _, m := range misses {
				got = append(got, filepath.Base(m))
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("demoted %v, want %v", got, tc.want)
			}
			if len(kept)+len(got) != len(hits) {
				t.Fatalf("hits not conserved: kept=%d demoted=%d", len(kept), len(got))
			}
		})
	}
}

// An inline function's body is expanded into callers, so editing it is a
// signature change.
func TestDeclarationSignatureCoversInlineAndContractBodies(t *testing.T) {
	for _, src := range []string{
		"package p\ninline fun pick(x: Int): Int {\n    return %s\n}\n",
		"package p\nfun pick(x: Int): Int {\n    contract { returns() }\n    return %s\n}\n",
	} {
		dir := t.TempDir()
		use := writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")
		declaring := writeKt(t, dir, "New.kt", strings.Replace(src, "%s", "1", 1))
		ledger := refreshedLedger(dir, []string{use, declaring})
		hit := analyzedEntry(t, ledger, use)
		writeKt(t, dir, "New.kt", strings.Replace(src, "%s", "1 + 2", 1))
		kept, _ := demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, refreshedLedger(dir, []string{use, declaring}), []*CacheEntry{hit}, nil, nil, false)
		if len(kept) != 0 {
			t.Fatalf("editing the body of %q must demote a caller that shares its name", src)
		}
	}
}

func TestCurrentDigestIsMemoizedUntilTheLedgerChanges(t *testing.T) {
	dir := t.TempDir()
	use := writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")
	lib := writeKt(t, dir, "Lib.kt", "package p\nfun pick(x: Any): Int = 1\n")
	ledger := refreshedLedger(dir, []string{use, lib})
	first, _ := ledger.currentDigest(use)
	if first == "" {
		t.Fatal("Use.kt sees pick in Lib.kt; digest must not be empty")
	}
	if again, _ := ledger.currentDigest(use); again != first || ledger.computed.Load() != 1 {
		t.Fatalf("second digest = %q after %d computations, want the memo", again, ledger.computed.Load())
	}
	// A body-only edit leaves the ledger version, so the memo still holds.
	writeKt(t, dir, "Lib.kt", "package p\nfun pick(x: Any): Int = 12345\n")
	ledger.refresh([]string{dir}, []string{use, lib}, nil)
	if d, _ := ledger.currentDigest(use); d == first {
		t.Fatal("an expression-bodied signature changed; the digest must move")
	}
	if ledger.computed.Load() != 2 {
		t.Fatalf("computed %d digests, want 2", ledger.computed.Load())
	}
}

func TestDeclLedgerRoundTripAndVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, declLedgerFile)
	use := writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")
	lib := writeKt(t, dir, "Lib.kt", "package p\nfun pick(x: Any): Int = 1\n")
	l := loadDeclLedger(path)
	l.refresh([]string{dir}, []string{use, lib}, nil)
	digest, _ := l.currentDigest(use)
	l.save()

	loaded := loadDeclLedger(path)
	if loaded.dirty {
		t.Fatal("a freshly loaded ledger must not be dirty")
	}
	loaded.refresh([]string{dir}, []string{use, lib}, nil)
	if loaded.dirty {
		t.Fatal("refreshing unchanged sources must not dirty the ledger")
	}
	if got, _ := loaded.currentDigest(use); got != digest || loaded.computed.Load() != 0 {
		t.Fatalf("persisted memo not used: digest %q (want %q), computed %d", got, digest, loaded.computed.Load())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var disk map[string]json.RawMessage
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	disk["v"] = json.RawMessage("999")
	bumped, _ := json.Marshal(disk)
	if err := os.WriteFile(path, bumped, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadDeclLedger(path); len(got.files) != 0 {
		t.Fatalf("a ledger of another version must be discarded, got %d records", len(got.files))
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadDeclLedger(path); len(got.files) != 0 {
		t.Fatal("a corrupt ledger must be discarded")
	}
}

func TestDeclLedgerRefreshKeepsRecordsOutsideTheScan(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	sub := filepath.Join(src, "sub")
	other := filepath.Join(root, "other")
	for _, d := range []string{sub, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inSub := writeKt(t, sub, "A.kt", "package p\nfun a() = 1\n")
	inOther := writeKt(t, other, "B.kt", "package q\nfun b() = 1\n")
	gone := writeKt(t, other, "C.kt", "package q\nfun c() = 1\n")
	l := newDeclLedger("")
	l.refresh([]string{src, other}, []string{inSub, inOther, gone}, nil)
	if len(l.files) != 3 {
		t.Fatalf("records = %d, want 3", len(l.files))
	}

	// A narrower scan of src only keeps other/B.kt and other/C.kt.
	l.refresh([]string{src}, []string{inSub}, nil)
	if len(l.files) != 3 {
		t.Fatalf("a subpath scan dropped records outside it: %d left", len(l.files))
	}
	// A deleted file outside the scan is noticed through the file system.
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	l.refresh([]string{src}, []string{inSub}, nil)
	if _, ok := l.files[ledgerKey(gone)]; ok || len(l.files) != 2 {
		t.Fatalf("a deleted file outside the scan stayed in the ledger: %d records", len(l.files))
	}
	// A file under the scanned root that is no longer collected leaves too.
	l.refresh([]string{src}, nil, nil)
	if _, ok := l.files[ledgerKey(inSub)]; ok {
		t.Fatal("a file under the scanned root that was not collected stayed in the ledger")
	}
	if _, ok := l.files[ledgerKey(inOther)]; !ok {
		t.Fatal("an existing file outside the scanned root must stay in the ledger")
	}
}

// fakeKAA stands in for the JVM. Its one fact is the issue's: Use.kt reports
// UNNECESSARY_NOT_NULL_ASSERTION on pick(1)!! only when some other file in the
// analyzed sources declares pick(x: Int). Use.kt's closure is Lib.kt alone, so
// a stale entry cannot be noticed through the closure check.
type fakeKAA struct {
	calls [][]string
}

func installFakeKAA(t *testing.T) *fakeKAA {
	t.Helper()
	f := &fakeKAA{}
	orig := missAnalyzer
	t.Cleanup(func() { missAnalyzer = orig })
	missAnalyzer = func(_ string, sourceDirs, misses []string, _, _, _ string, _ bool, _ perf.Tracker, _ InvocationOptions) (*Data, *CacheDepsFile, bool, error) {
		universe := map[string]string{}
		for _, dir := range sourceDirs {
			_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(p, ".kt") {
					body, _ := os.ReadFile(p)
					universe[p] = string(body)
				}
				return nil
			})
		}
		sorted := append([]string(nil), misses...)
		sort.Strings(sorted)
		f.calls = append(f.calls, sorted)
		data := &Data{Version: 1, Files: map[string]*File{}}
		deps := &CacheDepsFile{Version: 1, Approximation: ApproximationKAATaggedReferences, Files: map[string]*CacheDepsEntry{}}
		for _, p := range misses {
			facts := &File{}
			entry := &CacheDepsEntry{}
			if strings.Contains(universe[p], "pick(1)") {
				for q, body := range universe {
					if q == p {
						continue
					}
					if strings.Contains(body, "fun pick(x: Any)") {
						entry.DepPaths = append(entry.DepPaths, q)
						entry.PropagatingDepPaths = append(entry.PropagatingDepPaths, q)
					}
					if strings.Contains(body, "fun pick(x: Int)") {
						facts.Diagnostics = []*Diagnostic{{FactoryName: "UNNECESSARY_NOT_NULL_ASSERTION"}}
					}
				}
			}
			data.Files[p] = facts
			deps.Files[p] = entry
		}
		return data, deps, false, nil
	}
	return f
}

// analyzed returns the base names the nth call analyzed, or nil past the end.
func (f *fakeKAA) analyzed(n int) []string {
	if n >= len(f.calls) {
		return nil
	}
	var names []string
	for _, p := range f.calls[n] {
		names = append(names, filepath.Base(p))
	}
	return names
}

type kaaRepo struct {
	t    *testing.T
	root string
	src  string
	runs int
}

func newKAARepo(t *testing.T, files map[string]string) *kaaRepo {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &kaaRepo{t: t, root: root, src: filepath.Join(root, "src")}
	for name, body := range files {
		r.write(name, body)
	}
	return r
}

func (r *kaaRepo) write(rel, body string) string {
	r.t.Helper()
	path := filepath.Join(r.src, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return path
}

// scan runs the cached oracle over sourceDirs (the whole src tree when none
// are given) and returns the diagnostics reported for Use.kt.
func (r *kaaRepo) scan(filterList []string, sourceDirs ...string) (useDiagnostics []string, tracker perf.Tracker) {
	r.t.Helper()
	if len(sourceDirs) == 0 {
		sourceDirs = []string{r.src}
	}
	filter := ""
	if filterList != nil {
		filter = filepath.Join(r.root, "filter.txt")
		if err := os.WriteFile(filter, []byte(strings.Join(filterList, "\n")+"\n"), 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	r.runs++
	out := filepath.Join(r.root, "oracle-"+string(rune('a'+r.runs))+".json")
	tracker = perf.New(true)
	if _, err := InvokeCachedWithOptions("", sourceDirs, r.root, out, filter, false, nil,
		InvocationOptions{Backend: BackendKAA, Tracker: tracker}); err != nil {
		r.t.Fatalf("scan: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		r.t.Fatal(err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		r.t.Fatal(err)
	}
	for path, f := range data.Files {
		if filepath.Base(path) == "Use.kt" {
			for _, d := range f.Diagnostics {
				useDiagnostics = append(useDiagnostics, d.FactoryName)
			}
		}
	}
	return useDiagnostics, tracker
}

// cleanDiagnostics is what a run with no cache reports for r's sources.
func (r *kaaRepo) cleanDiagnostics() []string {
	r.t.Helper()
	files := map[string]string{}
	_ = filepath.Walk(r.src, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(r.src, p)
			body, _ := os.ReadFile(p)
			files[rel] = string(body)
		}
		return nil
	})
	clean := newKAARepo(r.t, files)
	got, _ := clean.scan(nil)
	return got
}

const (
	libSource = "package p\n\nfun pick(x: Any): Int? = null\n"
	useSource = "package p\n\nfun usePick() = pick(1)!!\n"
	newSource = "package p\n\nfun pick(x: Int): Int = 1\n"
)

func eq(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

// Repro 1 of the issue: an oracle filter list that drops the declaring file.
func TestNameDigestCatchesDeclaringFileExcludedByTheFilterList(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{"Lib.kt": libSource, "Use.kt": useSource})
	filter := []string{filepath.Join(r.src, "Lib.kt"), filepath.Join(r.src, "Use.kt")}
	if got, _ := r.scan(filter); len(got) != 0 {
		t.Fatalf("seed run reported %v", got)
	}

	// New.kt matches no active rule's identifiers, so the list leaves it out.
	r.write("New.kt", newSource)
	got, _ := r.scan(filter)
	if !eq(got, []string{"UNNECESSARY_NOT_NULL_ASSERTION"}) {
		t.Fatalf("warm run reported %v, want the diagnostic a clean run reports (%v)", got, r.cleanDiagnostics())
	}
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"Lib.kt", "Use.kt"}) {
		t.Fatalf("warm run analyzed %v, want the callers and no filtered file", analyzed)
	}
	if clean := r.cleanDiagnostics(); !eq(got, clean) {
		t.Fatalf("warm %v disagrees with clean %v", got, clean)
	}
}

// Repro 2 of the issue: the declaring file is analyzed in another invocation.
func TestNameDigestSurvivesAnotherInvocationAnalyzingTheDeclaringFile(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{"Lib.kt": libSource, "Use.kt": useSource})
	if got, _ := r.scan(nil); len(got) != 0 {
		t.Fatalf("seed run reported %v", got)
	}

	r.write("sub/New.kt", newSource)
	r.scan(nil, filepath.Join(r.src, "sub"))
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"New.kt"}) {
		t.Fatalf("subpath scan analyzed %v, want New.kt", analyzed)
	}

	got, _ := r.scan(nil)
	if !eq(got, []string{"UNNECESSARY_NOT_NULL_ASSERTION"}) {
		t.Fatalf("full scan reported %v, want the diagnostic a clean run reports", got)
	}
	if analyzed := fake.analyzed(2); !eq(analyzed, []string{"Lib.kt", "Use.kt"}) {
		t.Fatalf("full scan analyzed %v, want Lib.kt and Use.kt (New.kt was cached by the subpath scan)", analyzed)
	}
	if clean := r.cleanDiagnostics(); !eq(got, clean) {
		t.Fatalf("warm %v disagrees with clean %v", got, clean)
	}
}

func TestNameDigestDemotesCallersWhenTheDeclaringFileIsDeleted(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{"Lib.kt": libSource, "Use.kt": useSource, "New.kt": newSource})
	if got, _ := r.scan(nil); !eq(got, []string{"UNNECESSARY_NOT_NULL_ASSERTION"}) {
		t.Fatalf("seed run reported %v", got)
	}

	if err := os.Remove(filepath.Join(r.src, "New.kt")); err != nil {
		t.Fatal(err)
	}
	got, _ := r.scan(nil)
	if len(got) != 0 {
		t.Fatalf("after the deletion the warm run still reports %v", got)
	}
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"Lib.kt", "Use.kt"}) {
		t.Fatalf("deletion analyzed %v, want the callers that saw the deleted declaration", analyzed)
	}
	if clean := r.cleanDiagnostics(); !eq(got, clean) {
		t.Fatalf("warm %v disagrees with clean %v", got, clean)
	}
	ledger := loadDeclLedger(filepath.Join(r.root, ".krit", "types-cache", declLedgerFile))
	if _, ok := ledger.files[ledgerKey(filepath.Join(r.src, "New.kt"))]; ok {
		t.Fatal("the deleted file is still in the ledger")
	}
}

// A scan of one directory still learns that a file in another was deleted,
// through the file system, and demotes the callers that saw its names.
func TestNameDigestDeletionOutsideTheScannedRoots(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{
		"main/Lib.kt":  libSource,
		"main/Use.kt":  useSource,
		"other/New.kt": newSource,
	})
	if got, _ := r.scan(nil); !eq(got, []string{"UNNECESSARY_NOT_NULL_ASSERTION"}) {
		t.Fatalf("seed run reported %v", got)
	}
	if err := os.Remove(filepath.Join(r.src, "other", "New.kt")); err != nil {
		t.Fatal(err)
	}
	got, _ := r.scan(nil, filepath.Join(r.src, "main"))
	if len(got) != 0 {
		t.Fatalf("subpath scan reported %v after the declaring file was deleted", got)
	}
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"Lib.kt", "Use.kt"}) {
		t.Fatalf("subpath scan analyzed %v, want the callers that saw the deleted declaration", analyzed)
	}
}

func TestNameDigestBodyEditOfADeclaringFileDemotesNoOneElse(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{
		"Lib.kt": libSource,
		"Use.kt": useSource,
		"New.kt": "package p\n\nfun pick(x: Int): Int {\n    return 1\n}\n",
	})
	r.scan(nil)
	r.write("New.kt", "package p\n\nfun pick(x: Int): Int {\n    return 2 + 3\n}\n")
	r.scan(nil)
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"New.kt"}) {
		t.Fatalf("body edit analyzed %v, want only the edited file", analyzed)
	}
	r.write("New.kt", "package p\n\nfun pick(x: Long): Int {\n    return 2 + 3\n}\n")
	r.scan(nil)
	if analyzed := fake.analyzed(2); !eq(analyzed, []string{"Lib.kt", "New.kt", "Use.kt"}) {
		t.Fatalf("signature edit analyzed %v, want the edited file and the files that see the name", analyzed)
	}
}

func TestNameDigestWarmRunReadsNoSourceAndAnalyzesNothing(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{"Lib.kt": libSource, "Use.kt": useSource, "New.kt": newSource})
	r.scan(nil)
	_, tracker := r.scan(nil)
	if len(fake.calls) != 1 {
		t.Fatalf("a second identical run analyzed %v", fake.calls[1:])
	}
	for _, e := range tracker.GetTimings() {
		if e.Name == "nameDigestVerify" {
			if e.Metrics["computed"] != 0 || e.Metrics["demoted"] != 0 {
				t.Fatalf("warm verification = %v, want no recomputation and no demotion", e.Metrics)
			}
			return
		}
	}
	t.Fatal("no nameDigestVerify entry recorded")
}

func TestNameDigestSeesJavaDeclaringFiles(t *testing.T) {
	fake := installFakeKAA(t)
	r := newKAARepo(t, map[string]string{
		"Lib.kt": libSource,
		"Use.kt": "package p\n\nfun usePick() = Greeter()\n",
	})
	r.scan(nil)
	r.write("Greeter.java", "package p;\n\npublic class Greeter {}\n")
	r.scan(nil)
	if analyzed := fake.analyzed(1); !eq(analyzed, []string{"Use.kt"}) {
		t.Fatalf("a new Java type analyzed %v, want the Kotlin file that names it", analyzed)
	}
	r.write("Greeter.java", "package p;\n\npublic class Greeter {\n    public Greeter(int x) {}\n}\n")
	r.scan(nil)
	if analyzed := fake.analyzed(2); !eq(analyzed, []string{"Use.kt"}) {
		t.Fatalf("an edited Java type analyzed %v, want the Kotlin file that names it", analyzed)
	}
}

func TestNameDigestIsNotStampedForWholeCompilationBackends(t *testing.T) {
	r := newKAARepo(t, map[string]string{"Lib.kt": libSource, "Use.kt": useSource})
	installFakeKAA(t)
	out := filepath.Join(r.root, "fir.json")
	// FIR needs a jar to fingerprint; with none, the run fails before it
	// could touch a ledger, which is what this checks.
	_, _ = InvokeCachedWithOptions("", []string{r.src}, r.root, out, "", false, nil, InvocationOptions{Backend: BackendFIR})
	if _, err := os.Stat(filepath.Join(r.root, ".krit", "types-cache", declLedgerFile)); err == nil {
		t.Fatal("a whole-compilation backend wrote a declared-name ledger")
	}
}
