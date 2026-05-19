package oracle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// cacheKeyHashLen keeps 12 hex chars = 48 bits, collision-safe for per-repo cache keys.
const cacheKeyHashLen = 12

// jarCachePath returns a cache path keyed by the JAR's content hash, with the
// given suffix appended.  The file lives under $TMPDIR/krit-cache/ (or
// ~/.krit/cache/ if HOME is set) and is named krit-types-<hash><suffix>.
func jarCachePath(jarPath, suffix string) (string, error) {
	full, err := hashutil.HashFile(jarPath)
	if err != nil {
		return "", err
	}
	hash := full[:cacheKeyHashLen]

	cacheDir := filepath.Join(os.TempDir(), "krit-cache")
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".krit", "cache")
		if err := os.MkdirAll(candidate, 0755); err == nil {
			cacheDir = candidate
		}
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "krit-types-"+hash+suffix), nil
}

// cdsArchivePath returns the path for an AppCDS shared archive keyed by the
// JAR's content hash.
func cdsArchivePath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".jsa")
}

// cracCheckpointPath returns the directory for a CRaC checkpoint keyed by the
// JAR's content hash.  Only meaningful on CRaC-enabled JDKs (Azul Zulu, Liberica NIK).
func cracCheckpointPath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".crac")
}

// aotConfigPath returns the path for a Project Leyden AOT configuration file
// keyed by the JAR's content hash. Used during the record phase (JDK 25+).
func aotConfigPath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".aotconf")
}

// aotCachePath returns the path for a Project Leyden AOT cache file keyed by
// the JAR's content hash. Used during the create and use phases (JDK 25+).
func aotCachePath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".aot")
}

var (
	jdkVersionOnce  sync.Once
	jdkVersionCache int
)

// cachedJDKMajorVersion returns the major version of the java binary in PATH,
// caching the result after the first call to avoid repeated subprocess spawns.
func cachedJDKMajorVersion() int {
	jdkVersionOnce.Do(func() {
		javaPath, err := exec.LookPath("java")
		if err != nil {
			return
		}
		jdkVersionCache = jdkMajorVersion(javaPath)
	})
	return jdkVersionCache
}

// jdkMajorVersion parses the major version from the output of "java -version".
// Returns 0 on any error; callers should treat 0 as "unknown, fall back to
// compatible behavior".
func jdkMajorVersion(javaPath string) int {
	// java -version writes to stderr; CombinedOutput captures both.
	out, _ := exec.CommandContext(context.Background(), javaPath, "-version").CombinedOutput()
	s := string(out)
	// Version string is quoted: openjdk version "25.0.2" or java version "1.8.0_352"
	start := strings.Index(s, `"`)
	if start < 0 {
		return 0
	}
	end := strings.Index(s[start+1:], `"`)
	if end < 0 {
		return 0
	}
	parts := strings.SplitN(s[start+1:start+1+end], ".", 3)
	// Strip any pre-release suffix (e.g. "24-ea" → "24")
	majorStr := strings.SplitN(parts[0], "-", 2)[0]
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0
	}
	// Old-style 1.X versioning (JDK 8 is "1.8.0")
	if major == 1 && len(parts) >= 2 {
		if minor, err := strconv.Atoi(parts[1]); err == nil {
			return minor
		}
	}
	return major
}

