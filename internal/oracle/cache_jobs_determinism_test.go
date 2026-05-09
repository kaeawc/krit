package oracle

import (
	"fmt"
	"reflect"
	"testing"
)

// TestFreshOracleEntryJobs_StableOrder asserts that
// freshOracleEntryJobs produces an identical jobs slice across runs
// for identical inputs. Regression for #34: previously map iteration
// over fresh.Files and deps.Crashed yielded different slice orders
// per run, propagating into pack-write batching.
func TestFreshOracleEntryJobs_StableOrder(t *testing.T) {
	fresh := &Data{
		Version:       1,
		KotlinVersion: "1.9.20",
		Files:         map[string]*File{},
	}
	deps := &CacheDepsFile{
		Version:       1,
		Approximation: "exact",
		Files:         map[string]*CacheDepsEntry{},
		Crashed:       map[string]string{},
	}

	// Lexically interleaved paths so an unsorted iteration is
	// statistically impossible to match the expected order by chance.
	paths := []string{
		"/src/main/kotlin/zzz.kt",
		"/src/main/kotlin/aaa.kt",
		"/src/main/kotlin/mmm.kt",
		"/src/main/kotlin/bbb.kt",
		"/src/main/kotlin/yyy.kt",
		"/src/main/kotlin/ccc.kt",
		"/src/main/kotlin/qqq.kt",
		"/src/main/kotlin/ddd.kt",
		"/src/main/kotlin/rrr.kt",
		"/src/main/kotlin/eee.kt",
		"/src/main/kotlin/lll.kt",
		"/src/main/kotlin/fff.kt",
	}
	for _, p := range paths {
		fresh.Files[p] = &File{Package: "com.example"}
		deps.Files[p] = &CacheDepsEntry{DepPaths: []string{p + ".dep"}}
	}

	crashedPaths := []string{
		"/src/main/kotlin/crash_z.kt",
		"/src/main/kotlin/crash_a.kt",
		"/src/main/kotlin/crash_m.kt",
	}
	for _, p := range crashedPaths {
		deps.Crashed[p] = "compile failed"
	}

	wantPaths := []string{
		"/src/main/kotlin/aaa.kt",
		"/src/main/kotlin/bbb.kt",
		"/src/main/kotlin/ccc.kt",
		"/src/main/kotlin/ddd.kt",
		"/src/main/kotlin/eee.kt",
		"/src/main/kotlin/fff.kt",
		"/src/main/kotlin/lll.kt",
		"/src/main/kotlin/mmm.kt",
		"/src/main/kotlin/qqq.kt",
		"/src/main/kotlin/rrr.kt",
		"/src/main/kotlin/yyy.kt",
		"/src/main/kotlin/zzz.kt",
		// Crashed entries follow, also sorted.
		"/src/main/kotlin/crash_a.kt",
		"/src/main/kotlin/crash_m.kt",
		"/src/main/kotlin/crash_z.kt",
	}

	// 200 iterations to amplify any scheduler-driven non-determinism.
	for i := 0; i < 200; i++ {
		jobs := freshOracleEntryJobs(fresh, deps)
		if len(jobs) != len(wantPaths) {
			t.Fatalf("iter %d: got %d jobs, want %d", i, len(jobs), len(wantPaths))
		}
		got := make([]string, len(jobs))
		for k, j := range jobs {
			got[k] = j.path
		}
		if !reflect.DeepEqual(got, wantPaths) {
			t.Fatalf("iter %d: jobs path order differs\n  got:  %v\n  want: %v", i, got, wantPaths)
		}
	}
}

// TestFreshOracleEntryJobs_NoDeps covers the deps==nil branch (no
// crashed entries, file paths still sorted).
func TestFreshOracleEntryJobs_NoDeps(t *testing.T) {
	fresh := &Data{
		Version: 1,
		Files: map[string]*File{
			"/c.kt": {}, "/a.kt": {}, "/b.kt": {},
		},
	}
	for i := 0; i < 50; i++ {
		jobs := freshOracleEntryJobs(fresh, nil)
		got := []string{jobs[0].path, jobs[1].path, jobs[2].path}
		want := []string{"/a.kt", "/b.kt", "/c.kt"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iter %d: got %v, want %v", i, got, want)
		}
	}
}

// TestFreshOracleEntryJobs_StableUnderManyKeys probes a larger map
// where Go's randomization is most likely to expose ordering bugs.
func TestFreshOracleEntryJobs_StableUnderManyKeys(t *testing.T) {
	fresh := &Data{
		Version: 1,
		Files:   make(map[string]*File, 256),
	}
	for i := 0; i < 256; i++ {
		fresh.Files[fmt.Sprintf("/src/file_%03x.kt", i*1009%256)] = &File{}
	}

	first := freshOracleEntryJobs(fresh, nil)
	firstPaths := make([]string, len(first))
	for i, j := range first {
		firstPaths[i] = j.path
	}

	for iter := 0; iter < 100; iter++ {
		jobs := freshOracleEntryJobs(fresh, nil)
		got := make([]string, len(jobs))
		for i, j := range jobs {
			got[i] = j.path
		}
		if !reflect.DeepEqual(got, firstPaths) {
			t.Fatalf("iter %d: paths differ", iter)
		}
	}
}
