package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const onClickHandlerLayout = `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android">
    <Button android:id="@+id/submit" android:onClick="onSubmitClicked" />
</LinearLayout>
`

const onClickActivityHead = "package p\n\nimport android.app.Activity\nimport android.os.Bundle\nimport android.view.View\n\nclass MainActivity : Activity() {\n    override fun onCreate(savedInstanceState: Bundle?) {\n        super.onCreate(savedInstanceState)\n        setContentView(R.layout.activity_main)\n    }\n"

// OnClick reads the parsed sources, but only for a res directory whose
// layouts name an onClick handler. Only that directory's resource-rule entry
// keys on the sources, so a source edit re-runs the resource rules there and
// leaves every other directory a cache hit (#1860 review).
func TestResourceCacheKeysOnSourcesOnlyWhereARuleReadsThem(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain", "res")
	writeLayout(t, plain, "activity_plain", dupIDLayout)
	click := filepath.Join(root, "click", "res")
	writeLayout(t, click, "activity_main", onClickHandlerLayout)
	// OnClick checks a layout against its own module's Activities.
	src := filepath.Join(root, "click", "src", "MainActivity.kt")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(root, "cache")
	active := []*api.Rule{findV2RuleForTest(t, "OnClick"), findV2RuleForTest(t, "DuplicateIdsResource")}

	run := func(activity string) (found []string, hits, misses int64) {
		t.Helper()
		if err := os.WriteFile(src, []byte(activity), 0o644); err != nil {
			t.Fatal(err)
		}
		parsed, err := scanner.ParseFile(context.Background(), src)
		if err != nil {
			t.Fatal(err)
		}
		writer := scanner.NewAndroidCacheWriter(1)
		tracker := perf.New(true)
		res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
			Project:        &android.Project{ResDirs: []string{plain, click}},
			ActiveRules:    active,
			Dispatcher:     rules.NewDispatcher(active, nil),
			SourceFiles:    []*scanner.File{parsed},
			SourcePaths:    []string{src},
			RuleHash:       "test-rule-hash",
			LibraryFactsFP: "test-lf",
			CacheDir:       cacheDir,
			CacheWriter:    writer,
			Tracker:        tracker,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		for _, f := range res.Findings.Findings() {
			found = append(found, f.Rule+":"+filepath.Base(f.File))
		}
		sort.Strings(found)
		entry, ok := findTimingDeep(tracker.GetTimings(), "resourceCache")
		if !ok {
			t.Fatal("no resourceCache timing entry")
		}
		return found, entry.Metrics["hits"], entry.Metrics["misses"]
	}

	without := onClickActivityHead + "}\n"
	with := onClickActivityHead + "\n    fun onSubmitClicked(v: View) {}\n}\n"
	for _, s := range []struct {
		name         string
		activity     string
		want         []string
		hits, misses int64
	}{
		{"cold", without, []string{"DuplicateIdsResource:activity_plain.xml", "OnClick:activity_main.xml"}, 0, 2},
		// The handler edit reaches OnClick; the directory without a handler
		// stays a hit.
		{"handler added", with, []string{"DuplicateIdsResource:activity_plain.xml"}, 1, 1},
		{"unchanged", with, []string{"DuplicateIdsResource:activity_plain.xml"}, 2, 0},
		// Back to the cold sources: the entry keyed on them is a hit again.
		{"handler removed", without, []string{"DuplicateIdsResource:activity_plain.xml", "OnClick:activity_main.xml"}, 2, 0},
	} {
		found, hits, misses := run(s.activity)
		if !equalStrings(found, s.want) {
			t.Fatalf("%s: findings %v, want %v", s.name, found, s.want)
		}
		if hits != s.hits || misses != s.misses {
			t.Fatalf("%s: resource cache hits=%d misses=%d, want hits=%d misses=%d", s.name, hits, misses, s.hits, s.misses)
		}
	}
}

