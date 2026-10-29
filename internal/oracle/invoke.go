package oracle

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/env"
	"github.com/kaeawc/krit/internal/fileignore"
)

// invokeTimeout returns the max wall-clock duration allowed for a krit-fir
// one-shot run, from env KRIT_ORACLE_TIMEOUT (default 15 minutes).
//
// The default leaves headroom for large monorepo cold populates. The env var
// override remains for users with even larger corpora.
func invokeTimeout() time.Duration {
	return invokeTimeoutFrom(env.Default)
}

// invokeTimeoutFrom is the testable form of invokeTimeout.
func invokeTimeoutFrom(r env.Reader) time.Duration {
	if v, ok := r.Lookup("KRIT_ORACLE_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 15 * time.Minute
}

// invokeGraceExit returns how long to wait for the krit-fir subprocess to
// exit on its own after it has written the output file, from env
// KRIT_ORACLE_GRACE_EXIT (default 15 seconds). After this grace period the
// subprocess is force-killed: a JVM that lingers after writing its output
// (for example on non-daemon background threads) has already produced a
// complete file, so it's safe to proceed.
func invokeGraceExit() time.Duration {
	return invokeGraceExitFrom(env.Default)
}

// invokeGraceExitFrom is the testable form of invokeGraceExit.
func invokeGraceExitFrom(r env.Reader) time.Duration {
	if v, ok := r.Lookup("KRIT_ORACLE_GRACE_EXIT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return 15 * time.Second
}

// FindJar locates the krit-fir shadow JAR without downloading; see
// FindBackendJar for the lookup order. Use EnsureJar instead when the caller
// should auto-download a missing jar for the current krit release.
func FindJar(scanPaths []string) string {
	return FindBackendJar(BackendFIR, scanPaths)
}

// nonJVMSourceSetFamilies are the Kotlin Multiplatform target families that
// never compile to JVM bytecode. A Gradle source set whose name (minus its
// Main/Test suffix) is one of these, or starts with one followed by an
// upper-case letter or digit (iosArm64, linuxX64, wasmJs, androidNativeArm64,
// jsAndWasmShared, ...), holds code for a non-JVM compilation. web (Kotlin's
// shared js + wasm set), posix (native-only), and nonJvm are shared sets whose
// actuals live only in dropped sets; keeping them would leave expects without
// a JVM actual.
var nonJVMSourceSetFamilies = []string{
	"js", "wasm", "native", "apple", "ios", "macos", "tvos", "watchos",
	"linux", "mingw", "androidNative", "web", "posix", "nonJvm",
}

// isJVMCompilableSourceSet reports whether a discovered kotlin/java root
// belongs in the JVM oracle compilation. The oracle compiles every
// root as one JVM module, so a KMP project's JS/Native/WASM actuals would
// otherwise collide with the JVM actuals and erase facts for every call that
// resolves through the shared expect declaration.
//
// This is a deny-list on purpose: only roots shaped src/<sourceSet>/kotlin|java
// whose source set belongs to a known non-JVM target family are dropped.
// Everything else stays, including main/test, android*, and custom
// intermediate sets (jvmAndroidMain, concurrentMain, nonJsMain, desktopMain,
// ...), which compile on the JVM and must keep their facts. Roots outside the
// src/<sourceSet>/ layout stay too: their target cannot be read from the path.
func isJVMCompilableSourceSet(dirPath string) bool {
	parent := filepath.Dir(dirPath)
	if filepath.Base(filepath.Dir(parent)) != "src" {
		return true
	}
	return !isNonJVMSourceSetName(filepath.Base(parent))
}

func isNonJVMSourceSetName(name string) bool {
	stem, ok := strings.CutSuffix(name, "Main")
	if !ok {
		stem = strings.TrimSuffix(name, "Test")
	}
	for _, family := range nonJVMSourceSetFamilies {
		rest, ok := strings.CutPrefix(stem, family)
		if !ok {
			continue
		}
		if rest == "" {
			return true
		}
		if c := rest[0]; (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return true
		}
	}
	return false
}

// FindSourceDirs discovers JVM-compilable Kotlin and Java source directories.
func FindSourceDirs(scanPaths []string) []string {
	var dirs []string
	seen := map[string]bool{}
	ignoreMatchers := make(map[string]*fileignore.Matcher)

	for _, root := range scanPaths {
		rootInfo, err := os.Stat(root)
		if err != nil {
			continue
		}
		matcher := fileignore.MatcherForPath(root, rootInfo, ignoreMatchers)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if !info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if fileignore.DefaultPrunedDir(base) || base == ".krit" || matcher.Ignored(path, true) {
				return filepath.SkipDir
			}
			if info.Name() == "kotlin" || info.Name() == "java" {
				if isJVMCompilableSourceSet(path) && !seen[path] {
					seen[path] = true
					dirs = append(dirs, path)
				}
				return filepath.SkipDir
			}
			return nil
		})
	}
	return dirs
}

