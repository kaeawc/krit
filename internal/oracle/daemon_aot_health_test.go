package oracle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// aotDaemon is a fake daemon that claims it was launched from an AOT cache
// and answers analyze requests with the given result/errors JSON.
func aotDaemon(t *testing.T, cachePath string, result func(call int) string) (*Daemon, *int) {
	t.Helper()
	calls := 0
	d := pipeDaemon(t, func(req daemonRequest) string {
		calls++
		body := result(calls)
		if req.Method == "analyzeWithDeps" {
			return fmt.Sprintf(`{"id":%d,%s,"cacheDeps":{"version":1,"files":{}}}`, req.ID, body)
		}
		return fmt.Sprintf(`{"id":%d,%s}`, req.ID, body)
	})
	d.shared = false
	d.aotCachePath = cachePath
	return d, &calls
}

func aotCacheFiles(t *testing.T) string {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "krit-types-abc.aot")
	for _, p := range []string{cache, cache + ".meta.json"} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cache
}

const emptyResult = `"result":{"files":{},"dependencies":{}}`

func TestAOTDaemonWithEmptyAnalysisIsRetired(t *testing.T) {
	for _, method := range []string{"analyzeWithDeps", "analyzeFiles"} {
		t.Run(method, func(t *testing.T) {
			cache := aotCacheFiles(t)
			d, _ := aotDaemon(t, cache, func(int) string { return emptyResult })
			var err error
			if method == "analyzeWithDeps" {
				_, _, err = d.AnalyzeWithDeps([]string{"/src/A.kt"})
			} else {
				_, err = d.AnalyzeFilesWithCallFilter([]string{"/src/A.kt"}, nil)
			}
			if !errors.Is(err, ErrOracleAOTDegraded) {
				t.Fatalf("err = %v, want ErrOracleAOTDegraded", err)
			}
			for _, p := range []string{cache, cache + ".meta.json"} {
				if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
					t.Errorf("poisoned cache file %s survived: %v", p, statErr)
				}
			}
			if d.started {
				t.Error("poisoned daemon is still marked started")
			}
		})
	}
}

func TestAOTDaemonHealthyFirstResponseIsTrusted(t *testing.T) {
	cache := aotCacheFiles(t)
	d, calls := aotDaemon(t, cache, func(call int) string {
		if call == 1 {
			return `"result":{"files":{"/src/A.kt":{"package":"a","declarations":[]}},"dependencies":{}}`
		}
		return emptyResult
	})
	if _, _, err := d.AnalyzeWithDeps([]string{"/src/A.kt"}); err != nil {
		t.Fatalf("healthy response: %v", err)
	}
	// Only the first response is checked; later empty answers are normal
	// (for example, a request for files outside the session).
	if _, _, err := d.AnalyzeWithDeps([]string{"/src/B.kt"}); err != nil {
		t.Fatalf("second response: %v", err)
	}
	if *calls != 2 {
		t.Fatalf("calls = %d, want 2", *calls)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("healthy cache was discarded: %v", err)
	}
}

func TestAOTDaemonReportedErrorsAreNotDegraded(t *testing.T) {
	for name, body := range map[string]string{
		"flat errors":   emptyResult + `,"errors":{"/src/A.kt":"File not found in source module"}`,
		"nested errors": `"result":{"files":{},"dependencies":{},"errors":{"/src/A.kt":"Analysis failed"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			cache := aotCacheFiles(t)
			d, _ := aotDaemon(t, cache, func(int) string { return body })
			if _, err := d.AnalyzeFilesWithCallFilter([]string{"/src/A.kt"}, nil); err != nil {
				t.Fatalf("explained empty answer treated as degraded: %v", err)
			}
			if _, err := os.Stat(cache); err != nil {
				t.Fatalf("cache discarded: %v", err)
			}
		})
	}
}

func TestNonAOTDaemonEmptyAnalysisIsNotChecked(t *testing.T) {
	d, _ := aotDaemon(t, "", func(int) string { return emptyResult })
	if _, _, err := d.AnalyzeWithDeps([]string{"/src/A.kt"}); err != nil {
		t.Fatalf("non-AOT daemon: %v", err)
	}
}

func TestStartupAOTCache(t *testing.T) {
	if got := startupAOTCache([]string{"-Xmx1g", "-XX:AOTCache=/c/x.aot", "-jar", "k.jar"}); got != "/c/x.aot" {
		t.Errorf("startupAOTCache = %q", got)
	}
	if got := startupAOTCache([]string{"-XX:AOTMode=record", "-XX:AOTConfiguration=/c/x.aotconf.tmp.1.a"}); got != "" {
		t.Errorf("recording launch reported cache %q", got)
	}
}
