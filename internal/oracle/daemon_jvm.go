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
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/jvmaot"
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
		if err := os.MkdirAll(candidate, 0755); err == nil && cacheDirWritable(candidate) {
			cacheDir = candidate
		}
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "krit-types-"+hash+suffix), nil
}

// A pre-existing home cache may be readable but not writable (for example in
// a sandbox). ArchiveClassesAtExit otherwise makes an otherwise successful
// JVM process exit nonzero after it has already written its oracle output.
func cacheDirWritable(dir string) bool {
	probe, err := os.CreateTemp(dir, ".krit-write-")
	if err != nil {
		return false
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	return true
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

// CachedJDKMajorVersion exposes the oracle's java -version probe to other JVM
// launchers so they use the same Leyden support detection.
func CachedJDKMajorVersion() int { return cachedJDKMajorVersion() }

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

// JavaPath selects the launcher used by krit-fir. JAVA_HOME takes precedence
// when it is set, so preflight and the actual subprocess use the same JVM.
func JavaPath() (string, error) {
	if home := os.Getenv("JAVA_HOME"); home != "" {
		path := filepath.Join(home, "bin", "java")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("JAVA_HOME does not contain bin/java: %s", home)
	}
	return exec.LookPath("java")
}

// JavaMajorVersion reads the launcher version without the process-wide cache:
// tests and long-lived servers may change JAVA_HOME between scans.
func JavaMajorVersion(path string) int { return jdkMajorVersion(path) }

type preflightJavaKey struct {
	path  string
	size  int64
	mtime time.Time
}

var preflightJavaVersions sync.Map

// PreflightJavaMajorVersion caches only preflight probes. Other callers of
// JavaMajorVersion continue to get a fresh result.
func PreflightJavaMajorVersion(path string) int {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return JavaMajorVersion(path)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return JavaMajorVersion(path)
	}
	key := preflightJavaKey{resolved, info.Size(), info.ModTime()}
	if major, ok := preflightJavaVersions.Load(key); ok {
		return major.(int)
	}
	major := JavaMajorVersion(path)
	preflightJavaVersions.Store(key, major)
	return major
}

// leydenAOTSkipPath returns the sentinel-file path that records a
// past `buildLeydenAOTCache` failure for `jarPath`. Lives alongside
// the `.aotconf` / `.aot` files keyed on the jar's content hash.
// The sentinel is auto-invalidated when the JDK version changes
// (encoded in the file body) so a JDK upgrade reattempts the build.
func leydenAOTSkipPath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".aot.skip")
}

// leydenAOTCreateSkipped reports whether a prior buildLeydenAOTCache
// attempt failed against the current JDK + jar pair and we should
// short-circuit straight to the AppCDS fallback instead of paying
// the ~30s failing-create cost again. Returns false on any I/O
// or parse error so a corrupt sentinel doesn't lock us out of
// retrying.
func leydenAOTCreateSkipped(skipPath string, jdkVersion int) bool {
	data, err := os.ReadFile(skipPath)
	if err != nil {
		return false
	}
	recordedVersion, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	return recordedVersion == jdkVersion
}

// markLeydenAOTCreateFailed writes the sentinel so subsequent
// invocations short-circuit. Failures are intentionally swallowed
// — the worst case is we pay one more failing-create cycle next
// run, which is the pre-sentinel behavior anyway.
func markLeydenAOTCreateFailed(skipPath string, jdkVersion int, verbose bool) {
	if err := os.WriteFile(skipPath, []byte(strconv.Itoa(jdkVersion)), 0o644); err != nil && verbose {
		reporter().Verbosef("verbose: Leyden AOT: failed to write skip sentinel %s: %v\n", skipPath, err)
	}
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
	token, err := hashutil.HashFile(jarPath)
	if err != nil {
		token = ""
	}
	return jvmaot.AppendArgsWithToken(args, javaPath, jarPath, token, "", cachedJDKMajorVersion(), verbose, reporter().Verbosef)
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
var jvmCacheSuffixes = []string{".jsa", ".crac", ".aotconf", ".aot", ".aot.skip"}

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
	// Hash the jar once and append every established oracle cache suffix.
	base, err := jarCachePath(jarPath, "")
	if err != nil {
		return
	}
	for _, suffix := range jvmCacheSuffixes {
		removeJVMCachesForPath(base+suffix, verbose)
	}
}

func removeJVMCachesForPath(p string, verbose bool) {
	if rmErr := os.Remove(p); rmErr == nil && verbose {
		reporter().Verbosef("verbose: purged stale JVM cache %s\n", p)
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