// CachePath returns the path for the cached oracle JSON.
func CachePath(scanPaths []string) string {
	projectDir := FindRepoDir(scanPaths)
	if projectDir == "" {
		return ""
	}
	return filepath.Join(projectDir, ".krit", "types.json")
}

// stderrTailSize is the maximum number of stderr bytes retained to attach to
// error messages when the oracle subprocess fails or times out. Anything
// beyond this cap is discarded from the tail but still streamed to the
// user's terminal when verbose mode is active.
const stderrTailSize = 8 * 1024

// stderrTail is an io.Writer that keeps the last stderrTailSize bytes it
// sees. Safe for concurrent writes from a single goroutine (the exec.Cmd
// stderr reader).
type stderrTail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *stderrTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > stderrTailSize {
		t.buf = t.buf[len(t.buf)-stderrTailSize:]
	}
	return len(p), nil
}

func (t *stderrTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// firstLines returns up to n newline-separated lines from s, joined back with
// single newlines. Used to keep oracle error messages compact — the full tail
// is still available for verbose debugging.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// Invoke runs krit-fir.jar to produce a type oracle JSON file. It enforces
// a hard timeout (KRIT_ORACLE_TIMEOUT, default 15m) and a soft grace period
// (KRIT_ORACLE_GRACE_EXIT, default 15s) that starts once the output file
// appears on disk. The grace period exists because a JVM can linger after
// writing its output when background threads keep it alive — once the output
// is complete we can force-kill the subprocess without losing work.
//
// On failure, the returned error includes the first few lines of the
// subprocess's captured stderr (up to 8 KB tail) so the caller can surface a
// diagnostic instead of a bare exit code. On success the output path is
// returned even when the grace period fired — the caller is responsible for
// validating the JSON via oracle.Load / oracle.LoadFromData.
func Invoke(jarPath string, sourceDirs []string, outputPath string, verbose bool) (string, error) {
	return InvokeWithFiles(jarPath, sourceDirs, outputPath, "", verbose)
}

// InvokeWithFiles is identical to Invoke but additionally passes a
// --files LISTFILE flag to krit-fir if filesListPath is non-empty.
// LISTFILE is expected to contain one absolute .kt path per line and is
// produced by the rule-classification oracle filter
// (oracle.CollectOracleFiles + oracle.WriteFilterListFile). krit-fir
// still builds the FIR session from the full --sources tree so that
// cross-file resolution works; the flag only narrows which files
// contribute expressions/declarations to the output JSON.
func InvokeWithFiles(jarPath string, sourceDirs []string, outputPath, filesListPath string, verbose bool) (string, error) {
	return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filesListPath, verbose, InvocationOptions{})
}

