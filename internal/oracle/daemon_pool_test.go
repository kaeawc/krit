package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

func TestConfiguredDaemonPoolSize(t *testing.T) {
	t.Setenv("KRIT_DAEMON_POOL", "")
	if got := configuredDaemonPoolSize(100); got != 1 {
		t.Fatalf("unset pool = %d, want 1", got)
	}

	t.Setenv("KRIT_DAEMON_POOL", "4")
	if got := configuredDaemonPoolSize(100); got != 4 {
		t.Fatalf("configured pool = %d, want 4", got)
	}

	t.Setenv("KRIT_DAEMON_POOL", "8")
	if got := configuredDaemonPoolSize(3); got != 3 {
		t.Fatalf("capped pool = %d, want 3", got)
	}

	t.Setenv("KRIT_DAEMON_POOL", "auto")
	if got := configuredDaemonPoolSize(100); got != 1 {
		t.Fatalf("unsupported pool = %d, want 1", got)
	}
}

func TestShouldUseDaemonPoolThreshold(t *testing.T) {
	if shouldUseDaemonPool(daemonPoolMinMisses-1, 2) {
		t.Fatal("tiny miss set should not use daemon pool")
	}
	if !shouldUseDaemonPool(daemonPoolMinMisses, 2) {
		t.Fatal("threshold-sized miss set should use daemon pool")
	}
	if shouldUseDaemonPool(daemonPoolMinMisses, 1) {
		t.Fatal("pool size 1 should not use daemon pool")
	}
}

func TestDaemonPIDFileSlotsKeepLegacySlotZero(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcHash := hashSources([]string{"/pool/repo"})
	if err := writePIDFileSlot(100, 200, srcHash, 0); err != nil {
		t.Fatalf("write slot 0: %v", err)
	}
	if err := writePIDFileSlot(101, 201, srcHash, 1); err != nil {
		t.Fatalf("write slot 1: %v", err)
	}

	daemonDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	if _, err := os.Stat(filepath.Join(daemonDir, srcHash+".pid")); err != nil {
		t.Fatalf("legacy slot-0 pid missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(daemonDir, srcHash+".1.pid")); err != nil {
		t.Fatalf("slot-1 pid missing: %v", err)
	}

	removePIDFileSlot(srcHash, 1)
	if _, err := readPIDFileSlot(srcHash, 1); err == nil {
		t.Fatal("slot 1 should be removed")
	}
	if info, err := readPIDFileSlot(srcHash, 0); err != nil || info.PID != 100 || info.Port != 200 {
		t.Fatalf("slot 0 should remain, info=%+v err=%v", info, err)
	}
}

func TestCleanStaleDaemonSlotRemovesOnlyDeadSlot(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcHash := hashSources(testSourceDirs)
	if err := writePIDFileSlot(os.Getpid(), 1, srcHash, 0); err != nil {
		t.Fatalf("write slot 0: %v", err)
	}
	if err := writePIDFileSlot(99999999, 2, srcHash, 1); err != nil {
		t.Fatalf("write slot 1: %v", err)
	}

	cleanStaleDaemonSlot(testSourceDirs, false, 1)
	if _, err := readPIDFileSlot(srcHash, 1); err == nil {
		t.Fatal("dead slot 1 should be removed")
	}
	if info, err := readPIDFileSlot(srcHash, 0); err != nil || info.PID != os.Getpid() {
		t.Fatalf("live slot 0 should remain, info=%+v err=%v", info, err)
	}
}

func TestConnectOrStartDaemonPoolReusesLiveMembers(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	fake0 := NewFakeDaemon(t)
	defer fake0.Close()
	fake1 := NewFakeDaemon(t)
	defer fake1.Close()

	srcHash := hashSources(testSourceDirs)
	if err := writePIDFileSlot(os.Getpid(), fake0.Port, srcHash, 0); err != nil {
		t.Fatalf("write slot 0: %v", err)
	}
	if err := writePIDFileSlot(os.Getpid(), fake1.Port, srcHash, 1); err != nil {
		t.Fatalf("write slot 1: %v", err)
	}

	pool, err := ConnectOrStartDaemonPool("unused.jar", testSourceDirs, nil, 2, false)
	if err != nil {
		t.Fatalf("ConnectOrStartDaemonPool: %v", err)
	}
	defer pool.Release()

	if pool.Requested != 2 || pool.Connected != 2 || pool.Started != 0 {
		t.Fatalf("unexpected pool stats: %+v", pool)
	}
	if len(pool.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(pool.Members))
	}
	if pool.Members[0].slot != 0 || pool.Members[1].slot != 1 {
		t.Fatalf("unexpected member slots: %d, %d", pool.Members[0].slot, pool.Members[1].slot)
	}
	if !pool.MatchesRepo(testSourceDirs) {
		t.Fatal("pool should match repo")
	}
}

