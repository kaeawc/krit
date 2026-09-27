// Package jvmaot contains the shared Project Leyden AOT cache machinery used
// by krit-types and krit-fir JVMs.
package jvmaot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/buildid"
)

const cacheKeyHashLen = 12

// Paths returns the recording configuration, AOT cache, and failed-training
// sentinel for one jar and workload. Workload separates caches trained with
// different entrypoints while retaining the established oracle names when it
// is empty.
func Paths(jarPath, workload string) (configPath, cachePath, skipPath string, err error) {
	token := buildid.JarToken(jarPath)
	return PathsWithToken(token, jarPath, workload)
}

// PathsWithToken derives paths from a caller-selected content identity. The
// oracle uses its existing hash identity to preserve its cache paths; new
// workloads should use Paths, which keys with buildid.JarToken.
func PathsWithToken(token, jarPath, workload string) (configPath, cachePath, skipPath string, err error) {
	if token == "missing" || token == "unknown" || len(token) < cacheKeyHashLen {
		return "", "", "", fmt.Errorf("identify JVM jar %q: %s", jarPath, token)
	}
	cacheDir := filepath.Join(os.TempDir(), "krit-cache")
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		candidate := filepath.Join(home, ".krit", "cache")
		if mkdirErr := os.MkdirAll(candidate, 0o755); mkdirErr == nil {
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

// AppendArgs appends Leyden AOT arguments using the same JDK support check,
// training flow, and failure fallback as the oracle. supported is supplied by
// the caller's cached java -version probe, so both JVMs share that detection.
// It returns false when the caller should use its normal non-AOT fallback.
func AppendArgs(args []string, javaPath, jarPath, workload string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	return AppendArgsWithToken(args, javaPath, jarPath, buildid.JarToken(jarPath), workload, jdkMajor, verbose, logf)
}

// AppendArgsWithToken shares the AOT training flow while allowing an existing
// caller to retain its established cache identity.
func AppendArgsWithToken(args []string, javaPath, jarPath, token, workload string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	if jdkMajor < 25 {
		return args, false
	}
	args = append(args, "-Xlog:aot=off")
	configPath, cachePath, skipPath, err := PathsWithToken(token, jarPath, workload)
	if err != nil {
		return args, false
	}
	if args, used, found := useCache(args, cachePath, verbose, logf); found {
		return args, used
	}
	if _, statErr := os.Stat(configPath); statErr == nil {
		return trainCache(args, javaPath, jarPath, configPath, cachePath, skipPath, jdkMajor, verbose, logf)
	}
	return recordConfig(args, configPath, verbose, logf), true
}

func useCache(args []string, cachePath string, verbose bool, logf func(string, ...any)) ([]string, bool, bool) {
	info, err := os.Stat(cachePath)
	if err != nil {
		return args, false, false
	}
	if info.Size() == 0 {
		_ = os.Remove(cachePath)
		if verbose && logf != nil {
			logf("verbose: Leyden AOT: discarded empty cache %s\n", cachePath)
		}
		return args, false, false
	}
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: using cache %s\n", cachePath)
	}
	return append(args, "-XX:AOTCache="+cachePath), true, true
}

func trainCache(args []string, javaPath, jarPath, configPath, cachePath, skipPath string, jdkMajor int, verbose bool, logf func(string, ...any)) ([]string, bool) {
	if leydenAOTCreateSkipped(skipPath, jdkMajor) {
		if verbose && logf != nil {
			logf("verbose: Leyden AOT: skip sentinel set (JDK %d); falling back\n", jdkMajor)
		}
		return args, false
	}
	if err := buildCache(javaPath, jarPath, configPath, cachePath, verbose, logf); err == nil {
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

func recordConfig(args []string, configPath string, verbose bool, logf func(string, ...any)) []string {
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: recording class profile → %s\n", configPath)
	}
	return append(args, "-XX:AOTMode=record", "-XX:AOTConfiguration="+configPath)
}

func buildCache(javaPath, jarPath, configPath, cachePath string, verbose bool, logf func(string, ...any)) error {
	if verbose && logf != nil {
		logf("verbose: Leyden AOT: building cache %s → %s\n", configPath, cachePath)
	}
	cmd := exec.CommandContext(context.Background(), javaPath, "-XX:AOTMode=create", "-XX:AOTConfiguration="+configPath, "-XX:AOTCache="+cachePath, "-jar", jarPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(cachePath)
		return fmt.Errorf("leyden AOT create: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if info, statErr := os.Stat(cachePath); statErr != nil || info.Size() == 0 {
		_ = os.Remove(cachePath)
		return fmt.Errorf("leyden AOT create: produced empty cache %s", cachePath)
	}
	return nil
}

func leydenAOTCreateSkipped(skipPath string, jdkMajor int) bool {
	data, err := os.ReadFile(skipPath)
	if err != nil {
		return false
	}
	version, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return err == nil && version == jdkMajor
}