// InvokeWithFilesWithOptions is InvokeWithFiles plus optional perf
// instrumentation and compile context.
func InvokeWithFilesWithOptions(jarPath string, sourceDirs []string, outputPath, filesListPath string, verbose bool, opts InvocationOptions) (string, error) {
	tracker := opts.tracker()
	// Check java is available
	var javaPath string
	if err := trackOracle(tracker, "javaLookup", func() error {
		var err error
		javaPath, err = exec.LookPath("java")
		if err != nil {
			return fmt.Errorf("java not found in PATH: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}

	// Ensure output directory exists
	if err := trackOracle(tracker, "outputDirCreate", func() error {
		return os.MkdirAll(filepath.Dir(outputPath), 0755)
	}); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Match the daemon launch's arg shape so the one-shot JVM gets
	// the same Leyden AOT / AppCDS startup-cache acceleration. On
	// the first call this writes the per-jar `.aotconf` / `.jsa`
	// under ~/.krit/cache/; subsequent one-shot calls reuse them.
	// G1 + StringDeduplication + Xms1g come from buildJVMBaseArgs:
	// G1 is the default GC on JDK 21+, dedup is a consistent
	// 10-15% win on FQN-heavy FIR state, and -Xms1g avoids early
	// heap-grow pauses without capping with -Xmx.
	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = append(args,
		"-jar", jarPath,
		"--sources", strings.Join(AbsolutePaths(sourceDirs), ","),
		"--output", outputPath,
	)
	extraJVMArgs := configuredExtraJVMArgs(opts)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgs)
	recordOracleJVMArgs(tracker, extraJVMArgs)
	if filesListPath != "" {
		args = append(args, "--files", filesListPath)
	}
	args = append(args, compileContextArgs(jarPath, opts)...)
	rider, riderArgs, err := startCheckRider(opts)
	if err != nil {
		return "", err
	}
	defer rider.cleanup()
	args = append(args, riderArgs...)
	rider.noteLaunch(args)
	if verbose {
		reporter().Verbosef("verbose: Running krit-fir: %s %s\n", javaPath, strings.Join(args, " "))
	}
	args, cleanupArgs, err := prepareJavaArgs(args)
	if err != nil {
		return "", err
	}
	defer cleanupArgs()

	timeout := invokeTimeout()
	graceExit := invokeGraceExit()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var res string
	var proc oracleProcessResult
	processErr := trackOracle(tracker, "oracleProcess", func() error {
		var err error
		proc, err = runOracleProcessMeasured(ctx, javaPath, args, outputPath, timeout, graceExit, verbose)
		res = proc.OutputPath
		return err
	})
	addOracleProcessResources(tracker, "oracleProcessResources", proc.PeakRSSMB)
	rider.finish(tracker, processErr)
	return res, processErr
}

// compileContextArgs returns the one-shot arguments that set the compile
// context: the configured classpath, the JVM target and the project's
// Kotlin compiler settings.
func compileContextArgs(jarPath string, opts InvocationOptions) []string {
	var args []string
	if len(opts.Classpath) > 0 {
		// krit-fir parses `--classpath <joined>` as the user-configured
		// classpath. Joined with the OS path separator so the parser
		// (which splits on `:` / `;`) stays consistent with the env var
		// convention.
		args = append(args, "--classpath", strings.Join(opts.Classpath, string(os.PathListSeparator)))
	}
	if opts.JvmTarget != "" {
		args = append(args, "--jvm-target", opts.JvmTarget)
	}
	// Each setting is its own flag so values never need quoting; krit-fir
	// re-filters them through its own allowlist.
	args = append(args, KotlinLaunchArgs(jarPath, opts.Kotlin)...)
	return args
}

// runOracleProcess is the exec+wait+grace-period+stderr-capture core shared by
// Invoke and the test harness. It's separated from Invoke to enable driving
// the full failure-mode matrix (clean exit, non-zero exit, hard timeout,
// grace-period force-kill) with cheap subprocess fixtures like `sh -c`
// instead of a real krit-fir.jar. Drops verbose=false in to keep the
// test harness terse — production paths call runOracleProcessMeasured
// directly when they need to plumb verbose through.
func runOracleProcess(
	ctx context.Context,
	binaryPath string,
	args []string,
	outputPath string,
	timeout time.Duration,
	graceExit time.Duration,
) (string, error) {
	res, err := runOracleProcessMeasured(ctx, binaryPath, args, outputPath, timeout, graceExit, false)
	return res.OutputPath, err
}

func runOracleProcessMeasured(
	ctx context.Context,
	binaryPath string,
	args []string,
	outputPath string,
	timeout time.Duration,
	graceExit time.Duration,
	verbose bool,
) (oracleProcessResult, error) {
	tail := &stderrTail{}
	var stderrWriter io.Writer = tail
	if verbose {
		stderrWriter = io.MultiWriter(tail, os.Stderr)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = stderrWriter

	// A one-shot run writes to a stable path (<repo>/.krit/types.json), so
	// a previous run's output is usually already there. Only a file that
	// differs from this snapshot counts as this run's output; otherwise the
	// grace timer would start immediately and kill a JVM that is still
	// analyzing, returning the previous run's facts.
	preStart, _ := os.Stat(outputPath)

	if err := cmd.Start(); err != nil {
		return oracleProcessResult{}, fmt.Errorf("krit-fir start: %w", err)
	}

	done := make(chan oracleProcessWaitResult, 1)
	go func() {
		err := cmd.Wait()
		done <- oracleProcessWaitResult{
			Err:       err,
			PeakRSSMB: processStatePeakRSSMB(cmd.ProcessState),
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var outputSeenAt time.Time
	result := oracleProcessResult{OutputPath: outputPath}
	for {
		select {
		case wait := <-done:
			result.PeakRSSMB = wait.PeakRSSMB
			if ctx.Err() == context.DeadlineExceeded {
				return result, fmt.Errorf("krit-fir timed out after %s\nstderr tail:\n%s",
					timeout, firstLines(tail.String(), 10))
			}
			if wait.Err != nil {
				return result, fmt.Errorf("krit-fir failed: %w\nstderr tail:\n%s",
					wait.Err, firstLines(tail.String(), 10))
			}
			return result, nil
		case <-ticker.C:
			if !outputSeenAt.IsZero() {
				if time.Since(outputSeenAt) >= graceExit {
					if verbose {
						reporter().Verbosef("verbose: krit-fir wrote output but subprocess did not exit within %s grace period; force-killing\n", graceExit)
					}
					_ = cmd.Process.Kill()
					wait := <-done // drain Wait() so exec doesn't leak the pipe
					result.PeakRSSMB = wait.PeakRSSMB
					return result, nil
				}
				continue
			}
			if fi, err := os.Stat(outputPath); err == nil && fi.Size() > 0 && !sameOracleFile(preStart, fi) {
				outputSeenAt = time.Now()
				if verbose {
					reporter().Verbosef("verbose: krit-fir wrote %d bytes to %s; waiting up to %s for clean exit\n", fi.Size(), outputPath, graceExit)
				}
			}
		}
	}
}

// InvokeDaemon finds the JAR and source directories, then starts a long-lived
// krit-fir daemon process. The caller is responsible for calling Close() on
// the returned Daemon. Missing jars are auto-downloaded for tagged releases;
// see EnsureJar.
func InvokeDaemon(scanPaths []string, verbose bool, opts ...DaemonOption) (*Daemon, error) {
	return InvokeDaemonWithClasspath(scanPaths, nil, verbose, opts...)
}

// InvokeDaemonWithClasspath is InvokeDaemon with the effective oracle
// classpath, so library calls resolve as they do on the one-shot
// InvokeCachedWithOptions path.
func InvokeDaemonWithClasspath(scanPaths []string, classpath []string, verbose bool, opts ...DaemonOption) (*Daemon, error) {
	jarPath, err := EnsureJar(context.Background(), scanPaths, verbose)
	if err != nil {
		return nil, err
	}

	var cfg daemonConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	sourceDirs := FilterFIRSourceDirs(UnionSourceDirs(FindSourceDirs(scanPaths), cfg.extraSourceDirs))
	if len(sourceDirs) == 0 {
		return nil, fmt.Errorf("no Kotlin source directories found")
	}

	if verbose {
		reporter().Verbosef("verbose: Found %d source directories for daemon\n", len(sourceDirs))
	}

	return StartDaemon(jarPath, sourceDirs, classpath, verbose, append([]DaemonOption{WithBackend(BackendFIR)}, opts...)...)
}
