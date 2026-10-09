package jvmaot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func aotSetup(t *testing.T) (string, string, string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	token := "abcdef1234567890"
	config, cache, _, err := PathsWithToken(token, jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	return jar, token, config, cache
}

func TestHalfPublishedPairIsNotReadyWhileBuilderHoldsLock(t *testing.T) {
	for _, half := range []string{"cache without sidecar", "sidecar without cache"} {
		t.Run(half, func(t *testing.T) {
			jar, token, _, cache := aotSetup(t)
			path := cache
			if half == "sidecar without cache" {
				path = cache + ".meta.json"
			}
			if err := os.WriteFile(path, []byte("builder in progress"), 0o600); err != nil {
				t.Fatal(err)
			}
			// A live builder between its sidecar and cache renames.
			if err := os.WriteFile(buildLockPath(cache), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
				t.Fatal(err)
			}
			args, added := AppendArgsWithToken(nil, "missing-java", jar, token, "fir", 27, false, nil)
			if added || IsRecording(args) {
				t.Fatalf("expected no AOT while cache pair incomplete, got %v", args)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("in-progress %s was removed: %v", half, err)
			}
		})
	}
}

// A crash between the sidecar and cache renames leaves a half pair with no
// lock holder. It must be cleared so the key can be recorded and rebuilt
// instead of disabling AOT for that key forever (#762).
func TestCrashLeftHalfPairIsRemovedUnderLock(t *testing.T) {
	for _, half := range []string{"cache without sidecar", "sidecar without cache"} {
		t.Run(half, func(t *testing.T) {
			jar, token, _, cache := aotSetup(t)
			path := cache
			if half == "sidecar without cache" {
				path = cache + ".meta.json"
			}
			if err := os.WriteFile(path, []byte("orphan"), 0o600); err != nil {
				t.Fatal(err)
			}
			args, added := AppendArgsWithToken(nil, "missing-java", jar, token, "fir", 27, false, nil)
			if !added || !IsRecording(args) {
				t.Fatalf("orphaned %s blocked a new recording: %v", half, args)
			}
			for _, p := range []string{cache, cache + ".meta.json"} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("orphaned pair file %s survived: %v", p, err)
				}
			}
			FinalizeRecording(args, false)
		})
	}
}

func TestFreshBuildLockSkipsRecordWithoutTouchingBuilder(t *testing.T) {
	jar, token, _, cache := aotSetup(t)
	lock := buildLockPath(cache)
	if err := os.WriteFile(lock, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	tmp := cache + ".tmp." + strconv.Itoa(os.Getpid()) + ".builder"
	if err := os.WriteFile(tmp, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, added := AppendArgsWithToken(nil, "missing-java", jar, token, "fir", 27, false, nil)
	if added || IsRecording(args) {
		t.Fatalf("lock holder did not cause AOT fallback: %v", args)
	}
	for _, path := range []string{lock, tmp} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("builder artifact removed %s: %v", path, err)
		}
	}
}

