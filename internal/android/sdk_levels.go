package android

import (
	"os"
	"path/filepath"
	"strconv"
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

// SDKLevelResolver is ResolveSDKLevels memoized per source directory, for
// resolving many files in one pass. It is not safe for concurrent use and
// never notices a later edit, so it must not outlive the pass.
type SDKLevelResolver struct {
	byDir map[string]SDKLevels
}

// Resolve returns ResolveSDKLevels(sourcePath).
func (r *SDKLevelResolver) Resolve(sourcePath string) SDKLevels {
	dir := filepath.Dir(sourcePath)
	if levels, ok := r.byDir[dir]; ok {
		return levels
	}
	levels := sdkLevelsAbove(dir)
	if r.byDir == nil {
		r.byDir = map[string]SDKLevels{}
	}
	r.byDir[dir] = levels
	return levels
}

func sdkLevelsAbove(dir string) SDKLevels {
	var levels SDKLevels
	for _, dir := range ancestorDirs(dir) {
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