// buildLeydenAOTCache runs the Leyden AOT create step: it compiles the AOT
// cache from an existing configuration file and exits immediately without
// running the application. Fast (seconds), must complete before daemon launch.
//
// On JVM crashes during create (seen on some JDK 26 builds with Kotlin
// shadow JARs that exercise IntelliJ verification paths) the JVM leaves
// behind a zero-byte or partial cache file. Removing it here keeps the
// next daemon launch from picking it up and aborting VM init with
// "Unable to read generic CDS file map header from AOT cache".
func buildLeydenAOTCache(javaPath, jarPath, configPath, cachePath string, verbose bool) error {
	args := []string{
		"-XX:AOTMode=create",
		"-XX:AOTConfiguration=" + configPath,
		"-XX:AOTCache=" + cachePath,
		"-jar", jarPath,
	}
	if verbose {
		reporter().Verbosef("verbose: Leyden AOT: building cache %s → %s\n", configPath, cachePath)
	}
	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(cachePath)
		return fmt.Errorf("leyden AOT create: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	// Sanity-check the produced cache. Some JVM error paths exit
	// non-zero but only after touching the cache file, so cmd.Run
	// might still return nil on certain builds. Drop the file if it
	// looks empty so the use-branch fallback path stays correct.
	if info, statErr := os.Stat(cachePath); statErr != nil || info.Size() == 0 {
		_ = os.Remove(cachePath)
		return fmt.Errorf("leyden AOT create: produced empty cache %s", cachePath)
	}
	return nil
}

// buildJVMBaseArgs returns the common JVM flags shared by all daemon launch paths.
func buildJVMBaseArgs() []string {
	return []string{
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms1g",
		"-XX:ReservedCodeCacheSize=256m",
		"-Djava.awt.headless=true",
	}
}

// appendAppCDSArgs appends AppCDS flags to args and returns the result.
//
// Mutually exclusive with Leyden AOT: on JDK 25+ where Project Leyden
// applies, AppCDS's `-Xshare:auto` and `-XX:SharedArchiveFile=` /
// `-XX:ArchiveClassesAtExit=` flags conflict with `-XX:AOTCache=` /
// `-XX:AOTMode=record`. Callers should call appendStartupCacheArgs
// instead so the daemon launch picks Leyden when available and falls
// back to AppCDS otherwise.
//
// `-Xlog:cds=off` is appended because `-Xshare:auto`'s fallback warning
// (e.g. on classpath mismatch) writes `[cds]` chatter to stdout, which
// overwrites the daemon's first stdout line — the JSON ready message
// the parent expects.
func appendAppCDSArgs(args []string, jarPath string, verbose bool) []string {
	archivePath, err := cdsArchivePath(jarPath)
	if err != nil {
		return args
	}
	if _, statErr := os.Stat(archivePath); statErr == nil {
		args = append(args, "-XX:SharedArchiveFile="+archivePath, "-Xshare:auto")
		if verbose {
			reporter().Verbosef("verbose: AppCDS: using archive %s\n", archivePath)
		}
	} else {
		args = append(args, "-XX:ArchiveClassesAtExit="+archivePath, "-Xshare:auto")
		if verbose {
			reporter().Verbosef("verbose: AppCDS: training archive %s\n", archivePath)
		}
	}
	args = append(args, "-Xlog:cds=off")
	return args
}

// appendLeydenAOTArgs appends Project Leyden AOT flags (JDK 25+) to args
// and returns (newArgs, true) when at least one AOT flag was added.
// Returns (args, false) on older JDKs or when path resolution failed,
// signalling the caller it should fall back to AppCDS.
//
// Leyden AOT and AppCDS are mutually exclusive on the launch command
// line: both `-XX:AOTCache=` and `-XX:AOTMode=record` reject
// `-Xshare:*` and `-XX:SharedArchiveFile=`/`-XX:ArchiveClassesAtExit=`.
// Callers must never combine the two — use appendStartupCacheArgs.
//
// `-Xlog:aot=off` is gated on the same JDK 25+ check because older JDKs
// reject the unknown tag with an `[error][logging] Invalid tag 'aot'`
// line on stdout, which itself corrupts the daemon handshake.
func appendLeydenAOTArgs(args []string, javaPath, jarPath string, verbose bool) ([]string, bool) {
	if cachedJDKMajorVersion() < 25 {
		return args, false
	}
	args = append(args, "-Xlog:aot=off")
	leydenConfig, configErr := aotConfigPath(jarPath)
	leydenCache, cacheErr := aotCachePath(jarPath)
	if configErr != nil || cacheErr != nil {
		return args, false
	}
	if info, statErr := os.Stat(leydenCache); statErr == nil && info.Size() > 0 {
		args = append(args, "-XX:AOTCache="+leydenCache)
		if verbose {
			reporter().Verbosef("verbose: Leyden AOT: using cache %s\n", leydenCache)
		}
		return args, true
	} else if statErr == nil {
		// Empty or unreadable cache from a previous crash — discard
		// so we don't poison the daemon launch with an invalid cache.
		_ = os.Remove(leydenCache)
		if verbose {
			reporter().Verbosef("verbose: Leyden AOT: discarded empty cache %s\n", leydenCache)
		}
	}
	if _, statErr := os.Stat(leydenConfig); statErr == nil {
		if err := buildLeydenAOTCache(javaPath, jarPath, leydenConfig, leydenCache, verbose); err == nil {
			args = append(args, "-XX:AOTCache="+leydenCache)
			if verbose {
				reporter().Verbosef("verbose: Leyden AOT: built and using cache %s\n", leydenCache)
			}
			return args, true
		} else if verbose {
			reporter().Verbosef("verbose: Leyden AOT: cache build failed (%v), starting without AOT\n", err)
		}
		return args, false
	}
	args = append(args, "-XX:AOTMode=record", "-XX:AOTConfiguration="+leydenConfig)
	if verbose {
		reporter().Verbosef("verbose: Leyden AOT: recording class profile → %s\n", leydenConfig)
	}
	return args, true
}

// appendStartupCacheArgs prefers Project Leyden AOT (JDK 25+) and falls
// back to AppCDS on older JDKs or when Leyden setup failed. The two are
// mutually exclusive on the JVM command line — combining them aborts
// VM init with `Option AOTConfiguration cannot be used at the same time
// with -Xshare:auto, ...`.
func appendStartupCacheArgs(args []string, javaPath, jarPath string, verbose bool) []string {
	args, addedAOT := appendLeydenAOTArgs(args, javaPath, jarPath, verbose)
	if addedAOT {
		return args
	}
	return appendAppCDSArgs(args, jarPath, verbose)
}

// jvmCacheSuffixes lists every per-jar artifact purgeJVMCachesForJar
// removes. Keep in sync with the suffix arguments passed to jarCachePath
// elsewhere in this file (cdsArchivePath, cracCheckpointPath,
// aotConfigPath, aotCachePath).
var jvmCacheSuffixes = []string{".jsa", ".crac", ".aotconf", ".aot"}

// purgeJVMCachesForJar deletes the AppCDS/CRaC/Leyden cache files keyed by
// `jarPath`'s content hash. Used as a self-heal after the daemon hands us
// JVM unified-log chatter instead of a JSON ready message — almost always
// because a `.jsa` archive recorded a classpath at training time that no
// longer matches the runtime jar location.
//
// Failures to remove are intentionally swallowed: if the file is already
// gone there is nothing to do, and if it's locked the retraining write
// will overwrite it anyway.
func purgeJVMCachesForJar(jarPath string, verbose bool) {
	// Hash the jar once and append each suffix, instead of calling four
	// separate path-derivation helpers that each re-hash the file.
	base, err := jarCachePath(jarPath, "")
	if err != nil {
		return
	}
	for _, suffix := range jvmCacheSuffixes {
		p := base + suffix
		if rmErr := os.Remove(p); rmErr == nil && verbose {
			reporter().Verbosef("verbose: purged stale JVM cache %s\n", p)
		}
	}
}

// openDaemonLogFile opens (or creates) the daemon log file and wires it to cmd.Stderr.
func openDaemonLogFile(cmd *exec.Cmd, verbose bool) *os.File {
	logFile, logPath, err := fsutil.CreateUserKritFile("krit-types-daemon.log")
	if err != nil {
		cmd.Stderr = os.Stderr
		return nil
	}
	cmd.Stderr = logFile
	if verbose {
		reporter().Verbosef("verbose: Daemon log: %s\n", logPath)
	}
	return logFile
}
