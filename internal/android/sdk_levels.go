package android

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

// SDKLevels is the minSdk / targetSdk a source file is built against. A zero
// level is unknown: no build file or manifest above the file declares it.
type SDKLevels struct {
	MinSdk    int `json:"minSdk,omitempty"`
	TargetSdk int `json:"targetSdk,omitempty"`
}

// IsZero reports whether neither level is known.
func (l SDKLevels) IsZero() bool { return l.MinSdk == 0 && l.TargetSdk == 0 }

// ResolveSDKLevels returns the SDK levels of the source file at sourcePath:
// those of the nearest ancestor directory whose build.gradle(.kts) or
// AndroidManifest.xml declares a minSdk or a targetSdk. The walk follows
// sourcePath as spelled, so a relative path is never resolved above the
// working directory.
func ResolveSDKLevels(sourcePath string) SDKLevels {
	return sdkLevelsAbove(filepath.Dir(sourcePath))
}

// SDKLevelResolver is ResolveSDKLevels memoized per directory, for resolving many files in one pass.
// Every ancestor directory is memoized too, so a build file shared by many source directories is
// read and parsed once. It is not safe for concurrent use and never notices a later edit, so it
// must not outlive the pass.
type SDKLevelResolver struct {
	byDir map[string]SDKLevels
}

// WarmSDKLevels reads the SDK declarations of every directory the given source paths resolve through, in
// parallel, so later Resolve calls hit the memo. It changes no result.
func WarmSDKLevels(sourcePaths []string) {
	seen := map[string]bool{}
	var dirs []string
	for _, p := range sourcePaths {
		for _, dir := range ancestorDirs(filepath.Dir(p)) {
			if seen[dir] {
				break
			}
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	var wg sync.WaitGroup
	jobs := make(chan string)
	for range min(runtime.GOMAXPROCS(0), len(dirs)) {
		wg.Go(func() {
			for dir := range jobs {
				sdkLevelsDeclaredIn(dir)
			}
		})
	}
	for _, dir := range dirs {
		jobs <- dir
	}
	close(jobs)
	wg.Wait()
}

// Resolve returns ResolveSDKLevels(sourcePath).
func (r *SDKLevelResolver) Resolve(sourcePath string) SDKLevels {
	dir := filepath.Dir(sourcePath)
	if dir == "" || dir == "." {
		return SDKLevels{}
	}
	if r.byDir == nil {
		r.byDir = map[string]SDKLevels{}
	}
	dir = filepath.Clean(dir)
	// Walk up to the nearest memoized directory, then resolve the unmemoized ones from the top
	// down: a directory's levels are its own declaration, or else its parent's.
	var chain []string
	var inherited SDKLevels
	for cur := dir; ; {
		if levels, ok := r.byDir[cur]; ok {
			inherited = levels
			break
		}
		chain = append(chain, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	for i := len(chain) - 1; i >= 0; i-- {
		levels := sdkLevelsDeclaredIn(chain[i])
		if levels.IsZero() {
			levels = inherited
		}
		r.byDir[chain[i]] = levels
		inherited = levels
	}
	return inherited
}

func sdkLevelsAbove(dir string) SDKLevels {
	for _, dir := range ancestorDirs(dir) {
		if levels := sdkLevelsDeclaredIn(dir); !levels.IsZero() {
			return levels
		}
	}
	return SDKLevels{}
}

// sdkLevelsDeclaredIn returns the levels declared by dir's own build.gradle(.kts) or manifest, or
// the zero value when it declares none. A build file wins over a manifest.
func sdkLevelsDeclaredIn(dir string) SDKLevels {
	stamp := sdkInputStamp(dir)
	sdkDeclaredMu.RLock()
	entry, ok := sdkDeclaredMemo[dir]
	sdkDeclaredMu.RUnlock()
	if ok && entry.stamp == stamp {
		return entry.levels
	}
	levels := readSDKLevelsDeclaredIn(dir)
	sdkDeclaredMu.Lock()
	sdkDeclaredMemo[dir] = sdkDeclaredEntry{stamp: stamp, levels: levels}
	sdkDeclaredMu.Unlock()
	return levels
}

// sdkInputs are the files sdkLevelsDeclaredIn reads, relative to a directory.
var sdkInputs = [...]string{"build.gradle.kts", "build.gradle", "src/main/AndroidManifest.xml", "AndroidManifest.xml"}

type sdkInputStat struct {
	modTime int64
	size    int64
	exists  bool
}

type sdkDeclaredEntry struct {
	stamp  [len(sdkInputs)]sdkInputStat
	levels SDKLevels
}

var (
	sdkDeclaredMu   sync.RWMutex
	sdkDeclaredMemo = map[string]sdkDeclaredEntry{}
)

// sdkInputStamp stats the input files so a memoized result is reused only while none of them changed.
func sdkInputStamp(dir string) [len(sdkInputs)]sdkInputStat {
	var stamp [len(sdkInputs)]sdkInputStat
	for i, rel := range sdkInputs {
		if info, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			stamp[i] = sdkInputStat{modTime: info.ModTime().UnixNano(), size: info.Size(), exists: true}
		}
	}
	return stamp
}

func readSDKLevelsDeclaredIn(dir string) SDKLevels {
	var levels SDKLevels
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		cfg, err := ParseBuildGradleContent(string(data))
		if err != nil {
			continue
		}
		if cfg.MinSdkVersion > 0 {
			levels.MinSdk = cfg.MinSdkVersion
		}
		if cfg.TargetSdkVersion > 0 {
			levels.TargetSdk = cfg.TargetSdkVersion
		}
		if !levels.IsZero() {
			return levels
		}
	}
	for _, rel := range []string{"src/main/AndroidManifest.xml", "AndroidManifest.xml"} {
		manifest, err := ParseManifest(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		if manifest.UsesSdk.MinSdkVersion != "" {
			levels.MinSdk, _ = strconv.Atoi(manifest.UsesSdk.MinSdkVersion)
		}
		if manifest.UsesSdk.TargetSdkVersion != "" {
			levels.TargetSdk, _ = strconv.Atoi(manifest.UsesSdk.TargetSdkVersion)
		}
		if !levels.IsZero() {
			return levels
		}
	}
	return levels
}

// ancestorDirs returns dir and each directory above it, nearest first.
func ancestorDirs(dir string) []string {
	if dir == "" || dir == "." {
		return nil
	}
	dir = filepath.Clean(dir)
	var dirs []string
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}
