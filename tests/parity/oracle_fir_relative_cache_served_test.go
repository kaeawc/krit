package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// A cache-served one-shot FIR run on a relative scan root (`krit .`) must
// return the facts the fresh run returned: the same files under the same
// (relative) spelling, the same per-file facts, and the dependencies the
// cache entries recorded. The absolute-root run of the same tree must agree
// with both once its keys are re-spelled (kaeawc/krit#829).
func TestOracleFIROneShotRelativeRootCacheServedMatchesFresh(t *testing.T) {
	var fir relativeOracleCase
	for _, tc := range relativeOracleCases() {
		if tc.name == "fir" {
			fir = tc
		}
	}
	jar := relativeOracleProject(t, fir)
	t.Setenv("KRIT_DAEMON_CACHE", "off")
	opts := oracle.InvocationOptions{Backend: fir.backend}
	relUse := filepath.Join("src", "main", "kotlin", "app", "Use.kt")
	// Use.kt is a file no other file needs: the kind the issue saw go missing
	// from the cache-served run.
	writeFile(t, relUse, "package app\n\nclass T : R()\n\nfun use(r: R): R = f(r)\n")
	want := []string{relA, relB, relUse}
	sort.Strings(want)

	relDir := []string{filepath.Join("src", "main", "kotlin")}
	fresh := runOneShotCached(t, jar, relDir, "", opts)
	assertRelativeKeys(t, "fresh", fresh, want)
	if fresh.Dependencies["app.R"] == nil {
		t.Fatalf("fresh dependencies %v lack app.R; the test cannot tell whether they survive the cache", keys(fresh.Dependencies))
	}

	cacheDir, err := oracle.CacheDir(".")
	if err != nil {
		t.Fatal(err)
	}
	hits, misses := oracle.ClassifyFilesScopedV3(cacheDir, want, "", "", opts.Backend.CacheApproximation())
	if len(hits) != len(want) || len(misses) != 0 {
		t.Fatalf("after the fresh run the cache has hits=%d misses=%v, want all %d files cached", len(hits), misses, len(want))
	}

	served := runOneShotCached(t, jar, relDir, "", opts)
	assertRelativeKeys(t, "cache-served", served, want)
	if !reflect.DeepEqual(served.Files, fresh.Files) {
		t.Errorf("cache-served per-file facts differ from the fresh run:\nfresh : %s\nserved: %s", mustJSON(t, fresh.Files), mustJSON(t, served.Files))
	}
	// Dependencies are cached per file: a class no analyzed file records as
	// its dependency (app.S, app.T) is only in the fresh run's global set, on
	// absolute roots as well. What the entries did record must come back
	// intact. app.R is the supertype of both B.kt's and Use.kt's classes.
	if served.Dependencies["app.R"] == nil {
		t.Errorf("cache-served dependencies %v lack app.R; fresh had %v", keys(served.Dependencies), keys(fresh.Dependencies))
	}
	for fqn, cls := range served.Dependencies {
		if !reflect.DeepEqual(cls, fresh.Dependencies[fqn]) {
			t.Errorf("cache-served dependency %q differs from the fresh run:\nfresh : %s\nserved: %s", fqn, mustJSON(t, fresh.Dependencies[fqn]), mustJSON(t, cls))
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := runOneShotCached(t, jar, []string{filepath.Join(cwd, "src", "main", "kotlin")}, "", opts)
	if len(abs.Files) != len(want) {
		t.Fatalf("absolute root files = %v, want %d", keys(abs.Files), len(want))
	}
	for _, name := range want {
		got := abs.Files[filepath.Join(cwd, name)]
		if got == nil {
			t.Fatalf("absolute root lost %q; files = %v", name, keys(abs.Files))
		}
		if !reflect.DeepEqual(got, fresh.Files[name]) {
			t.Errorf("absolute root facts for %q differ from the relative run:\nabs: %s\nrel: %s", name, mustJSON(t, got), mustJSON(t, fresh.Files[name]))
		}
	}
}

func assertRelativeKeys(t *testing.T, label string, data *oracle.Data, want []string) {
	t.Helper()
	got := keys(data.Files)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s run files = %v, want %v (relative keys, every file)", label, got, want)
	}
	for _, k := range got {
		if filepath.IsAbs(k) {
			t.Fatalf("%s run keyed by absolute %q", label, k)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