// A warm run that skipped the parse hands the Android phase stubs without a
// tree; a resource rule that reads the sources gets them parsed on demand,
// and the fingerprint is the same as for a run that parsed them.
func TestResourceRuleSourcesParsesSkippedFiles(t *testing.T) {
	src := filepath.Join(t.TempDir(), "MainActivity.kt")
	if err := os.WriteFile(src, []byte(onClickActivityHead+"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := scanner.ParseFile(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var asked []string
	skipped := AndroidInput{
		SourceFiles: []*scanner.File{{Path: src}},
		SourcePaths: []string{src},
		ParseSources: func(paths []string) ([]*scanner.File, error) {
			asked = append(asked, paths...)
			return []*scanner.File{parsed}, nil
		},
	}.withResourceRuleSources().ruleSources
	files, ok := skipped.sources()
	if !ok || len(files) != 1 || files[0] != parsed || !equalStrings(asked, []string{src}) {
		t.Fatalf("skipped parse: sources %v ok=%v, parse asked for %v", files, ok, asked)
	}
	full := AndroidInput{SourceFiles: []*scanner.File{parsed}, SourcePaths: []string{src}}.withResourceRuleSources().ruleSources
	if files, ok := full.sources(); !ok || len(files) != 1 || files[0] != parsed {
		t.Fatalf("full parse: sources %v ok=%v", files, ok)
	}
	a, okA := skipped.fingerprint()
	b, okB := full.fingerprint()
	if !okA || !okB || a != b {
		t.Fatalf("fingerprints differ: skipped %q (%v), full %q (%v)", a, okA, b, okB)
	}
	// The file changed between the hash the cache validated and the on-demand
	// parse: the findings describe bytes the key does not, so no caching.
	stale := AndroidInput{
		SourceFiles:  []*scanner.File{{Path: src}},
		SourcePaths:  []string{src},
		SourceHashes: map[string]string{src: hashutil.HashHex([]byte("an earlier version"))[:16]},
		ParseSources: func([]string) ([]*scanner.File, error) { return []*scanner.File{parsed}, nil },
	}.withResourceRuleSources().ruleSources
	if _, ok := stale.sources(); ok {
		t.Fatal("a parse that read other content than the fingerprint hashed must leave the result uncacheable")
	}
	noParser := AndroidInput{SourceFiles: []*scanner.File{{Path: src}}, SourcePaths: []string{src}}.withResourceRuleSources().ruleSources
	if _, ok := noParser.sources(); ok {
		t.Fatal("without a parser, the skipped files must leave the result uncacheable")
	}
}

func findTimingDeep(entries []perf.TimingEntry, name string) (perf.TimingEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
		if found, ok := findTimingDeep(entry.Children, name); ok {
			return found, true
		}
	}
	return perf.TimingEntry{}, false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// With no parsed sources the Android phase can answer from, and save, one
// bundle for the whole project. A resource rule that read sources the
// fingerprint does not describe must leave that bundle unsaved too (#1860
// review).
func TestAndroidProjectBundleSkipsSaveWhenOnDemandSourcesMismatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stale   bool
		wantHit bool
	}{
		{"hashes match", false, true},
		{"file changed since its hash was validated", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			click := filepath.Join(root, "click", "src", "main", "res")
			writeLayout(t, click, "activity_main", onClickHandlerLayout)
			src := filepath.Join(root, "click", "src", "main", "kotlin", "MainActivity.kt")
			if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
				t.Fatal(err)
			}
			content := []byte(onClickActivityHead + "}\n")
			if err := os.WriteFile(src, content, 0o644); err != nil {
				t.Fatal(err)
			}
			parsed, err := scanner.ParseFile(context.Background(), src)
			if err != nil {
				t.Fatal(err)
			}
			hash := hashutil.HashHex(content)[:16]
			if tc.stale {
				hash = hashutil.HashHex([]byte("an earlier version"))[:16]
			}
			cacheDir := filepath.Join(root, "cache")
			active := []*api.Rule{findV2RuleForTest(t, "OnClick")}
			run := func() int64 {
				t.Helper()
				writer := scanner.NewAndroidCacheWriter(1)
				tracker := perf.New(true)
				_, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
					Project:        &android.Project{ResDirs: []string{click}},
					ActiveRules:    active,
					Dispatcher:     rules.NewDispatcher(active, nil),
					SourcePaths:    []string{src},
					SourceHashes:   map[string]string{src: hash},
					ParseSources:   func([]string) ([]*scanner.File, error) { return []*scanner.File{parsed}, nil },
					RuleHash:       "test-rule-hash",
					LibraryFactsFP: "test-lf",
					CacheDir:       cacheDir,
					CacheWriter:    writer,
					Tracker:        tracker,
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				entry, ok := findTimingDeep(tracker.GetTimings(), "androidProjectCache")
				if !ok {
					t.Fatal("no androidProjectCache timing entry; the project bundle path did not run")
				}
				return entry.Metrics["hits"]
			}
			run()
			if hit := run() == 1; hit != tc.wantHit {
				t.Fatalf("second run hit the project bundle = %v, want %v", hit, tc.wantHit)
			}
		})
	}
}