func TestDaemonPoolAnalyzeWithDepsShardedMergesOutputs(t *testing.T) {
	d0, got0 := newAnalyzeWithDepsMockDaemon(t)
	d1, got1 := newAnalyzeWithDepsMockDaemon(t)
	pool := &DaemonPool{Members: []*Daemon{d0, d1}}

	files := []string{"/tmp/A.kt", "/tmp/B.kt", "/tmp/C.kt", "/tmp/D.kt"}
	fresh, deps, err := pool.AnalyzeWithDepsSharded(files, false, nil, perf.New(true))
	if err != nil {
		t.Fatalf("AnalyzeWithDepsSharded: %v", err)
	}
	if len(fresh.Files) != len(files) {
		t.Fatalf("fresh files = %d, want %d", len(fresh.Files), len(files))
	}
	if deps == nil || len(deps.Files) != len(files) {
		t.Fatalf("deps files = %#v, want %d entries", deps, len(files))
	}

	want0 := []string{"/tmp/A.kt", "/tmp/C.kt"}
	want1 := []string{"/tmp/B.kt", "/tmp/D.kt"}
	if !reflect.DeepEqual(<-got0, want0) || !reflect.DeepEqual(<-got1, want1) {
		t.Fatalf("unexpected daemon groups")
	}
}

func newAnalyzeWithDepsMockDaemon(t *testing.T) (*Daemon, chan []string) {
	t.Helper()
	d, reqReader, respWriter := newMockDaemon(t)
	got := make(chan []string, 1)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		if err := json.Unmarshal([]byte(sc.Text()), &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
			return
		}
		if req.Method != "analyzeWithDeps" {
			t.Errorf("method = %q, want analyzeWithDeps", req.Method)
			return
		}
		files := stringSliceParam(req.Params["files"])
		got <- files

		fresh := &OracleData{
			Version:       1,
			KotlinVersion: "2.3.20",
			Files:         map[string]*OracleFile{},
			Dependencies:  map[string]*OracleClass{},
		}
		deps := &CacheDepsFile{
			Version:       1,
			Approximation: "symbol-resolved-sources",
			Files:         map[string]*CacheDepsEntry{},
			Crashed:       map[string]string{},
		}
		for _, path := range files {
			fresh.Files[path] = &OracleFile{Package: "p"}
			fresh.Dependencies["dep."+path] = &OracleClass{FQN: "dep." + path, Kind: "class"}
			deps.Files[path] = &CacheDepsEntry{
				DepPaths:    []string{path + ".dep"},
				PerFileDeps: map[string]*OracleClass{"dep." + path: &OracleClass{FQN: "dep." + path, Kind: "class"}},
			}
		}
		freshJSON, _ := json.Marshal(fresh)
		depsJSON, _ := json.Marshal(deps)
		resp := fmt.Sprintf(`{"id": %d, "result": %s, "cacheDeps": %s}`, req.ID, freshJSON, depsJSON)
		respWriter.Write([]byte(resp + "\n")) //nolint:errcheck
	}()

	return d, got
}

func stringSliceParam(v interface{}) []string {
	raw, _ := v.([]interface{})
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