func TestValidCacheUsedWhileBuildLockHeld(t *testing.T) {
	jar, token, _, cache := aotSetup(t)
	content := []byte("complete")
	if err := os.WriteFile(cache, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	meta, err := json.Marshal(cacheMeta{sidecarSchema, "major:27", token, "fir", 0, int64(len(content)), hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache+".meta.json", meta, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(buildLockPath(cache), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	args, added := AppendArgsWithToken(nil, "missing-java", jar, token, "fir", 27, false, nil)
	if !added || !slices.Contains(args, "-XX:AOTCache="+cache) {
		t.Fatalf("valid cache was blocked by builder lock: %v", args)
	}
}

func TestStaleBuildLockCanBeReclaimed(t *testing.T) {
	for _, stale := range []string{"dead PID", "old malformed"} {
		t.Run(stale, func(t *testing.T) {
			_, _, _, cache := aotSetup(t)
			lock := buildLockPath(cache)
			content := strconv.Itoa(99999999)
			if stale == "old malformed" {
				content = "not-a-pid"
			}
			if err := os.WriteFile(lock, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if stale == "old malformed" {
				old := time.Now().Add(-3 * time.Hour)
				if err := os.Chtimes(lock, old, old); err != nil {
					t.Fatal(err)
				}
			}
			if !acquireBuildLock(cache) {
				t.Fatal("stale lock was not reclaimed")
			}
			data, err := os.ReadFile(lock)
			if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
				t.Fatalf("new owner: %q %v", data, err)
			}
			releaseBuildLock(cache)
		})
	}
}

// A recording keeps the lock until FinalizeRecording at daemon exit, which
// can be long after the lock file's mtime ages out.
func TestOldBuildLockWithLiveOwnerIsNotReclaimed(t *testing.T) {
	_, _, _, cache := aotSetup(t)
	lock := buildLockPath(cache)
	if err := os.WriteFile(lock, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if acquireBuildLock(cache) {
		t.Fatal("reclaimed a lock whose owner is still alive")
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("live owner's lock was removed: %v", err)
	}
}

func TestFreshMalformedBuildLockIsNotReclaimed(t *testing.T) {
	_, _, _, cache := aotSetup(t)
	// O_EXCL create happens before the PID is written; a fresh empty lock
	// is an owner mid-write, not a stale lock.
	if err := os.WriteFile(buildLockPath(cache), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if acquireBuildLock(cache) {
		t.Fatal("reclaimed a fresh malformed lock")
	}
}

func TestSweepTempsRespectsLivePIDAndDeadPIDAge(t *testing.T) {
	_, _, _, cache := aotSetup(t)
	dir := filepath.Dir(cache)
	live := cache + ".tmp." + strconv.Itoa(os.Getpid()) + ".live"
	dead := cache + ".tmp.99999999.dead"
	for _, path := range []string{live, dead} {
		if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-25 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	sweepTemps(dir)
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("live owner's temp removed: %v", err)
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatalf("dead owner's old temp remains: %v", err)
	}
}

func TestMismatchedSidecarIsDiscarded(t *testing.T) {
	for _, field := range []string{"jarToken", "jdkVersion", "size"} {
		t.Run(field, func(t *testing.T) {
			jar, token, _, cache := aotSetup(t)
			content := []byte("complete")
			if err := os.WriteFile(cache, content, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(content)
			meta := map[string]any{"schema": 1, "jarToken": token, "jdkVersion": "major:27", "workload": "fir", "exitStatus": 0, "size": len(content), "sha256": hex.EncodeToString(sum[:])}
			switch field {
			case "jarToken":
				meta[field] = "wrong"
			case "jdkVersion":
				meta[field] = "old"
			case "size":
				meta[field] = 1
			}
			data, _ := json.Marshal(meta)
			if err := os.WriteFile(cache+".meta.json", data, 0600); err != nil {
				t.Fatal(err)
			}
			args, _ := AppendArgsWithToken(nil, "missing-java", jar, token, "fir", 27, false, nil)
			if !IsRecording(args) {
				t.Fatalf("expected record, got %v", args)
			}
			for _, p := range []string{cache, cache + ".meta.json"} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("%s remains: %v", p, err)
				}
			}
			FinalizeRecording(args, false)
		})
	}
}

func TestInterruptedCreateLeavesNoFinalCache(t *testing.T) {
	jar, token, config, cache := aotSetup(t)
	if err := os.WriteFile(config, []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	java := filepath.Join(t.TempDir(), "java")
	script := "#!/bin/sh\ncase \"$1\" in\n -version) echo 'openjdk version \"27.0.1\"' >&2; exit 0;;\nesac\nfor arg in \"$@\"; do case \"$arg\" in -XX:AOTCache=*) printf partial > \"${arg#-XX:AOTCache=}\";; esac; done\nexit 1\n"
	if err := os.WriteFile(java, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	args, added := AppendArgsWithToken(nil, java, jar, token, "fir", 27, false, nil)
	if added || strings.Contains(strings.Join(args, " "), "-XX:AOTCache=") {
		t.Fatalf("failed create used AOT: %v", args)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", p, err)
		}
	}
}

func TestRecordingFinalizesOnlyOnCleanExit(t *testing.T) {
	_, _, config, _ := aotSetup(t)
	args, _ := recordConfig(nil, config, false, nil)
	tmp := strings.TrimPrefix(args[len(args)-1], "-XX:AOTConfiguration=")
	if err := os.WriteFile(tmp, []byte("good"), 0600); err != nil {
		t.Fatal(err)
	}
	FinalizeRecording(args, true)
	if data, err := os.ReadFile(config); err != nil || string(data) != "good" {
		t.Fatalf("final config: %q %v", data, err)
	}
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	args, _ = recordConfig(nil, config, false, nil)
	tmp = strings.TrimPrefix(args[len(args)-1], "-XX:AOTConfiguration=")
	if err := os.WriteFile(tmp, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	FinalizeRecording(args, false)
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("bad profile finalized: %v", err)
	}
}
