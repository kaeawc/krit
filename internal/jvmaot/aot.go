// Package jvmaot contains the shared Project Leyden AOT cache machinery used
// by krit-types and krit-fir JVMs.
package jvmaot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/buildid"
)

const cacheKeyHashLen = 12
const sidecarSchema = 1

type cacheMeta struct {
	Schema     int    `json:"schema"`
	JDKVersion string `json:"jdkVersion"`
	JarToken   string `json:"jarToken"`
	Workload   string `json:"workload"`
	ExitStatus int    `json:"exitStatus"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
}

type hashKey struct {
	path  string
	size  int64
	mtime int64
}

var hashMemo sync.Map
var versionMemo sync.Map
var disabledForProcess sync.Map

// DisableForProcess prevents a poisoned cache from being rebuilt or recorded
// again by later daemon launches in this krit process.
func DisableForProcess(cachePath string) { disabledForProcess.Store(cachePath, true) }

func jdkVersion(javaPath string, major int) string {
	if value, ok := versionMemo.Load(javaPath); ok {
		return value.(string)
	}
	out, _ := exec.CommandContext(context.Background(), javaPath, "-version").CombinedOutput()
	version := fmt.Sprintf("major:%d", major)
	if start := strings.IndexByte(string(out), '"'); start >= 0 {
		if end := strings.IndexByte(string(out[start+1:]), '"'); end >= 0 {
			version = string(out[start+1 : start+1+end])
		}
	}
	actual, _ := versionMemo.LoadOrStore(javaPath, version)
	return actual.(string)
}

func hashCache(path string, info os.FileInfo) (string, error) {
	key := hashKey{path, info.Size(), info.ModTime().UnixNano()}
	if value, ok := hashMemo.Load(key); ok {
		return value.(string), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return "", err
	}
	value := hex.EncodeToString(h.Sum(nil))
	hashMemo.Store(key, value)
	return value, nil
}

// Paths returns the recording configuration, AOT cache, and failed-training sentinel.
func Paths(jarPath, workload string) (configPath, cachePath, skipPath string, err error) {
	return PathsWithToken(buildid.JarToken(jarPath), jarPath, workload)
}

// PathsWithToken preserves the oracle's existing content identity.
func PathsWithToken(token, jarPath, workload string) (configPath, cachePath, skipPath string, err error) {
	if token == "missing" || token == "unknown" || len(token) < cacheKeyHashLen {
		return "", "", "", fmt.Errorf("identify JVM jar %q: %s", jarPath, token)
	}
	cacheDir := filepath.Join(os.TempDir(), "krit-cache")
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		candidate := filepath.Join(home, ".krit", "cache")
		if os.MkdirAll(candidate, 0o755) == nil {
			cacheDir = candidate
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", "", err
	}
	base := "krit-types-" + token[:cacheKeyHashLen]
	if workload != "" {
		base += "-" + workload
	}
	return filepath.Join(cacheDir, base+".aotconf"), filepath.Join(cacheDir, base+".aot"), filepath.Join(cacheDir, base+".aot.skip"), nil
}

func AppendArgs(args []string, javaPath, jarPath, workload string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	return AppendArgsWithToken(args, javaPath, jarPath, buildid.JarToken(jarPath), workload, jdkMajor, verbose, logf)
}

func AppendArgsWithToken(args []string, javaPath, jarPath, token, workload string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	if jdkMajor < 25 {
		return args, false
	}
	configPath, cachePath, skipPath, err := PathsWithToken(token, jarPath, workload)
	if err != nil {
		return args, false
	}
	if _, disabled := disabledForProcess.Load(cachePath); disabled {
		return args, false
	}
	args = append(args, "-Xlog:aot=off")
	sweepTemps(filepath.Dir(cachePath))
	version := jdkVersion(javaPath, jdkMajor)
	checkedArgs, used, invalid, incomplete := useCache(args, cachePath, token, workload, version, verbose, logf)
	if used {
		return checkedArgs, true
	}
	if incomplete || !acquireBuildLock(cachePath) {
		return args, false
	}
	// The lock only guards recording/building. Existing validated caches can
	// always be used, including when another process holds the lock.
	var invalidAfterLock bool
	checkedArgs, used, invalidAfterLock, incomplete = useCache(args, cachePath, token, workload, version, verbose, logf)
	invalid = invalid || invalidAfterLock
	if used || incomplete {
		releaseBuildLock(cachePath)
		return checkedArgs, used
	}
	if invalid {
		// A prior cache failed validation. Do not build from its existing
		// recording and use another cache in this same invocation.
		if _, err := os.Stat(configPath); err == nil {
			releaseBuildLock(cachePath)
			return args, false
		}
	}
	if info, statErr := os.Stat(configPath); statErr == nil {
		if info.Size() == 0 {
			_ = os.Remove(configPath)
		} else {
			defer releaseBuildLock(cachePath)
			return trainCache(args, javaPath, jarPath, configPath, cachePath, skipPath, token, workload, version, jdkMajor, verbose, logf)
		}
	}
	recorded, added := recordConfig(args, configPath, verbose, logf)
	if !added {
		releaseBuildLock(cachePath)
	}
	return recorded, added
}

func useCache(args []string, cachePath, token, workload, version string, verbose bool, logf func(string, ...any)) ([]string, bool, bool, bool) {
	info, err := os.Stat(cachePath)
	data, readErr := os.ReadFile(cachePath + ".meta.json")
	if os.IsNotExist(err) {
		return args, false, false, readErr == nil
	}
	if os.IsNotExist(readErr) {
		return args, false, false, true
	}
	if err != nil || readErr != nil {
		return args, false, false, true
	}
	var meta cacheMeta
	if json.Unmarshal(data, &meta) != nil || info.Size() == 0 ||
		meta.Schema != sidecarSchema || meta.JDKVersion != version || meta.JarToken != token || meta.Workload != workload || meta.ExitStatus != 0 || meta.Size != info.Size() {
		DiscardCache(cachePath)
		return args, false, true, false
	}
	hash, hashErr := hashCache(cachePath, info)
	if hashErr != nil || hash != meta.SHA256 {
		DiscardCache(cachePath)
		return args, false, true, false
	}
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: using cache %s\n", cachePath)
	}
	return append(args, "-XX:AOTCache="+cachePath), true, false, false
}

func buildLockPath(cachePath string) string { return cachePath + ".lock" }

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	return err == nil && proc.Signal(syscall.Signal(0)) == nil
}

func acquireBuildLock(cachePath string) bool {
	path := buildLockPath(cachePath)
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, writeErr := fmt.Fprintln(file, os.Getpid())
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(path)
				return false
			}
			return true
		}
		if !os.IsExist(err) || attempt != 0 {
			return false
		}
		info, statErr := os.Stat(path)
		data, readErr := os.ReadFile(path)
		if statErr != nil || readErr != nil {
			return false
		}
		owner, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if !info.ModTime().Before(time.Now().Add(-2*time.Hour)) && (parseErr != nil || pidAlive(owner)) {
			return false
		}
		if err := os.Remove(path); err != nil {
			return false
		}
	}
	return false
}

func releaseBuildLock(cachePath string) {
	path := buildLockPath(cachePath)
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		return
	}
	_ = os.Remove(path)
}

// DiscardCache removes a cache and its proof together.
func DiscardCache(cachePath string) {
	_ = os.Remove(cachePath)
	_ = os.Remove(cachePath + ".meta.json")
}

func trainCache(args []string, javaPath, jarPath, configPath, cachePath, skipPath, token, workload, version string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	if leydenAOTCreateSkipped(skipPath, jdkMajor) {
		if verbose && logf != nil {
			logf("verbose: Leyden AOT: skip sentinel set (JDK %d); falling back\n", jdkMajor)
		}
		return args, false
	}
	if err := buildCache(javaPath, jarPath, configPath, cachePath, token, workload, version, verbose, logf); err == nil {
		if verbose && logf != nil {
			logf("verbose: Leyden AOT: built and using cache %s\n", cachePath)
		}
		return append(args, "-XX:AOTCache="+cachePath), true
	} else if verbose && logf != nil {
		logf("verbose: Leyden AOT: cache build failed (%v), starting without AOT\n", err)
	}
	_ = os.WriteFile(skipPath, []byte(strconv.Itoa(jdkMajor)), 0o644)
	return args, false
}

func tempPath(path string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp."+strconv.Itoa(os.Getpid())+".")
	if err != nil {
		return "", err
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return name, nil
}

func recordConfig(args []string, configPath string, verbose bool, logf func(string, ...any)) ([]string, bool) {
	tmp, err := tempPath(configPath)
	if err != nil {
		return args, false
	}
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: recording class profile → %s\n", tmp)
	}
	return append(args, "-XX:AOTMode=record", "-XX:AOTConfiguration="+tmp), true
}

// FinalizeRecording is called only after Wait has observed the daemon exit.
// Nonzero/signal exits discard the temporary JVM recording.
func FinalizeRecording(args []string, clean bool) {
	for _, arg := range args {
		path, ok := strings.CutPrefix(arg, "-XX:AOTConfiguration=")
		if !ok || !strings.Contains(filepath.Base(path), ".aotconf.tmp.") {
			continue
		}
		final := path[:strings.Index(path, ".tmp.")]
		defer releaseBuildLock(strings.TrimSuffix(final, ".aotconf") + ".aot")
		defer os.Remove(path)
		if !clean {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			return
		}
		_ = os.Rename(path, final)
		return
	}
}

// IsRecording reports whether this invocation owns an unfinished profile.
func IsRecording(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-XX:AOTConfiguration=") && strings.Contains(arg, ".aotconf.tmp.") {
			return true
		}
	}
	return false
}

func buildCache(javaPath, jarPath, configPath, cachePath, token, workload, version string, verbose bool, logf func(string, ...any)) error {
	tmp, err := tempPath(cachePath)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: building cache %s → %s\n", configPath, cachePath)
	}
	cmd := exec.CommandContext(context.Background(), javaPath, "-XX:AOTMode=create", "-XX:AOTConfiguration="+configPath, "-XX:AOTCache="+tmp, "-jar", jarPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leyden AOT create: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	info, err := os.Stat(tmp)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("leyden AOT create: produced empty cache %s", tmp)
	}
	// A completed subprocess cannot write again; verify the file is unchanged while hashing.
	hash, err := hashCache(tmp, info)
	if err != nil {
		return err
	}
	again, err := os.Stat(tmp)
	if err != nil || again.Size() != info.Size() || !again.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("leyden AOT create: cache changed while hashing")
	}
	meta := cacheMeta{sidecarSchema, version, token, workload, 0, info.Size(), hash}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	sidecar := cachePath + ".meta.json"
	metaTmp, err := tempPath(sidecar)
	if err != nil {
		return err
	}
	defer os.Remove(metaTmp)
	if err := os.WriteFile(metaTmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(metaTmp, sidecar); err != nil {
		return err
	}
	if err := os.Rename(tmp, cachePath); err != nil {
		_ = os.Remove(sidecar)
		return err
	}
	// The rename changes the path but not the file's mtime; memoize the new path.
	hashMemo.Store(hashKey{cachePath, info.Size(), info.ModTime().UnixNano()}, hash)
	return nil
}

func sweepTemps(dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "krit-types-*.tmp.*"))
	if err != nil {
		return
	}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		_, suffix, _ := strings.Cut(name, ".tmp.")
		ownerText, _, hasPID := strings.Cut(suffix, ".")
		owner, parseErr := strconv.Atoi(ownerText)
		if hasPID && parseErr == nil && owner > 0 {
			if info.ModTime().Before(time.Now().Add(-24*time.Hour)) && !pidAlive(owner) {
				_ = os.Remove(path)
			}
		} else if info.ModTime().Before(time.Now().Add(-time.Hour)) {
			_ = os.Remove(path)
		}
	}
}

func leydenAOTCreateSkipped(skipPath string, jdkMajor int) bool {
	data, err := os.ReadFile(skipPath)
	if err != nil {
		return false
	}
	version, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return err == nil && version == jdkMajor
}
